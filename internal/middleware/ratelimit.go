// Package middleware ratelimit.go - 频率限制中间件
// 基于 IP 的请求频率控制，防滥用
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/handler"
	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	ips sync.Map
	mu  sync.Mutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	i := &IPRateLimiter{
		r: r,
		b: b,
	}

	// Simple cleanup routine (optional, prevents memory leak)
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			i.ips.Range(func(key, value interface{}) bool {
				// In a real implementation, we would check last access time.
				// For now, we just clear everything every 10 mins to be safe and simple.
				i.ips.Delete(key)
				return true
			})
		}
	}()

	return i
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	limiter, exists := i.ips.Load(ip)
	if !exists {
		i.mu.Lock()
		defer i.mu.Unlock()
		// Double check
		limiter, exists = i.ips.Load(ip)
		if !exists {
			newLimiter := rate.NewLimiter(i.r, i.b)
			i.ips.Store(ip, newLimiter)
			return newLimiter
		}
	}
	return limiter.(*rate.Limiter)
}

func RateLimitMiddleware(r rate.Limit, b int) gin.HandlerFunc {
	limiter := NewIPRateLimiter(r, b)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.GetLimiter(ip).Allow() {
			handler.Error(c, http.StatusTooManyRequests, "Too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}
