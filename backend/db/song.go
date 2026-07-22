package db

import (
	"fmt"
	"os"

	"2fmusic/backend/core"
)

// GetAllSongs 获取去重后的全量歌曲列表
func GetAllSongs() ([]core.Song, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	rows, err := DB.Query(`SELECT id, path, filename, title, artist, album, album_artist, mtime, size, has_cover, has_lyrics, scrape_retry_count FROM songs ORDER BY title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	songs := make([]core.Song, 0)
	for rows.Next() {
		var s core.Song
		var hasCover, hasLyrics int
		if err := rows.Scan(&s.ID, &s.Path, &s.Filename, &s.Title, &s.Artist, &s.Album, &s.AlbumArtist, &s.MTime, &s.Size, &hasCover, &hasLyrics, &s.ScrapeRetryCount); err == nil {
			s.HasCover = hasCover == 1
			s.HasLyrics = hasLyrics == 1
			if s.HasCover {
				s.AlbumArt = fmt.Sprintf("/api/music/covers/%s.webp", s.ID)
			}
			songs = append(songs, s)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return songs, nil
}

// GetSongByID 根据 ID 查询歌曲
func GetSongByID(id string) (*core.Song, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	var s core.Song
	var hasCover, hasLyrics int
	err := DB.QueryRow(`SELECT id, path, filename, title, artist, album, album_artist, mtime, size, has_cover, has_lyrics, scrape_retry_count FROM songs WHERE id = ?`, id).Scan(
		&s.ID, &s.Path, &s.Filename, &s.Title, &s.Artist, &s.Album, &s.AlbumArtist, &s.MTime, &s.Size, &hasCover, &hasLyrics, &s.ScrapeRetryCount,
	)
	if err != nil {
		return nil, err
	}
	s.HasCover = hasCover == 1
	s.HasLyrics = hasLyrics == 1
	if s.HasCover {
		s.AlbumArt = fmt.Sprintf("/api/music/covers/%s.webp", s.ID)
	}
	return &s, nil
}

// GetSongByPath 根据绝对路径查询歌曲
func GetSongByPath(path string) (*core.Song, error) {
	path = core.NormalizePath(path)
	dbMu.RLock()
	defer dbMu.RUnlock()

	var s core.Song
	var hasCover, hasLyrics int
	err := DB.QueryRow(`SELECT id, path, filename, title, artist, album, album_artist, mtime, size, has_cover, has_lyrics, scrape_retry_count FROM songs WHERE path = ?`, path).Scan(
		&s.ID, &s.Path, &s.Filename, &s.Title, &s.Artist, &s.Album, &s.AlbumArtist, &s.MTime, &s.Size, &hasCover, &hasLyrics, &s.ScrapeRetryCount,
	)
	if err != nil {
		return nil, err
	}
	s.HasCover = hasCover == 1
	s.HasLyrics = hasLyrics == 1
	if s.HasCover {
		s.AlbumArt = fmt.Sprintf("/api/music/covers/%s.webp", s.ID)
	}
	return &s, nil
}

var (
	SongInsertedChan = make(chan *core.Song, 2000)
	SongDeletedChan  = make(chan string, 2000)
)

func NotifySongInserted(s *core.Song) {
	if s == nil || s.ID == "" {
		return
	}
	select {
	case SongInsertedChan <- s:
	default:
	}
}

func NotifySongDeleted(id string) {
	if id == "" {
		return
	}
	select {
	case SongDeletedChan <- id:
	default:
	}
}

// SaveSong 插入或更新歌曲
func SaveSong(s *core.Song) error {
	s.Path = core.NormalizePath(s.Path)
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
	if err == nil {
		NotifySongInserted(s)
	}
	return err
}

// DeleteSong 从数据库删除歌曲
func DeleteSong(id string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, _ = DB.Exec("DELETE FROM favorites WHERE song_id = ?", id)
	_, _ = DB.Exec("DELETE FROM play_history WHERE song_id = ?", id)
	_, err := DB.Exec("DELETE FROM songs WHERE id = ?", id)
	if err == nil {
		NotifySongDeleted(id)
	}
	return err
}

// DeleteSongByPath 根据物理路径删除歌曲及关联记录
func DeleteSongByPath(path string) error {
	path = core.NormalizePath(path)
	dbMu.Lock()
	defer dbMu.Unlock()

	var songID string
	err := DB.QueryRow("SELECT id FROM songs WHERE path = ?", path).Scan(&songID)
	if err == nil && songID != "" {
		_, _ = DB.Exec("DELETE FROM favorites WHERE song_id = ?", songID)
		_, _ = DB.Exec("DELETE FROM play_history WHERE song_id = ?", songID)
		_, err = DB.Exec("DELETE FROM songs WHERE id = ?", songID)
		if err == nil {
			NotifySongDeleted(songID)
		}
		return err
	}
	_, err = DB.Exec("DELETE FROM songs WHERE path = ?", path)
	return err
}

// DeleteSongsByPathPrefix 根据路径前缀批量从数据库删除歌曲（用于删除挂载点时清理）
func DeleteSongsByPathPrefix(prefix string) (int64, error) {
	prefix = core.NormalizePath(prefix)
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

// UpdateSongHasCover 单独更新封面状态
func UpdateSongHasCover(id string, hasCover bool) {
	dbMu.Lock()
	defer dbMu.Unlock()

	cVal := 0
	if hasCover {
		cVal = 1
	}
	_, _ = DB.Exec("UPDATE songs SET has_cover = ? WHERE id = ?", cVal, id)
}

// UpdateSongHasLyrics 单独更新歌词状态
func UpdateSongHasLyrics(id string, hasLyrics bool) {
	dbMu.Lock()
	defer dbMu.Unlock()

	lVal := 0
	if hasLyrics {
		lVal = 1
	}
	_, _ = DB.Exec("UPDATE songs SET has_lyrics = ? WHERE id = ?", lVal, id)
}

// CleanStaleSongs 清理磁盘已不存在的旧数据库记录
func CleanStaleSongs(validPaths map[string]bool) int {
	type songRef struct {
		id   string
		path string
	}

	dbMu.RLock()
	rows, err := DB.Query("SELECT id, path FROM songs")
	if err != nil {
		dbMu.RUnlock()
		return 0
	}

	var candidates []songRef
	for rows.Next() {
		var s songRef
		if err := rows.Scan(&s.id, &s.path); err == nil {
			candidates = append(candidates, s)
		}
	}
	_ = rows.Err()
	rows.Close()
	dbMu.RUnlock()

	var toDelete []string
	for _, c := range candidates {
		if !validPaths[c.path] {
			if _, err := os.Stat(c.path); os.IsNotExist(err) {
				toDelete = append(toDelete, c.id)
			}
		}
	}

	if len(toDelete) == 0 {
		return 0
	}

	dbMu.Lock()
	defer dbMu.Unlock()

	deletedCount := 0
	for _, id := range toDelete {
		_, _ = DB.Exec("DELETE FROM favorites WHERE song_id = ?", id)
		_, _ = DB.Exec("DELETE FROM play_history WHERE song_id = ?", id)
		if _, err := DB.Exec("DELETE FROM songs WHERE id = ?", id); err == nil {
			deletedCount++
		}
	}
	if deletedCount > 0 {
		core.Info("数据库已清理 %d 条失效记录", deletedCount)
	}
	return deletedCount
}
