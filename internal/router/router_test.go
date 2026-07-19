package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/handler"
)

func TestReadyChecksDatabaseAndUploadStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Run("ready when database and storage are available", func(t *testing.T) {
		r := gin.New()
		registerHealthAndMetrics(r, db, t.TempDir(), 2*time.Second, config.MetricsConfig{})

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
		registerHealthAndMetrics(r, db, badPath, 2*time.Second, config.MetricsConfig{})

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

func TestInstanceCapabilitiesAndClosedRegistration(t *testing.T) {
	router := gin.New()
	handlers := &Handlers{
		User:     &handler.UserHandler{},
		Music:    &handler.MusicHandler{},
		MusicTag: &handler.MusicTagHandler{},
		Admin:    &handler.AdminHandler{},
	}
	access := config.AccessConfig{
		LibraryMode:        config.LibraryAccessAuthenticated,
		RegistrationMode:   config.RegistrationAdmin,
		MediaURLTTLMinutes: 60,
	}
	registerAPIRoutes(router, handlers, nil, config.RateLimitConfig{}, access, config.IntegrationsConfig{}, "test-jwt-secret")

	capabilities := httptest.NewRecorder()
	router.ServeHTTP(capabilities, httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil))
	if capabilities.Code != http.StatusOK {
		t.Fatalf("instance status = %d: %s", capabilities.Code, capabilities.Body.String())
	}
	var response struct {
		Data struct {
			LibraryMode      string `json:"library_mode"`
			RegistrationOpen bool   `json:"registration_open"`
		} `json:"data"`
	}
	if err := json.Unmarshal(capabilities.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode capabilities: %v", err)
	}
	if response.Data.LibraryMode != config.LibraryAccessAuthenticated || response.Data.RegistrationOpen {
		t.Fatalf("unexpected capabilities: %+v", response.Data)
	}

	registration := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", strings.NewReader(`{"username":"blocked"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registration, request)
	if registration.Code != http.StatusForbidden {
		t.Fatalf("closed registration status = %d: %s", registration.Code, registration.Body.String())
	}
}

func TestMetricsEndpointRequiresConfigurationAndBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("disabled returns not found", func(t *testing.T) {
		r := gin.New()
		registerHealthAndMetrics(r, nil, t.TempDir(), 2*time.Second, config.MetricsConfig{})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	r := gin.New()
	registerHealthAndMetrics(r, nil, t.TempDir(), 2*time.Second, config.MetricsConfig{Enabled: true, Token: "scrape-secret"})
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
