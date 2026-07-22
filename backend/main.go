package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"2fmusic/backend/api"
	"2fmusic/backend/config"
	"2fmusic/backend/db"
	"2fmusic/backend/downloader"
	"2fmusic/backend/logger"
	"2fmusic/backend/middleware"
	"2fmusic/backend/scanner"
	"2fmusic/backend/websocket"

	"github.com/gin-gonic/gin"
)

func main() {
	downloader.BroadcastJSON = websocket.BroadcastJSON
	downloader.NotifyLibraryChanged = websocket.NotifyLibraryChanged
	scanner.NotifyLibraryChanged = websocket.NotifyLibraryChanged

	config.InitFromArgs()
	logger.Setup(config.GlobalConfig.LogFile)

	logger.Info("初始化运行配置及日志: library=%s, log=%s", config.GlobalConfig.MusicLibraryPath, config.GlobalConfig.LogFile)

	if err := db.InitDB(); err != nil {
		logger.Error("数据库初始化失败: %v", err)
		os.Exit(1)
	}

	scanner.CleanTempPartFiles()
	go websocket.GlobalHub.Run()

	if err := scanner.InitWatcher(); err != nil {
		logger.Error("文件监听器初始化失败: %v", err)
	}

	go scanner.TriggerScan()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.CacheControlMiddleware())
	r.Use(middleware.AuthMiddleware())

	var handler http.Handler = r
	if config.GlobalConfig.BaseURL != "" {
		handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			path := req.URL.Path
			prefix := config.GlobalConfig.BaseURL
			if path == prefix || (len(path) > len(prefix) && path[:len(prefix)] == prefix) {
				req.URL.Path = path[len(prefix):]
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			}
			r.ServeHTTP(w, req)
		})
	}

	api.RegisterRoutes(r)

	port := config.GlobalConfig.Port
	unixSocket := config.GlobalConfig.UnixSocket
	if port == 0 && unixSocket == "" {
		port = 23237
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if unixSocket != "" {
		_ = os.Remove(unixSocket)
		l, err := net.Listen("unix", unixSocket)
		if err != nil {
			logger.Error("UNIX socket 绑定失败: %v", err)
		} else {
			_ = os.Chmod(unixSocket, 0666)
			logger.Info("UNIX socket 监听已启动: %s", unixSocket)
			go func() {
				if err := http.Serve(l, handler); err != nil {
					logger.Error("UNIX socket 运行异常: %v", err)
				}
			}()
		}
	}

	if port > 0 {
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		logger.Info("HTTP 服务已启动: http://0.0.0.0:%d%s", port, config.GlobalConfig.BaseURL)
		go func() {
			if err := http.ListenAndServe(addr, handler); err != nil {
				logger.Error("HTTP 服务运行异常: %v", err)
			}
		}()
	}

	<-sigChan
	logger.Info("收到退出信号，停止服务...")
	if unixSocket != "" {
		_ = os.Remove(unixSocket)
	}
}
