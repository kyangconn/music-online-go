package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/config"
)

func TestReadyChecksDatabaseAndUploadStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Run("ready when database and storage are available", func(t *testing.T) {
		r := gin.New()
		registerHealthAndMetrics(r, db, t.TempDir(), config.MetricsConfig{})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
	})

	t.Run("not ready when upload path is not a directory", func(t *testing.T) {
		badPath := filepath.Join(t.TempDir(), "upload-file")
		if err := os.WriteFile(badPath, []byte("not a directory"), 0600); err != nil {
			t.Fatalf("write blocking file: %v", err)
		}
		r := gin.New()
		registerHealthAndMetrics(r, db, badPath, config.MetricsConfig{})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
		}
		if !strings.Contains(w.Body.String(), "upload storage unavailable") {
			t.Fatalf("unexpected response: %s", w.Body.String())
		}
	})
}

func TestMetricsEndpointRequiresConfigurationAndBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("disabled returns not found", func(t *testing.T) {
		r := gin.New()
		registerHealthAndMetrics(r, nil, t.TempDir(), config.MetricsConfig{})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	r := gin.New()
	registerHealthAndMetrics(r, nil, t.TempDir(), config.MetricsConfig{Enabled: true, Token: "scrape-secret"})
	for _, tc := range []struct {
		name   string
		header string
		want   int
	}{
		{name: "missing token", want: http.StatusUnauthorized},
		{name: "wrong token", header: "Bearer wrong", want: http.StatusUnauthorized},
		{name: "valid token", header: "Bearer scrape-secret", want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}
