// Package log logger.go - 日志系统
// 统一日志输出，支持终端+文件双写和文件轮转
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
	logger       = golog.New(os.Stdout, "", golog.LstdFlags)
	enabled      = true
	minimumLevel = levelInfo
)

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

type Options struct {
	Level      string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	LocalTime  bool
}

// Init 初始化日志输出。logFile 为空时仅输出到 stdout。MO_LOG_MAX_SIZE、
// MO_LOG_MAX_BACKUPS 和 MO_LOG_MAX_AGE 作为兼容别名继续覆盖统一配置。
func Init(logFile string, options Options) {
	minimumLevel = parseLevel(options.Level)
	if logFile == "" {
		return
	}

	lj := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    envInt("MO_LOG_MAX_SIZE", options.MaxSizeMB),
		MaxBackups: envInt("MO_LOG_MAX_BACKUPS", options.MaxBackups),
		MaxAge:     envInt("MO_LOG_MAX_AGE", options.MaxAgeDays),
		Compress:   options.Compress,
		LocalTime:  options.LocalTime,
	}

	logger = golog.New(io.MultiWriter(os.Stdout, lj), "", golog.LstdFlags)
}

func parseLevel(value string) int {
	switch value {
	case "debug":
		return levelDebug
	case "warn":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
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

func output(level int, prefix, format string, v ...interface{}) {
	if !enabled || level < minimumLevel {
		return
	}
	msg := "[" + prefix + "] " + fmt.Sprintf(format, v...)
	_ = logger.Output(3, msg)
}

func Debugf(format string, v ...interface{}) { output(levelDebug, "DEBUG", format, v...) }
func Infof(format string, v ...interface{})  { output(levelInfo, "INFO", format, v...) }
func Warnf(format string, v ...interface{})  { output(levelWarn, "WARN", format, v...) }
func Errorf(format string, v ...interface{}) { output(levelError, "ERROR", format, v...) }

// Fatalf 输出错误日志并调用 os.Exit(1)
func Fatalf(format string, v ...interface{}) {
	output(levelError, "FATAL", format, v...)
	os.Exit(1)
}
