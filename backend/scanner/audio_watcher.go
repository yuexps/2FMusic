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

var audioWatcher *fsnotify.Watcher

// InitAudioWatcher 初始化音频物理目录监听器 (Audio_Watcher)
func InitAudioWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	audioWatcher = w

	go audioWatchLoop()
	RefreshAudioWatchPaths()
	return nil
}

// RefreshAudioWatchPaths 刷入监听路径：AudiosDir, NetEase, Mounts
func RefreshAudioWatchPaths() {
	if audioWatcher == nil {
		return
	}

	pathsToWatch := []string{core.GlobalConfig.AudiosDir}

	if d := core.GlobalConfig.NeteaseDownloadDir; d != "" {
		pathsToWatch = append(pathsToWatch, d)
	}

	mounts, err := db.GetMountPoints()
	if err == nil {
		for _, m := range mounts {
			pathsToWatch = append(pathsToWatch, m.Path)
		}
	}

	for _, p := range pathsToWatch {
		if p == "" {
			continue
		}
		_ = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				// 排除 .cache, covers, lyrics
				baseName := filepath.Base(path)
				if baseName == ".cache" || baseName == "covers" || baseName == "lyrics" || (strings.HasPrefix(baseName, ".") && baseName != ".") {
					return filepath.SkipDir
				}
				_ = audioWatcher.Add(path)
			}
			return nil
		})
	}
}

// AddAudioWatchPath 动态添加单个挂载路径
func AddAudioWatchPath(dirPath string) {
	if audioWatcher == nil || dirPath == "" {
		return
	}
	_ = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			baseName := filepath.Base(path)
			if baseName == ".cache" || baseName == "covers" || baseName == "lyrics" || (strings.HasPrefix(baseName, ".") && baseName != ".") {
				return filepath.SkipDir
			}
			_ = audioWatcher.Add(path)
		}
		return nil
	})
}

// audioWatchLoop 音频监听循环
func audioWatchLoop() {
	audioExts := core.AudioExtsMap

	var (
		lastPathTime = make(map[string]time.Time)
		lastPathMu   sync.Mutex
	)

	for {
		select {
		case event, ok := <-audioWatcher.Events:
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
			if shouldSkip || strings.HasSuffix(cleanPath, ".part") || strings.HasSuffix(cleanPath, ".tmp") {
				continue
			}

			// 100ms 路径防抖：针对同一个音频文件路径，100ms 内的重复事件瞬间忽略
			lastPathMu.Lock()
			if t, ok := lastPathTime[cleanPath]; ok && time.Since(t) < 100*time.Millisecond {
				lastPathMu.Unlock()
				continue
			}
			lastPathTime[cleanPath] = time.Now()
			if len(lastPathTime) > 1000 {
				lastPathTime = make(map[string]time.Time)
			}
			lastPathMu.Unlock()

			// 忽略纯属性变更 (Chmod) 事件，仅响应文件创建、写入、删除与重命名
			if event.Op&fsnotify.Chmod != 0 && event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			ext := strings.ToLower(filepath.Ext(cleanPath))
			if !audioExts[ext] {
				continue
			}

			fi, err := os.Stat(cleanPath)
			if err == nil && !fi.IsDir() {
				// 文件物理存在 -> 提取基础元数据入库
				indexAudioFileBasic(cleanPath, fi)
			} else {
				// 文件已被移动或删除 -> 从数据库清理记录
				core.Info("[Audio_Watcher] 音频文件移除: %s", filepath.Base(cleanPath))
				_ = db.DeleteSongByPath(cleanPath)
			}

		case err, ok := <-audioWatcher.Errors:
			if !ok {
				return
			}
			core.Error("[Audio_Watcher] 监听异常: %v", err)
		}
	}
}

// indexAudioFileBasic 仅解析基础 Tag 并加入数据库索引 (默认 HasCover/HasLyrics 为 0)
func indexAudioFileBasic(filePath string, fi os.FileInfo) {
	filePath = core.NormalizePath(filePath)

	song, _, _, err := ExtractAudioMetadata(filePath)
	if err != nil || song == nil {
		song = &core.Song{
			Title:  strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)),
			Artist: "Unknown Artist",
			Album:  "Unknown Album",
		}
	}

	songID, err := CalculateMD5(filePath)
	if err != nil || songID == "" {
		return
	}

	song.ID = songID
	song.Path = filePath
	song.Filename = filepath.Base(filePath)
	song.MTime = float64(fi.ModTime().UnixNano()) / 1e9
	song.Size = fi.Size()
	song.HasCover = false
	song.HasLyrics = false

	// 存入数据库，数据库 SaveSong 会自动向 SongInsertedChan 投递对象触发 DB_Watcher
	if err := db.SaveSong(song); err != nil {
		core.Error("[Audio_Watcher] 音频入库失败: %s, err: %v", song.Filename, err)
	} else {
		core.Info("[Audio_Watcher] 音频入库索引成功: %s (ID: %s)", song.Filename, song.ID)
	}
}
