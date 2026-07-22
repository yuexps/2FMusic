package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/core"
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

// LongestCommonSubstring 最长公共子串
func LongestCommonSubstring(str1, str2 string) int {
	r1, r2 := []rune(str1), []rune(str2)
	m, n := len(r1), len(r2)
	if m == 0 || n == 0 {
		return 0
	}
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	maxLen := 0
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if r1[i-1] == r2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
				if dp[i][j] > maxLen {
					maxLen = dp[i][j]
				}
			}
		}
	}
	return maxLen
}

// CharDuplicateRate 字符交并比
func CharDuplicateRate(str1, str2 string) float64 {
	set1 := make(map[rune]bool)
	set2 := make(map[rune]bool)
	for _, r := range []rune(str1) {
		set1[r] = true
	}
	for _, r := range []rune(str2) {
		set2[r] = true
	}
	if len(set1) == 0 || len(set2) == 0 {
		return 0.0
	}
	intersection := 0
	unionMap := make(map[rune]bool)
	for r := range set1 {
		unionMap[r] = true
		if set2[r] {
			intersection++
		}
	}
	for r := range set2 {
		unionMap[r] = true
	}
	return float64(intersection) / float64(len(unionMap))
}

// StringSimilarity 文本相似度计算
func StringSimilarity(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	if s1 == s2 {
		return 1.0
	}
	if s1 == "" || s2 == "" {
		return 0.0
	}

	r1Len := float64(len([]rune(s1)))
	lcs := float64(LongestCommonSubstring(s1, s2))
	commonRatio := lcs / r1Len
	if commonRatio > 1.0 {
		commonRatio = 1.0
	}

	dupRate := CharDuplicateRate(s1, s2)
	similarRatio := commonRatio * math.Pow(math.Sqrt(dupRate), 1.0/1.5)
	return similarRatio
}

var suffixBracketRegex = regexp.MustCompile(`(\([^)]+\)|（[^）]+）|\[[^\]]+\])`)

// cleanSearchText 清理非开头括号后缀
func cleanSearchText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 0 && !strings.HasPrefix(s, "(") && !strings.HasPrefix(s, "（") && !strings.HasPrefix(s, "[") {
		s = suffixBracketRegex.ReplaceAllString(s, "")
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// isExactMatch 规范化完全匹配
func isExactMatch(title, artist, searchTitle, searchArtist string) bool {
	cTitle := cleanSearchText(title)
	cSearchTitle := cleanSearchText(searchTitle)
	if cTitle == "" || cTitle != cSearchTitle {
		return false
	}

	if artist == "" {
		return true
	}

	cArtist := cleanSearchText(artist)
	cSearchArtist := cleanSearchText(searchArtist)
	if cSearchArtist == "" {
		return true
	}

	return strings.Contains(cSearchArtist, cArtist) || strings.Contains(cArtist, cSearchArtist)
}

// SearchSongFastSequential 批量顺序刮削 (网易云 -> QQ -> 酷狗)

func SearchSongFastSequential(title, artist, album string) map[string]interface{} {
	if title == "" {
		return nil
	}

	sources := []string{"netease", "qq", "kugou"}
	var bestResult *searchResult
	bestScore := 0.0

	for _, source := range sources {
		var list []searchResult
		switch source {
		case "netease":
			list = searchNetease(title, artist)
		case "qq":
			list = searchQQ(title, artist)
		case "kugou":
			list = searchKugou(title, artist)
		}

		if len(list) == 0 {
			continue
		}

		var currentBest *searchResult
		currentBestScore := 0.0

		for _, item := range list {
			if isExactMatch(title, artist, item.title, item.artist) && (item.cover != "" || len(item.lyrics) > 20) {
				return map[string]interface{}{
					"title":  item.title,
					"artist": item.artist,
					"album":  item.album,
					"lyrics": item.lyrics,
					"cover":  item.cover,
					"source": item.source,
				}
			}

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
			switch item.platformRank {
			case 0:
				score += 0.05
			case 1:
				score += 0.03
			case 2:
				score += 0.01
			}

			if score > currentBestScore {
				currentBestScore = score
				itemCopy := item
				currentBest = &itemCopy
			}
		}

		if currentBestScore > bestScore && currentBest != nil {
			bestScore = currentBestScore
			bestResult = currentBest
		}
	}

	if bestResult == nil {
		return nil
	}

	return map[string]interface{}{
		"title":  bestResult.title,
		"artist": bestResult.artist,
		"album":  bestResult.album,
		"lyrics": bestResult.lyrics,
		"cover":  bestResult.cover,
		"source": bestResult.source,
	}
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

	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()

	select {
	case <-c:
	case <-time.After(6 * time.Second):
		core.Info("刮削搜索超时 (6s)，基于已返回的结果打分排序")
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
		sMap, ok := s.(map[string]interface{})
		if !ok || sMap == nil {
			continue
		}

		sID := ""
		if idFloat, ok := sMap["id"].(float64); ok {
			sID = fmt.Sprintf("%.0f", idFloat)
		} else if idStr, ok := sMap["id"].(string); ok {
			sID = idStr
		}
		if sID == "" {
			continue
		}

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
