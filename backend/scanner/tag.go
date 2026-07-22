package scanner

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"2fmusic/backend/core"
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
	maxDim := 600

	targetImg := srcImg
	if w > maxDim || h > maxDim {
		var newW, newH int
		if w >= h {
			newW = maxDim
			newH = int(float64(h) * float64(maxDim) / float64(w))
		} else {
			newH = maxDim
			newW = int(float64(w) * float64(maxDim) / float64(h))
		}
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}

		dstImg := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.Draw(dstImg, dstImg.Bounds(), srcImg, bounds.Min, draw.Over)
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
