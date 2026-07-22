package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"2fmusic/backend/config"
	"2fmusic/backend/db"
	"2fmusic/backend/logger"
	"2fmusic/backend/model"
	"2fmusic/backend/scanner"
)

var (
	BroadcastJSON        func(v interface{})
	NotifyLibraryChanged func()
	downloadTasks        = make(map[string]*model.DownloadTask)
	downloadTasksMu      sync.RWMutex
)

// ClearTask 清理单条任务
func ClearTask(taskID string) {
	downloadTasksMu.Lock()
	defer downloadTasksMu.Unlock()
	delete(downloadTasks, taskID)
}

// ClearAllTasks 清理所有任务
func ClearAllTasks() {
	downloadTasksMu.Lock()
	defer downloadTasksMu.Unlock()
	downloadTasks = make(map[string]*model.DownloadTask)
}

// GetTasks 获取所有下载任务
func GetTasks() []model.DownloadTask {
	downloadTasksMu.RLock()
	defer downloadTasksMu.RUnlock()

	var list []model.DownloadTask
	for _, t := range downloadTasks {
		list = append(list, *t)
	}
	return list
}

// CallNeteaseAPI 核心透传请求 NCM API 服务
func CallNeteaseAPI(apiPath string, params map[string]string) (map[string]interface{}, error) {
	apiBase := config.GlobalConfig.NeteaseAPIBase
	if apiBase == "" {
		apiBase = "http://localhost:23236"
	}

	u, err := url.Parse(apiBase + apiPath)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	if config.GlobalConfig.NeteaseCookie != "" {
		req.Header.Set("Cookie", config.GlobalConfig.NeteaseCookie)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetNeteaseQRKeyAndImage 申请真正的网易云扫码 unikey 和 二维码 base64
func GetNeteaseQRKeyAndImage() (string, string, error) {
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
	keyRes, err := CallNeteaseAPI("/login/qr/key", map[string]string{"timestamp": timestamp})
	if err != nil {
		return "", "", err
	}

	dataMap, _ := keyRes["data"].(map[string]interface{})
	unikey, _ := dataMap["unikey"].(string)
	if unikey == "" {
		return "", "", fmt.Errorf("failed to get unikey from netease api")
	}

	imgRes, err := CallNeteaseAPI("/login/qr/create", map[string]string{"key": unikey, "qrimg": "1", "timestamp": timestamp})
	if err != nil {
		return "", "", err
	}

	imgDataMap, _ := imgRes["data"].(map[string]interface{})
	qrimg, _ := imgDataMap["qrimg"].(string)

	// 启动协程后台轮询检测
	go PollNeteaseQRStatus(unikey)

	return unikey, qrimg, nil
}

// PollNeteaseQRStatus 轮询检查扫码登录结果并持久化 Cookie
func PollNeteaseQRStatus(unikey string) {
	logger.Info("启动网易云扫码轮询协程: unikey=%s", unikey)
	maxDuration := 300 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxDuration {
		timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
		res, err := CallNeteaseAPI("/login/qr/check", map[string]string{"key": unikey, "timestamp": timestamp})
		if err == nil {
			codeFloat, _ := res["code"].(float64)
			code := int(codeFloat)
			cookieStr, _ := res["cookie"].(string)

			if BroadcastJSON != nil {
				BroadcastJSON(map[string]interface{}{
					"type":   "broadcast",
					"action": "netease_login_status",
					"data": map[string]interface{}{
						"key":     unikey,
						"code":    code,
						"message": res["message"],
					},
				})
			}

			if code == 803 && cookieStr != "" {
				_ = db.SaveSystemSetting("netease_cookie", cookieStr)
				logger.Info("网易云扫码授权成功, Cookie 已保存 (unikey=%s)", unikey)
				break
			}
			if code == 800 {
				logger.Info("网易云二维码已过期 (unikey=%s)", unikey)
				break
			}
		}

		time.Sleep(2 * time.Second)
	}
}

// GetNeteaseLoginStatus 真实验证登录状态
func GetNeteaseLoginStatus() (map[string]interface{}, error) {
	if config.GlobalConfig.NeteaseCookie == "" {
		return map[string]interface{}{"logged_in": false, "error": "未登录"}, nil
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
	res, err := CallNeteaseAPI("/login/status", map[string]string{"timestamp": timestamp})
	if err != nil {
		return nil, err
	}

	dataMap, _ := res["data"].(map[string]interface{})
	profile, _ := dataMap["profile"].(map[string]interface{})
	if profile != nil {
		return map[string]interface{}{
			"logged_in": true,
			"nickname":  profile["nickname"],
			"user_id":   profile["userId"],
			"avatar":    profile["avatarUrl"],
		}, nil
	}

	return map[string]interface{}{"logged_in": false, "error": "未登录"}, nil
}

// StartNeteaseDownload 发起网易云在线下载任务
func StartNeteaseDownload(songID, title, artist, album, level string) string {
	taskID := fmt.Sprintf("dl_%s_%d", songID, time.Now().UnixNano())

	task := &model.DownloadTask{
		TaskID:    taskID,
		SongID:    songID,
		Title:     title,
		Artist:    artist,
		Album:     album,
		Progress:  0,
		Status:    "pending",
		CreatedAt: float64(time.Now().UnixNano()) / 1e9,
	}

	downloadTasksMu.Lock()
	downloadTasks[taskID] = task
	downloadTasksMu.Unlock()

	go executeDownload(task, level)
	return taskID
}

// executeDownload 执行下载状态机流转
func executeDownload(task *model.DownloadTask, level string) {
	updateTaskStatus(task.TaskID, "preparing", 0, "")

	if level == "" {
		level = "lossless"
	}

	// 获取在线音频地址
	playURL := fetchNeteaseSongURL(task.SongID, level)
	if playURL == "" && level != "standard" {
		logger.Info("音质 %s 获取失败，自动回退到 standard 音质重试", level)
		playURL = fetchNeteaseSongURL(task.SongID, "standard")
	}

	if playURL == "" {
		updateTaskStatus(task.TaskID, "error", 0, "无法获取网易云音频播放地址，可能需要登录或无版权")
		return
	}

	// 流式下载临时文件
	updateTaskStatus(task.TaskID, "downloading", 5, "")

	ext := ".mp3"
	if level == "lossless" {
		ext = ".flac"
	}

	fileName := fmt.Sprintf("%s - %s%s", task.Artist, task.Title, ext)
	if task.Artist == "" {
		fileName = fmt.Sprintf("%s%s", task.Title, ext)
	}

	targetDir := config.GlobalConfig.NeteaseDownloadDir
	if targetDir == "" {
		targetDir = config.GlobalConfig.MusicLibraryPath
	}

	partPath := filepath.Join(targetDir, fileName+".part")
	finalPath := filepath.Join(targetDir, fileName)

	// 标记物理 Watchdog/fsnotify 忽略 (包含 .part 临时文件与最终目标路径)
	scanner.AddWatchdogIgnorePath(partPath)
	scanner.AddWatchdogIgnorePath(finalPath)

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

	var downloaded int64

	for {
		n, err := out.ReadFrom(io.LimitReader(resp.Body, 8192))
		downloaded += n
		if totalSize > 0 {
			prog := int(float64(downloaded) / float64(totalSize) * 90)
			updateTaskStatus(task.TaskID, "downloading", 5+prog, "")
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

	// 物理移除 .part
	if err := os.Rename(partPath, finalPath); err != nil {
		updateTaskStatus(task.TaskID, "error", 0, err.Error())
		return
	}

	// 物理单文件入库
	s := scanner.IndexSingleFile(finalPath)
	if s != nil && NotifyLibraryChanged != nil {
		NotifyLibraryChanged()
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
	}
	downloadTasksMu.Unlock()

	if exists && BroadcastJSON != nil {
		BroadcastJSON(map[string]interface{}{
			"type":   "broadcast",
			"action": "download_progress",
			"data":   t,
		})
	}
}

// fetchNeteaseSongURL 从本地/外置 NCM API 拉取在线音频链接
func fetchNeteaseSongURL(songID, level string) string {
	res, err := CallNeteaseAPI("/song/url/v1", map[string]string{"id": songID, "level": level})
	if err != nil {
		return ""
	}

	dataArr, ok := res["data"].([]interface{})
	if !ok || len(dataArr) == 0 {
		return ""
	}

	firstItem, ok := dataArr[0].(map[string]interface{})
	if !ok {
		return ""
	}

	playURL, _ := firstItem["url"].(string)
	return playURL
}
