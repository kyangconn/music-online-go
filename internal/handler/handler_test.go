// Package handler_test handler_test.go - 处理器集成测试
// 包含注册、登录、获取用户资料等端到端测试用例
package handler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
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

func TestCreateMusicDefaultsToSingle(t *testing.T) {
	token := registerAndLogin(t, "defaulttypeuser")

	body := `{"title":"No Type Song","artist":"Test Artist"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create music without type: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("create music response parse: %v", err)
	}
	if resp.Data.Type != "single" {
		t.Fatalf("type = %q, want single", resp.Data.Type)
	}
}

func TestUploadAndStreamMusic(t *testing.T) {
	token := registerAndLogin(t, "mediauser")
	musicID := createMusic(t, token, "Media Song")

	w := uploadMusicFiles(t, token, musicID)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var uploadResp struct {
		Data struct {
			Img  string `json:"img"`
			Path string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("upload response parse: %v", err)
	}

	expectedStreamPath := "/api/v1/musics/" + strconv.Itoa(int(musicID)) + "/stream"
	expectedCoverPath := "/api/v1/musics/" + strconv.Itoa(int(musicID)) + "/cover"
	if uploadResp.Data.Path != expectedStreamPath {
		t.Fatalf("path = %q, want %q", uploadResp.Data.Path, expectedStreamPath)
	}
	if uploadResp.Data.Img != expectedCoverPath {
		t.Fatalf("img = %q, want %q", uploadResp.Data.Img, expectedCoverPath)
	}
	if strings.Contains(uploadResp.Data.Path, `\`) || strings.Contains(uploadResp.Data.Img, `\`) {
		t.Fatalf("media URLs should not expose OS path separators: %+v", uploadResp.Data)
	}

	var stored domain.Music
	if err := database.DB.First(&stored, musicID).Error; err != nil {
		t.Fatalf("load stored music: %v", err)
	}
	audioPath := filepath.ToSlash(stored.Path)
	coverPath := filepath.ToSlash(stored.Img)
	if !strings.HasSuffix(audioPath, "/"+strconv.Itoa(int(musicID))+"/audio.mp3") {
		t.Fatalf("stored audio path = %q, want uploads/<id>/audio.mp3", stored.Path)
	}
	if !strings.HasSuffix(coverPath, "/"+strconv.Itoa(int(musicID))+"/cover.png") {
		t.Fatalf("stored cover path = %q, want uploads/<id>/cover.png", stored.Img)
	}

	w = httptest.NewRecorder()
	req, _ := http.NewRequest("GET", expectedStreamPath, nil)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stream: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "fake audio bytes" {
		t.Fatalf("stream body = %q", w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", expectedCoverPath, nil)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cover: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMusicUpdateDeleteRequiresOwner(t *testing.T) {
	ownerToken := registerAndLogin(t, "owneruser")
	otherToken := registerAndLogin(t, "otheruser")
	musicID := createMusic(t, ownerToken, "Owned Song")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/musics/"+strconv.Itoa(int(musicID)), strings.NewReader(`{"title":"Stolen"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+otherToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other user update: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/musics/"+strconv.Itoa(int(musicID)), nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other user delete: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/v1/musics/"+strconv.Itoa(int(musicID)), strings.NewReader(`{"title":"Updated"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("owner update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/musics/"+strconv.Itoa(int(musicID)), nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("owner delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func registerAndLogin(t *testing.T, username string) string {
	t.Helper()

	body := `{"username":"` + username + `","email":"` + username + `@test.com","password":"password123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register %s: expected 201, got %d: %s", username, w.Code, w.Body.String())
	}

	loginBody := `{"username":"` + username + `","password":"password123"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/users/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: expected 200, got %d: %s", username, w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("login response parse: %v", err)
	}
	if resp.Data.Token == "" {
		t.Fatal("token should not be empty")
	}
	return resp.Data.Token
}

func createMusic(t *testing.T, token string, title string) uint {
	t.Helper()

	body := `{"title":"` + title + `","artist":"Test Artist","type":"single"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create music: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("create music response parse: %v", err)
	}
	if resp.Data.ID == 0 {
		t.Fatal("music id should not be zero")
	}
	return resp.Data.ID
}

func uploadMusicFiles(t *testing.T, token string, musicID uint) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	audioPart, err := writer.CreateFormFile("file", "song.mp3")
	if err != nil {
		t.Fatalf("create audio multipart: %v", err)
	}
	if _, err := audioPart.Write([]byte("fake audio bytes")); err != nil {
		t.Fatalf("write audio multipart: %v", err)
	}

	coverPart, err := writer.CreateFormFile("cover", "cover.png")
	if err != nil {
		t.Fatalf("create cover multipart: %v", err)
	}
	if _, err := coverPart.Write([]byte("fake cover bytes")); err != nil {
		t.Fatalf("write cover multipart: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics/"+strconv.Itoa(int(musicID))+"/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	return w
}
