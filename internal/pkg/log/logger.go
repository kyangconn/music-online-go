// Package log logger.go - 日志系统
// 统一日志输出，支持终端+文件双写、文件轮转，通过环境变量控制
package log

import (
	"fmt"
	"io"
	golog "log"
	"os"
	"strconv"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger  = golog.New(os.Stdout, "", golog.LstdFlags)
	enabled = true
)

// Init 初始化日志输出。logFile 为空时仅输出到 stdout。
// 日志文件自动轮转：默认单文件 50MB，保留 3 个备份，保留 28 天。
// 可通过环境变量覆盖：
//
//	MO_LOG_MAX_SIZE=100    单文件最大 MB
//	MO_LOG_MAX_BACKUPS=5   备份数量
//	MO_LOG_MAX_AGE=14      保留天数
func Init(logFile string) {
	if logFile == "" {
		return
	}

	lj := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    envInt("MO_LOG_MAX_SIZE", 50),
		MaxBackups: envInt("MO_LOG_MAX_BACKUPS", 3),
		MaxAge:     envInt("MO_LOG_MAX_AGE", 28),
		Compress:   true,
		LocalTime:  true,
	}

	logger = golog.New(io.MultiWriter(os.Stdout, lj), "", golog.LstdFlags)
}

func envInt(key string, fallback int) int {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// Disable 关闭所有日志输出（静默模式）。
func Disable() { enabled = false }

func output(prefix, format string, v ...interface{}) {
	if !enabled {
		return
	}
	msg := "[" + prefix + "] " + fmt.Sprintf(format, v...)
	_ = logger.Output(3, msg)
}

func Infof(format string, v ...interface{})  { output("INFO", format, v...) }
func Warnf(format string, v ...interface{})  { output("WARN", format, v...) }
func Errorf(format string, v ...interface{}) { output("ERROR", format, v...) }

// Fatalf 输出错误日志并调用 os.Exit(1)
func Fatalf(format string, v ...interface{}) {
	output("FATAL", format, v...)
	os.Exit(1)
}
