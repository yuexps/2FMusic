package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"2fmusic/backend/api"
	"2fmusic/backend/core"
	"2fmusic/backend/db"
	"2fmusic/backend/downloader"
	"2fmusic/backend/scanner"
	"2fmusic/backend/static"

	"github.com/gin-gonic/gin"
)

func main() {
	downloader.BroadcastJSON = api.BroadcastJSON
	downloader.NotifyLibraryChanged = api.NotifyLibraryChanged
	scanner.NotifyLibraryChanged = api.NotifyLibraryChanged
	scanner.BroadcastScanStatus = api.BroadcastScanStatus

	core.InitFromArgs()
	core.SetupLogger(core.GlobalConfig.LogFile)

	core.Info("音乐库路径: %s", core.GlobalConfig.MusicLibraryPath)
	core.Info("日志文件路径: %s", core.GlobalConfig.LogFile)

	if err := db.InitDB(); err != nil {
		core.Error("数据库初始化失败: %v", err)
		os.Exit(1)
	}

	scanner.CleanTempPartFiles()
	go api.GlobalHub.Run()

	if err := scanner.InitWatcher(); err != nil {
		core.Error("文件监听器初始化失败: %v", err)
	}

	go scanner.TriggerScan()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(static.CORSMiddleware())
	r.Use(static.CacheControlMiddleware())
	r.Use(static.AuthMiddleware())

	var handler http.Handler = r
	if core.GlobalConfig.BaseURL != "" {
		handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			path := req.URL.Path
			prefix := core.GlobalConfig.BaseURL
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

	port := core.GlobalConfig.Port
	unixSocket := core.GlobalConfig.UnixSocket
	if port == 0 && unixSocket == "" {
		port = 23237
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if unixSocket != "" {
		_ = os.Remove(unixSocket)
		l, err := net.Listen("unix", unixSocket)
		if err != nil {
			core.Error("UNIX socket 绑定失败: %v", err)
		} else {
			_ = os.Chmod(unixSocket, 0666)
			core.Info("UNIX socket 监听已启动: %s", unixSocket)
			go func() {
				if err := http.Serve(l, handler); err != nil {
					core.Error("UNIX socket 运行异常: %v", err)
				}
			}()
		}
	}

	if port > 0 {
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		core.Info("HTTP 服务已启动: http://0.0.0.0:%d%s", port, core.GlobalConfig.BaseURL)
		go func() {
			if err := http.ListenAndServe(addr, handler); err != nil {
				core.Error("HTTP 服务运行异常: %v", err)
			}
		}()
	}

	<-sigChan
	core.Info("收到退出信号，停止服务...")
	if unixSocket != "" {
		_ = os.Remove(unixSocket)
	}
}
