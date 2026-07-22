package config

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AppConfig 全局配置结构体
type AppConfig struct {
	BaseDir              string
	WWWDir               string
	MusicLibraryPath     string
	DataDir              string
	CoversDir            string
	LyricsDir            string
	CacheDir             string
	DBPath               string
	LogFile              string
	Port                 int
	UnixSocket           string
	BaseURL              string
	Password             string
	NeteaseCookie        string
	NeteaseDownloadDir   string
	NeteaseAPIBase       string
	NeteaseQuality       string
	NeteaseMaxConcurrent int
	LyricsPreference     string
}

var GlobalConfig = &AppConfig{}

// NormalizePath 统一进行绝对路径转换与 Clean 规范化，消除跨平台斜杠与路径比较差异
func NormalizePath(path string) string {
	// 剥离首尾空格及 Windows 右键“复制为路径”时带入的外层双引号/单引号
	path = strings.Trim(strings.TrimSpace(path), "\"'")
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

// InitFromArgs 解析命令行及环境变量初始化全局配置
func InitFromArgs() {
	execPath, err := os.Executable()
	if err != nil {
		GlobalConfig.BaseDir = NormalizePath(".")
	} else {
		GlobalConfig.BaseDir = NormalizePath(filepath.Dir(execPath))
	}

	GlobalConfig.WWWDir = NormalizePath(filepath.Join(GlobalConfig.BaseDir, "../www"))

	// 默认路径定义
	defaultMusicLib := os.Getenv("MUSIC_LIBRARY_PATH")
	if defaultMusicLib == "" {
		defaultMusicLib = NormalizePath(".")
	}

	defaultLogPath := os.Getenv("LOG_PATH")
	if defaultLogPath == "" {
		defaultLogPath = filepath.Join(GlobalConfig.BaseDir, "app.log")
	}

	var portFlag int
	portEnv := os.Getenv("PORT")
	if portEnv != "" {
		portFlag, _ = strconv.Atoi(portEnv)
	}

	flag.StringVar(&GlobalConfig.MusicLibraryPath, "music-library-path", defaultMusicLib, "Path to music library")
	flag.StringVar(&GlobalConfig.LogFile, "log-path", defaultLogPath, "Path to log file")
	flag.IntVar(&GlobalConfig.Port, "port", portFlag, "Server port")
	flag.StringVar(&GlobalConfig.UnixSocket, "unix-socket", os.Getenv("UNIX_SOCKET"), "Unix Domain Socket path")
	flag.StringVar(&GlobalConfig.BaseURL, "base-url", os.Getenv("BASE_URL"), "Application base URL prefix")

	passwordEnv := os.Getenv("APP_AUTH_PASSWORD")
	if passwordEnv == "" {
		passwordEnv = os.Getenv("APP_PASSWORD")
	}
	flag.StringVar(&GlobalConfig.Password, "password", passwordEnv, "Optional password for web access")

	if !flag.Parsed() {
		flag.Parse()
	}

	// 规范化 BaseURL
	if GlobalConfig.BaseURL != "" && GlobalConfig.BaseURL != "/" {
		if !strings.HasPrefix(GlobalConfig.BaseURL, "/") {
			GlobalConfig.BaseURL = "/" + GlobalConfig.BaseURL
		}
		GlobalConfig.BaseURL = strings.TrimRight(GlobalConfig.BaseURL, "/")
	} else {
		GlobalConfig.BaseURL = ""
	}

	GlobalConfig.MusicLibraryPath = NormalizePath(GlobalConfig.MusicLibraryPath)
	_ = os.MkdirAll(GlobalConfig.MusicLibraryPath, 0755)

	// 派生相关缓存与关联目录
	GlobalConfig.LyricsDir = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, "lyrics"))
	GlobalConfig.CoversDir = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, "covers"))
	GlobalConfig.CacheDir = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, ".cache"))
	GlobalConfig.DBPath = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, "data.db"))
	GlobalConfig.NeteaseDownloadDir = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, "NetEase"))

	_ = os.MkdirAll(GlobalConfig.LyricsDir, 0755)
	_ = os.MkdirAll(GlobalConfig.CoversDir, 0755)
	_ = os.MkdirAll(GlobalConfig.CacheDir, 0755)
	_ = os.MkdirAll(GlobalConfig.NeteaseDownloadDir, 0755)

	// 网易云 API 基础地址
	neteaseAPIBaseEnv := os.Getenv("NETEASE_API_BASE")
	if neteaseAPIBaseEnv == "" {
		neteaseAPIBaseEnv = "http://localhost:23236"
	}
	GlobalConfig.NeteaseAPIBase = neteaseAPIBaseEnv
	GlobalConfig.NeteaseQuality = "exhigh"
	GlobalConfig.NeteaseMaxConcurrent = 5
	GlobalConfig.LyricsPreference = "embedded"
}
