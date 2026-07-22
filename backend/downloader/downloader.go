package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/core"
	"2fmusic/backend/scanner"
	"2fmusic/backend/utils"
)

var (
	BroadcastJSON        func(v interface{})
	NotifyLibraryChanged func()
	downloadTasks        = make(map[string]*core.DownloadTask)
	downloadTasksMu      sync.RWMutex
)

// ClearTask 清理单条任务
func ClearTask(taskID string) {
	downloadTasksMu.Lock()
	defer downloadTasksMu.Unlock()
	delete(downloadTasks, taskID)
}

// ClearAllTasks 清理所有已完成与失败的任务
func ClearAllTasks() {
	downloadTasksMu.Lock()
	defer downloadTasksMu.Unlock()
	for id, t := range downloadTasks {
		if t.Status == "success" || t.Status == "error" {
			delete(downloadTasks, id)
		}
	}
}

// GetTasks 获取所有下载任务状态
func GetTasks() []core.DownloadTask {
	downloadTasksMu.RLock()
	defer downloadTasksMu.RUnlock()

	list := make([]core.DownloadTask, 0, len(downloadTasks))
	for _, t := range downloadTasks {
		list = append(list, *t)
	}
	return list
}

// StartNeteaseDownload 发起网易云在线下载任务
func StartNeteaseDownload(songID, title, artist, album, level string) string {
	taskID := fmt.Sprintf("dl_%s_%d", songID, time.Now().UnixNano())

	task := &core.DownloadTask{
		TaskID:    taskID,
		SongID:    songID,
		Title:     title,
		Artist:    artist,
		Album:     album,
		Level:     level,
		Progress:  0,
		Status:    "pending",
		CreatedAt: float64(time.Now().UnixNano()) / 1e9,
	}

	downloadTasksMu.Lock()
	downloadTasks[taskID] = task
	downloadTasksMu.Unlock()

	core.Info("创建网易云下载任务: %s - %s (Level=%s, TaskID=%s)", title, artist, level, taskID)

	go executeDownload(task, level)
	return taskID
}

// executeDownload 执行下载状态机流转
func executeDownload(task *core.DownloadTask, level string) {
	updateTaskStatus(task.TaskID, "preparing", 0, "")

	if task.Title == "" || level == "" {
		metaResp, err := CallNeteaseAPI("/song/detail", map[string]string{"ids": task.SongID})
		if err == nil {
			if songs, ok := metaResp["songs"].([]interface{}); ok && len(songs) > 0 {
				if info, ok := songs[0].(map[string]interface{}); ok {
					if task.Title == "" {
						if n, ok := info["name"].(string); ok {
							task.Title = n
						}
					}
					if task.Artist == "" || task.Artist == "未知艺术家" {
						if arList, ok := info["ar"].([]interface{}); ok && len(arList) > 0 {
							names := []string{}
							for _, ar := range arList {
								if arMap, ok := ar.(map[string]interface{}); ok {
									if n, ok := arMap["name"].(string); ok && n != "" {
										names = append(names, n)
									}
								}
							}
							if len(names) > 0 {
								task.Artist = strings.Join(names, " / ")
							}
						}
					}
					if level == "" {
						if priv, ok := info["privilege"].(map[string]interface{}); ok {
							_, maxL := extractSongLevel(priv)
							level = maxL
						}
					}
				}
			}
		}
	}

	if level == "" {
		level = "exhigh"
	}

	playURL := fetchNeteaseSongURL(task.SongID, level)
	if playURL == "" && level != "standard" {
		core.Info("音质 %s 获取失败，自动回退到 standard 音质重试", level)
		playURL = fetchNeteaseSongURL(task.SongID, "standard")
	}

	if playURL == "" {
		updateTaskStatus(task.TaskID, "error", 0, "无法获取网易云音频播放地址，可能需要登录或无版权")
		return
	}

	updateTaskStatus(task.TaskID, "downloading", 5, "")

	ext := ".mp3"
	if level == "lossless" {
		ext = ".flac"
	}

	fileName := fmt.Sprintf("%s - %s%s", task.Artist, task.Title, ext)
	if task.Artist == "" {
		fileName = fmt.Sprintf("%s%s", task.Title, ext)
	}

	targetDir := core.GlobalConfig.NeteaseDownloadDir
	if targetDir == "" {
		targetDir = core.GlobalConfig.MusicLibraryPath
	}

	cacheDir := core.GlobalConfig.CacheDir
	if cacheDir == "" {
		cacheDir = targetDir
	}

	partPath := filepath.Join(cacheDir, fmt.Sprintf("dl_%s%s.part", task.TaskID, ext))
	finalPath := filepath.Join(targetDir, fileName)

	// 移除 Watchdog Ignore 限制，以便物理文件移入 NetEase 目录后触发自动感应入库

	req, err := http.NewRequest("GET", playURL, nil)
	if err != nil {
		updateTaskStatus(task.TaskID, "error", 0, err.Error())
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		updateTaskStatus(task.TaskID, "error", 0, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		updateTaskStatus(task.TaskID, "error", 0, fmt.Sprintf("HTTP 状态码: %d", resp.StatusCode))
		return
	}

	totalSize := resp.ContentLength
	out, err := os.Create(partPath)
	if err != nil {
		updateTaskStatus(task.TaskID, "error", 0, err.Error())
		return
	}

	buf := make([]byte, 128*1024)
	var downloaded int64
	lastProg := -1
	lastBroadcast := time.Now()

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, wErr := out.Write(buf[:n])
			if wErr != nil {
				out.Close()
				_ = os.Remove(partPath)
				updateTaskStatus(task.TaskID, "error", 0, fmt.Sprintf("写入文件失败: %v", wErr))
				return
			}
			downloaded += int64(n)
			if totalSize > 0 {
				prog := int(float64(downloaded) / float64(totalSize) * 85)
				if prog != lastProg && time.Since(lastBroadcast) >= 100*time.Millisecond {
					lastProg = prog
					lastBroadcast = time.Now()
					updateTaskStatus(task.TaskID, "downloading", 5+prog, "")
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			out.Close()
			_ = os.Remove(partPath)
			updateTaskStatus(task.TaskID, "error", 0, fmt.Sprintf("下载中断: %v", err))
			return
		}
	}
	out.Close()

	// 闭合下载流后，将 .part 临时文件重命名为带有真实后缀 (.mp3/.flac) 的缓存临时文件
	tempAudioPath := strings.TrimSuffix(partPath, ".part")
	if err := os.Rename(partPath, tempAudioPath); err != nil {
		tempAudioPath = partPath
	}

	// 拉取网易云歌词 (优先 YRC 逐字歌词) 与封面图片
	updateTaskStatus(task.TaskID, "downloading", 92, "")
	lyrics := fetchNeteaseLyric(task.SongID)
	var coverBytes []byte
	if coverURL := fetchNeteaseCoverURL(task.SongID); coverURL != "" {
		if cBytes, err := scanner.DownloadImageBytes(coverURL); err == nil {
			coverBytes = cBytes
		}
	}

	// 在 .cache 临时目录下将元数据、封面及歌词直接内嵌写入音频文件
	updateTaskStatus(task.TaskID, "downloading", 96, "")
	if err := utils.EmbedAudioMetadata(tempAudioPath, task.Title, task.Artist, task.Album, task.Artist, coverBytes, lyrics); err != nil {
		core.Warn("内嵌音频元数据失败: %v", err)
	}

	// 将完成内嵌的物理音频文件移动至最终 NetEase 存储路径
	if err := os.Rename(tempAudioPath, finalPath); err != nil {
		_ = os.Remove(tempAudioPath)
		updateTaskStatus(task.TaskID, "error", 0, fmt.Sprintf("文件落盘失败: %v", err))
		return
	}
	updateTaskStatus(task.TaskID, "success", 100, "")
}

// updateTaskStatus 更新下载状态并进行 WS 广播
func updateTaskStatus(taskID, status string, progress int, errMsg string) {
	downloadTasksMu.Lock()
	t, exists := downloadTasks[taskID]
	if exists {
		t.Status = status
		t.Progress = progress
		t.Error = errMsg

		switch status {
		case "success":
			core.Info("网易云下载任务成功落盘: %s - %s", t.Title, t.Artist)
		case "error":
			core.Error("网易云下载任务异常终止: %s - %s (原因: %s)", t.Title, t.Artist, errMsg)
		}
	}
	downloadTasksMu.Unlock()

	if exists && BroadcastJSON != nil {
		BroadcastJSON(map[string]interface{}{
			"type":   "broadcast",
			"action": "download_status",
			"data":   t,
		})
		// 下载成功后通知库变更，触发前端刷新歌曲列表
		if status == "success" && NotifyLibraryChanged != nil {
			NotifyLibraryChanged()
		}
	}
}
