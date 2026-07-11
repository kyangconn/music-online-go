// Package middleware middleware_test.go - 中间件单元测试
// 测试 CORS 中间件的 Origin 反射行为和角色中间件的权限拦截
package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kyangconn/music-online-go/internal/config"
)

func TestMain(m *testing.M) {
	config.AppConfig = &config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		JWT:    config.JWTConfig{Secret: "test-secret"},
	}
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestCORSMiddlewareOriginReflection(t *testing.T) {
	r := gin.New()
	r.Use(CORSMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	t.Run("reflects request Origin header", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
		}
	})

	t.Run("returns wildcard when no Origin header", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "*")
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
		}
	})

	t.Run("sets Access-Control-Allow-Credentials", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
		}
	})
}

func TestRoleMiddlewareRejectsNonAdmin(t *testing.T) {
	buildRouter := func(setRole string) *gin.Engine {
		r := gin.New()

		handlers := make([]gin.HandlerFunc, 0, 3)
		if setRole != "" {
			handlers = append(handlers, func(c *gin.Context) {
				c.Set("role", setRole)
				c.Next()
			})
		}
		handlers = append(handlers, RoleMiddleware("admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		r.GET("/admin", handlers...)
		return r
	}

	t.Run("no role returns 401", func(t *testing.T) {
		r := buildRouter("")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("user role returns 403", func(t *testing.T) {
		r := buildRouter("user")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("admin role returns 200", func(t *testing.T) {
		r := buildRouter("admin")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})
}
