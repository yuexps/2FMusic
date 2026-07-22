package model

// Song 歌曲元数据模型
type Song struct {
	ID               string  `json:"id"`
	Path             string  `json:"path,omitempty"`
	Filename         string  `json:"filename"`
	Title            string  `json:"title"`
	Artist           string  `json:"artist"`
	Album            string  `json:"album"`
	AlbumArtist      string  `json:"album_artist,omitempty"`
	MTime            float64 `json:"mtime"`
	Size             int64   `json:"size"`
	HasCover         bool    `json:"has_cover"`
	HasLyrics        bool    `json:"has_lyrics"`
	ScrapeRetryCount int     `json:"scrape_retry_count,omitempty"`
	AlbumArt         string  `json:"album_art,omitempty"`
}

// FavoritePlaylist 歌单模型
type FavoritePlaylist struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	IsDefault bool    `json:"is_default"`
	CreatedAt float64 `json:"created_at"`
}

// Favorite 歌单关联歌曲
type Favorite struct {
	SongID     string  `json:"song_id"`
	PlaylistID string  `json:"playlist_id"`
	Title      string  `json:"title"`
	Artist     string  `json:"artist"`
	CreatedAt  float64 `json:"created_at"`
}

// PlayHistory 播放历史
type PlayHistory struct {
	ID       int64   `json:"id"`
	SongID   string  `json:"song_id"`
	PlayTime float64 `json:"play_time"`
	Song     *Song   `json:"song,omitempty"`
}

// MountPoint 挂载点模型
type MountPoint struct {
	Path      string  `json:"path"`
	CreatedAt float64 `json:"created_at"`
}

// SystemSetting 系统偏好
type SystemSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WSClientRequest WS 客户端请求帧
type WSClientRequest struct {
	Seq    interface{}            `json:"seq"`
	Action string                 `json:"action"`
	Data   map[string]interface{} `json:"data"`
}

// WSResponseFrame WS 服务端响应帧
type WSResponseFrame struct {
	Seq     interface{} `json:"seq,omitempty"`
	Type    string      `json:"type"`
	Action  string      `json:"action,omitempty"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// DownloadTask 下载任务模型
type DownloadTask struct {
	TaskID    string  `json:"task_id"`
	SongID    string  `json:"song_id"`
	Title     string  `json:"title"`
	Artist    string  `json:"artist"`
	Album     string  `json:"album"`
	PicURL    string  `json:"pic_url"`
	Progress  int     `json:"progress"`
	Status    string  `json:"status"` // pending, preparing, downloading, success, error
	Error     string  `json:"error"`
	CreatedAt float64 `json:"created_at"`
}
