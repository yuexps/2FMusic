package utils

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"2fmusic/backend/core"

	"github.com/bogem/id3v2"
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

	// MP3 格式退避解析：当 dhowden/tag 解析报错或封面/歌词缺失时，使用 id3v2 提取
	if strings.ToLower(filepath.Ext(filePath)) == ".mp3" && (len(pictureBytes) == 0 || embeddedLyrics == "" || artist == "未知歌手") {
		if id3Tag, id3Err := id3v2.Open(filePath, id3v2.Options{Parse: true}); id3Err == nil && id3Tag != nil {
			defer id3Tag.Close()
			if title == "" || title == strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)) {
				if t := strings.TrimSpace(id3Tag.Title()); t != "" {
					title = t
				}
			}
			if artist == "未知歌手" || artist == "" {
				if a := strings.TrimSpace(id3Tag.Artist()); a != "" {
					artist = a
				}
			}
			if album == "未知专辑" || album == "" {
				if al := strings.TrimSpace(id3Tag.Album()); al != "" {
					album = al
				}
			}
			if len(pictureBytes) == 0 {
				if apicFrames := id3Tag.GetFrames("APIC"); len(apicFrames) > 0 {
					if apic, ok := apicFrames[0].(id3v2.PictureFrame); ok && len(apic.Picture) > 0 {
						pictureBytes = apic.Picture
					}
				}
			}
			if embeddedLyrics == "" {
				if usltFrames := id3Tag.GetFrames("USLT"); len(usltFrames) > 0 {
					if uslt, ok := usltFrames[0].(id3v2.UnsynchronisedLyricsFrame); ok && strings.TrimSpace(uslt.Lyrics) != "" {
						embeddedLyrics = strings.TrimSpace(uslt.Lyrics)
					}
				}
			}
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
