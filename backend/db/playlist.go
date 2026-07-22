package db

import (
	"database/sql"
	"fmt"
	"time"

	"2fmusic/backend/core"
)

// GetMountPoints 获取所有挂载点路径
func GetMountPoints() ([]core.MountPoint, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := DB.Query("SELECT path, created_at FROM mount_points ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pts := make([]core.MountPoint, 0)
	for rows.Next() {
		var pt core.MountPoint
		if err := rows.Scan(&pt.Path, &pt.CreatedAt); err == nil {
			pts = append(pts, pt)
		}
	}
	_ = rows.Err()
	return pts, nil
}

// AddMountPoint 添加挂载点
func AddMountPoint(path string) error {
	path = core.NormalizePath(path)
	dbMu.Lock()
	defer dbMu.Unlock()

	now := float64(time.Now().UnixNano()) / 1e9
	_, err := DB.Exec("INSERT INTO mount_points (path, created_at) VALUES (?, ?) ON CONFLICT(path) DO NOTHING", path, now)
	return err
}

// DeleteMountPoint 删除挂载点
func DeleteMountPoint(path string) error {
	path = core.NormalizePath(path)
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("DELETE FROM mount_points WHERE path = ?", path)
	return err
}

// GetFavoritePlaylists 获取歌单列表
func GetFavoritePlaylists() ([]core.FavoritePlaylist, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := DB.Query("SELECT id, name, is_default, created_at FROM favorite_playlists ORDER BY is_default DESC, created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	playlists := make([]core.FavoritePlaylist, 0)
	for rows.Next() {
		var pl core.FavoritePlaylist
		var isDef int
		if err := rows.Scan(&pl.ID, &pl.Name, &isDef, &pl.CreatedAt); err == nil {
			pl.IsDefault = (isDef == 1)
			playlists = append(playlists, pl)
		}
	}
	_ = rows.Err()
	return playlists, nil
}

// GetFavoritesByPlaylist 获取指定歌单下包含的歌曲ID列表
func GetFavoritesByPlaylist(playlistID string) ([]string, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := DB.Query("SELECT song_id FROM favorites WHERE playlist_id = ? ORDER BY created_at DESC", playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	songIDs := make([]string, 0)
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err == nil {
			songIDs = append(songIDs, sid)
		}
	}
	_ = rows.Err()
	return songIDs, nil
}

// AddFavorite 向歌单添加歌曲
func AddFavorite(songID, playlistID, title, artist string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	now := float64(time.Now().UnixNano()) / 1e9
	_, err := DB.Exec("INSERT INTO favorites (song_id, playlist_id, title, artist, created_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(song_id, playlist_id) DO NOTHING", songID, playlistID, title, artist, now)
	return err
}

// RemoveFavorite 从歌单移除歌曲
func RemoveFavorite(songID, playlistID string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("DELETE FROM favorites WHERE song_id = ? AND playlist_id = ?", songID, playlistID)
	return err
}

// CreateFavoritePlaylist 新建歌单
func CreateFavoritePlaylist(id, name string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	now := float64(time.Now().UnixNano()) / 1e9
	_, err := DB.Exec("INSERT INTO favorite_playlists (id, name, is_default, created_at) VALUES (?, ?, 0, ?)", id, name, now)
	return err
}

// DeleteFavoritePlaylist 删除歌单
func DeleteFavoritePlaylist(id string) error {
	if id == "default" {
		return fmt.Errorf("cannot delete default playlist")
	}

	dbMu.Lock()
	defer dbMu.Unlock()

	_, _ = DB.Exec("DELETE FROM favorites WHERE playlist_id = ?", id)
	_, err := DB.Exec("DELETE FROM favorite_playlists WHERE id = ?", id)
	return err
}

// AddPlayHistory 添加播放历史
func AddPlayHistory(songID string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	now := float64(time.Now().UnixNano()) / 1e9
	_, err := DB.Exec("INSERT INTO play_history (song_id, play_time) VALUES (?, ?)", songID, now)
	return err
}

// GetPlayHistory 获取播放历史
func GetPlayHistory(limit int) ([]core.PlayHistory, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT 
			ph.id, ph.song_id, ph.play_time,
			s.id, s.path, s.filename, s.title, s.artist, s.album, s.album_artist, s.mtime, s.size, s.has_cover, s.has_lyrics, s.scrape_retry_count
		FROM play_history ph
		LEFT JOIN songs s ON ph.song_id = s.id
		ORDER BY ph.id DESC LIMIT ?`

	rows, err := DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := make([]core.PlayHistory, 0)
	for rows.Next() {
		var h core.PlayHistory
		var sID, sPath, sFilename, sTitle, sArtist, sAlbum, sAlbumArtist sql.NullString
		var sMtime sql.NullFloat64
		var sSize sql.NullInt64
		var sHasCover, sHasLyrics, sScrapeRetry sql.NullInt64

		err := rows.Scan(
			&h.ID, &h.SongID, &h.PlayTime,
			&sID, &sPath, &sFilename, &sTitle, &sArtist, &sAlbum, &sAlbumArtist, &sMtime, &sSize, &sHasCover, &sHasLyrics, &sScrapeRetry,
		)
		if err == nil {
			if sID.Valid && sID.String != "" {
				song := &core.Song{
					ID:               sID.String,
					Path:             sPath.String,
					Filename:         sFilename.String,
					Title:            sTitle.String,
					Artist:           sArtist.String,
					Album:            sAlbum.String,
					AlbumArtist:      sAlbumArtist.String,
					MTime:            sMtime.Float64,
					Size:             sSize.Int64,
					HasCover:         sHasCover.Int64 == 1,
					HasLyrics:        sHasLyrics.Int64 == 1,
					ScrapeRetryCount: int(sScrapeRetry.Int64),
				}
				if song.HasCover {
					song.AlbumArt = fmt.Sprintf("/api/music/covers/%s.webp", song.ID)
				}
				h.Song = song
			}
			history = append(history, h)
		}
	}
	_ = rows.Err()
	return history, nil
}

// DeletePlayHistoryItem 删除歌曲播放历史
func DeletePlayHistoryItem(songID string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("DELETE FROM play_history WHERE song_id = ?", songID)
	return err
}

// DeletePlayHistoryItemWithTime 根据歌曲与时间戳删除单条播放历史
func DeletePlayHistoryItemWithTime(songID string, playTime float64) error {
	if playTime <= 0 {
		return DeletePlayHistoryItem(songID)
	}

	if playTime > 1e11 {
		playTime = playTime / 1000.0
	}

	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("DELETE FROM play_history WHERE song_id = ? AND ABS(play_time - ?) < 0.5", songID, playTime)
	return err
}

// ClearPlayHistory 清空播放历史
func ClearPlayHistory() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("DELETE FROM play_history")
	return err
}
