package downloader

import (
	"fmt"
	"strings"
	"time"

	"2fmusic/backend/core"
	"2fmusic/backend/db"
)

// GetNeteaseQRKeyAndImage 申请网易云扫码 unikey 和 二维码 base64
func GetNeteaseQRKeyAndImage() (string, string, error) {
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
	keyRes, err := CallNeteaseAPI("/login/qr/key", map[string]string{"timestamp": timestamp})
	if err != nil {
		return "", "", err
	}

	dataMap, _ := keyRes["data"].(map[string]interface{})
	unikey, _ := dataMap["unikey"].(string)
	if unikey == "" {
		return "", "", fmt.Errorf("failed to get unikey from netease api")
	}

	imgRes, err := CallNeteaseAPI("/login/qr/create", map[string]string{"key": unikey, "qrimg": "1", "timestamp": timestamp})
	if err != nil {
		return "", "", err
	}

	imgDataMap, _ := imgRes["data"].(map[string]interface{})
	qrimg, _ := imgDataMap["qrimg"].(string)

	go PollNeteaseQRStatus(unikey)

	return unikey, qrimg, nil
}

// PollNeteaseQRStatus 轮询检查扫码登录结果并持久化 Cookie
func PollNeteaseQRStatus(unikey string) {
	core.Info("启动网易云扫码轮询协程: unikey=%s", unikey)
	maxDuration := 300 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxDuration {
		timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
		res, err := CallNeteaseAPI("/login/qr/check", map[string]string{"key": unikey, "timestamp": timestamp})
		if err == nil {
			codeFloat, _ := res["code"].(float64)
			code := int(codeFloat)

			var statusStr string
			switch code {
			case 803:
				statusStr = "authorized"
			case 802:
				statusStr = "scanned"
			case 800:
				statusStr = "expired"
			default:
				statusStr = "waiting"
			}

			if code == 803 {
				cookieStr, _ := res["cookie"].(string)
				if cookieStr == "" {
					if cookiesList, ok := res["cookies"].([]interface{}); ok {
						list := []string{}
						for _, c := range cookiesList {
							if cStr, ok := c.(string); ok && cStr != "" {
								list = append(list, cStr)
							}
						}
						if len(list) > 0 {
							cookieStr = strings.Join(list, "; ")
						}
					}
				}

				if cookieStr != "" {
					cookieStr = NormalizeCookieString(cookieStr)
					_ = db.SaveSystemSetting("netease_cookie", cookieStr)
					core.GlobalConfig.NeteaseCookie = cookieStr
					core.Info("网易云扫码授权成功, Cookie 已保存 (unikey=%s)", unikey)
				} else {
					core.Warn("网易云扫码授权成功但响应中未发现 Cookie 字段 (unikey=%s)", unikey)
				}
			}

			if BroadcastJSON != nil {
				BroadcastJSON(map[string]interface{}{
					"type":   "broadcast",
					"action": "netease_login_status",
					"data": map[string]interface{}{
						"key":     unikey,
						"code":    code,
						"status":  statusStr,
						"message": res["message"],
					},
				})
			}

			if code == 803 || code == 800 {
				if code == 800 {
					core.Info("网易云二维码已过期 (unikey=%s)", unikey)
				}
				break
			}
		}

		time.Sleep(2 * time.Second)
	}
}

// GetNeteaseLoginStatus 真实验证登录状态
func GetNeteaseLoginStatus() (map[string]interface{}, error) {
	if core.GlobalConfig.NeteaseAPIBase == "" {
		return map[string]interface{}{"logged_in": false, "error": "网易云 API 未配置"}, nil
	}
	if core.GlobalConfig.NeteaseCookie == "" {
		return map[string]interface{}{"logged_in": false, "error": "未登录"}, nil
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
	res, err := CallNeteaseAPI("/login/status", map[string]string{"timestamp": timestamp})
	if err != nil {
		return nil, err
	}

	dataMap, _ := res["data"].(map[string]interface{})
	profile, _ := dataMap["profile"].(map[string]interface{})
	if profile != nil {
		isVip := false
		if uid, ok := profile["userId"]; ok {
			uidStr := fmt.Sprintf("%v", uid)
			vipResp, err := CallNeteaseAPI("/vip/info", map[string]string{"uid": uidStr})
			if err == nil {
				if vipData, ok := vipResp["data"].(map[string]interface{}); ok {
					if v, ok := vipData["isVip"].(bool); ok {
						isVip = v
					} else if vFloat, ok := vipData["isVip"].(float64); ok && vFloat > 0 {
						isVip = true
					}
				}
			}
		}

		return map[string]interface{}{
			"logged_in": true,
			"nickname":  profile["nickname"],
			"user_id":   profile["userId"],
			"avatar":    profile["avatarUrl"],
			"is_vip":    isVip,
		}, nil
	}

	return map[string]interface{}{"logged_in": false, "error": "未登录"}, nil
}
