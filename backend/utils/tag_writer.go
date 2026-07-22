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

// embedMP3Metadata 增量写入 MP3 ID3v2 封面与歌词标签
func embedMP3Metadata(filePath, title, artist, album, albumArtist string, coverBytes []byte, lyrics string) error {
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		tag = id3v2.NewEmptyTag()
	}
	defer tag.Close()

	tag.SetDefaultEncoding(id3v2.EncodingUTF8)

	if strings.TrimSpace(tag.Title()) == "" && title != "" {
		tag.SetTitle(title)
	}
	if strings.TrimSpace(tag.Artist()) == "" && artist != "" {
		tag.SetArtist(artist)
	}
	if strings.TrimSpace(tag.Album()) == "" && album != "" {
		tag.SetAlbum(album)
	}
	if albumArtist != "" && len(tag.GetFrames("TPE2")) == 0 {
		tag.AddTextFrame("TPE2", id3v2.EncodingUTF8, albumArtist)
	}

	if lyrics != "" {
		tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
			Encoding: id3v2.EncodingUTF8,
			Language: "XXX",
			Lyrics:   lyrics,
		})
	}

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

// embedFLACMetadata 增量写入 FLAC VorbisComment 及 Picture 块
func embedFLACMetadata(filePath, title, artist, album, albumArtist string, coverBytes []byte, lyrics string) error {
	f, err := flac.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("解析 FLAC 文件失败: %w", err)
	}

	existingComments := parseVorbisCommentBlock(f.Meta)
	commentMap := make(map[string]string)
	for _, c := range existingComments {
		parts := strings.SplitN(c, "=", 2)
		if len(parts) == 2 {
			commentMap[strings.ToUpper(parts[0])] = parts[1]
		}
	}

	if _, ok := commentMap["TITLE"]; !ok && title != "" {
		commentMap["TITLE"] = title
	}
	if _, ok := commentMap["ARTIST"]; !ok && artist != "" {
		commentMap["ARTIST"] = artist
	}
	if _, ok := commentMap["ALBUM"]; !ok && album != "" {
		commentMap["ALBUM"] = album
	}
	if _, ok := commentMap["ALBUMARTIST"]; !ok && albumArtist != "" {
		commentMap["ALBUMARTIST"] = albumArtist
	}
	if lyrics != "" {
		commentMap["LYRICS"] = lyrics
	}

	comments := make([]string, 0, len(commentMap))
	for k, v := range commentMap {
		comments = append(comments, k+"="+v)
	}

	// 重新排列 FLAC 元数据块（排除旧 VorbisComment 与 Picture，放入全新构造块）
	var newMeta []*flac.MetaDataBlock
	for _, block := range f.Meta {
		if block.Type != flac.VorbisComment && block.Type != flac.Picture {
			newMeta = append(newMeta, block)
		}
	}

	if len(comments) > 0 {
		vcBlock := buildVorbisCommentBlock(comments)
		newMeta = append(newMeta, vcBlock)
	}

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
		}
	}

	f.Meta = newMeta
	return f.Save(filePath)
}

// parseVorbisCommentBlock 从 FLAC MetaDataBlock 中解算已有的 Vorbis Comments 键值对数组
func parseVorbisCommentBlock(blocks []*flac.MetaDataBlock) []string {
	for _, block := range blocks {
		if block.Type == flac.VorbisComment && len(block.Data) >= 8 {
			data := block.Data
			vendorLen := binary.LittleEndian.Uint32(data[0:4])
			if uint32(len(data)) < 4+vendorLen+4 {
				continue
			}
			offset := 4 + vendorLen
			commentCount := binary.LittleEndian.Uint32(data[offset : offset+4])
			offset += 4

			comments := make([]string, 0, commentCount)
			for i := uint32(0); i < commentCount; i++ {
				if uint32(len(data)) < offset+4 {
					break
				}
				cLen := binary.LittleEndian.Uint32(data[offset : offset+4])
				offset += 4
				if uint32(len(data)) < offset+cLen {
					break
				}
				cStr := string(data[offset : offset+cLen])
				offset += cLen
				comments = append(comments, cStr)
			}
			return comments
		}
	}
	return nil
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
