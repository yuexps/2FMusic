package static

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"2fmusic/backend/config"
)

// DistFS 内嵌的前端编译资源
//go:embed dist/*
var DistFS embed.FS

// GetFileSystem 获取高可用静态资源 FileSystem (优先外部磁盘 www 目录，后备嵌入式 DistFS)
func GetFileSystem() http.FileSystem {
	wwwDir := filepath.Join(config.GlobalConfig.BaseDir, "www")
	if fi, err := os.Stat(wwwDir); err == nil && fi.IsDir() {
		return http.Dir(wwwDir)
	}

	sub, err := fs.Sub(DistFS, "dist")
	if err != nil {
		return http.Dir(wwwDir)
	}
	return http.FS(sub)
}

// ServeStaticFile 处理 SPA 前端页面路由降级与内嵌资源响应
func ServeStaticFile(reqPath string) ([]byte, bool) {
	wwwDir := filepath.Join(config.GlobalConfig.BaseDir, "www")
	targetDiskFile := filepath.Join(wwwDir, filepath.FromSlash(reqPath))
	if b, err := os.ReadFile(targetDiskFile); err == nil {
		return b, true
	}

	cleanRelPath := strings.ReplaceAll(strings.TrimPrefix(reqPath, "/"), "\\", "/")
	embeddedPath := "dist/" + path.Clean(cleanRelPath)
	if b, err := DistFS.ReadFile(embeddedPath); err == nil {
		return b, true
	}

	return nil, false
}
