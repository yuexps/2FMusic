package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"2fmusic/backend/config"
	"2fmusic/backend/logger"
	"2fmusic/backend/model"

	_ "modernc.org/sqlite"
)

var (
	DB   *sql.DB
	dbMu sync.RWMutex
)

// InitDB 初始化 SQLite 数据库与 WAL 模式
func InitDB() error {
	dbPath := config.GlobalConfig.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(config.GlobalConfig.DataDir, "2fmusic.db")
	}
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	// modernc.org/sqlite dsn 参数: _pragma=journal_mode(WAL)&_pragma=busy_timeout(30000)
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(30000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite db error: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(10 * time.Minute)

	DB = db

	if err := createTables(); err != nil {
		return err
	}

	LoadSystemSettings()
	return nil
}

// createTables 执行 DDL 语句
func createTables() error {
	ddls := []string{
		`CREATE TABLE IF NOT EXISTS songs (
			id TEXT PRIMARY KEY,
			path TEXT UNIQUE,
			filename TEXT,
			title TEXT,
			artist TEXT,
			album TEXT,
			album_artist TEXT,
			mtime REAL,
			size INTEGER,
			has_cover INTEGER DEFAULT 0,
			has_lyrics INTEGER DEFAULT 0,
			scrape_retry_count INTEGER DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS favorite_playlists (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			is_default INTEGER DEFAULT 0,
			created_at REAL
		);`,
		`CREATE TABLE IF NOT EXISTS favorites (
			song_id TEXT,
			playlist_id TEXT,
			title TEXT DEFAULT '',
			artist TEXT DEFAULT '',
			created_at REAL,
			PRIMARY KEY (song_id, playlist_id)
		);`,
		`CREATE TABLE IF NOT EXISTS play_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			song_id TEXT NOT NULL,
			play_time REAL NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS mount_points (
			path TEXT PRIMARY KEY,
			created_at REAL
		);`,
		`CREATE TABLE IF NOT EXISTS system_settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
	}

	for _, ddl := range ddls {
		if _, err := DB.Exec(ddl); err != nil {
			return fmt.Errorf("exec ddl error: %v", err)
		}
	}

	// 初始化默认收藏夹
	var defaultCount int
	_ = DB.QueryRow("SELECT COUNT(*) FROM favorite_playlists WHERE id = 'default'").Scan(&defaultCount)
	if defaultCount == 0 {
		now := float64(time.Now().UnixNano()) / 1e9
		_, _ = DB.Exec("INSERT INTO favorite_playlists (id, name, is_default, created_at) VALUES ('default', '默认收藏夹', 1, ?)", now)
	}

	return nil
}

// LoadSystemSettings 提取数据库中的底层配置到内存
func LoadSystemSettings() {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := DB.Query("SELECT key, value FROM system_settings")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			switch k {
			case "netease_cookie":
				config.GlobalConfig.NeteaseCookie = v
			case "netease_download_dir":
				if v != "" {
					config.GlobalConfig.NeteaseDownloadDir = v
				}
			case "netease_api_base":
				if v != "" {
					config.GlobalConfig.NeteaseAPIBase = v
				}
			case "lyrics_source_preference":
				if v != "" {
					config.GlobalConfig.LyricsPreference = v
				}
			}
		}
	}
	_ = rows.Err()
}

// GetSystemSetting 查询系统偏好设置
func GetSystemSetting(key, defaultValue string) string {
	dbMu.RLock()
	defer dbMu.RUnlock()

	var val string
	err := DB.QueryRow("SELECT value FROM system_settings WHERE key = ?", key).Scan(&val)
	if err != nil || val == "" {
		return defaultValue
	}
	return val
}

// SaveSystemSetting 保存单个系统配置
func SaveSystemSetting(key, value string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?", key, value, value)
	if err == nil {
		switch key {
		case "netease_cookie":
			config.GlobalConfig.NeteaseCookie = value
		case "netease_download_dir":
			config.GlobalConfig.NeteaseDownloadDir = value
		case "netease_api_base":
			config.GlobalConfig.NeteaseAPIBase = value
		case "lyrics_source_preference":
			config.GlobalConfig.LyricsPreference = value
		}
	}
	return err
}

// GetAllSongs 获取去重后的全量歌曲列表
func GetAllSongs() ([]model.Song, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	query := `SELECT id, path, filename, title, artist, album, album_artist, mtime, size, has_cover, has_lyrics, scrape_retry_count FROM songs ORDER BY title ASC`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	songs := make([]model.Song, 0)
	for rows.Next() {
		var s model.Song
		var hasCover, hasLyrics int
		err := rows.Scan(&s.ID, &s.Path, &s.Filename, &s.Title, &s.Artist, &s.Album, &s.AlbumArtist, &s.MTime, &s.Size, &hasCover, &hasLyrics, &s.ScrapeRetryCount)
		if err != nil {
			continue
		}
		s.HasCover = (hasCover == 1)
		s.HasLyrics = (hasLyrics == 1)
		if s.HasCover {
			s.AlbumArt = fmt.Sprintf("/api/music/covers/%s.webp", s.ID)
		}
		songs = append(songs, s)
	}
	_ = rows.Err()
	return songs, nil
}

// GetSongByID 获取单首歌曲
func GetSongByID(id string) (*model.Song, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	query := `SELECT id, path, filename, title, artist, album, album_artist, mtime, size, has_cover, has_lyrics, scrape_retry_count FROM songs WHERE id = ?`
	var s model.Song
	var hasCover, hasLyrics int
	err := DB.QueryRow(query, id).Scan(&s.ID, &s.Path, &s.Filename, &s.Title, &s.Artist, &s.Album, &s.AlbumArtist, &s.MTime, &s.Size, &hasCover, &hasLyrics, &s.ScrapeRetryCount)
	if err != nil {
		return nil, err
	}
	s.HasCover = (hasCover == 1)
	s.HasLyrics = (hasLyrics == 1)
	if s.HasCover {
		s.AlbumArt = fmt.Sprintf("/api/music/covers/%s.webp", s.ID)
	}
	return &s, nil
}

// GetSongByPath 根据路径获取歌曲
func GetSongByPath(path string) (*model.Song, error) {
	path = config.NormalizePath(path)
	dbMu.RLock()
	defer dbMu.RUnlock()

	query := `SELECT id, path, filename, title, artist, album, album_artist, mtime, size, has_cover, has_lyrics, scrape_retry_count FROM songs WHERE path = ?`
	var s model.Song
	var hasCover, hasLyrics int
	err := DB.QueryRow(query, path).Scan(&s.ID, &s.Path, &s.Filename, &s.Title, &s.Artist, &s.Album, &s.AlbumArtist, &s.MTime, &s.Size, &hasCover, &hasLyrics, &s.ScrapeRetryCount)
	if err != nil {
		return nil, err
	}
	s.HasCover = (hasCover == 1)
	s.HasLyrics = (hasLyrics == 1)
	if s.HasCover {
		s.AlbumArt = fmt.Sprintf("/api/music/covers/%s.webp", s.ID)
	}
	return &s, nil
}

// SaveSong 插入或更新歌曲
func SaveSong(s *model.Song) error {
	s.Path = config.NormalizePath(s.Path)
	dbMu.Lock()
	defer dbMu.Unlock()

	hasCoverInt := 0
	if s.HasCover {
		hasCoverInt = 1
	}
	hasLyricsInt := 0
	if s.HasLyrics {
		hasLyricsInt = 1
	}

	query := `INSERT INTO songs (id, path, filename, title, artist, album, album_artist, mtime, size, has_cover, has_lyrics, scrape_retry_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			id = excluded.id,
			filename = excluded.filename,
			title = excluded.title,
			artist = excluded.artist,
			album = excluded.album,
			album_artist = excluded.album_artist,
			mtime = excluded.mtime,
			size = excluded.size,
			has_cover = excluded.has_cover,
			has_lyrics = excluded.has_lyrics,
			scrape_retry_count = excluded.scrape_retry_count`

	_, err := DB.Exec(query, s.ID, s.Path, s.Filename, s.Title, s.Artist, s.Album, s.AlbumArtist, s.MTime, s.Size, hasCoverInt, hasLyricsInt, s.ScrapeRetryCount)
	return err
}

// DeleteSong 从数据库删除歌曲
func DeleteSong(id string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, _ = DB.Exec("DELETE FROM favorites WHERE song_id = ?", id)
	_, _ = DB.Exec("DELETE FROM play_history WHERE song_id = ?", id)
	_, err := DB.Exec("DELETE FROM songs WHERE id = ?", id)
	return err
}

// DeleteSongByPath 根据物理路径删除歌曲及关联记录
func DeleteSongByPath(path string) error {
	path = config.NormalizePath(path)
	dbMu.Lock()
	defer dbMu.Unlock()

	var songID string
	err := DB.QueryRow("SELECT id FROM songs WHERE path = ?", path).Scan(&songID)
	if err == nil && songID != "" {
		_, _ = DB.Exec("DELETE FROM favorites WHERE song_id = ?", songID)
		_, _ = DB.Exec("DELETE FROM play_history WHERE song_id = ?", songID)
		_, err = DB.Exec("DELETE FROM songs WHERE id = ?", songID)
		return err
	}
	_, err = DB.Exec("DELETE FROM songs WHERE path = ?", path)
	return err
}

// DeleteSongsByPathPrefix 根据路径前缀批量从数据库删除歌曲（用于删除挂载点时清理）
func DeleteSongsByPathPrefix(prefix string) (int64, error) {
	prefix = config.NormalizePath(prefix)
	dbMu.Lock()
	defer dbMu.Unlock()

	// 查出将被删除的歌曲 ID 列表以清理关联表
	rows, err := DB.Query("SELECT id FROM songs WHERE path LIKE ? || '%'", prefix)
	if err == nil {
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		_ = rows.Err()
		rows.Close()
		for _, id := range ids {
			_, _ = DB.Exec("DELETE FROM favorites WHERE song_id = ?", id)
			_, _ = DB.Exec("DELETE FROM play_history WHERE song_id = ?", id)
		}
	}

	res, err := DB.Exec("DELETE FROM songs WHERE path LIKE ? || '%'", prefix)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

// IncrementScrapeRetryCount 重试次数自增 1
func IncrementScrapeRetryCount(id string) {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, _ = DB.Exec("UPDATE songs SET scrape_retry_count = scrape_retry_count + 1 WHERE id = ?", id)
}

// UpdateSongMediaStatus 更新封面与歌词状态
func UpdateSongMediaStatus(id string, hasCover, hasLyrics bool) {
	dbMu.Lock()
	defer dbMu.Unlock()

	cVal, lVal := 0, 0
	if hasCover {
		cVal = 1
	}
	if hasLyrics {
		lVal = 1
	}

	_, _ = DB.Exec("UPDATE songs SET has_cover = ?, has_lyrics = ? WHERE id = ?", cVal, lVal, id)
}

// GetMountPoints 获取所有挂载点路径
func GetMountPoints() ([]model.MountPoint, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := DB.Query("SELECT path, created_at FROM mount_points ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pts := make([]model.MountPoint, 0)
	for rows.Next() {
		var pt model.MountPoint
		if err := rows.Scan(&pt.Path, &pt.CreatedAt); err == nil {
			pts = append(pts, pt)
		}
	}
	_ = rows.Err()
	return pts, nil
}

// AddMountPoint 添加挂载点
func AddMountPoint(path string) error {
	path = config.NormalizePath(path)
	dbMu.Lock()
	defer dbMu.Unlock()

	now := float64(time.Now().UnixNano()) / 1e9
	_, err := DB.Exec("INSERT INTO mount_points (path, created_at) VALUES (?, ?) ON CONFLICT(path) DO NOTHING", path, now)
	return err
}

// DeleteMountPoint 删除挂载点
func DeleteMountPoint(path string) error {
	path = config.NormalizePath(path)
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("DELETE FROM mount_points WHERE path = ?", path)
	return err
}

// GetFavoritePlaylists 获取歌单列表
func GetFavoritePlaylists() ([]model.FavoritePlaylist, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := DB.Query("SELECT id, name, is_default, created_at FROM favorite_playlists ORDER BY is_default DESC, created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	playlists := make([]model.FavoritePlaylist, 0)
	for rows.Next() {
		var pl model.FavoritePlaylist
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
func GetPlayHistory(limit int) ([]model.PlayHistory, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := DB.Query("SELECT id, song_id, play_time FROM play_history ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := make([]model.PlayHistory, 0)
	for rows.Next() {
		var h model.PlayHistory
		if err := rows.Scan(&h.ID, &h.SongID, &h.PlayTime); err == nil {
			if s, err := GetSongByID(h.SongID); err == nil {
				h.Song = s
			}
			history = append(history, h)
		}
	}
	_ = rows.Err()
	return history, nil
}

// DeletePlayHistoryItem 删除单条播放历史
func DeletePlayHistoryItem(songID string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("DELETE FROM play_history WHERE song_id = ?", songID)
	return err
}

// GetSongCount 获取全库歌曲总数
func GetSongCount() (int, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM songs").Scan(&count)
	return count, err
}

// GetFavoritePlaylistCount 获取收藏歌单总数
func GetFavoritePlaylistCount() (int, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM favorite_playlists").Scan(&count)
	return count, err
}

// ClearPlayHistory 清空播放历史
func ClearPlayHistory() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := DB.Exec("DELETE FROM play_history")
	return err
}

// CleanStaleSongs 清理磁盘已不存在的旧数据库记录
func CleanStaleSongs(validPaths map[string]bool) int {
	dbMu.Lock()
	defer dbMu.Unlock()

	rows, err := DB.Query("SELECT id, path FROM songs")
	if err != nil {
		return 0
	}
	defer rows.Close()

	var toDelete []string
	for rows.Next() {
		var id, p string
		if err := rows.Scan(&id, &p); err == nil {
			if !validPaths[p] {
				if _, err := os.Stat(p); os.IsNotExist(err) {
					toDelete = append(toDelete, id)
				}
			}
		}
	}
	_ = rows.Err()

	deletedCount := 0
	for _, id := range toDelete {
		_, _ = DB.Exec("DELETE FROM favorites WHERE song_id = ?", id)
		_, _ = DB.Exec("DELETE FROM play_history WHERE song_id = ?", id)
		if _, err := DB.Exec("DELETE FROM songs WHERE id = ?", id); err == nil {
			deletedCount++
		}
	}
	if deletedCount > 0 {
		logger.Info("数据库已清理 %d 条失效物理文件记录", deletedCount)
	}
	return deletedCount
}
