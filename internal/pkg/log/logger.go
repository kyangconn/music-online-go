package log

import (
	"fmt"
	"io"
	golog "log"
	"os"
)

var (
	logger  = golog.New(os.Stdout, "", golog.LstdFlags)
	enabled = true
)

// Init 初始化日志输出。logFile 为空时仅输出到 stdout。
func Init(logFile string) {
	if logFile == "" {
		return
	}
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] cannot open log file %s: %v\n", logFile, err)
		return
	}
	logger = golog.New(io.MultiWriter(os.Stdout, f), "", golog.LstdFlags)
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
