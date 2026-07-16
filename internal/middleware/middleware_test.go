// Package middleware middleware_test.go - 中间件单元测试
// 测试 CORS 白名单行为和角色中间件的权限拦截
package middleware

import (
	"crypto/tls"
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

func TestCORSMiddlewareAllowlist(t *testing.T) {
	r := gin.New()
	r.Use(CORSMiddleware([]string{"https://allowed.example.com"}, nil))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	t.Run("allows configured cross origin", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://allowed.example.com")
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.example.com" {
			t.Fatalf("Access-Control-Allow-Origin = %q", got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
		}
	})

	t.Run("rejects unconfigured cross origin", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://blocked.example.com")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
		}
	})

	t.Run("allows same origin without configuration", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Host = "music.example.com"
		req.Header.Set("Origin", "https://music.example.com")
		req.TLS = &tls.ConnectionState{}
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("omits CORS headers without Origin", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
		}
	})

	t.Run("handles allowed preflight", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://allowed.example.com")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
	})
}

func TestCORSMiddlewareOnlyTrustsForwardedProtoFromConfiguredProxy(t *testing.T) {
	newRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "http://music.example.com/test", nil)
		req.Host = "music.example.com"
		req.RemoteAddr = "192.0.2.1:4321"
		req.Header.Set("Origin", "https://music.example.com")
		req.Header.Set("X-Forwarded-Proto", "https")
		return req
	}
	serve := func(trustedProxies []string) *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(CORSMiddleware(nil, trustedProxies))
		r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, newRequest())
		return w
	}

	if got := serve(nil).Code; got != http.StatusForbidden {
		t.Fatalf("untrusted forwarded proto status = %d, want %d", got, http.StatusForbidden)
	}
	if got := serve([]string{"192.0.2.1"}).Code; got != http.StatusOK {
		t.Fatalf("trusted forwarded proto status = %d, want %d", got, http.StatusOK)
	}
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
