package scanner

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

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

		sDt, _ := sMap["dt"].(float64)
		lyrics, hasTrans := getNeteaseLyric(sID)

		list = append(list, searchResult{
			title:          sTitle,
			artist:         sArtist,
			album:          sAlbum,
			lyrics:         lyrics,
			cover:          sCover,
			source:         "netease",
			platformRank:   idx,
			durationMs:     int(sDt),
			hasTranslation: hasTrans,
		})
	}

	return list
}

func getNeteaseLyric(songID string) (string, bool) {
	u := fmt.Sprintf("https://music.163.com/api/song/lyric?id=%s&lv=1&tv=1", songID)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", false
	}

	lrcMap, _ := body["lrc"].(map[string]interface{})
	originLyric, _ := lrcMap["lyric"].(string)

	tlyricMap, _ := body["tlyric"].(map[string]interface{})
	transLyric, _ := tlyricMap["lyric"].(string)

	return mergeLyricsWithTranslation(originLyric, transLyric)
}

func getQQLyric(songmid string) (string, bool) {
	if songmid == "" {
		return "", false
	}
	u := fmt.Sprintf("https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=%s&g_tk=5381&format=json", songmid)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X)")
	req.Header.Set("Referer", "https://y.qq.com/portal/player.html")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}

	raw := strings.TrimSpace(string(bodyBytes))
	reg := regexp.MustCompile(`^\w+\((.*)\)$`)
	if matches := reg.FindStringSubmatch(raw); len(matches) > 1 {
		raw = matches[1]
	}

	var body map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return "", false
	}

	originLyric := ""
	if b64, ok := body["lyric"].(string); ok && b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err == nil {
			originLyric = string(decoded)
		}
	}

	transLyric := ""
	if b64, ok := body["trans"].(string); ok && b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err == nil {
			transLyric = string(decoded)
		}
	}

	return mergeLyricsWithTranslation(originLyric, transLyric)
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
		songMID, _ := item["songmid"].(string)
		duration, _ := item["interval"].(float64)

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

		lyrics, hasTrans := getQQLyric(songMID)

		list = append(list, searchResult{
			title:          sTitle,
			artist:         sArtist,
			album:          sAlbum,
			lyrics:         lyrics,
			cover:          sCover,
			source:         "qq",
			platformRank:   idx,
			durationMs:     int(duration) * 1000,
			hasTranslation: hasTrans,
		})
	}
	return list
}

func getKugouLyric(title, artist, kgHash string, duration int) (string, bool) {
	query := title
	if artist != "" {
		query += " " + artist
	}
	searchURL := fmt.Sprintf("https://krcs.kugou.com/search?ver=1&man=yes&client=mobi&keyword=%s&duration=%d&hash=%s",
		url.QueryEscape(query), duration, kgHash)
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	var searchBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&searchBody); err != nil {
		return "", false
	}

	candidates, _ := searchBody["candidates"].([]interface{})
	if len(candidates) == 0 {
		return "", false
	}

	candMap, ok := candidates[0].(map[string]interface{})
	if !ok {
		return "", false
	}

	kID, _ := candMap["id"].(string)
	accessKey, _ := candMap["accesskey"].(string)
	if kID == "" || accessKey == "" {
		return "", false
	}

	downURL := fmt.Sprintf("http://lyrics.kugou.com/download?ver=1&client=pc&id=%s&accesskey=%s&fmt=lrc&charset=utf8", kID, accessKey)
	dReq, err := http.NewRequest("GET", downURL, nil)
	if err != nil {
		return "", false
	}
	dReq.Header.Set("User-Agent", "Mozilla/5.0")

	dResp, err := httpClient.Do(dReq)
	if err != nil {
		return "", false
	}
	defer dResp.Body.Close()

	var downBody map[string]interface{}
	if err := json.NewDecoder(dResp.Body).Decode(&downBody); err != nil {
		return "", false
	}

	base64Content, _ := downBody["content"].(string)
	if base64Content == "" {
		return "", false
	}

	rawBytes, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil || len(rawBytes) == 0 {
		return "", false
	}

	return string(rawBytes), false
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
		duration, _ := item["Duration"].(float64)

		imgRaw, _ := item["Image"].(string)
		sCover := ""
		if imgRaw != "" {
			sCover = strings.Replace(imgRaw, "{size}", "400", 1)
		}
		kgHash, _ := item["FileHash"].(string)

		lyrics, hasTrans := getKugouLyric(sTitle, sArtist, kgHash, int(duration)*1000)

		list = append(list, searchResult{
			title:          sTitle,
			artist:         sArtist,
			album:          sAlbum,
			lyrics:         lyrics,
			cover:          sCover,
			source:         "kugou",
			platformRank:   idx,
			durationMs:     int(duration) * 1000,
			hasTranslation: hasTrans,
		})
	}
	return list
}
