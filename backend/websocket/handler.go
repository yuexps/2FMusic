package websocket

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"2fmusic/backend/config"
	"2fmusic/backend/db"
	"2fmusic/backend/downloader"
	"2fmusic/backend/model"
	"2fmusic/backend/scanner"
	"2fmusic/backend/searcher"
)

// HandleWSAction 分发并响应所有 WebSocket 请求动作
func HandleWSAction(c *Client, req model.WSClientRequest) {
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
			ext := strings.ToLower(filepath.Ext(targetPath))
			validAudioExts := map[string]bool{".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true, ".m4a": true}

			// 安全性约束：仅允许物理删除 6 种标准音频后缀的文件
			if !validAudioExts[ext] {
				SendErrorResponse(c, seq, action, fmt.Sprintf("安全性约束：禁止物理删除 %s 类型的文件", ext))
				return
			}

			// 重载重试机制应对 Windows 文件锁定占用 (最多重试 10 次，每次间隔 200ms)
			deletedSuccess := false
			for i := 0; i < 10; i++ {
				if err := os.Remove(targetPath); err == nil || os.IsNotExist(err) {
					deletedSuccess = true
					break
				}
				time.Sleep(200 * time.Millisecond)
			}

			if !deletedSuccess {
				SendErrorResponse(c, seq, action, "文件正被其他进程锁定，无法物理删除")
				return
			}

			// 清理同级关联附属文件
			basePath := strings.TrimSuffix(targetPath, filepath.Ext(targetPath))
			for _, subExt := range []string{".lrc", ".yrc", ".jpg", ".webp"} {
				_ = os.Remove(basePath + subExt)
			}
			// 清理集中缓存 covers/lyrics
			_ = os.Remove(filepath.Join(config.GlobalConfig.CoversDir, songID+".webp"))
			_ = os.Remove(filepath.Join(config.GlobalConfig.LyricsDir, songID+".lrc"))
			_ = os.Remove(filepath.Join(config.GlobalConfig.LyricsDir, songID+".yrc"))
			_ = db.DeleteSong(songID)
			NotifyLibraryChanged()
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "music/clear_metadata":
		songID, _ := data["song_id"].(string)
		songPath, _ := data["path"].(string)

		var song *model.Song
		if songID != "" {
			song, _ = db.GetSongByID(songID)
		} else if songPath != "" {
			song, _ = db.GetSongByPath(songPath)
		}

		if song != nil {
			_ = os.Remove(filepath.Join(config.GlobalConfig.CoversDir, song.ID+".webp"))
			_ = os.Remove(filepath.Join(config.GlobalConfig.LyricsDir, song.ID+".lrc"))
			_ = os.Remove(filepath.Join(config.GlobalConfig.LyricsDir, song.ID+".yrc"))
			db.UpdateSongMediaStatus(song.ID, false, false)
			NotifyLibraryChanged()
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

		best := searcher.SearchSongBest(title, artist, album)
		if best == nil {
			SendErrorResponse(c, seq, action, "No matching result found")
			return
		}
		SendSuccessResponse(c, seq, action, best)

	case "favorites/list", "favorite/list_playlists":
		playlists, err := db.GetFavoritePlaylists()
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		type PLWithSongs struct {
			model.FavoritePlaylist
			SongIDs []string `json:"song_ids"`
		}
		var result []PLWithSongs
		for _, pl := range playlists {
			ids, _ := db.GetFavoritesByPlaylist(pl.ID)
			result = append(result, PLWithSongs{
				FavoritePlaylist: pl,
				SongIDs:          ids,
			})
		}
		SendSuccessResponse(c, seq, action, result)

	case "favorite/playlist_songs", "favorites/playlist_songs":
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

	case "favorites/add", "favorite/add":
		songID, _ := data["song_id"].(string)
		playlistID, _ := data["playlist_id"].(string)
		if playlistID == "" {
			playlistID = "default"
		}
		title, _ := data["title"].(string)
		artist, _ := data["artist"].(string)
		err := db.AddFavorite(songID, playlistID, title, artist)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "favorites/remove", "favorite/delete":
		songID, _ := data["song_id"].(string)
		playlistID, _ := data["playlist_id"].(string)
		if playlistID == "" {
			playlistID = "default"
		}
		err := db.RemoveFavorite(songID, playlistID)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "favorite/batch_move", "favorites/batch_move":
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

	case "favorites/create_playlist", "favorite/create_playlist":
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

	case "favorites/delete_playlist", "favorite/delete_playlist":
		id, _ := data["id"].(string)
		err := db.DeleteFavoritePlaylist(id)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "history/list", "history/get":
		limit := 100
		if l, ok := data["limit"].(float64); ok {
			limit = int(l)
		}
		history, err := db.GetPlayHistory(limit)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		SendSuccessResponse(c, seq, action, history)

	case "history/add":
		songID, _ := data["song_id"].(string)
		if songID != "" {
			_ = db.AddPlayHistory(songID)
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "history/remove":
		songID, _ := data["song_id"].(string)
		if songID != "" {
			_ = db.DeletePlayHistoryItem(songID)
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "history/clear":
		_ = db.ClearPlayHistory()
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "mounts/list", "mount/list":
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

	case "mounts/add", "mount/add":
		rawPath, _ := data["path"].(string)
		path := config.NormalizePath(rawPath)
		if path == "" {
			SendErrorResponse(c, seq, action, "Path required")
			return
		}
		err := db.AddMountPoint(path)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		scanner.RefreshWatchPaths()
		scanner.TriggerScan()
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "mounts/delete", "mount/delete":
		rawPath, _ := data["path"].(string)
		path := config.NormalizePath(rawPath)
		if path == "" {
			SendErrorResponse(c, seq, action, "Path required")
			return
		}
		// 1. 物理删除 songs 表中该挂载目录下的歌曲关联记录
		_, _ = db.DeleteSongsByPathPrefix(path)
		// 2. 从 mount_points 表中移除挂载点
		err := db.DeleteMountPoint(path)
		if err != nil {
			SendErrorResponse(c, seq, action, err.Error())
			return
		}
		NotifyLibraryChanged()
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "mount/scan":
		rawPath, _ := data["path"].(string)
		path := config.NormalizePath(rawPath)
		go scanner.ScanDirectorySingle(path)
		SendSuccessResponse(c, seq, action, map[string]string{"status": "scanning_started"})

	case "mount/retry_scrape":
		rawPath, _ := data["path"].(string)
		path := config.NormalizePath(rawPath)
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
			"value": config.GlobalConfig.LyricsPreference,
		})

	case "system/save_lyrics_preference":
		val, _ := data["value"].(string)
		_ = db.SaveSystemSetting("lyrics_source_preference", val)
		config.GlobalConfig.LyricsPreference = val
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "system/get_settings":
		SendSuccessResponse(c, seq, action, map[string]string{
			"netease_cookie":           config.GlobalConfig.NeteaseCookie,
			"netease_download_dir":     config.GlobalConfig.NeteaseDownloadDir,
			"netease_api_base":         config.GlobalConfig.NeteaseAPIBase,
			"lyrics_source_preference": config.GlobalConfig.LyricsPreference,
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
			"download_dir": config.GlobalConfig.NeteaseDownloadDir,
			"api_base":     config.GlobalConfig.NeteaseAPIBase,
		})

	case "netease/save_config":
		dir, _ := data["download_dir"].(string)
		api, _ := data["api_base"].(string)
		if dir != "" {
			_ = db.SaveSystemSetting("netease_download_dir", dir)
			config.GlobalConfig.NeteaseDownloadDir = dir
		}
		if api != "" {
			_ = db.SaveSystemSetting("netease_api_base", api)
			config.GlobalConfig.NeteaseAPIBase = api
		}
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "netease/check_container", "netease/install_status":
		SendSuccessResponse(c, seq, action, map[string]interface{}{
			"running": true,
			"status":  "success",
		})

	case "netease/install_service":
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "netease/clear_task":
		taskID, _ := data["task_id"].(string)
		downloader.ClearTask(taskID)
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	case "netease/search":
		keywords, _ := data["keywords"].(string)
		best := searcher.SearchSongBest(keywords, "", "")
		if best != nil {
			SendSuccessResponse(c, seq, action, []interface{}{best})
		} else {
			SendSuccessResponse(c, seq, action, []interface{}{})
		}

	case "netease/download":
		songID, _ := data["song_id"].(string)
		if songID == "" {
			if idFloat, ok := data["id"].(float64); ok {
				songID = fmt.Sprintf("%.0f", idFloat)
			}
		}
		title, _ := data["title"].(string)
		artist, _ := data["artist"].(string)
		album, _ := data["album"].(string)
		level, _ := data["level"].(string)

		taskID := downloader.StartNeteaseDownload(songID, title, artist, album, level)
		SendSuccessResponse(c, seq, action, map[string]string{"task_id": taskID, "status": "pending"})

	case "netease/download_status":
		tasks := downloader.GetTasks()
		SendSuccessResponse(c, seq, action, tasks)

	case "netease/qr_key", "netease/login_qrcode":
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
		SendSuccessResponse(c, seq, action, map[string]bool{"success": true})

	default:
		SendErrorResponse(c, seq, action, fmt.Sprintf("Unknown action: %s", action))
	}
}
