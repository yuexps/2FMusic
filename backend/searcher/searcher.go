package searcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/logger"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

type searchResult struct {
	title        string
	artist       string
	album        string
	lyrics       string
	cover        string
	source       string
	platformRank int
}

// DownloadImageBytes 通用下载图片字节流
func DownloadImageBytes(imgURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// StringSimilarity 相似度计算 (0.0 ~ 1.0)
func StringSimilarity(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	if s1 == s2 {
		return 1.0
	}
	if s1 == "" || s2 == "" {
		return 0.0
	}

	// 包含关系的基础打分
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return 0.85
	}

	r1, r2 := []rune(s1), []rune(s2)
	len1, len2 := len(r1), len(r2)

	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}
			minVal := matrix[i-1][j] + 1
			if matrix[i][j-1]+1 < minVal {
				minVal = matrix[i][j-1] + 1
			}
			if matrix[i-1][j-1]+cost < minVal {
				minVal = matrix[i-1][j-1] + cost
			}
			matrix[i][j] = minVal
		}
	}

	dist := matrix[len1][len2]
	maxLen := len1
	if len2 > maxLen {
		maxLen = len2
	}

	return 1.0 - (float64(dist) / float64(maxLen))
}

// SearchSongBest 并发检索 QQ/网易云/酷狗 返回最佳匹配
func SearchSongBest(title, artist, album string) map[string]interface{} {
	if title == "" {
		return nil
	}

	var results []searchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	sources := []string{"netease", "qq", "kugou"}

	for _, src := range sources {
		wg.Add(1)
		go func(source string) {
			defer wg.Done()
			var res []searchResult

			switch source {
			case "netease":
				res = searchNetease(title, artist)
			case "qq":
				res = searchQQ(title, artist)
			case "kugou":
				res = searchKugou(title, artist)
			}

			mu.Lock()
			results = append(results, res...)
			mu.Unlock()
		}(src)
	}

	// 限制并发等待最长 6.0 秒
	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()

	select {
	case <-c:
	case <-time.After(6 * time.Second):
		logger.Info("刮削搜索超时 (6s)，基于已返回的结果打分排序")
	}

	if len(results) == 0 {
		return nil
	}

	type scoredItem struct {
		score float64
		item  searchResult
	}

	var scored []scoredItem

	apiBonus := map[string]float64{"qq": 0.01, "netease": 0.005, "kugou": 0.0}

	for _, item := range results {
		titleScore := StringSimilarity(title, item.title)
		artistScore := 1.0
		if artist != "" {
			artistScore = StringSimilarity(artist, item.artist)
		}
		albumScore := 1.0
		if album != "" {
			albumScore = StringSimilarity(album, item.album)
		}

		score := 0.5*titleScore + 0.35*artistScore + 0.15*albumScore
		if album != "" && strings.EqualFold(strings.TrimSpace(item.album), strings.TrimSpace(album)) {
			score += 0.2
		}

		score += apiBonus[item.source]

		switch item.platformRank {
		case 0:
			score += 0.05
		case 1:
			score += 0.03
		case 2:
			score += 0.01
		}

		scored = append(scored, scoredItem{score: score, item: item})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// 选出最佳结果 (卡口 0.55)
	var best *searchResult
	for _, sc := range scored {
		if sc.score > 0.55 && sc.item.cover != "" && len(sc.item.lyrics) > 50 {
			best = &sc.item
			break
		}
	}

	if best == nil {
		for _, sc := range scored {
			if sc.item.cover != "" {
				best = &sc.item
				break
			}
		}
	}

	if best == nil && len(scored) > 0 {
		best = &scored[0].item
	}

	if best == nil {
		return nil
	}

	return map[string]interface{}{
		"title":  best.title,
		"artist": best.artist,
		"album":  best.album,
		"lyrics": best.lyrics,
		"cover":  best.cover,
		"source": best.source,
	}
}

// searchNetease 网易云刮削接口
func searchNetease(title, artist string) []searchResult {
	query := title
	if artist != "" {
		query += " " + artist
	}

	u := fmt.Sprintf("https://music.163.com/api/cloudsearch/pc?s=%s&type=1&offset=0&limit=5", url.QueryEscape(query))
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://music.163.com")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil
	}

	resultMap, _ := body["result"].(map[string]interface{})
	songsArr, _ := resultMap["songs"].([]interface{})

	var list []searchResult
	for idx, s := range songsArr {
		sMap, _ := s.(map[string]interface{})
		sID := fmt.Sprintf("%.0f", sMap["id"].(float64))
		sTitle, _ := sMap["name"].(string)

		sArtist := ""
		if arArr, ok := sMap["ar"].([]interface{}); ok && len(arArr) > 0 {
			if firstAr, ok := arArr[0].(map[string]interface{}); ok {
				sArtist, _ = firstAr["name"].(string)
			}
		}

		sAlbum := ""
		sCover := ""
		if alMap, ok := sMap["al"].(map[string]interface{}); ok {
			sAlbum, _ = alMap["name"].(string)
			sCover, _ = alMap["picUrl"].(string)
		}

		// 异步拉取歌词
		lyrics := getNeteaseLyric(sID)

		list = append(list, searchResult{
			title:        sTitle,
			artist:       sArtist,
			album:        sAlbum,
			lyrics:       lyrics,
			cover:        sCover,
			source:       "netease",
			platformRank: idx,
		})
	}

	return list
}

func getNeteaseLyric(songID string) string {
	u := fmt.Sprintf("https://music.163.com/api/song/lyric?id=%s&lv=1&tv=1", songID)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ""
	}

	lrcMap, _ := body["lrc"].(map[string]interface{})
	lyric, _ := lrcMap["lyric"].(string)
	return lyric
}

// searchQQ QQ 音乐刮削接口
func searchQQ(title, artist string) []searchResult {
	query := title
	if artist != "" {
		query += " " + artist
	}

	u := fmt.Sprintf("https://c.y.qq.com/soso/fcgi-bin/client_search_cp?w=%s&n=5&format=json", url.QueryEscape(query))
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil
	}

	dataMap, _ := body["data"].(map[string]interface{})
	songMap, _ := dataMap["song"].(map[string]interface{})
	listArr, _ := songMap["list"].([]interface{})

	var list []searchResult
	for idx, s := range listArr {
		item, _ := s.(map[string]interface{})
		sTitle, _ := item["songname"].(string)
		sAlbum, _ := item["albumname"].(string)
		albumMID, _ := item["albummid"].(string)

		sArtist := ""
		if singerArr, ok := item["singer"].([]interface{}); ok && len(singerArr) > 0 {
			if firstSinger, ok := singerArr[0].(map[string]interface{}); ok {
				sArtist, _ = firstSinger["name"].(string)
			}
		}

		sCover := ""
		if albumMID != "" {
			sCover = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", albumMID)
		}

		list = append(list, searchResult{
			title:        sTitle,
			artist:       sArtist,
			album:        sAlbum,
			lyrics:       "",
			cover:        sCover,
			source:       "qq",
			platformRank: idx,
		})
	}
	return list
}

// searchKugou 酷狗音乐刮削接口
func searchKugou(title, artist string) []searchResult {
	query := title
	if artist != "" {
		query += " " + artist
	}

	u := fmt.Sprintf("http://songsearch.kugou.com/song_search_v2?keyword=%s&page=1&pagesize=5", url.QueryEscape(query))
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil
	}

	dataMap, _ := body["data"].(map[string]interface{})
	listsArr, _ := dataMap["lists"].([]interface{})

	var list []searchResult
	for idx, s := range listsArr {
		item, _ := s.(map[string]interface{})
		sTitle, _ := item["SongName"].(string)
		sArtist, _ := item["SingerName"].(string)
		sAlbum, _ := item["AlbumName"].(string)

		list = append(list, searchResult{
			title:        sTitle,
			artist:       sArtist,
			album:        sAlbum,
			lyrics:       "",
			cover:        "",
			source:       "kugou",
			platformRank: idx,
		})
	}
	return list
}
