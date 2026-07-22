package utils

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"2fmusic/backend/core"

	"github.com/dhowden/tag"
)

// CalculateMD5 计算物理文件二进制 MD5 哈希
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

// ExtractAudioMetadata 提取音频物理文件的 ID3/FLAC Tag 元数据
func ExtractAudioMetadata(filePath string) (*core.Song, []byte, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, "", err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, "", err
	}

	songID, _ := CalculateMD5(filePath)
	title := ""
	artist := ""
	album := ""
	albumArtist := ""
	var pictureBytes []byte
	embeddedLyrics := ""

	m, err := tag.ReadFrom(f)
	if err == nil && m != nil {
		title = strings.TrimSpace(m.Title())
		artist = strings.TrimSpace(m.Artist())
		album = strings.TrimSpace(m.Album())
		albumArtist = strings.TrimSpace(m.AlbumArtist())
		if pic := m.Picture(); pic != nil && len(pic.Data) > 0 {
			pictureBytes = pic.Data
		}

		if lyrics := m.Lyrics(); lyrics != "" {
			embeddedLyrics = strings.TrimSpace(lyrics)
		}
	}

	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	if artist == "" {
		artist = "未知歌手"
	}
	if album == "" {
		album = "未知专辑"
	}

	mtime := float64(fi.ModTime().UnixNano()) / 1e9
	size := fi.Size()

	song := &core.Song{
		ID:          songID,
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
