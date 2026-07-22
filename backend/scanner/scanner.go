package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/core"
	"2fmusic/backend/db"
)

var (
	NotifyLibraryChanged func()
	BroadcastScanStatus  func(scanning, isScraping bool, total, processed, failed int, currentFile, currentPath string)
	scanExecutionLock    sync.Mutex
	isScanningVal        bool
	isScrapingVal        bool
	totalFilesVal        int
	processedFilesVal    int
	failedFilesVal       int
	currentFileVal       string
	currentPathVal       string
	statusMu             sync.RWMutex
	lastBroadcastTime    time.Time
	lastBroadcastMu      sync.Mutex
)

func triggerStatusBroadcast() {
	if BroadcastScanStatus == nil {
		return
	}

	lastBroadcastMu.Lock()
	now := time.Now()
	if now.Sub(lastBroadcastTime) < 200*time.Millisecond {
		lastBroadcastMu.Unlock()
		return
	}
	lastBroadcastTime = now
	lastBroadcastMu.Unlock()

	statusMu.RLock()
	scanning := isScanningVal
	isScraping := isScrapingVal
	total := totalFilesVal
	processed := processedFilesVal
	failed := failedFilesVal
	currentFile := currentFileVal
	currentPath := currentPathVal
	statusMu.RUnlock()

	BroadcastScanStatus(scanning, isScraping, total, processed, failed, currentFile, currentPath)
}

// IsScanning 是否处于物理扫描中
func IsScanning() bool {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return isScanningVal
}

// IsScraping 是否处于在线元数据刮削中
func IsScraping() bool {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return isScrapingVal
}

// GetTotalFiles 获取待扫描文件总数
func GetTotalFiles() int {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return totalFilesVal
}

// GetProcessedFiles 获取已处理文件数
func GetProcessedFiles() int {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return processedFilesVal
}

// GetFailedFiles 获取失败处理文件数
func GetFailedFiles() int {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return failedFilesVal
}

// GetCurrentFile 获取当前正在处理的文件名
func GetCurrentFile() string {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return currentFileVal
}

// GetCurrentPath 获取当前正在处理的路径
func GetCurrentPath() string {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return currentPathVal
}

// CleanTempPartFiles 清理残留的 .part 与 .tmp 临时文件
func CleanTempPartFiles() {
	dirs := []string{core.GlobalConfig.MusicLibraryPath}
	if core.GlobalConfig.CacheDir != "" {
		dirs = append(dirs, core.GlobalConfig.CacheDir)
	}
	if core.GlobalConfig.NeteaseDownloadDir != "" {
		dirs = append(dirs, core.GlobalConfig.NeteaseDownloadDir)
	}
	mounts, err := db.GetMountPoints()
	if err == nil {
		for _, m := range mounts {
			dirs = append(dirs, m.Path)
		}
	}

	oldLock := filepath.Join(core.GlobalConfig.MusicLibraryPath, ".scan_lock")
	_ = os.Remove(oldLock)

	cleanedCount := 0
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				if strings.HasSuffix(path, ".part") || strings.HasSuffix(path, ".tmp") {
					if os.Remove(path) == nil {
						cleanedCount++
					}
				}
			}
			return nil
		})
	}
	if cleanedCount > 0 {
		core.Info("启动清理: 物理清除 %d 个 .part/.tmp 临时碎片文件", cleanedCount)
	}
}

// TriggerScan 触发物理扫描
func TriggerScan() {
	go ScanLibraryIncremental()
}

// ScanLibraryIncremental 全量/增量扫描
func ScanLibraryIncremental() {
	ScanDirectoryInternal("")
}

// ScanDirectorySingle 针对单个挂载目录的扫描
func ScanDirectorySingle(targetDir string) {
	ScanDirectoryInternal(targetDir)
}

// ScanDirectoryInternal 通用扫描逻辑
func ScanDirectoryInternal(targetDir string) {
	if !scanExecutionLock.TryLock() {
		core.Info("另一次物理扫描正在运行中，跳过本次扫描")
		return
	}
	defer scanExecutionLock.Unlock()

	cacheDir := core.GlobalConfig.CacheDir
	if cacheDir == "" {
		cacheDir = core.GlobalConfig.MusicLibraryPath
	}
	lockFile := filepath.Join(cacheDir, ".scan_lock")
	fi, err := os.Stat(lockFile)
	if err == nil {
		if time.Since(fi.ModTime()) > 5*time.Minute {
			core.Info("发现超过 5 分钟的过期 .scan_lock，自动清除自愈")
			_ = os.Remove(lockFile)
		} else {
			core.Info("存在活跃的 .scan_lock，取消本次扫描")
			return
		}
	}

	_ = os.WriteFile(lockFile, fmt.Appendf(nil, "%d", time.Now().Unix()), 0644)
	defer func() {
		_ = os.Remove(lockFile)
	}()

	statusMu.Lock()
	isScanningVal = true
	processedFilesVal = 0
	totalFilesVal = 0
	statusMu.Unlock()
	triggerStatusBroadcast()

	defer func() {
		statusMu.Lock()
		isScanningVal = false
		currentFileVal = ""
		currentPathVal = ""
		statusMu.Unlock()
		triggerStatusBroadcast()
	}()

	if targetDir != "" {
		core.Info("开始物理扫描曲库 (targetDir=%s)", targetDir)
	} else {
		core.Info("开始物理扫描曲库 (全量扫描)")
	}

	scanDirs := []string{}
	if targetDir != "" {
		scanDirs = append(scanDirs, targetDir)
	} else {
		scanDirs = append(scanDirs, core.GlobalConfig.MusicLibraryPath)
		mounts, err := db.GetMountPoints()
		if err == nil {
			for _, m := range mounts {
				if m.Path != "" {
					scanDirs = append(scanDirs, m.Path)
				}
			}
		}
	}

	diskPaths := make(map[string]bool)

	for _, dir := range scanDirs {
		if dir == "" {
			continue
		}
		_ = os.MkdirAll(dir, 0755)

		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				baseName := filepath.Base(path)
				if baseName == "covers" || baseName == "lyrics" || baseName == ".cache" || (strings.HasPrefix(baseName, ".") && baseName != ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if core.IsAudioFile(path) {
				diskPaths[path] = true
				statusMu.Lock()
				processedFilesVal++
				totalFilesVal = len(diskPaths)
				currentFileVal = filepath.Base(path)
				currentPathVal = path
				statusMu.Unlock()

				triggerStatusBroadcast()
				IndexSingleFile(path)
			}
			return nil
		})
	}

	cleanedCount := db.CleanStaleSongs(diskPaths)
	core.Info("物理扫描完成: 共扫描 %d 首有效曲目，物理清理 %d 条失效记录", len(diskPaths), cleanedCount)

	// 主动通知 WebSocket 客户端曲库已更新
	if NotifyLibraryChanged != nil {
		NotifyLibraryChanged()
	}

	// 触发缺漏元数据自动刮削
	go AutoScrapeMissingMetadata(targetDir)
}

// AutoScrapeMissingMetadata 后台自动为缺漏元数据的歌曲发起在线刮削
func AutoScrapeMissingMetadata(targetDir ...string) {
	filterDir := ""
	if len(targetDir) > 0 && targetDir[0] != "" {
		filterDir = core.NormalizePath(targetDir[0])
	}

	statusMu.Lock()
	isScrapingVal = true
	processedFilesVal = 0
	totalFilesVal = 0
	failedFilesVal = 0
	statusMu.Unlock()
	triggerStatusBroadcast()

	defer func() {
		statusMu.Lock()
		isScrapingVal = false
		currentFileVal = ""
		currentPathVal = ""
		statusMu.Unlock()
		triggerStatusBroadcast()

		if NotifyLibraryChanged != nil {
			NotifyLibraryChanged()
		}
	}()

	songs, err := db.GetAllSongs()
	if err != nil {
		return
	}

	if filterDir != "" {
		// 若指定了目标目录，重置该目录下所有歌曲的重试次数
		for _, s := range songs {
			if strings.HasPrefix(core.NormalizePath(s.Path), filterDir) {
				_ = db.SaveSong(&core.Song{
					ID:                s.ID,
					Path:              s.Path,
					Filename:          s.Filename,
					Title:             s.Title,
					Artist:            s.Artist,
					Album:             s.Album,
					AlbumArtist:       s.AlbumArtist,
					MTime:             s.MTime,
					Size:              s.Size,
					HasCover:          s.HasCover,
					HasLyrics:         s.HasLyrics,
					ScrapeRetryCount: 0,
				})
			}
		}
		songs, _ = db.GetAllSongs()
	}

	var songsToScrape []core.Song
	for _, s := range songs {
		if filterDir != "" && !strings.HasPrefix(core.NormalizePath(s.Path), filterDir) {
			continue
		}
		if s.ScrapeRetryCount >= 3 {
			continue
		}
		if IsNeteaseDownloadFile(s.Path) {
			// 网易云下载目录隔离刮削
			continue
		}
		if s.HasCover && s.HasLyrics {
			continue
		}
		songsToScrape = append(songsToScrape, s)
	}

	statusMu.Lock()
	totalFilesVal = len(songsToScrape)
	processedFilesVal = 0
	failedFilesVal = 0
	statusMu.Unlock()
	triggerStatusBroadcast()

	core.Info("发现 %d 首曲目需要刮削封面与歌词，开始并发拉取...", len(songsToScrape))

	semaphore := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for _, s := range songsToScrape {
		semaphore <- struct{}{}
		wg.Add(1)
		go func(song core.Song) {
			defer func() {
				statusMu.Lock()
				processedFilesVal++
				statusMu.Unlock()
				triggerStatusBroadcast()
				<-semaphore
				wg.Done()
			}()

			statusMu.Lock()
			currentFileVal = filepath.Base(song.Path)
			currentPathVal = song.Path
			statusMu.Unlock()
			triggerStatusBroadcast()

			core.Info("正在在线刮削元数据: %s - %s", song.Artist, song.Title)
			best := SearchSongFastSequential(song.Title, song.Artist, song.Album)
			if best != nil {
				updated := false
				if !song.HasCover {
					if coverURL, ok := best["cover"].(string); ok && coverURL != "" {
						imgData, err := DownloadImageBytes(coverURL)
						if err == nil && SaveCoverWebP(imgData, song.ID) {
							song.HasCover = true
							updated = true
						}
					}
				}
				if !song.HasLyrics {
					lyricsPref := strings.ToLower(core.GlobalConfig.LyricsPreference)
					embeddedHandled := false
					if lyricsPref != "network" { // 优先内嵌或默认
						if _, _, embeddedLyrics, err := ExtractAudioMetadata(song.Path); err == nil && embeddedLyrics != "" {
							lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, song.ID+".lrc")
							if saveLyricsFile(lrcPath, []byte(embeddedLyrics)) {
								song.HasLyrics = true
								updated = true
								embeddedHandled = true
							}
						}
					}
					if !embeddedHandled {
						if lyrics, ok := best["lyrics"].(string); ok && lyrics != "" {
							_ = os.WriteFile(filepath.Join(core.GlobalConfig.LyricsDir, song.ID+".lrc"), []byte(lyrics), 0644)
							song.HasLyrics = true
							updated = true
						} else if lyricsPref == "network" {
							// 优先网络但网络源无歌词时，退避解出内嵌歌词
							if _, _, embeddedLyrics, err := ExtractAudioMetadata(song.Path); err == nil && embeddedLyrics != "" {
								lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, song.ID+".lrc")
								if saveLyricsFile(lrcPath, []byte(embeddedLyrics)) {
									song.HasLyrics = true
									updated = true
								}
							}
						}
					}
				}

				if updated {
					core.Info("元数据刮削成功并同步落盘: %s", song.Title)
					db.UpdateSongMediaStatus(song.ID, song.HasCover, song.HasLyrics)
					if NotifyLibraryChanged != nil {
						NotifyLibraryChanged()
					}
					return
				}
			}

			// 退避重试计数加 1
			statusMu.Lock()
			failedFilesVal++
			statusMu.Unlock()
			db.IncrementScrapeRetryCount(song.ID)
		}(s)
	}
	wg.Wait()
	core.Info("元数据并发刮削任务全部完成")
}

// IsNeteaseDownloadFile 判断物理文件是否处于数据库配置的网易云下载存储目录中
func IsNeteaseDownloadFile(filePath string) bool {
	downloadDir := core.GlobalConfig.NeteaseDownloadDir
	if downloadDir == "" {
		downloadDir = filepath.Join(core.GlobalConfig.MusicLibraryPath, "NetEase")
	}
	normPath := core.NormalizePath(filePath)
	normDir := core.NormalizePath(downloadDir)
	return strings.HasPrefix(normPath, normDir)
}

// IndexSingleFile 单文件物理入库
func IndexSingleFile(filePath string) *core.Song {
	if !core.IsAudioFile(filePath) {
		return nil
	}

	fi, err := os.Stat(filePath)
	if err != nil || fi.IsDir() {
		return nil
	}

	existingSong, _ := db.GetSongByPath(filePath)
	mtime := float64(fi.ModTime().UnixNano()) / 1e9
	size := fi.Size()

	songID := ""
	if existingSong != nil && existingSong.Size == size && existingSong.MTime == mtime {
		songID = existingSong.ID
	} else {
		calcID, err := CalculateMD5(filePath)
		if err != nil {
			return nil
		}
		songID = calcID
	}

	song, picData, embeddedLyrics, err := ExtractAudioMetadata(filePath)
	if err != nil || song == nil {
		return nil
	}

	song.ID = songID

	// 检查封面
	coverPath := filepath.Join(core.GlobalConfig.CoversDir, songID+".webp")
	if _, err := os.Stat(coverPath); err == nil {
		song.HasCover = true
	} else if len(picData) > 0 {
		if SaveCoverWebP(picData, songID) {
			song.HasCover = true
		}
	}

	// 检查同级物理封面 (.webp)
	if !song.HasCover {
		dir := filepath.Dir(filePath)
		baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		sameImg := filepath.Join(dir, baseName+".webp")
		if b, err := os.ReadFile(sameImg); err == nil {
			if SaveCoverWebP(b, songID) {
				song.HasCover = true
			}
		}
	}

	// 检查歌词
	lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, songID+".lrc")
	yrcPath := filepath.Join(core.GlobalConfig.LyricsDir, songID+".yrc")
	if _, err := os.Stat(lrcPath); err == nil {
		song.HasLyrics = true
	} else if _, err := os.Stat(yrcPath); err == nil {
		song.HasLyrics = true
	} else if embeddedLyrics != "" {
		// 校验网易云下载目录与刮削偏好策略
		if saveLyricsFile(lrcPath, []byte(embeddedLyrics)) {
			if IsNeteaseDownloadFile(filePath) || strings.ToLower(core.GlobalConfig.LyricsPreference) != "network" {
				song.HasLyrics = true
			}
		}
	} else {
		// 同级外部歌词
		dir := filepath.Dir(filePath)
		baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		sameLrc := filepath.Join(dir, baseName+".lrc")
		if b, err := os.ReadFile(sameLrc); err == nil {
			if saveLyricsFile(lrcPath, b) {
				song.HasLyrics = true
			}
		}
	}

	_ = db.SaveSong(song)
	return song
}

// saveLyricsFile 写入歌词文件
func saveLyricsFile(lrcPath string, content []byte) bool {
	cacheDir := core.GlobalConfig.CacheDir
	if cacheDir == "" {
		cacheDir = core.GlobalConfig.LyricsDir
	}
	tmpPath := filepath.Join(cacheDir, fmt.Sprintf("lrc_%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return false
	}
	if err := os.Rename(tmpPath, lrcPath); err != nil {
		_ = os.Remove(tmpPath)
		return false
	}
	return true
}

// GetOrScrapeLyrics 获取或刮削歌词
func GetOrScrapeLyrics(songID, title, artist, filename string, yrc bool) (string, error) {
	if songID != "" {
		if yrc {
			yrcPath := filepath.Join(core.GlobalConfig.LyricsDir, songID+".yrc")
			if b, err := os.ReadFile(yrcPath); err == nil {
				return string(b), nil
			}
		}
		lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, songID+".lrc")
		if b, err := os.ReadFile(lrcPath); err == nil {
			return string(b), nil
		}
		// 若为网易云下载目录下的歌曲且没有本地缓存歌词，避免发起第三方刮削
		if s, err := db.GetSongByID(songID); err == nil && s != nil && IsNeteaseDownloadFile(s.Path) {
			return "", fmt.Errorf("lyrics not found for netease download file")
		}
	}

	// 刮削
	if title == "" && filename != "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	best := SearchSongBest(title, artist, "")
	if best != nil {
		lyrics, _ := best["lyrics"].(string)
		if lyrics != "" && songID != "" {
			lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, songID+".lrc")
			if saveLyricsFile(lrcPath, []byte(lyrics)) {
				db.UpdateSongMediaStatus(songID, true, true)
			}
		}
		return lyrics, nil
	}

	return "", fmt.Errorf("lyrics not found")
}

// GetOrScrapeCover 获取或刮削封面
func GetOrScrapeCover(songID, title, artist, album string) (string, error) {
	if songID != "" {
		coverPath := filepath.Join(core.GlobalConfig.CoversDir, songID+".webp")
		if _, err := os.Stat(coverPath); err == nil {
			return fmt.Sprintf("/api/music/covers/%s.webp", songID), nil
		}
		// 若为网易云下载目录下的歌曲且没有本地缓存封面，避免发起第三方刮削
		if s, err := db.GetSongByID(songID); err == nil && s != nil && IsNeteaseDownloadFile(s.Path) {
			return "", fmt.Errorf("cover not found for netease download file")
		}
	}

	best := SearchSongBest(title, artist, album)
	if best != nil {
		coverURL, _ := best["cover"].(string)
		if coverURL != "" && songID != "" {
			// 下载封面并转码落盘
			imgData, err := DownloadImageBytes(coverURL)
			if err == nil && len(imgData) > 0 {
				if SaveCoverWebP(imgData, songID) {
					db.UpdateSongMediaStatus(songID, true, true)
					return fmt.Sprintf("/api/music/covers/%s.webp", songID), nil
				}
			}
		}
		return coverURL, nil
	}

	return "", fmt.Errorf("cover not found")
}


