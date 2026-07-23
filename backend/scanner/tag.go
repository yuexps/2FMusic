package scanner

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	golangdraw "golang.org/x/image/draw"

	"2fmusic/backend/core"
	"2fmusic/backend/db"
	"2fmusic/backend/utils"

	"github.com/gen2brain/webp"
)

// CalculateMD5 计算文件二进制 MD5
func CalculateMD5(filePath string) (string, error) {
	return utils.CalculateMD5(filePath)
}

// ExtractAudioMetadata 提取音频文件 Tag 元数据
func ExtractAudioMetadata(filePath string) (*core.Song, []byte, string, error) {
	return utils.ExtractAudioMetadata(filePath)
}

// SaveCoverWebP 保存 WebP 格式封面
func SaveCoverWebP(imgData []byte, songID string) bool {
	if len(imgData) == 0 {
		return false
	}

	targetPath := filepath.Join(core.GlobalConfig.CoversDir, songID+".webp")
	cacheDir := core.GlobalConfig.CacheDir
	if cacheDir == "" {
		cacheDir = core.GlobalConfig.CoversDir
	}
	tmpPath := filepath.Join(cacheDir, fmt.Sprintf("cover_%s_%d.tmp", songID, time.Now().UnixNano()))

	writeToTmpAndMove := func(b []byte) bool {
		if err := os.WriteFile(tmpPath, b, 0644); err != nil {
			return false
		}
		if err := os.Rename(tmpPath, targetPath); err != nil {
			_ = os.Remove(tmpPath)
			return false
		}
		return true
	}

	srcImg, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil || srcImg == nil {
		return writeToTmpAndMove(imgData)
	}

	bounds := srcImg.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	maxDim := 500

	targetImg := srcImg
	if w > maxDim || h > maxDim {
		var newW, newH int
		if w >= h {
			newW = maxDim
			newH = int(math.Round(float64(h) * float64(maxDim) / float64(w)))
		} else {
			newH = maxDim
			newW = int(math.Round(float64(w) * float64(maxDim) / float64(h)))
		}
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}

		dstImg := image.NewRGBA(image.Rect(0, 0, newW, newH))
		golangdraw.CatmullRom.Scale(dstImg, dstImg.Bounds(), srcImg, bounds, draw.Over, nil)
		targetImg = dstImg
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, targetImg, webp.Options{Quality: 80}); err != nil {
		return writeToTmpAndMove(imgData)
	}

	return writeToTmpAndMove(buf.Bytes())
}

func GetNowUnix() int64 {
	return time.Now().Unix()
}

// EnsureSongMediaResolved 集中解析与保底刮削主键已入库曲目的 Cache 资源（封面与歌词）
// 前置契约：song 必须已完成 db.SaveSong 且包含非空 song.ID
func EnsureSongMediaResolved(song *core.Song) bool {
	if song == nil || song.ID == "" {
		return false
	}

	updated := false

	// 1. 检查物理 Cache 文件全状态
	coverPath := filepath.Join(core.GlobalConfig.CoversDir, song.ID+".webp")
	lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, song.ID+".lrc")
	yrcPath := filepath.Join(core.GlobalConfig.LyricsDir, song.ID+".yrc")

	hasCoverFile := fileExists(coverPath)
	hasLyricsFile := fileExists(lrcPath) || fileExists(yrcPath)

	_, picData, embeddedLyrics, _ := ExtractAudioMetadata(song.Path)

	// 本地内嵌封面解析落盘
	if !hasCoverFile && len(picData) > 0 {
		if SaveCoverWebP(picData, song.ID) {
			hasCoverFile = true
			song.HasCover = true
			updated = true
		}
	}

	// 同级物理图片 (.webp) 解析落盘
	if !hasCoverFile {
		dir := filepath.Dir(song.Path)
		baseName := strings.TrimSuffix(filepath.Base(song.Path), filepath.Ext(song.Path))
		sameImg := filepath.Join(dir, baseName+".webp")
		if b, err := os.ReadFile(sameImg); err == nil {
			if SaveCoverWebP(b, song.ID) {
				hasCoverFile = true
				song.HasCover = true
				updated = true
			}
		}
	}

	lyricsPref := strings.ToLower(core.GlobalConfig.LyricsPreference)
	isNeteaseDir := IsNeteaseDownloadFile(song.Path)

	// 内嵌优先提取内嵌歌词
	if !hasLyricsFile && embeddedLyrics != "" && (isNeteaseDir || lyricsPref != "network") {
		if saveLyricsFile(lrcPath, []byte(embeddedLyrics)) {
			hasLyricsFile = true
			song.HasLyrics = true
			updated = true
		}
	}

	// 2. 缺失资源在线网络保底下钻
	needScrapeCover := !hasCoverFile
	needScrapeLyrics := !hasLyricsFile

	if (needScrapeCover || needScrapeLyrics) && song.ScrapeRetryCount < 3 {
		best := SearchSongFastSequential(song.Title, song.Artist, song.Album, song.DurationMs)
		if best != nil {
			if needScrapeCover {
				if coverURL, ok := best["cover"].(string); ok && coverURL != "" {
					if imgData, err := DownloadImageBytes(coverURL); err == nil && len(imgData) > 0 {
						if SaveCoverWebP(imgData, song.ID) {
							hasCoverFile = true
							song.HasCover = true
							updated = true
						}
					}
				}
			}

			if needScrapeLyrics {
				if lyrics, ok := best["lyrics"].(string); ok && lyrics != "" {
					if saveLyricsFile(lrcPath, []byte(lyrics)) {
						hasLyricsFile = true
						song.HasLyrics = true
						updated = true
					}
				} else if embeddedLyrics != "" {
					// 优先网络但网络无歌词时退避解出内嵌歌词
					if saveLyricsFile(lrcPath, []byte(embeddedLyrics)) {
						hasLyricsFile = true
						song.HasLyrics = true
						updated = true
					}
				}
			}
		}
	}

	// 3. 持久化数据库状态与广播通知
	if updated {
		db.UpdateSongMediaStatus(song.ID, song.HasCover, song.HasLyrics)
		if NotifySongChangedDebounced != nil {
			NotifySongChangedDebounced(song.ID, "update", []string{"has_cover", "has_lyrics"})
		}
		if NotifyLibraryChanged != nil {
			NotifyLibraryChanged()
		}
	}

	return updated
}
