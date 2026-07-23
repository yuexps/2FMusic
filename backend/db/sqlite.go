package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"2fmusic/backend/core"

	_ "modernc.org/sqlite"
)

var (
	DB   *sql.DB
	dbMu sync.RWMutex
)

// InitDB 初始化 SQLite 数据库与 WAL 模式
func InitDB() error {
	dbPath := core.GlobalConfig.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(core.GlobalConfig.DataDir, "2fmusic.db")
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
			duration_ms INTEGER DEFAULT 0,
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
				core.GlobalConfig.NeteaseCookie = v
			case "netease_download_dir":
				if v != "" {
					core.GlobalConfig.NeteaseDownloadDir = v
				}
			case "netease_api_base":
				if v != "" {
					core.GlobalConfig.NeteaseAPIBase = v
				}
			case "lyrics_source_preference":
				if v != "" {
					core.GlobalConfig.LyricsPreference = v
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
			core.GlobalConfig.NeteaseCookie = value
		case "netease_download_dir":
			core.GlobalConfig.NeteaseDownloadDir = value
		case "netease_api_base":
			core.GlobalConfig.NeteaseAPIBase = value
		case "lyrics_source_preference":
			core.GlobalConfig.LyricsPreference = value
		}

		keyName := key
		logVal := value

		switch key {
		case "lyrics_source_preference":
			keyName = "歌词来源偏好"
			switch value {
			case "network":
				logVal = "网络优先 (network)"
			case "embedded":
				logVal = "本地内嵌优先 (embedded)"
			}
		case "netease_api_base":
			keyName = "网易云 API 地址"
			if value == "" {
				logVal = "[未配置]"
			}
		case "netease_download_dir":
			keyName = "网易云下载目录"
			if value == "" {
				logVal = "[未配置]"
			}
		case "netease_cookie":
			keyName = "网易云 Cookie"
			if value == "" {
				logVal = "[已清除]"
			} else if len(value) > 10 {
				logVal = value[:10] + "..."
			}
		}

		core.Info("更新系统配置 [%s]: %s", keyName, logVal)
	}
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
