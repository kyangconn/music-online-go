// Package middleware logger.go - 日志中间件
// 记录每个 HTTP 请求的方法、路径、状态码、IP 和耗时
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/pkg/log"
)

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		if status >= 500 {
			log.Errorf("%s %s %d %s %v", c.Request.Method, c.Request.URL.Path, status, c.ClientIP(), duration)
		} else if status >= 400 {
			log.Warnf("%s %s %d %s %v", c.Request.Method, c.Request.URL.Path, status, c.ClientIP(), duration)
		} else {
			log.Infof("%s %s %d %s %v", c.Request.Method, c.Request.URL.Path, status, c.ClientIP(), duration)
		}
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
