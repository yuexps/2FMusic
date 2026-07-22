package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/core"
	"2fmusic/backend/db"

	"github.com/fsnotify/fsnotify"
)

var (
	watcher       *fsnotify.Watcher
	ignorePaths   = make(map[string]time.Time)
	ignorePathsMu sync.RWMutex
)

// AddWatchdogIgnorePath 添加忽略路径 (Web 上传/下载临时文件)
func AddWatchdogIgnorePath(path string) {
	ignorePathsMu.Lock()
	defer ignorePathsMu.Unlock()
	ignorePaths[path] = time.Now().Add(30 * time.Second)
}

// isIgnored 检查路径是否应被忽略
func isIgnored(path string) bool {
	ignorePathsMu.RLock()
	defer ignorePathsMu.RUnlock()

	exp, exists := ignorePaths[path]
	if exists {
		if time.Now().Before(exp) {
			return true
		}
	}
	return false
}

// InitWatcher 初始化 fsnotify 监听与防抖队列
func InitWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	watcher = w

	go watchLoop()
	RefreshWatchPaths()
	return nil
}

// RefreshWatchPaths 重新扫描并添加监听目录
func RefreshWatchPaths() {
	if watcher == nil {
		return
	}

	pathsToWatch := []string{core.GlobalConfig.MusicLibraryPath}
	mounts, err := db.GetMountPoints()
	if err == nil {
		for _, m := range mounts {
			pathsToWatch = append(pathsToWatch, m.Path)
		}
	}
	// 自定义下载单独加入监听
	if d := core.GlobalConfig.NeteaseDownloadDir; d != "" {
		pathsToWatch = append(pathsToWatch, d)
	}

	for _, p := range pathsToWatch {
		_ = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = watcher.Add(path)
			}
			return nil
		})
	}
}

// watchLoop 处理文件事件
func watchLoop() {
	audioExts := core.AudioExtsMap
	miscExts := core.MiscExtsMap

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			cleanPath := filepath.Clean(event.Name)
			parts := strings.Split(cleanPath, string(filepath.Separator))
			shouldSkip := false
			for _, part := range parts {
				if part == "lyrics" || part == "covers" || part == ".cache" || (strings.HasPrefix(part, ".") && part != ".") {
					shouldSkip = true
					break
				}
			}
			if shouldSkip || strings.HasSuffix(event.Name, ".part") || strings.HasSuffix(event.Name, ".scan_lock") || isIgnored(event.Name) {
				continue
			}

			ext := strings.ToLower(filepath.Ext(event.Name))
			if !audioExts[ext] && !miscExts[ext] {
				continue
			}

			path := event.Name
			fExt := strings.ToLower(filepath.Ext(path))
			if audioExts[fExt] {
				fi, err := os.Stat(path)
				if err == nil && !fi.IsDir() {
					core.Info("[Watcher] 文件变动 新增: %s", filepath.Base(path))
					IndexSingleFile(path)
				} else {
					core.Info("[Watcher] 文件变动 删除: %s", filepath.Base(path))
					_ = db.DeleteSongByPath(path)
				}
			} else if miscExts[fExt] {
				basePath := strings.TrimSuffix(path, filepath.Ext(path))
				for audExt := range audioExts {
					audPath := basePath + audExt
					if fi, err := os.Stat(audPath); err == nil && !fi.IsDir() {
						IndexSingleFile(audPath)
					}
				}
			}
			if NotifyLibraryChanged != nil {
				NotifyLibraryChanged()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			core.Error("Watcher 错误: %v", err)
		}
	}
}
