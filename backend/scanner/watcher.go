package scanner

import (
	"sync"
	"time"

	"2fmusic/backend/core"
)

var (
	ignorePaths   = make(map[string]time.Time)
	ignorePathsMu sync.RWMutex
)

// AddWatchdogIgnorePath 添加忽略路径 (Web 上传/下载临时文件避让)
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

// InitWatcher 初始化曲库监听服务
func InitWatcher() error {
	core.Info("正在启动曲库监听服务...")

	if err := InitAudioWatcher(); err != nil {
		core.Error("初始化 Audio_Watcher 失败: %v", err)
		return err
	}

	InitDBWatcher()

	if err := InitLCWatcher(); err != nil {
		core.Error("初始化 LC_Watcher 失败: %v", err)
		return err
	}

	core.Info("曲库监听服务启动成功")
	return nil
}

// RefreshWatchPaths 刷入与更新所有 Watcher 的监听路径
func RefreshWatchPaths() {
	RefreshAudioWatchPaths()
	RefreshLCWatchPaths()
}
