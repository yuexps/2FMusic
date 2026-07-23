package scanner

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"2fmusic/backend/core"
	"2fmusic/backend/db"
	"2fmusic/backend/utils"
)

var (
	inFlightScrape   = make(map[string]bool)
	inFlightScrapeMu sync.Mutex
)

// InitDBWatcher 初始化数据库监听器 (DB_Watcher)
func InitDBWatcher() {
	go dbWatchLoop()
	go processPendingMediaExtraction()
}

// dbWatchLoop 监听数据库主键增删 Channel
func dbWatchLoop() {
	for {
		select {
		case song, ok := <-db.SongInsertedChan:
			if !ok {
				return
			}
			if song != nil && song.ID != "" {
				go handleSongInserted(song)
			}
		case songID, ok := <-db.SongDeletedChan:
			if !ok {
				return
			}
			if songID != "" {
				go handleSongDeleted(songID)
			}
		}
	}
}

// processPendingMediaExtraction 启动时扫描未完成封面/歌词提取的历史数据兜底
func processPendingMediaExtraction() {
	time.Sleep(3 * time.Second) // 充裕缓冲等待系统全面就绪
	songs, err := db.GetAllSongs()
	if err != nil {
		return
	}

	for _, s := range songs {
		if s.ScrapeRetryCount >= 3 {
			continue
		}
		if !s.HasCover || !s.HasLyrics {
			songRef := s
			go handleSongInserted(&songRef)
		}
	}
}

// handleSongInserted 处理新条目：提取/刮削封面与歌词落盘至 covers/ & lyrics/
func handleSongInserted(song *core.Song) {
	inFlightScrapeMu.Lock()
	if inFlightScrape[song.ID] {
		inFlightScrapeMu.Unlock()
		return
	}
	inFlightScrape[song.ID] = true
	inFlightScrapeMu.Unlock()

	defer func() {
		inFlightScrapeMu.Lock()
		delete(inFlightScrape, song.ID)
		inFlightScrapeMu.Unlock()
	}()

	if !EnsureSongMediaResolved(song) {
		db.IncrementScrapeRetryCount(song.ID)
	}
}

// handleSongDeleted 处理主键删除：物理联动清理磁盘上的 covers/ 和 lyrics/ 缓存文件
func handleSongDeleted(songID string) {
	coverPath := filepath.Join(core.GlobalConfig.CoversDir, songID+".webp")
	lrcPath := filepath.Join(core.GlobalConfig.LyricsDir, songID+".lrc")

	hasCoverFile := fileExists(coverPath)
	hasLyricsFile := fileExists(lrcPath)

	if hasCoverFile {
		_ = utils.SafeRemoveFile(coverPath)
		core.Info("[DB_Watcher] 清理封面缓存: %s.webp", songID)
	}
	if hasLyricsFile {
		_ = utils.SafeRemoveFile(lrcPath)
		core.Info("[DB_Watcher] 清理歌词缓存: %s.lrc", songID)
	}

	// 兜底：被删曲目若此前无任何物理缓存，LC_Watcher 不会被唤醒，显式广播通知
	if !hasCoverFile && !hasLyricsFile {
		if NotifyLibraryChanged != nil {
			NotifyLibraryChanged()
		}
	}
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
