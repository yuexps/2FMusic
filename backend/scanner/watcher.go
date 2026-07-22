package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/config"
	"2fmusic/backend/db"
	"2fmusic/backend/logger"

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

	pathsToWatch := []string{config.GlobalConfig.MusicLibraryPath}
	mounts, err := db.GetMountPoints()
	if err == nil {
		for _, m := range mounts {
			pathsToWatch = append(pathsToWatch, m.Path)
		}
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

// watchLoop 处理物理事件并执行 2.0s 防抖 + 1.0s 文件稳定判定
func watchLoop() {
	timers := make(map[string]*time.Timer)
	lastSizes := make(map[string]int64)
	var mu sync.Mutex

	audioExts := map[string]bool{".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true, ".m4a": true}
	miscExts := map[string]bool{".lrc": true, ".yrc": true, ".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

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

			targetFile := event.Name

			mu.Lock()
			if t, exists := timers[targetFile]; exists {
				t.Stop()
			}

			// 单文件防抖消费处理 (完全对齐 Python 原版 _execute_watchdog_event)
			executeWatchdogSync := func(path string) {
				fExt := strings.ToLower(filepath.Ext(path))
				if audioExts[fExt] {
					fi, err := os.Stat(path)
					if err == nil && !fi.IsDir() {
						logger.Info("Watchdog 物理更新单曲: %s", filepath.Base(path))
						IndexSingleFile(path)
					} else {
						logger.Info("Watchdog 物理移除单曲: %s", filepath.Base(path))
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
			}

			timers[targetFile] = time.AfterFunc(2000*time.Millisecond, func() {
				// 判定文件稳定度
				fi, err := os.Stat(targetFile)
				if err != nil {
					// 物理删除事件
					executeWatchdogSync(targetFile)
					mu.Lock()
					delete(timers, targetFile)
					delete(lastSizes, targetFile)
					mu.Unlock()
					return
				}

				currentSize := fi.Size()
				mu.Lock()
				prevSize, exists := lastSizes[targetFile]
				if exists && currentSize != prevSize {
					// 大小还在物理写入中，追加 1.0 秒自适应延迟
					lastSizes[targetFile] = currentSize
					timers[targetFile] = time.AfterFunc(1000*time.Millisecond, func() {
						executeWatchdogSync(targetFile)
					})
					mu.Unlock()
					return
				}

				lastSizes[targetFile] = currentSize
				mu.Unlock()

				executeWatchdogSync(targetFile)

				mu.Lock()
				delete(timers, targetFile)
				delete(lastSizes, targetFile)
				mu.Unlock()
			})
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Error("Watcher 错误: %v", err)
		}
	}
}
