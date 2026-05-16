package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid response body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

func TestRegisterAndLogin(t *testing.T) {
	body := `{"username":"testuser","email":"test@test.com","password":"password123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	loginBody := `{"username":"testuser","password":"password123"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/users/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("login response parse: %v", err)
	}

	token := resp["data"].(map[string]interface{})["token"].(string)
	if token == "" {
		t.Fatal("token should not be empty")
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("profile: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMusicTagSearchPublic(t *testing.T) {
	body := `{"artist":"test","title":"song"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/music-tags/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if _, ok := resp["tags"]; !ok {
		t.Error("response should have 'tags' field")
	}
}

func TestCreateMusicTagAuthRequired(t *testing.T) {
	body := `{"artist":"foo","title":"bar"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/music-tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated create, got %d", w.Code)
	}
}
