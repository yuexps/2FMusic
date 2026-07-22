package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"2fmusic/backend/config"
	"2fmusic/backend/db"
	"2fmusic/backend/middleware"
	"2fmusic/backend/scanner"
	"2fmusic/backend/static"
	"2fmusic/backend/websocket"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册所有 HTTP REST API 路由
func RegisterRoutes(r *gin.Engine) {
	apiGroup := r.Group("/api")

	apiGroup.POST("/login", handleLogin)
	apiGroup.GET("/music/play/:id", handlePlayMusic)
	apiGroup.GET("/music/external/play", handlePlayExternalMusic)
	apiGroup.GET("/music/covers/:name", handleGetCoverImage)
	apiGroup.POST("/music/upload", handleUploadMusic)
	apiGroup.Any("/netease/*path", handleNeteaseProxy)
	apiGroup.GET("/ws", websocket.ServeWS)

	r.NoRoute(handleStaticSPA)
}

func handleLogin(c *gin.Context) {
	clientIP := c.ClientIP()

	if middleware.CheckIPBlocked(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "尝试次数过多，请一小时后再试"})
		return
	}

	var req struct {
		Password string `json:"password"`
	}

	_ = c.ShouldBindJSON(&req)
	pass := req.Password
	if pass == "" {
		pass = c.Query("password")
	}

	if middleware.ValidatePassword(pass) {
		middleware.RecordIPSuccess(clientIP)
		token := middleware.SHA256String(pass)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "登录成功", "token": token})
	} else {
		middleware.RecordIPFailure(clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "密码错误"})
	}
}

func handlePlayMusic(c *gin.Context) {
	songID := c.Param("id")
	song, err := db.GetSongByID(songID)
	if err != nil || song == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Music not found"})
		return
	}

	c.File(song.Path)
}

func handlePlayExternalMusic(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing path parameter"})
		return
	}

	filePath, _ = url.QueryUnescape(filePath)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filePath)
}

func handleGetCoverImage(c *gin.Context) {
	coverName := c.Param("name")
	coverPath := filepath.Join(config.GlobalConfig.CoversDir, coverName)

	if _, err := os.Stat(coverPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cover not found"})
		return
	}

	c.Header("Cache-Control", "public, max-age=2592000")
	c.File(coverPath)
}

func handleUploadMusic(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No file uploaded"})
		return
	}

	targetDir := c.PostForm("target_dir")
	if targetDir == "" {
		targetDir = config.GlobalConfig.MusicLibraryPath
	}

	destPath := filepath.Join(targetDir, file.Filename)

	// 忽略 Watchdog 防抖
	scanner.AddWatchdogIgnorePath(destPath)

	if err := c.SaveUploadedFile(file, destPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 异步索引与自动刮削
	go func() {
		s := scanner.IndexSingleFile(destPath)
		if s != nil {
			websocket.NotifyLibraryChanged()
		}
	}()

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func handleNeteaseProxy(c *gin.Context) {
	proxyPath := c.Param("path")
	targetURL := fmt.Sprintf("%s%s", config.GlobalConfig.NeteaseAPIBase, proxyPath)
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	req.Header = c.Request.Header.Clone()
	if config.GlobalConfig.NeteaseCookie != "" {
		req.Header.Set("Cookie", config.GlobalConfig.NeteaseCookie)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "NCM API service unreachable"})
		return
	}
	defer resp.Body.Close()

	// 复制 Header，过滤 Set-Cookie 防止反代 502 报错
	for k, vv := range resp.Header {
		if strings.EqualFold(k, "Set-Cookie") {
			continue
		}
		for _, v := range vv {
			c.Writer.Header().Add(k, v)
		}
	}

	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func handleStaticSPA(c *gin.Context) {
	reqPath := c.Request.URL.Path

	if reqPath == "/login" {
		if bytes, ok := static.ServeStaticFile("/login.html"); ok {
			c.Data(http.StatusOK, "text/html; charset=utf-8", bytes)
			return
		}
	}

	// 尝试匹配具体物理/内嵌静态文件
	if bytes, ok := static.ServeStaticFile(reqPath); ok {
		// 识别 Content-Type
		ext := filepath.Ext(reqPath)
		switch ext {
		case ".html":
			c.Data(http.StatusOK, "text/html; charset=utf-8", bytes)
		case ".js":
			c.Data(http.StatusOK, "application/javascript; charset=utf-8", bytes)
		case ".css":
			c.Data(http.StatusOK, "text/css; charset=utf-8", bytes)
		case ".png":
			c.Data(http.StatusOK, "image/png", bytes)
		case ".svg":
			c.Data(http.StatusOK, "image/svg+xml", bytes)
		case ".json", ".webmanifest":
			c.Data(http.StatusOK, "application/json; charset=utf-8", bytes)
		default:
			c.Data(http.StatusOK, "application/octet-stream", bytes)
		}
		return
	}

	// 若带有扩展名但文件不存在，返回 404
	if filepath.Ext(reqPath) != "" {
		c.String(http.StatusNotFound, "Resource not found")
		return
	}

	// 降级返回 SPA index.html
	if bytes, ok := static.ServeStaticFile("/index.html"); ok {
		c.Data(http.StatusOK, "text/html; charset=utf-8", bytes)
		return
	}

	c.String(http.StatusNotFound, "2FMusic Backend Active (Go Embedded Version)")
}
