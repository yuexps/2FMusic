package utils

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"2fmusic/backend/core"

	"github.com/bogem/id3v2"
	"github.com/dhowden/tag"
	"github.com/lizc2003/audioduration"
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

// ExtractAudioMetadata 提取音频文件的 ID3/FLAC Tag 元数据
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
	durationMs := 0

	ext := strings.ToLower(filepath.Ext(filePath))

	// 1. FLAC 原生解析
	if ext == ".flac" {
		if flacMeta, err := ExtractFlacNative(filePath); err == nil && flacMeta != nil {
			if flacMeta.Title != "" {
				title = flacMeta.Title
			}
			if flacMeta.Artist != "" {
				artist = flacMeta.Artist
			}
			if flacMeta.Album != "" {
				album = flacMeta.Album
			}
			if flacMeta.AlbumArtist != "" {
				albumArtist = flacMeta.AlbumArtist
			}
			if flacMeta.DurationMs > 0 {
				durationMs = flacMeta.DurationMs
			}
			if len(flacMeta.PictureBytes) > 0 {
				pictureBytes = flacMeta.PictureBytes
			}
			if flacMeta.Lyrics != "" {
				embeddedLyrics = flacMeta.Lyrics
			}
		}
	}

	// 2. 通用 Tag 库解析
	m, err := tag.ReadFrom(f)
	if err == nil && m != nil {
		if title == "" {
			title = strings.TrimSpace(m.Title())
		}
		if artist == "" {
			artist = strings.TrimSpace(m.Artist())
		}
		if album == "" {
			album = strings.TrimSpace(m.Album())
		}
		if albumArtist == "" {
			albumArtist = strings.TrimSpace(m.AlbumArtist())
		}
		if len(pictureBytes) == 0 {
			if pic := m.Picture(); pic != nil && len(pic.Data) > 0 {
				pictureBytes = pic.Data
			}
		}
		if embeddedLyrics == "" {
			if lyrics := m.Lyrics(); lyrics != "" {
				embeddedLyrics = strings.TrimSpace(lyrics)
			}
		}
	}

	// 3. MP3 退避至 id3v2 解析
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
			if albumArtist == "" {
				if tpe2Frames := id3Tag.GetFrames("TPE2"); len(tpe2Frames) > 0 {
					if tf, ok := tpe2Frames[0].(id3v2.TextFrame); ok && strings.TrimSpace(tf.Text) != "" {
						albumArtist = strings.TrimSpace(tf.Text)
					}
				}
			}
			if tlenFrames := id3Tag.GetFrames("TLEN"); len(tlenFrames) > 0 {
				if tf, ok := tlenFrames[0].(id3v2.TextFrame); ok {
					var dur int
					for _, c := range tf.Text {
						if c >= '0' && c <= '9' {
							dur = dur*10 + int(c-'0')
						}
					}
					if dur > 0 {
						durationMs = dur
					}
				}
			}
		}
	}

	// 4. 默认值兜底
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	if artist == "" {
		artist = "未知歌手"
	}
	if album == "" {
		album = "未知专辑"
	}
	if albumArtist == "" {
		albumArtist = artist
	}

	// 5. 帧解码计算时长
	if durationMs <= 0 {
		durationMs = GetAudioDurationMs(filePath)
	}

	mtime := float64(fi.ModTime().UnixNano()) / 1e9
	size := fi.Size()

	song := &core.Song{
		ID:          songID,
		Filename:    filepath.Base(filePath),
		Title:       title,
		Artist:      artist,
		Album:       album,
		AlbumArtist: albumArtist,
		DurationMs:  durationMs,
		MTime:       mtime,
		Size:        size,
		Path:        filePath,
	}

	return song, pictureBytes, embeddedLyrics, nil
}

// GetAudioDurationMs 提取各类音频格式 (FLAC/MP3/WAV/OGG/M4A/AAC) 的播放时长(毫秒)
func GetAudioDurationMs(filePath string) int {
	ext := strings.ToLower(filepath.Ext(filePath))
	f, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer f.Close()

	var durSec float64
	var errDur error
	switch ext {
	case ".mp3":
		durSec, errDur = audioduration.Mp3(f)
	case ".flac":
		durSec, errDur = audioduration.FLAC(f)
	case ".wav":
		durSec, errDur = audioduration.Wav(f)
	case ".m4a", ".mp4":
		durSec, errDur = audioduration.Mp4(f)
	case ".ogg":
		durSec, errDur = audioduration.Ogg(f)
	case ".aac":
		durSec, errDur = audioduration.AAC(f)
	}

	if errDur == nil && durSec > 0 {
		return int(durSec * 1000)
	}
	return 0
}

// FlacMeta 存储 FLAC 元数据。
type FlacMeta struct {
	Title        string
	Artist       string
	Album        string
	AlbumArtist  string
	Lyrics       string
	PictureBytes []byte
	DurationMs   int
	SampleRate   int
	TotalSamples int64
}

// ExtractFlacNative 解析 FLAC METADATA_BLOCK。
func ExtractFlacNative(filePath string) (*FlacMeta, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil || string(header) != "fLaC" {
		return nil, fmt.Errorf("not a flac file")
	}

	meta := &FlacMeta{}

	for {
		blockHead := make([]byte, 4)
		if _, err := io.ReadFull(f, blockHead); err != nil {
			break
		}

		isLast := (blockHead[0] & 0x80) != 0
		blockType := blockHead[0] & 0x7F
		length := int(blockHead[1])<<16 | int(blockHead[2])<<8 | int(blockHead[3])

		data := make([]byte, length)
		if _, err := io.ReadFull(f, data); err != nil {
			break
		}

		switch blockType {
		case 0: // STREAMINFO
			if length >= 34 {
				meta.SampleRate = int((uint32(data[10]) << 12) | (uint32(data[11]) << 4) | (uint32(data[12]) >> 4))
				meta.TotalSamples = int64((uint64(data[13]&0x0F) << 32) | (uint64(data[14]) << 24) | (uint64(data[15]) << 16) | (uint64(data[16]) << 8) | uint64(data[17]))
				if meta.SampleRate > 0 && meta.TotalSamples > 0 {
					meta.DurationMs = int(float64(meta.TotalSamples) / float64(meta.SampleRate) * 1000)
				}
			}
		case 4: // VORBIS_COMMENT
			parseVorbisComment(data, meta)
		case 6: // PICTURE
			parseFlacPicture(data, meta)
		}

		if isLast {
			break
		}
	}

	return meta, nil
}

// parseVorbisComment 解析 VORBIS_COMMENT 标签。
func parseVorbisComment(data []byte, meta *FlacMeta) {
	if len(data) < 8 {
		return
	}
	offset := 0
	vendorLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4 + vendorLen
	if offset+4 > len(data) {
		return
	}

	commentCount := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4

	for i := 0; i < commentCount; i++ {
		if offset+4 > len(data) {
			break
		}
		cLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if offset+cLen > len(data) {
			break
		}
		commentStr := string(data[offset : offset+cLen])
		offset += cLen

		kv := strings.SplitN(commentStr, "=", 2)
		if len(kv) == 2 {
			key := strings.ToUpper(strings.TrimSpace(kv[0]))
			val := strings.TrimSpace(kv[1])

			switch key {
			case "TITLE":
				if meta.Title == "" {
					meta.Title = val
				}
			case "ARTIST":
				if meta.Artist == "" {
					meta.Artist = val
				}
			case "ALBUM":
				if meta.Album == "" {
					meta.Album = val
				}
			case "ALBUMARTIST", "ALBUM ARTIST", "ALBUM_ARTIST", "ENSEMBLE", "PERFORMER":
				if meta.AlbumArtist == "" {
					meta.AlbumArtist = val
				}
			case "LYRICS", "UNSYNCEDLYRICS", "LYRIC":
				if meta.Lyrics == "" {
					meta.Lyrics = val
				}
			}
		}
	}
}

// parseFlacPicture 解析 PICTURE 封面数据。
func parseFlacPicture(data []byte, meta *FlacMeta) {
	if len(data) < 32 {
		return
	}
	offset := 4
	mimeLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4 + mimeLen

	descLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4 + descLen

	offset += 16

	if offset+4 > len(data) {
		return
	}
	imgLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4

	if offset+imgLen <= len(data) {
		meta.PictureBytes = data[offset : offset+imgLen]
	}
}
