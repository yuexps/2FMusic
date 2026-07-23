package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// SanitizeFilename 优雅清洗文件名，避免斜杠等非法字符破坏路径并保持美观
func SanitizeFilename(filename string) string {
	res := filename

	// 1. 替换多歌手与路径斜杠分隔符为逗号
	res = strings.ReplaceAll(res, " / ", ", ")
	res = strings.ReplaceAll(res, "/", ", ")
	res = strings.ReplaceAll(res, "\\", ", ")

	// 2. 替换冒号为连字符
	res = strings.ReplaceAll(res, " : ", " - ")
	res = strings.ReplaceAll(res, ":", " - ")

	// 3. 移除其它操作系统非法文件名字符
	invalidChars := []string{"*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		res = strings.ReplaceAll(res, char, "")
	}

	res = strings.Join(strings.Fields(res), " ")
	if res == "" {
		res = "unnamed"
	}
	return res
}

// SafeMoveFile 跨设备/跨磁盘安全移动文件。先尝试 os.Rename，失败时自动退避为复制+删除源文件
func SafeMoveFile(src, dst string) error {
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	// 1. 尝试直接重命名/移动
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 2. 若重命名失败（例如跨物理驱动器 EXDEV 或权限限制），降级为 Safe Copy + Remove
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = os.Remove(dst)
		return err
	}

	// 强制刷盘闭合
	_ = dstFile.Sync()
	srcFile.Close()
	dstFile.Close()

	// 移动成功后删除源文件
	_ = os.Remove(src)
	return nil
}

// SafeRemoveFile 安全物理删除文件。尝试多次重试与 GC 句柄释放，若仍被独占则显式返回错误
func SafeRemoveFile(targetPath string) error {
	if targetPath == "" {
		return nil
	}

	fi, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return os.RemoveAll(targetPath)
	}

	// 5 轮轻量重试 + 强制 GC 释放文件句柄
	for i := 0; i < 5; i++ {
		if err := os.Remove(targetPath); err == nil || os.IsNotExist(err) {
			return nil
		}
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("文件正被操作系统其他进程独占，无法物理删除: %s", filepath.Base(targetPath))
}
