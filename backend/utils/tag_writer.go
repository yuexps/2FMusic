package utils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"path/filepath"
	"strings"

	"2fmusic/backend/core"

	"github.com/bogem/id3v2"
	flac "github.com/go-flac/go-flac"
	flacpicture "github.com/go-flac/flacpicture"
)

// EmbedAudioMetadata 将元数据（标题、歌手、专辑、专辑艺术家、封面图片及 YRC/LRC 歌词）内嵌写入物理音频文件
func EmbedAudioMetadata(filePath, title, artist, album, albumArtist string, coverBytes []byte, lyrics string) error {
	if albumArtist == "" {
		albumArtist = artist
	}
	cleanPath := strings.TrimSuffix(filePath, ".part")
	ext := strings.ToLower(filepath.Ext(cleanPath))
	switch ext {
	case ".mp3":
		return embedMP3Metadata(filePath, title, artist, album, albumArtist, coverBytes, lyrics)
	case ".flac":
		return embedFLACMetadata(filePath, title, artist, album, albumArtist, coverBytes, lyrics)
	default:
		return fmt.Errorf("不支持内嵌元数据的音频格式: %s", ext)
	}
}

// embedMP3Metadata 给 MP3 写入 ID3v2 标签
func embedMP3Metadata(filePath, title, artist, album, albumArtist string, coverBytes []byte, lyrics string) error {
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		tag = id3v2.NewEmptyTag()
	}
	defer tag.Close()

	if title != "" {
		tag.SetTitle(title)
	}
	if artist != "" {
		tag.SetArtist(artist)
	}
	if album != "" {
		tag.SetAlbum(album)
	}
	if albumArtist != "" {
		tag.AddTextFrame("TPE2", id3v2.EncodingUTF8, albumArtist)
	}

	// 内嵌歌词 (USLT 帧)
	if lyrics != "" {
		tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
			Encoding: id3v2.EncodingUTF8,
			Language: "XXX",
			Lyrics:   lyrics,
		})
	}

	// 内嵌封面图片 (APIC 帧)
	if len(coverBytes) > 0 {
		mimeType := detectMIMEType(coverBytes)
		tag.AddAttachedPicture(id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    mimeType,
			PictureType: id3v2.PTFrontCover,
			Description: "Cover",
			Picture:     coverBytes,
		})
	}

	return tag.Save()
}

// embedFLACMetadata 给 FLAC 写入 Vorbis Comment 块及 Picture 块
func embedFLACMetadata(filePath, title, artist, album, albumArtist string, coverBytes []byte, lyrics string) error {
	f, err := flac.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("解析 FLAC 文件失败: %w", err)
	}

	// 1. 过滤已有的 Vorbis Comment 与 Picture 块
	var newMeta []*flac.MetaDataBlock
	for _, block := range f.Meta {
		if block.Type != flac.VorbisComment && block.Type != flac.Picture {
			newMeta = append(newMeta, block)
		}
	}

	// 2. 构建 Vorbis Comment (Block Type 4)
	comments := []string{}
	if title != "" {
		comments = append(comments, "TITLE="+title)
	}
	if artist != "" {
		comments = append(comments, "ARTIST="+artist)
	}
	if album != "" {
		comments = append(comments, "ALBUM="+album)
	}
	if albumArtist != "" {
		comments = append(comments, "ALBUMARTIST="+albumArtist)
		comments = append(comments, "ALBUM ARTIST="+albumArtist)
	}
	if lyrics != "" {
		comments = append(comments, "LYRICS="+lyrics)
	}

	if len(comments) > 0 {
		vcBlock := buildVorbisCommentBlock(comments)
		newMeta = append(newMeta, vcBlock)
	}

	// 3. 构建 Picture Block (Block Type 6)
	if len(coverBytes) > 0 {
		mimeType := detectMIMEType(coverBytes)
		pic, err := flacpicture.NewFromImageData(
			flacpicture.PictureTypeFrontCover,
			"Cover",
			coverBytes,
			mimeType,
		)
		if err == nil {
			picBlock := pic.Marshal()
			newMeta = append(newMeta, &picBlock)
		} else {
			core.Warn("构建 FLAC 封面块失败: %v", err)
		}
	}

	f.Meta = newMeta
	return f.Save(filePath)
}

// buildVorbisCommentBlock 构造 Vorbis Comment 二进制 MetaDataBlock
func buildVorbisCommentBlock(comments []string) *flac.MetaDataBlock {
	var buf bytes.Buffer
	vendor := "2FMusic Engine"
	vendorLen := uint32(len(vendor))

	_ = binary.Write(&buf, binary.LittleEndian, vendorLen)
	buf.WriteString(vendor)

	count := uint32(len(comments))
	_ = binary.Write(&buf, binary.LittleEndian, count)

	for _, c := range comments {
		cLen := uint32(len(c))
		_ = binary.Write(&buf, binary.LittleEndian, cLen)
		buf.WriteString(c)
	}

	return &flac.MetaDataBlock{
		Type: flac.VorbisComment,
		Data: buf.Bytes(),
	}
}

// detectMIMEType 检测图片二进制 MIME 类型
func detectMIMEType(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return "image/png"
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte("\xff\xd8\xff")) {
		return "image/jpeg"
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}

	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && format != "" {
		return mime.TypeByExtension("." + format)
	}
	return "image/jpeg"
}
