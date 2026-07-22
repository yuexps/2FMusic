package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/core"
	"2fmusic/backend/db"
)

var (
	inFlightScrape   = make(map[string]bool)
	inFlightScrapeMu sync.Mutex
)

// InitDBWatcher 初始化数据库监听器 (DB_Watcher)
func InitDBWatcher() {
	go dbWatchLoop()
	go processPendingMediaExtraction()
}

// dbWatchLoop 监听数据库主键增删 Channel
func dbWatchLoop() {
	for {
		select {
		case song, ok := <-db.SongInsertedChan:
			if !ok {
				return
			}
			if song != nil && song.ID != "" {
				go handleSongInserted(song)
			}
		case songID, ok := <-db.SongDeletedChan:
			if !ok {
				return
			}
			if songID != "" {
				go handleSongDeleted(songID)
			}
		}
	}
}

// processPendingMediaExtraction 启动时扫描未完成封面/歌词提取的历史数据兜底
func processPendingMediaExtraction() {
	time.Sleep(3 * time.Second) // 充裕缓冲等待系统全面就绪
	songs, err := db.GetAllSongs()
	if err != nil {
		return
	}

	for _, s := range songs {
		if !s.HasCover || !s.HasLyrics {
			songRef := s
			handleSongInserted(&songRef)
		}
	}
}

// handleSongInserted 处理新条目：提取/刮削封面与歌词落盘至 covers/ & lyrics/
func handleSongInserted(song *core.Song) {
	inFlightScrapeMu.Lock()
	if inFlightScrape[song.ID] {
		inFlightScrapeMu.Unlock()
		return
	}
	inFlightScrape[song.ID] = true
	inFlightScrapeMu.Unlock()

	defer func() {
		inFlightScrapeMu.Lock()
		delete(inFlightScrape, song.ID)
		inFlightScrapeMu.Unlock()
	}()

	coverPath := filepath.Join(core.GlobalConfig.CoversDir, song.ID+".webp")
	lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, song.ID+".lrc")

	hasCoverFile := fileExists(coverPath)
	hasLyricsFile := fileExists(lrcPath)

	if hasCoverFile && hasLyricsFile {
		return
	}

	// 1. 尝试从物理文件提取 Tag 内嵌数据
	_, picData, embeddedLyrics, _ := ExtractAudioMetadata(song.Path)

	// 2. 处理封面
	if !hasCoverFile {
		if len(picData) > 0 {
			// 有内嵌封面 -> 压缩 WebP 保存
			_ = SaveCoverWebP(picData, song.ID)
		} else if !IsNeteaseDownloadFile(song.Path) {
			// 网易云下载目录以外的音源，发起在线网络刮削
			if best := SearchSongBest(song.Title, song.Artist, song.Album); best != nil {
				if coverURL, ok := best["cover"].(string); ok && coverURL != "" {
					if imgData, err := DownloadImageBytes(coverURL); err == nil && len(imgData) > 0 {
						_ = SaveCoverWebP(imgData, song.ID)
					}
				}
			}
		}
	}

	// 3. 处理歌词
	if !hasLyricsFile {
		lyricsPref := strings.ToLower(core.GlobalConfig.LyricsPreference)
		isNeteaseDir := IsNeteaseDownloadFile(song.Path)

		if embeddedLyrics != "" && (isNeteaseDir || lyricsPref != "network") {
			_ = saveLyricsFile(lrcPath, []byte(embeddedLyrics))
		} else if !isNeteaseDir {
			// 发起在线歌词刮削
			if best := SearchSongBest(song.Title, song.Artist, song.Album); best != nil {
				if lyrics, ok := best["lyrics"].(string); ok && lyrics != "" {
					_ = saveLyricsFile(lrcPath, []byte(lyrics))
				} else if embeddedLyrics != "" {
					// 网络无歌词时退避使用内嵌歌词
					_ = saveLyricsFile(lrcPath, []byte(embeddedLyrics))
				}
			}
		}
	}

	// 兜底：处理前后均无封面/歌词落盘时，LC_Watcher 不会被唤醒，显式广播通知
	if !hasCoverFile && !hasLyricsFile && !fileExists(coverPath) && !fileExists(lrcPath) {
		if NotifyLibraryChanged != nil {
			NotifyLibraryChanged()
		}
	}
}

// handleSongDeleted 处理主键删除：物理联动清理磁盘上的 covers/ 和 lyrics/ 缓存文件
func handleSongDeleted(songID string) {
	coverPath := filepath.Join(core.GlobalConfig.CoversDir, songID+".webp")
	lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, songID+".lrc")

	hasCoverFile := fileExists(coverPath)
	hasLyricsFile := fileExists(lrcPath)

	if hasCoverFile {
		_ = os.Remove(coverPath)
		core.Info("[DB_Watcher] 清理封面缓存: %s.webp", songID)
	}
	if hasLyricsFile {
		_ = os.Remove(lrcPath)
		core.Info("[DB_Watcher] 清理歌词缓存: %s.lrc", songID)
	}

	// 兜底：被删曲目若此前无任何物理缓存，LC_Watcher 不会被唤醒，显式广播通知
	if !hasCoverFile && !hasLyricsFile {
		if NotifyLibraryChanged != nil {
			NotifyLibraryChanged()
		}
	}
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
