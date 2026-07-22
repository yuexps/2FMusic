package middleware

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/config"

	"github.com/gin-gonic/gin"
)

// IP 登录失败尝试记录
type ipAttempt struct {
	count     int
	lastTime  time.Time
	blockedTo time.Time
}

var (
	ipAttempts   = make(map[string]*ipAttempt)
	ipAttemptsMu sync.Mutex
)

// CheckIPBlocked 检查 IP 是否被封禁
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
	return false
}

// RecordIPFailure 记录 IP 登录失败
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

	if att.count >= 3 {
		att.blockedTo = now.Add(time.Hour)
	}
}

// RecordIPSuccess 登录成功，重置失败计数
func RecordIPSuccess(ip string) {
	ipAttemptsMu.Lock()
	defer ipAttemptsMu.Unlock()
	delete(ipAttempts, ip)
}

// SHA256String 工具
func SHA256String(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidatePassword 校验密码
func ValidatePassword(provided string) bool {
	expected := config.GlobalConfig.Password
	if expected == "" {
		return true
	}
	if provided == "" {
		return false
	}
	if provided == expected {
		return true
	}
	if strings.EqualFold(provided, SHA256String(expected)) {
		return true
	}
	return false
}

// CORSMiddleware 跨域放行
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

// AuthMiddleware 密码鉴权拦截
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.GlobalConfig.Password == "" {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		exemptPaths := map[string]bool{
			"":                  true,
			"/":                 true,
			"/index.html":       true,
			"/login":            true,
			"/api/login":        true,
			"/favicon.ico":      true,
			"/icon.svg":         true,
			"/ICON.PNG":         true,
			"/manifest.json":    true,
			"/site.webmanifest": true,
		}
		if exemptPaths[path] {
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

// CacheControlMiddleware HTML 强 ETag 与 No-Cache 控制
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

// MD5Bytes 工具
func MD5Bytes(b []byte) string {
	h := md5.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
