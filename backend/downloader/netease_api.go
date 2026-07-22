package downloader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"2fmusic/backend/core"
)

// NormalizeCookieString 提纯规范化 Cookie 键值对，剔除 Path, Domain, Expires 等拓展属性
func NormalizeCookieString(cookieStr string) string {
	if cookieStr == "" {
		return ""
	}

	ignoredKeys := map[string]bool{
		"path":     true,
		"domain":   true,
		"expires":  true,
		"max-age":  true,
		"samesite": true,
		"httponly": true,
		"secure":   true,
	}

	pairs := make([]string, 0)
	seen := make(map[string]bool)

	for _, item := range strings.Split(cookieStr, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}

		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		kLower := strings.ToLower(k)

		if ignoredKeys[kLower] || seen[kLower] {
			continue
		}

		seen[kLower] = true
		pairs = append(pairs, k+"="+v)
	}

	return strings.Join(pairs, "; ")
}

// CallNeteaseAPI 请求 NCM API 服务
func CallNeteaseAPI(apiPath string, params map[string]string) (map[string]interface{}, error) {
	apiBase := strings.TrimRight(core.GlobalConfig.NeteaseAPIBase, "/")
	if apiBase == "" {
		return nil, fmt.Errorf("网易云 API 未配置")
	}

	u, err := url.Parse(apiBase + apiPath)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	if core.GlobalConfig.NeteaseCookie != "" {
		req.Header.Set("Cookie", core.GlobalConfig.NeteaseCookie)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// fetchNeteaseSongURL 从本地/外置 NCM API 拉取在线音频链接
func fetchNeteaseSongURL(songID, level string) string {
	parseURLFromRes := func(res map[string]interface{}) string {
		if res == nil {
			return ""
		}
		var item map[string]interface{}
		if dataArr, ok := res["data"].([]interface{}); ok && len(dataArr) > 0 {
			item, _ = dataArr[0].(map[string]interface{})
		} else if dataMap, ok := res["data"].(map[string]interface{}); ok {
			item = dataMap
		}
		if item == nil {
			return ""
		}
		playURL, _ := item["url"].(string)
		if playURL == "" {
			playURL, _ = item["proxyUrl"].(string)
		}
		return playURL
	}

	res, err := CallNeteaseAPI("/song/url/v1", map[string]string{"id": songID, "level": level})
	if err == nil {
		if u := parseURLFromRes(res); u != "" {
			return u
		}
	}

	oldRes, err := CallNeteaseAPI("/song/url", map[string]string{"id": songID})
	if err == nil {
		if u := parseURLFromRes(oldRes); u != "" {
			return u
		}
	}

	return ""
}

// fetchNeteaseLyric 从 NCM API 获取歌词，优先提取 yrc 逐字歌词，缺失时退避使用 lrc 歌词
func fetchNeteaseLyric(songID string) string {
	res, err := CallNeteaseAPI("/lyric/new", map[string]string{"id": songID})
	if err != nil || res == nil {
		res, err = CallNeteaseAPI("/lyric", map[string]string{"id": songID})
	}
	if err != nil || res == nil {
		core.Warn("获取网易云歌词接口响应失败 (SongID=%s): %v", songID, err)
		return ""
	}

	// 1. 优先校验 yrc.lyric (网易云逐字歌词)
	if yrcMap, ok := res["yrc"].(map[string]interface{}); ok && yrcMap != nil {
		if yrcStr, ok := yrcMap["lyric"].(string); ok && strings.TrimSpace(yrcStr) != "" {
			core.Info("拉取网易云 YRC 逐字歌词成功 (SongID=%s)", songID)
			return strings.TrimSpace(yrcStr)
		}
	}

	// 2. 退避校验 lrc.lyric (标准 Lrc 歌词)
	if lrcMap, ok := res["lrc"].(map[string]interface{}); ok && lrcMap != nil {
		if lrcStr, ok := lrcMap["lyric"].(string); ok && strings.TrimSpace(lrcStr) != "" {
			core.Info("拉取网易云 LRC 标准歌词成功 (SongID=%s)", songID)
			return strings.TrimSpace(lrcStr)
		}
	}

	core.Warn("网易云 API 未返回有效歌词内容 (SongID=%s)", songID)
	return ""
}

// fetchNeteaseCoverURL 从 NCM API 获取曲目的封面图片链接
func fetchNeteaseCoverURL(songID string) string {
	res, err := CallNeteaseAPI("/song/detail", map[string]string{"ids": songID})
	if err != nil || res == nil {
		core.Warn("获取网易云歌曲详情接口失败 (SongID=%s): %v", songID, err)
		return ""
	}

	if songs, ok := res["songs"].([]interface{}); ok && len(songs) > 0 {
		if info, ok := songs[0].(map[string]interface{}); ok {
			if al, ok := info["al"].(map[string]interface{}); ok {
				if picURL, ok := al["picUrl"].(string); ok && picURL != "" {
					core.Info("拉取网易云封面 URL 成功: %s (SongID=%s)", picURL, songID)
					return picURL
				}
			}
		}
	}
	core.Warn("网易云 API 未找到歌曲封面图片 (SongID=%s)", songID)
	return ""
}

