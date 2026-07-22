package downloader

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// extractSongLevel 提取网易云可听/最高音质
func extractSongLevel(privilege map[string]interface{}) (string, string) {
	if privilege == nil {
		return "standard", "standard"
	}
	norm := func(val interface{}) string {
		if val == nil {
			return "standard"
		}
		v := strings.ToLower(fmt.Sprintf("%v", val))
		if v == "none" || v == "" {
			return "standard"
		}
		if br, err := strconv.Atoi(v); err == nil {
			if br >= 999000 {
				return "lossless"
			}
			if br >= 320000 {
				return "exhigh"
			}
			if br >= 192000 {
				return "higher"
			}
			return "standard"
		}
		return v
	}

	maxBr := privilege["maxBrLevel"]
	if maxBr == nil {
		maxBr = privilege["maxbr"]
	}
	if maxBr == nil {
		maxBr = privilege["maxLevel"]
	}
	maxLevel := norm(maxBr)
	userBr := privilege["dlLevel"]
	if userBr == nil {
		userBr = privilege["plLevel"]
	}
	userLevel := norm(userBr)
	if userLevel == "" {
		userLevel = maxLevel
	}
	return userLevel, maxLevel
}

// FormatNeteaseSongs 将网易云 API 原始曲目数组转换为标准对象格式
func FormatNeteaseSongs(sourceTracks []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(sourceTracks))
	for _, raw := range sourceTracks {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		sid := fmt.Sprintf("%v", item["id"])
		if sid == "" || sid == "<nil>" {
			continue
		}

		fee, _ := item["fee"].(float64)
		privilege, _ := item["privilege"].(map[string]interface{})
		privilegeFee := 0.0
		if privilege != nil {
			privilegeFee, _ = privilege["fee"].(float64)
		}
		isVip := (fee == 1) || (privilegeFee == 1)

		userLevel, maxLevel := extractSongLevel(privilege)

		artistName := "未知艺术家"
		if arList, ok := item["ar"].([]interface{}); ok && len(arList) > 0 {
			names := []string{}
			for _, arRaw := range arList {
				if arMap, ok := arRaw.(map[string]interface{}); ok {
					if n, ok := arMap["name"].(string); ok && n != "" {
						names = append(names, n)
					}
				}
			}
			if len(names) > 0 {
				artistName = strings.Join(names, " / ")
			}
		}

		albumName := ""
		coverURL := ""
		if alMap, ok := item["al"].(map[string]interface{}); ok {
			albumName, _ = alMap["name"].(string)
			if pic, ok := alMap["picUrl"].(string); ok {
				coverURL = strings.Replace(pic, "http://", "https://", 1)
			}
		}

		duration := 0.0
		if dt, ok := item["dt"].(float64); ok {
			duration = dt / 1000.0
		}

		title, _ := item["name"].(string)
		if title == "" {
			title = fmt.Sprintf("未命名 %s", sid)
		}

		result = append(result, map[string]interface{}{
			"id":        sid,
			"title":     title,
			"artist":    artistName,
			"album":     albumName,
			"cover":     coverURL,
			"duration":  duration,
			"is_vip":    isVip,
			"level":     userLevel,
			"max_level": maxLevel,
		})
	}
	return result
}

// SearchNeteaseCloud 调用网易云云端 /cloudsearch 搜索歌曲
func SearchNeteaseCloud(keywords string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 30
	}
	res, err := CallNeteaseAPI("/cloudsearch", map[string]string{
		"keywords": keywords,
		"type":     "1",
		"limit":    strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}

	if resultObj, ok := res["result"].(map[string]interface{}); ok {
		if songsList, ok := resultObj["songs"].([]interface{}); ok {
			return FormatNeteaseSongs(songsList), nil
		}
	}
	return []map[string]interface{}{}, nil
}

// GetNeteaseRecommendSongs 获取每日推荐歌曲并格式化输出
func GetNeteaseRecommendSongs() (interface{}, error) {
	res, err := CallNeteaseAPI("/recommend/songs", map[string]string{"timestamp": fmt.Sprintf("%d", time.Now().UnixNano()/1e6)})
	if err != nil {
		return nil, err
	}

	if dataMap, ok := res["data"].(map[string]interface{}); ok {
		if dailySongs, ok := dataMap["dailySongs"].([]interface{}); ok {
			return FormatNeteaseSongs(dailySongs), nil
		}
	}
	if recommend, ok := res["recommend"].([]interface{}); ok {
		return FormatNeteaseSongs(recommend), nil
	}

	return []map[string]interface{}{}, nil
}

// ResolveNeteaseInput 短链跳转、URL 特征及单曲/歌单 ID 解析
func ResolveNeteaseInput(rawInput string) (map[string]interface{}, error) {
	rawInput = strings.TrimSpace(rawInput)
	if rawInput == "" {
		return nil, fmt.Errorf("请粘贴网易云分享链接或输入ID")
	}

	targetType := "song"
	targetID := ""

	isDigit := true
	for _, r := range rawInput {
		if r < '0' || r > '9' {
			isDigit = false
			break
		}
	}
	if isDigit {
		targetID = rawInput
	} else {
		currURL := rawInput
		if !strings.HasPrefix(currURL, "http://") && !strings.HasPrefix(currURL, "https://") {
			currURL = "https://" + currURL
		}

		if strings.Contains(currURL, "playlist") {
			targetType = "playlist"
		} else if strings.Contains(currURL, "song") {
			targetType = "song"
		}

		for _, part := range strings.Split(currURL, "/") {
			numStr := ""
			for _, r := range part {
				if r >= '0' && r <= '9' {
					numStr += string(r)
				} else if numStr != "" {
					break
				}
			}
			if len(numStr) >= 5 {
				targetID = numStr
				break
			}
		}
	}

	if targetID == "" {
		return nil, fmt.Errorf("未能从输入中提取出有效的网易云 ID")
	}

	if targetType == "playlist" {
		playlistName := ""
		if detailRes, err := CallNeteaseAPI("/playlist/detail", map[string]string{"id": targetID}); err == nil {
			if pl, ok := detailRes["playlist"].(map[string]interface{}); ok {
				if n, ok := pl["name"].(string); ok {
					playlistName = n
				}
			}
		}

		res, err := CallNeteaseAPI("/playlist/track/all", map[string]string{"id": targetID, "limit": "200"})
		if err != nil {
			return nil, err
		}
		var formattedSongs []map[string]interface{}
		if songsRaw, ok := res["songs"].([]interface{}); ok {
			formattedSongs = FormatNeteaseSongs(songsRaw)
		} else {
			formattedSongs = []map[string]interface{}{}
		}

		return map[string]interface{}{
			"type": "playlist",
			"id":   targetID,
			"name": playlistName,
			"data": formattedSongs,
		}, nil
	}

	res, err := CallNeteaseAPI("/song/detail", map[string]string{"ids": targetID})
	if err != nil {
		return nil, err
	}
	var formattedSongs []map[string]interface{}
	if songsRaw, ok := res["songs"].([]interface{}); ok {
		formattedSongs = FormatNeteaseSongs(songsRaw)
	} else {
		formattedSongs = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"type": "song",
		"id":   targetID,
		"data": formattedSongs,
	}, nil
}
