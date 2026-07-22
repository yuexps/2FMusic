package core

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

// AUDIO_EXTS 标准音频后缀
var AUDIO_EXTS = []string{".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a"}

// AudioExtsMap 音频后缀集合
var AudioExtsMap = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".ogg":  true,
	".flac": true,
	".aac":  true,
	".m4a":  true,
}

// IsAudioFile 检查是否为支持的音频格式
func IsAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return AudioExtsMap[ext]
}

// MISC_EXTS 附属文件后缀 (.lrc, .yrc, .webp)
var MISC_EXTS = []string{".lrc", ".yrc", ".webp"}

// MiscExtsMap 附属文件后缀集合
var MiscExtsMap = map[string]bool{
	".lrc":  true,
	".yrc":  true,
	".webp": true,
}

// IsMiscFile 检查是否为附属文件格式
func IsMiscFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return MiscExtsMap[ext]
}

var GlobalConfig = &AppConfig{}

// NormalizePath 统一进行绝对路径转换与 Clean 规范化，消除跨平台斜杠与路径比较差异
func NormalizePath(path string) string {
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

	GlobalConfig.LyricsDir = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, "lyrics"))
	GlobalConfig.CoversDir = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, "covers"))
	GlobalConfig.CacheDir = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, ".cache"))
	GlobalConfig.DBPath = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, "data.db"))
	GlobalConfig.NeteaseDownloadDir = NormalizePath(filepath.Join(GlobalConfig.MusicLibraryPath, "NetEase"))

	_ = os.MkdirAll(GlobalConfig.LyricsDir, 0755)
	_ = os.MkdirAll(GlobalConfig.CoversDir, 0755)
	_ = os.MkdirAll(GlobalConfig.CacheDir, 0755)
	_ = os.MkdirAll(GlobalConfig.NeteaseDownloadDir, 0755)

	neteaseAPIBaseEnv := os.Getenv("NETEASE_API_BASE")
	GlobalConfig.NeteaseAPIBase = strings.TrimSpace(neteaseAPIBaseEnv)
	GlobalConfig.NeteaseQuality = "exhigh"
	GlobalConfig.NeteaseMaxConcurrent = 5
	GlobalConfig.LyricsPreference = "embedded"
}
