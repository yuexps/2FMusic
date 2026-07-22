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

var lcWatcher *fsnotify.Watcher

// InitLCWatcher 初始化封面与歌词物理缓存监听器 (LC_Watcher)
func InitLCWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	lcWatcher = w

	go lcWatchLoop()
	RefreshLCWatchPaths()
	return nil
}

// RefreshLCWatchPaths 刷入监听路径：CoversDir 与 LyricsDir
func RefreshLCWatchPaths() {
	if lcWatcher == nil {
		return
	}
	if core.GlobalConfig.CoversDir != "" {
		_ = lcWatcher.Add(core.GlobalConfig.CoversDir)
	}
	if core.GlobalConfig.LyricsDir != "" {
		_ = lcWatcher.Add(core.GlobalConfig.LyricsDir)
	}
}

// lcWatchLoop 封面与歌词物理目录监听循环
func lcWatchLoop() {
	var lastNotifyTime time.Time

	notifyChanged := func() {
		if time.Since(lastNotifyTime) > 300*time.Millisecond {
			lastNotifyTime = time.Now()
			if NotifyLibraryChanged != nil {
				NotifyLibraryChanged()
			}
		}
	}

	var (
		lastPathTime = make(map[string]time.Time)
		lastPathMu   sync.Mutex
	)

	for {
		select {
		case event, ok := <-lcWatcher.Events:
			if !ok {
				return
			}

			cleanPath := filepath.Clean(event.Name)
			if strings.HasSuffix(cleanPath, ".tmp") || strings.HasSuffix(cleanPath, ".part") {
				continue
			}

			// 100ms 路径防抖：针对同一个文件路径，100ms 内的重复事件瞬间忽略
			lastPathMu.Lock()
			if t, ok := lastPathTime[cleanPath]; ok && time.Since(t) < 100*time.Millisecond {
				lastPathMu.Unlock()
				continue
			}
			lastPathTime[cleanPath] = time.Now()
			// 内存容量保护：防止 map 无限增大
			if len(lastPathTime) > 1000 {
				lastPathTime = make(map[string]time.Time)
			}
			lastPathMu.Unlock()

			// 忽略纯属性变更 (Chmod) 事件，仅响应文件创建、写入、删除与重命名
			if event.Op&fsnotify.Chmod != 0 && event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			ext := strings.ToLower(filepath.Ext(cleanPath))
			fileName := filepath.Base(cleanPath)
			songID := strings.TrimSuffix(fileName, ext)

			if songID == "" {
				continue
			}

			dir := filepath.Dir(cleanPath)

			// 判断是 Covers 目录还是 Lyrics 目录
			if dir == core.GlobalConfig.CoversDir && ext == ".webp" {
				song, _ := db.GetSongByID(songID)
				if fi, err := os.Stat(cleanPath); err == nil && !fi.IsDir() {
					if song == nil || !song.HasCover {
						core.Info("[LC_Watcher] 捕捉封面新增: %s.webp", songID)
						db.UpdateSongHasCover(songID, true)
						notifyChanged()
					}
				} else {
					if song != nil && song.HasCover {
						core.Info("[LC_Watcher] 捕捉封面删除: %s.webp", songID)
						db.UpdateSongHasCover(songID, false)
						notifyChanged()
					}
				}
			} else if dir == core.GlobalConfig.LyricsDir && ext == ".lrc" {
				song, _ := db.GetSongByID(songID)
				if fi, err := os.Stat(cleanPath); err == nil && !fi.IsDir() {
					if song == nil || !song.HasLyrics {
						core.Info("[LC_Watcher] 捕捉歌词新增: %s.lrc", songID)
						db.UpdateSongHasLyrics(songID, true)
						notifyChanged()
					}
				} else {
					if song != nil && song.HasLyrics {
						core.Info("[LC_Watcher] 捕捉歌词删除: %s.lrc", songID)
						db.UpdateSongHasLyrics(songID, false)
						notifyChanged()
					}
				}
			}

		case err, ok := <-lcWatcher.Errors:
			if !ok {
				return
			}
			core.Error("[LC_Watcher] 监听异常: %v", err)
		}
	}
}
