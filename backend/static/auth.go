package static

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/core"

	"github.com/gin-gonic/gin"
)

type ipAttempt struct {
	count     int
	lastTime  time.Time
	blockedTo time.Time
}

var (
	ipAttempts   = make(map[string]*ipAttempt)
	ipAttemptsMu sync.Mutex
)

// CheckIPBlocked 检查 IP 是否处于封禁状态
func CheckIPBlocked(ip string) bool {
	ipAttemptsMu.Lock()
	defer ipAttemptsMu.Unlock()

	att, exists := ipAttempts[ip]
	if !exists {
		return false
	}

	if time.Now().Before(att.blockedTo) {
		return true
	}

	delete(ipAttempts, ip)
	return false
}

// RecordIPFailure 记录 IP 登录失败尝试
func RecordIPFailure(ip string) {
	ipAttemptsMu.Lock()
	defer ipAttemptsMu.Unlock()

	now := time.Now()
	att, exists := ipAttempts[ip]
	if !exists {
		att = &ipAttempt{count: 0}
		ipAttempts[ip] = att
	}

	if now.Sub(att.lastTime) > time.Hour {
		att.count = 0
	}

	att.count++
	att.lastTime = now
	core.Warn("IP [%s] 鉴权失败 (尝试次数: %d/3)", ip, att.count)

	if att.count >= 3 {
		att.blockedTo = now.Add(time.Hour)
		core.Warn("IP [%s] 1小时内鉴权失败达到 3 次，已自动封禁 1 小时", ip)
	}
}

// RecordIPSuccess 登录成功，清除失败记录
func RecordIPSuccess(ip string) {
	ipAttemptsMu.Lock()
	defer ipAttemptsMu.Unlock()
	delete(ipAttempts, ip)
}

// SHA256String 计算字符串 SHA256
func SHA256String(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidatePassword 校验系统凭证 (仅允许 SHA-256 哈希匹配)
func ValidatePassword(provided string) bool {
	expected := core.GlobalConfig.Password
	if expected == "" {
		return true
	}
	if provided == "" {
		return false
	}
	if strings.EqualFold(provided, SHA256String(expected)) {
		return true
	}
	if len(expected) == 64 && strings.EqualFold(provided, expected) {
		return true
	}
	return false
}

// CORSMiddleware 跨域放行中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Password")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// AuthMiddleware 身份鉴权拦截中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if core.GlobalConfig.Password == "" {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		pathLower := strings.ToLower(path)

		exemptPaths := map[string]bool{
			"":                  true,
			"/":                 true,
			"/index.html":       true,
			"/login":            true,
			"/api/login":        true,
			"/icon.png":         true,
			"/manifest.json":    true,
			"/site.webmanifest": true,
		}
		if exemptPaths[pathLower] {
			c.Next()
			return
		}

		exemptPrefixes := []string{
			"/assets/",
			"/api/music/covers/",
		}
		for _, prefix := range exemptPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		if !strings.HasPrefix(path, "/api/") {
			ext := strings.ToLower(filepath.Ext(path))
			staticExts := map[string]bool{
				".js": true, ".css": true, ".png": true, ".jpg": true, ".jpeg": true,
				".svg": true, ".ico": true, ".webp": true, ".woff": true, ".woff2": true, ".ttf": true,
			}
			if staticExts[ext] {
				c.Next()
				return
			}
		}

		providedPass := c.GetHeader("X-Password")
		if providedPass == "" {
			providedPass = c.Query("auth")
		}

		if ValidatePassword(providedPass) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized", "message": "密码校验失败"})
	}
}

// CacheControlMiddleware 缓存控制中间件
func CacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		path := c.Request.URL.Path
		if strings.HasSuffix(path, ".html") || path == "/" || path == "" {
			c.Writer.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Writer.Header().Set("Pragma", "no-cache")
			c.Writer.Header().Set("Expires", "0")
		}
	}
}
