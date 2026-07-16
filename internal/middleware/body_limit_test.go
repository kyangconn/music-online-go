package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestJSONBodyLimitMiddleware(t *testing.T) {
	const maxBytes = 8
	newRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(JSONBodyLimitMiddleware(maxBytes))
		r.POST("/json", func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.Status(http.StatusBadRequest)
				return
			}
			c.Header("X-Body-Length", strconv.Itoa(len(body)))
			c.Status(http.StatusNoContent)
		})
		return r
	}

	t.Run("rejects known oversized body before the handler", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader("123456789"))
		req.Header.Set("Content-Type", "application/json")
		newRouter().ServeHTTP(w, req)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
		}
	})

	t.Run("passes an in-limit JSON body intact", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader("12345678"))
		req.Header.Set("Content-Type", "application/json")
		newRouter().ServeHTTP(w, req)
		if w.Code != http.StatusNoContent || w.Header().Get("X-Body-Length") != "8" {
			t.Fatalf("status/body length = %d/%q, want 204/8", w.Code, w.Header().Get("X-Body-Length"))
		}
	})

	t.Run("bounds a chunked body while it is read", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader("123456789"))
		req.ContentLength = -1
		req.Header.Set("Content-Type", "application/problem+json")
		newRouter().ServeHTTP(w, req)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
		}
	})

	t.Run("does not apply the JSON limit to another media type", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader("123456789"))
		req.Header.Set("Content-Type", "text/plain")
		newRouter().ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if got := w.Header().Get("X-Body-Length"); got != "9" {
			t.Fatalf("body length = %q, want 9", got)
		}
	})
}
