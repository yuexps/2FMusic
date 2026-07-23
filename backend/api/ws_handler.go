package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"2fmusic/backend/core"
	"2fmusic/backend/db"
	"2fmusic/backend/downloader"
	"2fmusic/backend/scanner"
	"2fmusic/backend/utils"
)

// HandleWSAction 分发并响应所有 WebSocket 请求动作
func HandleWSAction(c *Client, req core.WSClientRequest) {
	seq := req.Seq
	action := req.Action
	data := req.Data

	switch action {
	case "music/get_list":
		songs, err := db.GetAllSongs()
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, songs)

	case "music/delete":
		songID, _ := data["song_id"].(string)
		if songID == "" {
			SendErrorResponse(c, seq, action, "Missing song_id")
			return
		}
		song, err := db.GetSongByID(songID)
		if err == nil && song != nil {
			targetPath := song.Path
			if !core.IsAudioFile(targetPath) {
				ext := filepath.Ext(targetPath)
				SendErrorResponse(c, seq, action, fmt.Sprintf("安全性约束：禁止物理删除 %s 类型的文件", ext))
				return
			}

			if err := utils.SafeRemoveFile(targetPath); err != nil {
				SendErrorResponse(c, seq, action, err.Error())
				return
			}

			core.Info("删除单曲: %s (ID=%s, Path=%s)", song.Title, songID, targetPath)
			_ = db.DeleteSong(songID)
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "music/clear_metadata":
		songID, _ := data["song_id"].(string)
		songPath, _ := data["path"].(string)

		var song *core.Song
		if songID != "" {
			song, _ = db.GetSongByID(songID)
		} else if songPath != "" {
			song, _ = db.GetSongByPath(songPath)
		}

		if song != nil {
			core.Info("清理单曲元数据与缓存: %s (ID=%s)", song.Title, song.ID)
			_ = utils.SafeRemoveFile(filepath.Join(core.GlobalConfig.CoversDir, song.ID+".webp"))
			_ = utils.SafeRemoveFile(filepath.Join(core.GlobalConfig.LyricsDir, song.ID+".lrc"))
			_ = utils.SafeRemoveFile(filepath.Join(core.GlobalConfig.LyricsDir, song.ID+".yrc"))
			db.UpdateSongMediaStatus(song.ID, false, false)
			NotifySongChangedDebounced(song.ID, "update", []string{"cover", "lyrics", "metadata"})
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "music/lyrics":
		songID, _ := data["song_id"].(string)
		title, _ := data["title"].(string)
		artist, _ := data["artist"].(string)
		filename, _ := data["filename"].(string)
		yrc, _ := data["yrc"].(bool)

		lrcContent, err := scanner.GetOrScrapeLyrics(songID, title, artist, filename, yrc)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, map[string]string{"lyrics": lrcContent})

	case "music/album-art":
		songID, _ := data["song_id"].(string)
		title, _ := data["title"].(string)
		artist, _ := data["artist"].(string)
		album, _ := data["album"].(string)

		coverURL, err := scanner.GetOrScrapeCover(songID, title, artist, album)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, map[string]string{"album_art": coverURL})

	case "music/scrape":
		title, _ := data["title"].(string)
		artist, _ := data["artist"].(string)
		album, _ := data["album"].(string)
		durationMs := 0
		if d, ok := data["duration_ms"].(float64); ok {
			durationMs = int(d)
		}

		best := scanner.SearchSongBest(title, artist, album, durationMs)
		if best == nil {
			SendErrorResponse(c, seq, action, "No matching result found")
			return
		}
		SendSuccessResponse(c, seq, action, best)

	case "favorite/list_playlists":
		playlists, err := db.GetFavoritePlaylists()
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		var result []core.FavoritePlaylistResponse
		for _, pl := range playlists {
			ids, _ := db.GetFavoritesByPlaylist(pl.ID)
			isDef := 0
			if pl.IsDefault {
				isDef = 1
			}
			result = append(result, core.FavoritePlaylistResponse{
				ID:        pl.ID,
				Name:      pl.Name,
				IsDefault: isDef,
				CreatedAt: pl.CreatedAt,
				SongCount: len(ids),
			})
		}
		SendSuccessResponse(c, seq, action, result)

	case "favorite/playlist_songs":
		playlistID, _ := data["playlist_id"].(string)
		if playlistID == "" {
			playlistID = "default"
		}
		songIDs, err := db.GetFavoritesByPlaylist(playlistID)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, songIDs)

	case "favorite/add":
		var songIDs []string
		if sID, ok := data["song_id"].(string); ok && sID != "" {
			songIDs = append(songIDs, sID)
		}
		if sIDsRaw, ok := data["song_ids"].([]interface{}); ok {
			for _, id := range sIDsRaw {
				if sID, ok := id.(string); ok && sID != "" {
					songIDs = append(songIDs, sID)
				}
			}
		}

		var playlistIDs []string
		if pID, ok := data["playlist_id"].(string); ok && pID != "" {
			playlistIDs = append(playlistIDs, pID)
		}
		if pIDsRaw, ok := data["playlist_ids"].([]interface{}); ok {
			for _, id := range pIDsRaw {
				if pID, ok := id.(string); ok && pID != "" {
					playlistIDs = append(playlistIDs, pID)
				}
			}
		}
		if len(playlistIDs) == 0 {
			playlistIDs = []string{"default"}
		}

		title, _ := data["title"].(string)
		artist, _ := data["artist"].(string)

		for _, pID := range playlistIDs {
			for _, sID := range songIDs {
				_ = db.AddFavorite(sID, pID, title, artist)
			}
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "favorite/delete":
		var songIDs []string
		if sID, ok := data["song_id"].(string); ok && sID != "" {
			songIDs = append(songIDs, sID)
		}
		if sIDsRaw, ok := data["song_ids"].([]interface{}); ok {
			for _, id := range sIDsRaw {
				if sID, ok := id.(string); ok && sID != "" {
					songIDs = append(songIDs, sID)
				}
			}
		}

		var playlistIDs []string
		if pID, ok := data["playlist_id"].(string); ok && pID != "" {
			playlistIDs = append(playlistIDs, pID)
		}
		if pIDsRaw, ok := data["playlist_ids"].([]interface{}); ok {
			for _, id := range pIDsRaw {
				if pID, ok := id.(string); ok && pID != "" {
					playlistIDs = append(playlistIDs, pID)
				}
			}
		}
		if len(playlistIDs) == 0 {
			playlistIDs = []string{"default"}
		}

		for _, pID := range playlistIDs {
			for _, sID := range songIDs {
				_ = db.RemoveFavorite(sID, pID)
			}
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "favorite/batch_move":
		songIDsRaw, _ := data["song_ids"].([]interface{})
		fromPL, _ := data["from_playlist_id"].(string)
		toPL, _ := data["to_playlist_id"].(string)
		for _, id := range songIDsRaw {
			if sID, ok := id.(string); ok {
				if fromPL != "" {
					_ = db.RemoveFavorite(sID, fromPL)
				}
				if toPL != "" {
					_ = db.AddFavorite(sID, toPL, "", "")
				}
			}
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "favorite/create_playlist":
		id, _ := data["id"].(string)
		name, _ := data["name"].(string)
		if name == "" {
			SendErrorResponse(c, seq, action, "Name is required")
			return
		}
		if id == "" {
			id = fmt.Sprintf("pl_%d", scanner.GetNowUnix())
		}
		err := db.CreateFavoritePlaylist(id, name)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, map[string]string{"id": id, "name": name})

	case "favorite/delete_playlist":
		id, _ := data["playlist_id"].(string)
		if id == "" {
			id, _ = data["id"].(string)
		}
		err := db.DeleteFavoritePlaylist(id)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "history/get":
		limit := 100
		if l, ok := data["limit"].(float64); ok {
			limit = int(l)
		}
		history, err := db.GetPlayHistory(limit)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		res := make([]core.PlayHistoryResponse, 0, len(history))
		for _, h := range history {
			msTime := int64(h.PlayTime * 1000)
			res = append(res, core.PlayHistoryResponse{
				Time: msTime,
				Song: h.Song,
			})
		}
		SendSuccessResponse(c, seq, action, res)

	case "history/add":
		songID, _ := data["song_id"].(string)
		if songID != "" {
			_ = db.AddPlayHistory(songID)
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "history/remove":
		songID, _ := data["song_id"].(string)
		playTime, _ := data["play_time"].(float64)
		if songID != "" {
			_ = db.DeletePlayHistoryItemWithTime(songID, playTime)
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "history/clear":
		_ = db.ClearPlayHistory()
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "mount/list":
		pts, err := db.GetMountPoints()
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		paths := make([]string, 0)
		for _, pt := range pts {
			if pt.Path != "" {
				paths = append(paths, pt.Path)
			}
		}
		SendSuccessResponse(c, seq, action, paths)

	case "mount/add":
		rawPath, _ := data["path"].(string)
		path := core.NormalizePath(rawPath)
		if path == "" {
			SendErrorResponse(c, seq, action, "Path required")
			return
		}
		err := db.AddMountPoint(path)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		core.Info("新增挂载点目录: %s", path)
		scanner.RefreshWatchPaths()
		scanner.TriggerScan()
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "mount/delete":
		rawPath, _ := data["path"].(string)
		path := core.NormalizePath(rawPath)
		if path == "" {
			SendErrorResponse(c, seq, action, "Path required")
			return
		}
		affected, _ := db.DeleteSongsByPathPrefix(path)
		err := db.DeleteMountPoint(path)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		core.Info("移除挂载点目录: %s (已清理 %d 条关联记录)", path, affected)
		NotifyLibraryChanged()
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "mount/scan":
		rawPath, _ := data["path"].(string)
		path := core.NormalizePath(rawPath)
		go scanner.ScanDirectorySingle(path)
		SendSuccessResponse(c, seq, action, map[string]string{"status": "scanning_started"})

	case "mount/retry_scrape":
		rawPath, _ := data["path"].(string)
		path := core.NormalizePath(rawPath)
		go scanner.AutoScrapeMissingMetadata(path)
		SendSuccessResponse(c, seq, action, map[string]string{"status": "rescrape_started"})

	case "system/get_status":
		musicCnt, _ := db.GetSongCount()
		plCnt, _ := db.GetFavoritePlaylistCount()
		SendSuccessResponse(c, seq, action, map[string]interface{}{
			"scanning":        scanner.IsScanning(),
			"is_scraping":     scanner.IsScraping(),
			"total":           scanner.GetTotalFiles(),
			"processed":       scanner.GetProcessedFiles(),
			"failed":          scanner.GetFailedFiles(),
			"current_file":    scanner.GetCurrentFile(),
			"current_path":    scanner.GetCurrentPath(),
			"music_count":     musicCnt,
			"playlist_count":  plCnt,
			"library_version": float64(time.Now().UnixNano()) / 1e9,
		})

	case "system/get_lyrics_preference":
		SendSuccessResponse(c, seq, action, map[string]string{
			"value": core.GlobalConfig.LyricsPreference,
		})

	case "system/save_lyrics_preference":
		val, _ := data["value"].(string)
		_ = db.SaveSystemSetting("lyrics_source_preference", val)
		core.GlobalConfig.LyricsPreference = val
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "system/get_settings":
		SendSuccessResponse(c, seq, action, map[string]string{
			"netease_cookie":           core.GlobalConfig.NeteaseCookie,
			"netease_download_dir":     core.GlobalConfig.NeteaseDownloadDir,
			"netease_api_base":         core.GlobalConfig.NeteaseAPIBase,
			"lyrics_source_preference": core.GlobalConfig.LyricsPreference,
		})

	case "system/set_settings":
		for k, v := range data {
			if strVal, ok := v.(string); ok {
				_ = db.SaveSystemSetting(k, strVal)
			}
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "system/scan_library":
		go scanner.TriggerScan()
		SendSuccessResponse(c, seq, action, map[string]string{"status": "scanning_started"})

	case "netease/get_config":
		SendSuccessResponse(c, seq, action, map[string]interface{}{
			"download_dir": core.GlobalConfig.NeteaseDownloadDir,
			"api_base":     core.GlobalConfig.NeteaseAPIBase,
		})

	case "netease/save_config":
		dir, _ := data["download_dir"].(string)
		api, hasAPI := data["api_base"].(string)

		if dir != "" {
			_ = db.SaveSystemSetting("netease_download_dir", dir)
			core.GlobalConfig.NeteaseDownloadDir = dir
			_ = os.MkdirAll(dir, 0755)
			scanner.RefreshWatchPaths() // 刷新 [Watcher] 监听，覆盖新下载目录
		}

		if hasAPI {
			api = strings.TrimRight(strings.TrimSpace(api), "/")
			if api != "" {
				testURL := fmt.Sprintf("%s/login/status", api)
				client := &http.Client{Timeout: 3 * time.Second}
				resp, err := client.Get(testURL)
				if err != nil {
					core.Warn("网易云 API 连通性测试失败 (url: %s): %v", api, err)
					SendErrorResponse(c, seq, action, "API 连接测试失败：请检查服务是否已启动且地址正确")
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode < 200 || resp.StatusCode >= 400 {
					SendErrorResponse(c, seq, action, "API 连接测试失败：服务响应状态异常")
					return
				}

				var testData map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&testData); err != nil {
					SendErrorResponse(c, seq, action, "API 校验失败：该地址未返回合法的 JSON 数据")
					return
				}

				_, hasCode := testData["code"]
				_, hasData := testData["data"]
				if !hasCode && !hasData {
					SendErrorResponse(c, seq, action, "API 校验失败：该地址未返回符合网易云 API 特征的数据")
					return
				}
			}

			_ = db.SaveSystemSetting("netease_api_base", api)
			core.GlobalConfig.NeteaseAPIBase = api
		}

		SendSuccessResponse(c, seq, action, map[string]interface{}{
			"download_dir": core.GlobalConfig.NeteaseDownloadDir,
			"api_base":     core.GlobalConfig.NeteaseAPIBase,
		})

	case "netease/check_container":
		res := downloader.CheckDockerContainer()
		SendSuccessResponse(c, seq, action, res)

	case "netease/install_service":
		ok, errMsg := downloader.InstallNeteaseDockerService()
		if !ok {
			SendErrorResponse(c, seq, action, errMsg)
			return
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "netease/install_status":
		res := downloader.GetDockerInstallStatus()
		SendSuccessResponse(c, seq, action, res)

	case "netease/clear_task":
		taskID, _ := data["task_id"].(string)
		downloader.ClearTask(taskID)
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "netease/clear_all_tasks":
		downloader.ClearAllTasks()
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "netease/resolve":
		inputStr, _ := data["input"].(string)
		res, err := downloader.ResolveNeteaseInput(inputStr)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, res)

	case "netease/recommend":
		recData, err := downloader.GetNeteaseRecommendSongs()
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, recData)

	case "netease/search":
		keywords, _ := data["keywords"].(string)
		limit := 30
		if lFloat, ok := data["limit"].(float64); ok && lFloat > 0 {
			limit = int(lFloat)
		}
		songs, err := downloader.SearchNeteaseCloud(keywords, limit)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, songs)

	case "netease/download":
		extractString := func(v interface{}) string {
			if v == nil {
				return ""
			}
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if strings.Contains(s, "e") || strings.Contains(s, "E") {
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						return fmt.Sprintf("%.0f", f)
					}
				}
				return s
			}
			if f, ok := v.(float64); ok {
				return fmt.Sprintf("%.0f", f)
			}
			s := fmt.Sprintf("%v", v)
			if strings.Contains(s, "e") || strings.Contains(s, "E") {
				if f, err := strconv.ParseFloat(s, 64); err == nil {
					return fmt.Sprintf("%.0f", f)
				}
			}
			return s
		}

		songID := extractString(data["song_id"])
		if songID == "" {
			songID = extractString(data["id"])
		}
		title := extractString(data["title"])
		artist := extractString(data["artist"])
		album := extractString(data["album"])
		level := extractString(data["level"])

		taskID := downloader.StartNeteaseDownload(songID, title, artist, album, level)
		SendSuccessResponse(c, seq, action, map[string]string{"task_id": taskID, "status": "pending"})

	case "netease/download_status":
		tasks := downloader.GetTasks()
		SendSuccessResponse(c, seq, action, tasks)

	case "netease/login_qrcode":
		unikey, qrimg, err := downloader.GetNeteaseQRKeyAndImage()
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, map[string]string{"unikey": unikey, "qrimg": qrimg})

	case "netease/qr_check":
		unikey, _ := data["key"].(string)
		if unikey == "" {
			SendErrorResponse(c, seq, action, "Missing key parameter")
			return
		}
		timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
		res, err := downloader.CallNeteaseAPI("/login/qr/check", map[string]string{"key": unikey, "timestamp": timestamp})
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, res)

	case "netease/login_status":
		statusMap, err := downloader.GetNeteaseLoginStatus()
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, statusMap)

	case "netease/logout":
		_, _ = downloader.CallNeteaseAPI("/logout", nil)
		_ = db.SaveSystemSetting("netease_cookie", "")
		core.GlobalConfig.NeteaseCookie = ""
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	default:
		SendErrorResponse(c, seq, action, fmt.Sprintf("Unknown action: %s", action))
	}
}
