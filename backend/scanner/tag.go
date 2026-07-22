package scanner

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"2fmusic/backend/config"
	"2fmusic/backend/model"

	"github.com/dhowden/tag"
)

// CalculateMD5 计算文件二进制 MD5
func CalculateMD5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ExtractAudioMetadata 提取音频文件 Tag 元数据与嵌入图片/歌词
func ExtractAudioMetadata(filePath string) (*model.Song, []byte, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, "", err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, "", err
	}

	m, err := tag.ReadFrom(f)
	title := ""
	artist := ""
	album := ""
	albumArtist := ""
	var pictureBytes []byte
	embeddedLyrics := ""

	if err == nil && m != nil {
		title = strings.TrimSpace(m.Title())
		artist = strings.TrimSpace(m.Artist())
		album = strings.TrimSpace(m.Album())
		albumArtist = strings.TrimSpace(m.AlbumArtist())
		if pic := m.Picture(); pic != nil {
			pictureBytes = pic.Data
		}
	}

	filename := filepath.Base(filePath)
	if title == "" {
		ext := filepath.Ext(filename)
		title = strings.TrimSuffix(filename, ext)
	}

	mtime := float64(fi.ModTime().UnixNano()) / 1e9
	size := fi.Size()

	song := &model.Song{
		Filename:    filename,
		Title:       title,
		Artist:      artist,
		Album:       album,
		AlbumArtist: albumArtist,
		MTime:       mtime,
		Size:        size,
		Path:        filePath,
	}

	return song, pictureBytes, embeddedLyrics, nil
}

// SaveCoverWebP 纯 Go 极速保存封面流至 covers/ 缓存目录
func SaveCoverWebP(imgData []byte, songID string) bool {
	if len(imgData) == 0 {
		return false
	}

	targetPath := filepath.Join(config.GlobalConfig.CoversDir, songID+".webp")
	err := os.WriteFile(targetPath, imgData, 0644)
	return err == nil
}

func GetNowUnix() int64 {
	return time.Now().Unix()
}
