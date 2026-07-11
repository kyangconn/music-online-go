// Package handler_test handler_test.go - 处理器集成测试
// 包含注册、登录、获取用户资料等端到端测试用例
package handler_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
	"github.com/pquerna/otp/totp"
)

var (
	validAudioBytes = append([]byte("ID3\x04\x00\x00\x00\x00\x00\x10"), []byte("fake audio bytes")...)
	validCoverBytes = append([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}, []byte("fake cover bytes")...)
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

func TestLoginRequiresTOTPWhenEnabled(t *testing.T) {
	username := "totpuser"
	token := registerAndLogin(t, username)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users/totp/setup", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup totp: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var setupResp struct {
		Data struct {
			Secret string `json:"secret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &setupResp); err != nil {
		t.Fatalf("setup response parse: %v", err)
	}
	code, err := totp.GenerateCode(setupResp.Data.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/users/totp/enable", strings.NewReader(`{"code":"`+code+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("enable totp: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	loginBody := `{"username":"` + username + `","password":"password123"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/users/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("login without totp: expected 401, got %d: %s", w.Code, w.Body.String())
	}

	loginBody = `{"username":"` + username + `","password":"password123","totp_code":"` + code + `"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/users/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login with totp: expected 200, got %d: %s", w.Code, w.Body.String())
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
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response should have data object: %s", w.Body.String())
	}
	if _, ok := data["tags"]; !ok {
		t.Error("response data should have 'tags' field")
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

func TestUploadPolicy(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/upload-policy", nil)
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("upload policy: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			MaxAudioSizeBytes int64    `json:"max_audio_size_bytes"`
			MaxCoverSizeBytes int64    `json:"max_cover_size_bytes"`
			AudioExtensions   []string `json:"audio_extensions"`
			CoverExtensions   []string `json:"cover_extensions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("upload policy response parse: %v", err)
	}
	if resp.Data.MaxAudioSizeBytes <= 0 || resp.Data.MaxCoverSizeBytes <= 0 {
		t.Fatalf("upload policy should include positive size limits: %+v", resp.Data)
	}
	if len(resp.Data.AudioExtensions) == 0 || len(resp.Data.CoverExtensions) == 0 {
		t.Fatalf("upload policy should include supported extensions: %+v", resp.Data)
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
	expectedHash := fmt.Sprintf("%x", sha256.Sum256(validAudioBytes))
	if stored.FileHash != expectedHash {
		t.Fatalf("stored file hash = %q, want %q", stored.FileHash, expectedHash)
	}

	w = httptest.NewRecorder()
	req, _ := http.NewRequest("GET", expectedStreamPath, nil)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stream: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), validAudioBytes) {
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

	w = uploadMusicFiles(t, otherToken, musicID)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other user upload: expected 403, got %d: %s", w.Code, w.Body.String())
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

func TestUploadRejectsInvalidMediaFiles(t *testing.T) {
	token := registerAndLogin(t, "invalidmediauser")
	musicID := createMusic(t, token, "Invalid Media Song")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	audioPart, err := writer.CreateFormFile("file", "not-a-song.mp3")
	if err != nil {
		t.Fatalf("create invalid audio multipart: %v", err)
	}
	if _, err := audioPart.Write([]byte("this is not audio")); err != nil {
		t.Fatalf("write invalid audio multipart: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics/"+strconv.Itoa(int(musicID))+"/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid audio upload: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMusicSearchFiltersAndOptions(t *testing.T) {
	token := registerAndLogin(t, "filteruser")

	createMusicFromJSON(t, token, `{"title":"Filter Single","artist":"Filter Artist","year":2020,"type":"single"}`)
	likedID := createMusicFromJSON(t, token, `{"title":"Filter Album","artist":"Filter Artist","year":2021,"type":"album"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics/"+strconv.Itoa(int(likedID))+"/like", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("like filtered music: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/musics?artist=Filter%20Artist&year=2021&type=album&liked=true&page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("filtered music search: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var searchResp struct {
		Data struct {
			Items []domain.MusicResponse `json:"items"`
			Total int64                  `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &searchResp); err != nil {
		t.Fatalf("filtered music response parse: %v", err)
	}
	if searchResp.Data.Total != 1 || len(searchResp.Data.Items) != 1 || searchResp.Data.Items[0].ID != likedID {
		t.Fatalf("unexpected filtered music response: %+v", searchResp.Data)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/musics/filters", nil)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("music filter options: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var optionsResp struct {
		Data domain.MusicFilterOptions `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &optionsResp); err != nil {
		t.Fatalf("music filter options parse: %v", err)
	}
	if !containsString(optionsResp.Data.Artists, "Filter Artist") || !containsInt(optionsResp.Data.Years, 2021) {
		t.Fatalf("filter options missing created values: %+v", optionsResp.Data)
	}
}

func TestDuplicateCheckSuggestsAndEnrichesMetadata(t *testing.T) {
	token := registerAndLogin(t, "duplicateuser")
	fullID := createMusicFromJSON(t, token, `{"title":"Rich Duplicate","artist":"Duplicate Artist","album":"Rich Album","year":2022,"track_number":3,"genre":"Rock","duration":245}`)
	fullAudio := append(append([]byte{}, validAudioBytes...), []byte("rich-duplicate")...)
	w := uploadMusicAudio(t, token, fullID, fullAudio)
	if w.Code != http.StatusOK {
		t.Fatalf("upload rich duplicate source: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	fullHash := fmt.Sprintf("%x", sha256.Sum256(fullAudio))

	w = duplicateCheck(t, token, fmt.Sprintf(`{"file_hash":%q,"title":"Rich Duplicate","artist":"Duplicate Artist"}`, fullHash))
	if w.Code != http.StatusOK {
		t.Fatalf("exact duplicate check: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var exactResp struct {
		Data domain.MusicDuplicateCheckResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &exactResp); err != nil {
		t.Fatalf("exact duplicate response parse: %v", err)
	}
	if exactResp.Data.ExactMatch == nil || exactResp.Data.ExactMatch.ID != fullID {
		t.Fatalf("exact duplicate match = %+v, want music %d", exactResp.Data.ExactMatch, fullID)
	}
	if exactResp.Data.SuggestedMetadata.Album != "Rich Album" || exactResp.Data.SuggestedMetadata.Year != 2022 {
		t.Fatalf("suggested metadata should use richer existing record: %+v", exactResp.Data.SuggestedMetadata)
	}

	incompleteID := createMusicFromJSON(t, token, `{"title":"Incomplete Duplicate","artist":"Duplicate Artist"}`)
	incompleteAudio := append(append([]byte{}, validAudioBytes...), []byte("incomplete-duplicate")...)
	w = uploadMusicAudio(t, token, incompleteID, incompleteAudio)
	if w.Code != http.StatusOK {
		t.Fatalf("upload incomplete duplicate source: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	incompleteHash := fmt.Sprintf("%x", sha256.Sum256(incompleteAudio))
	w = duplicateCheck(t, token, fmt.Sprintf(`{"file_hash":%q,"title":"Incomplete Duplicate","artist":"Duplicate Artist","album":"Recovered Album","year":2023,"genre":"Jazz"}`, incompleteHash))
	if w.Code != http.StatusOK {
		t.Fatalf("enrichment duplicate check: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var enrichmentResp struct {
		Data struct {
			ExactMatch *domain.MusicResponse      `json:"exact_match"`
			Enrichment *domain.UpdateMusicRequest `json:"enrichment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &enrichmentResp); err != nil {
		t.Fatalf("enrichment duplicate response parse: %v", err)
	}
	if enrichmentResp.Data.ExactMatch == nil || enrichmentResp.Data.ExactMatch.ID != incompleteID || enrichmentResp.Data.Enrichment == nil {
		t.Fatalf("expected exact match with enrichment: %+v", enrichmentResp.Data)
	}
	if enrichmentResp.Data.Enrichment.Album == nil || *enrichmentResp.Data.Enrichment.Album != "Recovered Album" {
		t.Fatalf("album enrichment missing: %+v", enrichmentResp.Data.Enrichment)
	}
}

// TestDisabledAccountTokenRejected verifies that once an admin disables a user's
// account, the disabled user's existing JWT is rejected (401) on protected routes.
func TestDisabledAccountTokenRejected(t *testing.T) {
	userToken, userID := registerAndLoginWithID(t, "disableuser")

	_, adminID := registerAndLoginWithID(t, "disableadmin")
	database.DB.Model(&domain.User{}).Where("id = ?", adminID).Update("role", "admin")
	adminToken := loginUser(t, "disableadmin")

	w := httptest.NewRecorder()
	body := `{"is_active":false}`
	req, _ := http.NewRequest("PUT", "/api/v1/users/admin/users/"+strconv.Itoa(int(userID))+"/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable user: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("disabled user profile: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAdminCannotDisableSelf verifies that an admin cannot disable their own
// account or change their own role.
func TestAdminCannotDisableSelf(t *testing.T) {
	_, adminID := registerAndLoginWithID(t, "selfadmin")
	database.DB.Model(&domain.User{}).Where("id = ?", adminID).Update("role", "admin")
	adminToken := loginUser(t, "selfadmin")

	w := httptest.NewRecorder()
	body := `{"is_active":false}`
	req, _ := http.NewRequest("PUT", "/api/v1/users/admin/users/"+strconv.Itoa(int(adminID))+"/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusForbidden {
		t.Fatalf("admin disable self: expected 400 or 403, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	body = `{"role":"user"}`
	req, _ = http.NewRequest("PUT", "/api/v1/users/admin/users/"+strconv.Itoa(int(adminID))+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusForbidden {
		t.Fatalf("admin change own role: expected 400 or 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLastAdminCannotBeDemoted verifies that the last remaining admin cannot be
// demoted, either by self or by a non-admin.
func TestLastAdminCannotBeDemoted(t *testing.T) {
	_, admin1ID := registerAndLoginWithID(t, "lastadmin1")
	_, admin2ID := registerAndLoginWithID(t, "lastadmin2")
	database.DB.Model(&domain.User{}).Where("id IN ?", []uint{admin1ID, admin2ID}).Update("role", "admin")
	admin2Token := loginUser(t, "lastadmin2")

	// admin2 demotes admin1 – should succeed (2 admins → 1)
	w := httptest.NewRecorder()
	body := `{"role":"user"}`
	req, _ := http.NewRequest("PUT", "/api/v1/users/admin/users/"+strconv.Itoa(int(admin1ID))+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin2Token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin2 demote admin1: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// admin1 (now user) tries to demote admin2 → 403 (not admin)
	admin1Token := loginUser(t, "lastadmin1")
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/v1/users/admin/users/"+strconv.Itoa(int(admin2ID))+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin1Token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("former admin demote last admin: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// admin2 cannot demote self
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/v1/users/admin/users/"+strconv.Itoa(int(admin2ID))+"/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin2Token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusForbidden {
		t.Fatalf("admin2 demote self: expected 400 or 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestNonAdminCannotAccessAdminRoutes verifies that a regular user cannot access
// admin-protected endpoints.
func TestNonAdminCannotAccessAdminRoutes(t *testing.T) {
	userToken := registerAndLogin(t, "noroleuser")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin list users: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	body := `{"is_active":false}`
	req, _ = http.NewRequest("PUT", "/api/v1/users/admin/users/1/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin update status: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPathTraversalRejected verifies that the create music endpoint ignores
// img and path fields injected by the client, preventing path traversal.
func TestPathTraversalRejected(t *testing.T) {
	token := registerAndLogin(t, "pathtraversaluser")

	body := `{"title":"Traversal Song","artist":"Test Artist","img":"/etc/passwd","path":"/etc/passwd"}`
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
			Img  string `json:"img"`
			Path string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response parse: %v", err)
	}
	if resp.Data.Img != "" {
		t.Errorf("img should be empty, got %q", resp.Data.Img)
	}
	if resp.Data.Path != "" {
		t.Errorf("path should be empty, got %q", resp.Data.Path)
	}
}

// TestHealthAndReady verifies that the /health and /ready endpoints both return 200.
func TestHealthAndReady(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health: expected 200, got %d", w.Code)
	}
	var healthResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &healthResp); err != nil {
		t.Fatalf("health response parse: %v", err)
	}
	if healthResp["status"] != "ok" {
		t.Errorf("health status = %v, want ok", healthResp["status"])
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/ready", nil)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ready: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var readyResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &readyResp); err != nil {
		t.Fatalf("ready response parse: %v", err)
	}
	if readyResp["status"] != "ready" {
		t.Errorf("ready status = %v, want ready", readyResp["status"])
	}
}

func registerAndLoginWithID(t *testing.T, username string) (string, uint) {
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
			User struct {
				ID uint `json:"id"`
			} `json:"user"`
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("login response parse: %v", err)
	}
	if resp.Data.Token == "" {
		t.Fatal("token should not be empty")
	}
	return resp.Data.Token, resp.Data.User.ID
}

func loginUser(t *testing.T, username string) string {
	t.Helper()

	loginBody := `{"username":"` + username + `","password":"password123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users/login", strings.NewReader(loginBody))
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

func createMusicFromJSON(t *testing.T, token string, body string) uint {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create music from JSON: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("create music response parse: %v", err)
	}
	return resp.Data.ID
}

func duplicateCheck(t *testing.T, token string, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics/duplicate-check", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	return w
}

func uploadMusicAudio(t *testing.T, token string, musicID uint, audio []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "song.mp3")
	if err != nil {
		t.Fatalf("create audio multipart: %v", err)
	}
	if _, err := part.Write(audio); err != nil {
		t.Fatalf("write audio multipart: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close audio multipart: %v", err)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics/"+strconv.Itoa(int(musicID))+"/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	return w
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsInt(values []int, expected int) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func uploadMusicFiles(t *testing.T, token string, musicID uint) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	audioPart, err := writer.CreateFormFile("file", "song.mp3")
	if err != nil {
		t.Fatalf("create audio multipart: %v", err)
	}
	if _, err := audioPart.Write(validAudioBytes); err != nil {
		t.Fatalf("write audio multipart: %v", err)
	}

	coverPart, err := writer.CreateFormFile("cover", "cover.png")
	if err != nil {
		t.Fatalf("create cover multipart: %v", err)
	}
	if _, err := coverPart.Write(validCoverBytes); err != nil {
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
