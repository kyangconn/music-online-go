package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestServerAddress(t *testing.T) {
	tests := []struct {
		name    string
		listen  string
		port    string
		expects string
	}{
		{name: "all interfaces", port: "8080", expects: ":8080"},
		{name: "IPv4", listen: "127.0.0.1", port: "9000", expects: "127.0.0.1:9000"},
		{name: "IPv6", listen: "::1", port: "9000", expects: "[::1]:9000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serverAddress(tt.listen, tt.port); got != tt.expects {
				t.Fatalf("serverAddress() = %q, want %q", got, tt.expects)
			}
		})
	}
}

func TestCacheControlForAsset(t *testing.T) {
	tests := []struct {
		name    string
		asset   string
		expects string
	}{
		{name: "index", asset: "index.html", expects: cacheControlRevalidate},
		{name: "service worker", asset: "/sw.js", expects: cacheControlRevalidate},
		{name: "web manifest", asset: "manifest.webmanifest", expects: cacheControlRevalidate},
		{name: "json manifest", asset: "manifest.json", expects: cacheControlRevalidate},
		{name: "fingerprinted JavaScript", asset: "assets/index-BClP4xad.js", expects: cacheControlImmutable},
		{name: "fingerprinted CSS with underscore", asset: "assets/Edit-0_lsz2tY.css", expects: cacheControlImmutable},
		{name: "non-fingerprinted asset", asset: "assets/app.js", expects: cacheControlShort},
		{name: "icon", asset: "icons/pwa-192.png", expects: cacheControlShort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cacheControlForAsset(tt.asset); got != tt.expects {
				t.Fatalf("cacheControlForAsset(%q) = %q, want %q", tt.asset, got, tt.expects)
			}
		})
	}
}

func TestStaticAssetsUseServedPathForCachePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	configureStaticAssets(r)

	request := httptest.NewRequest(http.MethodGet, "/missing-route-that-looks-like-an-asset.js", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Cache-Control"); got != cacheControlRevalidate {
		t.Fatalf("Cache-Control = %q, want %q", got, cacheControlRevalidate)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("Content-Type = %q, want HTML SPA fallback", got)
	}
}
