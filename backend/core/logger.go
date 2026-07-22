package core

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
	logFile     *os.File
	once        sync.Once
)

// SetupLogger 初始化日志文件与控制台输出
func SetupLogger(logPath string) {
	once.Do(func() {
		if logPath != "" {
			_ = os.MkdirAll(filepath.Dir(logPath), 0755)
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				logFile = f
				multiOut := io.MultiWriter(os.Stdout, f)
				infoLogger = log.New(multiOut, "[INFO] ", log.LstdFlags)
				warnLogger = log.New(multiOut, "[WARN] ", log.LstdFlags)
				errorLogger = log.New(multiOut, "[ERROR] ", log.LstdFlags|log.Lshortfile)
				debugLogger = log.New(multiOut, "[DEBUG] ", log.LstdFlags)
				return
			}
		}

		infoLogger = log.New(os.Stdout, "[INFO] ", log.LstdFlags)
		warnLogger = log.New(os.Stdout, "[WARN] ", log.LstdFlags)
		errorLogger = log.New(os.Stdout, "[ERROR] ", log.LstdFlags|log.Lshortfile)
		debugLogger = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags)
	})
}

func Info(format string, v ...interface{}) {
	if infoLogger != nil {
		infoLogger.Printf(format, v...)
	} else {
		log.Printf("[INFO] "+format, v...)
	}
}

func Warn(format string, v ...interface{}) {
	if warnLogger != nil {
		warnLogger.Printf(format, v...)
	} else {
		log.Printf("[WARN] "+format, v...)
	}
}

func Error(format string, v ...interface{}) {
	if errorLogger != nil {
		errorLogger.Printf(format, v...)
	} else {
		log.Printf("[ERROR] "+format, v...)
	}
}

func Debug(format string, v ...interface{}) {
	if debugLogger != nil {
		debugLogger.Printf(format, v...)
	} else {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func Println(v ...interface{}) {
	if infoLogger != nil {
		infoLogger.Println(v...)
	} else {
		fmt.Println(v...)
	}
}
