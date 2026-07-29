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
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
	"github.com/kyangconn/music-online-go/internal/service"
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

func TestRegisterRejectsPasswordsOutsideBcryptSafePolicy(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "too short", password: "1234567"},
		{name: "too many ASCII bytes", password: strings.Repeat("a", 73)},
		{name: "too many multibyte UTF-8 bytes", password: strings.Repeat("密", 25)},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{
				"username": fmt.Sprintf("password-policy-%d", index),
				"email":    fmt.Sprintf("password-policy-%d@example.com", index),
				"password": tt.password,
			})
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/users/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			testRouter.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "72 UTF-8 bytes") {
				t.Fatalf("response does not explain password policy: %s", w.Body.String())
			}
		})
	}
}

func TestChangePasswordExplainsBcryptSafeLimit(t *testing.T) {
	token := registerAndLogin(t, "change-password-policy")
	body, err := json.Marshal(map[string]string{
		"old_password": "password123",
		"new_password": strings.Repeat("密", 25),
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users/change-password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "72 UTF-8 bytes") {
		t.Fatalf("response does not explain password policy: %s", w.Body.String())
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

func TestLegacyMusicTagWriteEndpointIsRemoved(t *testing.T) {
	body := `{"artist":"foo","title":"bar"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/music-tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for removed legacy write endpoint, got %d", w.Code)
	}
}

func TestCanonicalMusicMetadataRoundTrip(t *testing.T) {
	token := registerAndLogin(t, "metadata-roundtrip")
	body := `{
		"title":"Tagged Track","artist":"Artist feat. Guest","artists":["Artist","Guest"],
		"album":"Release","album_artist":"Album Artist","album_artists":["Album Artist"],
		"year":2024,"track_number":2,"track_total":12,"disc_number":1,"disc_total":2,
		"release_date":"2024-03-02","original_release_date":"2023",
		"genres":["Ambient / Chillout","Electronic"],"comment":"liner note","isrcs":["US-ABC-24-12345"],
		"duration":201,
		"musicbrainz_recording_id":"123e4567-e89b-42d3-a456-426614174000",
		"musicbrainz_track_id":"123e4567-e89b-42d3-a456-426614174001",
		"musicbrainz_release_id":"123e4567-e89b-42d3-a456-426614174002",
		"musicbrainz_release_group_id":"123e4567-e89b-42d3-a456-426614174003",
		"musicbrainz_artist_ids":["123e4567-e89b-42d3-a456-426614174004"],
		"musicbrainz_album_artist_ids":["123e4567-e89b-42d3-a456-426614174005"]
	}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/musics", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create canonical metadata: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		Data domain.MusicResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created music: %v", err)
	}
	if created.Data.MetadataRevision != 1 || created.Data.ISRCs[0] != "USABC2412345" {
		t.Fatalf("canonical metadata was not normalized: %+v", created.Data)
	}
	if got, want := created.Data.GenreTokens, (domain.StringList{"ambient", "chillout", "electronic"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("genre tokens = %#v, want %#v", got, want)
	}
	if len(created.Data.Artists) != 2 || created.Data.MusicBrainzTrackID == created.Data.MusicBrainzRecordingID {
		t.Fatalf("multi-values or entity IDs were lost: %+v", created.Data)
	}

	update := `{"comment":"updated note","genres":["Synthwave / Retrowave"]}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPut, "/api/v1/musics/"+strconv.Itoa(int(created.Data.ID)), strings.NewReader(update))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update canonical metadata: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated struct {
		Data domain.MusicResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated music: %v", err)
	}
	if updated.Data.MetadataRevision != 2 || updated.Data.Comment != "updated note" ||
		len(updated.Data.GenreTokens) != 2 || updated.Data.Genres[0] != "Synthwave / Retrowave" {
		t.Fatalf("metadata update did not round trip: %+v", updated.Data)
	}
}

func TestBrowseEndpointsExposeStableArtistAlbumAndFacetFilters(t *testing.T) {
	token := registerAndLogin(t, "browse-contract")
	artistID := "123e4567-e89b-42d3-a456-426614174020"
	releaseID := "123e4567-e89b-42d3-a456-426614174021"
	for _, body := range []string{
		fmt.Sprintf(`{"title":"Second","artist":"Browse Contract Artist","artists":["Browse Contract Artist"],"album":"Release","album_artist":"Browse Contract Artist","year":2024,"track_number":2,"disc_number":1,"genres":["Ambient / Electronic"],"musicbrainz_artist_ids":[%q],"musicbrainz_release_id":%q}`, artistID, releaseID),
		fmt.Sprintf(`{"title":"First","artist":"BROWSE CONTRACT ARTIST","artists":["BROWSE CONTRACT ARTIST"],"album":"release","album_artist":"Browse Contract Artist","year":2024,"track_number":1,"disc_number":1,"genres":["Ambient"],"musicbrainz_artist_ids":[%q],"musicbrainz_release_id":%q}`, artistID, releaseID),
	} {
		createMusicFromJSON(t, token, body)
	}

	artists := httptest.NewRecorder()
	testRouter.ServeHTTP(artists, httptest.NewRequest(http.MethodGet, "/api/v1/artists?q=Browse%20Contract", nil))
	if artists.Code != http.StatusOK {
		t.Fatalf("list artists: %d %s", artists.Code, artists.Body.String())
	}
	var artistResponse struct {
		Data struct {
			Items []domain.ArtistSummary `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(artists.Body.Bytes(), &artistResponse); err != nil {
		t.Fatalf("decode artists: %v", err)
	}
	wantedArtistKey := "mbid_" + artistID
	var stableArtist *domain.ArtistSummary
	for index := range artistResponse.Data.Items {
		if artistResponse.Data.Items[index].Key == wantedArtistKey {
			stableArtist = &artistResponse.Data.Items[index]
			break
		}
	}
	if stableArtist == nil || stableArtist.TrackCount != 2 || stableArtist.AlbumCount != 1 {
		t.Fatalf("stable artist aggregation missing: %+v", artistResponse.Data.Items)
	}

	albums := httptest.NewRecorder()
	testRouter.ServeHTTP(albums, httptest.NewRequest(http.MethodGet, "/api/v1/albums?artist_key="+wantedArtistKey+"&genre=ambient&year=2024", nil))
	if albums.Code != http.StatusOK {
		t.Fatalf("list albums: %d %s", albums.Code, albums.Body.String())
	}
	var albumResponse struct {
		Data struct {
			Items []domain.AlbumSummary `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(albums.Body.Bytes(), &albumResponse); err != nil {
		t.Fatalf("decode albums: %v", err)
	}
	if len(albumResponse.Data.Items) != 1 || albumResponse.Data.Items[0].Key != "mbid_"+releaseID || albumResponse.Data.Items[0].TrackCount != 2 {
		t.Fatalf("stable album aggregation missing: %+v", albumResponse.Data.Items)
	}

	tracks := httptest.NewRecorder()
	path := "/api/v1/musics?album_key=" + albumResponse.Data.Items[0].Key + "&page_size=10"
	testRouter.ServeHTTP(tracks, httptest.NewRequest(http.MethodGet, path, nil))
	var trackResponse struct {
		Data struct {
			Items []domain.MusicResponse `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(tracks.Body.Bytes(), &trackResponse); err != nil {
		t.Fatalf("decode album tracks: %v", err)
	}
	if tracks.Code != http.StatusOK || len(trackResponse.Data.Items) != 2 || trackResponse.Data.Items[0].Title != "First" ||
		trackResponse.Data.Items[0].AlbumKey != "mbid_"+releaseID {
		t.Fatalf("album track order/identity: status=%d items=%+v", tracks.Code, trackResponse.Data.Items)
	}

	filteredTracks := httptest.NewRecorder()
	path = "/api/v1/musics?album_key=" + albumResponse.Data.Items[0].Key + "&q=First&year=2024&page_size=10"
	testRouter.ServeHTTP(filteredTracks, httptest.NewRequest(http.MethodGet, path, nil))
	if err := json.Unmarshal(filteredTracks.Body.Bytes(), &trackResponse); err != nil {
		t.Fatalf("decode filtered album tracks: %v", err)
	}
	if filteredTracks.Code != http.StatusOK || len(trackResponse.Data.Items) != 1 || trackResponse.Data.Items[0].Title != "First" {
		t.Fatalf("combined album query filters: status=%d items=%+v", filteredTracks.Code, trackResponse.Data.Items)
	}
}

func TestPlaylistEndpointsArePrivateOrderedAndOwnerScoped(t *testing.T) {
	ownerToken := registerAndLogin(t, "playlist-owner")
	otherToken := registerAndLogin(t, "playlist-other")
	firstID := createMusic(t, ownerToken, "Playlist First")
	secondID := createMusic(t, ownerToken, "Playlist Second")

	create := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/playlists", strings.NewReader(`{"name":"Private list","description":"Owner only"}`))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(create, req)
	if create.Code != http.StatusCreated {
		t.Fatalf("create playlist: %d %s", create.Code, create.Body.String())
	}
	var created struct {
		Data domain.PlaylistDetailResponse `json:"data"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode playlist: %v", err)
	}
	playlistPath := "/api/v1/playlists/" + strconv.Itoa(int(created.Data.ID))

	for _, musicID := range []uint{firstID, secondID, firstID} {
		add := httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, playlistPath+"/items", strings.NewReader(fmt.Sprintf(`{"music_id":%d}`, musicID)))
		req.Header.Set("Authorization", "Bearer "+ownerToken)
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(add, req)
		if add.Code != http.StatusOK {
			t.Fatalf("add playlist item: %d %s", add.Code, add.Body.String())
		}
	}

	reorder := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, playlistPath+"/items/order", strings.NewReader(fmt.Sprintf(`{"music_ids":[%d,%d]}`, secondID, firstID)))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(reorder, req)
	if reorder.Code != http.StatusOK {
		t.Fatalf("reorder playlist: %d %s", reorder.Code, reorder.Body.String())
	}
	var reordered struct {
		Data domain.PlaylistDetailResponse `json:"data"`
	}
	if err := json.Unmarshal(reorder.Body.Bytes(), &reordered); err != nil {
		t.Fatalf("decode reordered playlist: %v", err)
	}
	if len(reordered.Data.Items) != 2 || reordered.Data.Items[0].Music.ID != secondID || reordered.Data.Items[1].Music.ID != firstID {
		t.Fatalf("playlist order = %+v", reordered.Data.Items)
	}

	for name, testCase := range map[string]struct {
		authorization string
		want          int
	}{
		"anonymous":  {want: http.StatusUnauthorized},
		"other user": {authorization: "Bearer " + otherToken, want: http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, playlistPath, nil)
			if testCase.authorization != "" {
				request.Header.Set("Authorization", testCase.authorization)
			}
			testRouter.ServeHTTP(response, request)
			if response.Code != testCase.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.want, response.Body.String())
			}
		})
	}
}

func TestPresetClassificationFiltersAndAdminOverride(t *testing.T) {
	ownerToken := registerAndLogin(t, "preset-owner")
	musicID := createMusicFromJSON(t, ownerToken, `{
		"title":"Preset fixture","artist":"Classifier","genres":["Dubstep"],"metadata_revision":1
	}`)

	list := httptest.NewRecorder()
	testRouter.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/musics?preset=bass_impact&page_size=10", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("automatic preset filter: %d %s", list.Code, list.Body.String())
	}
	var listed struct {
		Data struct {
			Items []domain.MusicResponse `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode classified music: %v", err)
	}
	if len(listed.Data.Items) != 1 || listed.Data.Items[0].ID != musicID ||
		listed.Data.Items[0].PresetClassification == nil ||
		listed.Data.Items[0].PresetClassification.AutomaticPreset != domain.PresetBassImpact {
		t.Fatalf("classified music response = %+v", listed.Data.Items)
	}

	forbidden := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut,
		fmt.Sprintf("/api/v1/users/admin/musics/%d/classification/manual", musicID),
		strings.NewReader(`{"preset":"calm_flow"}`))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(forbidden, req)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin manual override: %d %s", forbidden.Code, forbidden.Body.String())
	}

	_, administratorID := registerAndLoginWithID(t, "preset-admin")
	if err := database.DB.Model(&domain.User{}).Where("id = ?", administratorID).Update("role", "admin").Error; err != nil {
		t.Fatalf("promote classification admin: %v", err)
	}
	administratorToken := loginUser(t, "preset-admin")
	manualPath := fmt.Sprintf("/api/v1/users/admin/musics/%d/classification/manual", musicID)
	override := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, manualPath, strings.NewReader(`{"preset":"calm_flow"}`))
	req.Header.Set("Authorization", "Bearer "+administratorToken)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(override, req)
	if override.Code != http.StatusOK || !strings.Contains(override.Body.String(), `"effective_source":"manual"`) {
		t.Fatalf("admin manual override: %d %s", override.Code, override.Body.String())
	}

	reclassify := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/v1/users/admin/musics/%d/classification/reclassify", musicID), nil)
	req.Header.Set("Authorization", "Bearer "+administratorToken)
	testRouter.ServeHTTP(reclassify, req)
	if reclassify.Code != http.StatusOK || !strings.Contains(reclassify.Body.String(), `"manual_preset":"calm_flow"`) ||
		!strings.Contains(reclassify.Body.String(), `"automatic_preset":"bass_impact"`) {
		t.Fatalf("reclassification must preserve manual override: %d %s", reclassify.Code, reclassify.Body.String())
	}

	clear := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, manualPath, nil)
	req.Header.Set("Authorization", "Bearer "+administratorToken)
	testRouter.ServeHTTP(clear, req)
	if clear.Code != http.StatusOK || strings.Contains(clear.Body.String(), `"manual_preset"`) {
		t.Fatalf("clear manual override: %d %s", clear.Code, clear.Body.String())
	}

	invalid := httptest.NewRecorder()
	testRouter.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/v1/musics?preset=not-a-preset", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid preset filter: %d %s", invalid.Code, invalid.Body.String())
	}
}

func TestStableRecordingIDOutranksTextAndArtistIDNeverIdentifiesATrack(t *testing.T) {
	token := registerAndLogin(t, "stable-id-matching")
	recordingID := "123e4567-e89b-42d3-a456-426614174010"
	artistID := "123e4567-e89b-42d3-a456-426614174011"
	existingID := createMusicFromJSON(t, token, fmt.Sprintf(
		`{"title":"Canonical Title","artist":"Canonical Artist","album":"Known Release","musicbrainz_recording_id":%q,"musicbrainz_artist_ids":[%q]}`,
		recordingID,
		artistID,
	))

	w := duplicateCheck(t, token, fmt.Sprintf(
		`{"title":"Different Display Title","artist":"Different Credit","musicbrainz_recording_id":%q}`,
		recordingID,
	))
	if w.Code != http.StatusOK {
		t.Fatalf("stable duplicate check: %d %s", w.Code, w.Body.String())
	}
	var stable struct {
		Data domain.MusicDuplicateCheckResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stable); err != nil {
		t.Fatalf("decode stable duplicate response: %v", err)
	}
	if len(stable.Data.MetadataMatches) == 0 || stable.Data.MetadataMatches[0].ID != existingID ||
		stable.Data.SuggestedMetadata.Album != "Known Release" {
		t.Fatalf("recording ID did not produce the stable candidate: %+v", stable.Data)
	}

	w = duplicateCheck(t, token, fmt.Sprintf(
		`{"title":"Unrelated Track","artist":"Canonical Artist","musicbrainz_artist_ids":[%q]}`,
		artistID,
	))
	if w.Code != http.StatusOK {
		t.Fatalf("artist-only duplicate check: %d %s", w.Code, w.Body.String())
	}
	var artistOnly struct {
		Data domain.MusicDuplicateCheckResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &artistOnly); err != nil {
		t.Fatalf("decode artist-only response: %v", err)
	}
	if len(artistOnly.Data.MetadataMatches) != 0 {
		t.Fatalf("artist ID incorrectly identified a track: %+v", artistOnly.Data.MetadataMatches)
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

func TestAdminMusicAnalysisQueueAPI(t *testing.T) {
	userToken := registerAndLogin(t, "analysisqueueuser")
	musicID := createMusicFromJSON(t, userToken, `{"title":"Queued Trance","artist":"Fixture Artist","genres":["Trance"]}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users/admin/musics/"+strconv.Itoa(int(musicID))+"/analysis", strings.NewReader(`{"include_audio":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin analysis schedule: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	_, adminID := registerAndLoginWithID(t, "analysisqueueadmin")
	if err := database.DB.Model(&domain.User{}).Where("id = ?", adminID).Update("role", "admin").Error; err != nil {
		t.Fatalf("promote analysis admin: %v", err)
	}
	adminToken := loginUser(t, "analysisqueueadmin")

	schedule := func(force bool) (int, struct {
		Data domain.AnalysisScheduleResponse `json:"data"`
	}) {
		body := `{"include_audio":false}`
		if force {
			body = `{"include_audio":false,"force":true}`
		}
		response := httptest.NewRecorder()
		request, _ := http.NewRequest("POST", "/api/v1/users/admin/musics/"+strconv.Itoa(int(musicID))+"/analysis", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+adminToken)
		testRouter.ServeHTTP(response, request)
		var parsed struct {
			Data domain.AnalysisScheduleResponse `json:"data"`
		}
		if response.Code == http.StatusAccepted {
			if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
				t.Fatalf("parse analysis schedule: %v", err)
			}
		}
		return response.Code, parsed
	}

	code, first := schedule(false)
	if code != http.StatusAccepted || first.Data.MetadataJob == nil || first.Data.MetadataJob.Status != domain.AnalysisStatusPending {
		t.Fatalf("analysis schedule response: code=%d data=%+v", code, first.Data)
	}
	code, repeated := schedule(false)
	if code != http.StatusAccepted || repeated.Data.MetadataJob == nil || repeated.Data.MetadataJob.ID != first.Data.MetadataJob.ID || repeated.Data.Reused != 1 {
		t.Fatalf("repeated analysis schedule: code=%d data=%+v", code, repeated.Data)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/admin/analysis/jobs?music_id="+strconv.Itoa(int(musicID))+"&status=pending", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatalf("list analysis jobs: expected one pending job, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/users/admin/analysis/jobs/"+strconv.Itoa(int(first.Data.MetadataJob.ID))+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("cancel analysis job: got %d: %s", w.Code, w.Body.String())
	}
	code, forced := schedule(true)
	if code != http.StatusAccepted || forced.Data.MetadataJob == nil || forced.Data.MetadataJob.ID != first.Data.MetadataJob.ID || forced.Data.MetadataJob.Status != domain.AnalysisStatusPending {
		t.Fatalf("force analysis schedule: code=%d data=%+v", code, forced.Data)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/admin/analysis/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"queue_length":1`) {
		t.Fatalf("analysis metrics: got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/admin/analysis/jobs?status=unknown", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid analysis status: expected 400, got %d: %s", w.Code, w.Body.String())
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

func TestUploadRequestBodyValidation(t *testing.T) {
	token := registerAndLogin(t, "uploadbodylimituser")
	musicID := createMusic(t, token, "Upload Body Limit Song")

	var before domain.Music
	if err := database.DB.First(&before, musicID).Error; err != nil {
		t.Fatalf("load music before oversized upload: %v", err)
	}

	t.Run("oversized body is rejected before persistence", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "oversized.mp3")
		if err != nil {
			t.Fatalf("create oversized multipart file: %v", err)
		}
		payload := make([]byte, service.UploadRequestBodyLimit()+1)
		copy(payload, validAudioBytes)
		if _, err := part.Write(payload); err != nil {
			t.Fatalf("write oversized multipart file: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close oversized multipart writer: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/musics/"+strconv.Itoa(int(musicID))+"/upload", &body)
		req.ContentLength = -1 // Exercise the streaming limit instead of the Content-Length fast path.
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		testRouter.ServeHTTP(w, req)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversized upload: expected 413, got %d: %s", w.Code, w.Body.String())
		}

		var after domain.Music
		if err := database.DB.First(&after, musicID).Error; err != nil {
			t.Fatalf("load music after oversized upload: %v", err)
		}
		if after.Path != before.Path || after.Img != before.Img || after.FileHash != before.FileHash || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Fatalf("oversized upload changed stored music: before=%+v after=%+v", before, after)
		}
	})

	t.Run("malformed multipart is a bad request", func(t *testing.T) {
		body := strings.NewReader("--broken\r\nContent-Disposition: form-data; name=\"file\"; filename=\"song.mp3\"\r\n\r\nID3")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/musics/"+strconv.Itoa(int(musicID))+"/upload", body)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=broken")
		req.Header.Set("Authorization", "Bearer "+token)
		testRouter.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("malformed upload: expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing files keeps the existing response", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("metadata", "ignored"); err != nil {
			t.Fatalf("write multipart field: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close multipart writer: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/musics/"+strconv.Itoa(int(musicID))+"/upload", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		testRouter.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "At least one of 'file' or 'cover' is required") {
			t.Fatalf("missing upload files: expected existing 400 response, got %d: %s", w.Code, w.Body.String())
		}
	})
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

func TestAdminCanCreateAUser(t *testing.T) {
	_, adminID := registerAndLoginWithID(t, "create-user-admin")
	if err := database.DB.Model(&domain.User{}).Where("id = ?", adminID).Update("role", "admin").Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	adminToken := loginUser(t, "create-user-admin")
	body := `{"username":"admin-created-user","email":"admin-created@example.com","password":"password123","full_name":"Created User","role":"user"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/admin/users", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create user: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/users/login", strings.NewReader(`{"username":"admin-created-user","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin-created user login: expected 200, got %d: %s", w.Code, w.Body.String())
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

// TestUploadFilesCreatesAudioPath 验证上传后响应包含合法的 path/img 字段，
// 且流式播放和封面图请求返回 200 和正确的 Content-Type。
func TestUploadFilesCreatesAudioPath(t *testing.T) {
	token := registerAndLogin(t, "pathuser")
	musicID := createMusic(t, token, "Path Check Song")

	w := uploadMusicFiles(t, token, musicID)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Path string `json:"path"`
			Img  string `json:"img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse upload response: %v", err)
	}
	if resp.Data.Path == "" {
		t.Fatal("path must not be empty after upload")
	}
	if resp.Data.Img == "" {
		t.Fatal("img must not be empty after upload")
	}

	// 验证流式播放返回 200 且 Content-Type 为音频格式
	streamPath := "/api/v1/musics/" + strconv.Itoa(int(musicID)) + "/stream"
	w = httptest.NewRecorder()
	req, _ := http.NewRequest("GET", streamPath, nil)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stream: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "audio/") {
		t.Fatalf("stream Content-Type = %q, want audio/*", ct)
	}

	// 验证封面图返回 200 且 Content-Type 为图片格式
	coverPath := "/api/v1/musics/" + strconv.Itoa(int(musicID)) + "/cover"
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", coverPath, nil)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cover: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	ct = w.Header().Get("Content-Type")
	if ct != "image/png" {
		t.Fatalf("cover Content-Type = %q, want image/png", ct)
	}
}

func TestDeleteMusicRemovesDatabaseRelationsAndMedia(t *testing.T) {
	ownerToken := registerAndLogin(t, "delete-music-owner")
	likerToken := registerAndLogin(t, "delete-music-liker")
	musicID := createMusic(t, ownerToken, "Delete Lifecycle Song")
	if w := uploadMusicFiles(t, ownerToken, musicID); w.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stored domain.Music
	if err := database.DB.First(&stored, musicID).Error; err != nil {
		t.Fatalf("find uploaded music: %v", err)
	}
	musicDir := filepath.Dir(stored.Path)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/musics/"+strconv.Itoa(int(musicID))+"/like", nil)
	req.Header.Set("Authorization", "Bearer "+likerToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("like: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/musics/"+strconv.Itoa(int(musicID)), nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var musicCount int64
	if err := database.DB.Unscoped().Model(&domain.Music{}).Where("id = ?", musicID).Count(&musicCount).Error; err != nil {
		t.Fatalf("count deleted music: %v", err)
	}
	if musicCount != 0 {
		t.Fatalf("music rows after deletion = %d, want 0", musicCount)
	}
	var likeCount int64
	if err := database.DB.Model(&domain.UserMusicLike{}).Where("music_id = ?", musicID).Count(&likeCount).Error; err != nil {
		t.Fatalf("count deleted likes: %v", err)
	}
	if likeCount != 0 {
		t.Fatalf("like rows after deletion = %d, want 0", likeCount)
	}
	if _, err := os.Stat(musicDir); !os.IsNotExist(err) {
		t.Fatalf("music directory still exists or stat failed: %v", err)
	}
}

func TestDeleteAccountRequiresPasswordAndRemovesOwnedData(t *testing.T) {
	token, userID := registerAndLoginWithID(t, "delete-account-user")
	musicID := createMusicFromJSON(t, token, `{"title":"Account Lifecycle Song","artist":"Test Artist","album":"Cleanup Album","genres":["Ambient"],"type":"single"}`)
	if w := uploadMusicFiles(t, token, musicID); w.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stored domain.Music
	if err := database.DB.First(&stored, musicID).Error; err != nil {
		t.Fatalf("find uploaded music: %v", err)
	}
	musicDir := filepath.Dir(stored.Path)
	playlist := &domain.Playlist{UserID: userID, Name: "Account cleanup", Revision: 1}
	if err := database.DB.Create(playlist).Error; err != nil {
		t.Fatalf("create account playlist: %v", err)
	}
	if err := database.DB.Create(&domain.PlaylistItem{PlaylistID: playlist.ID, MusicID: musicID, Position: 0}).Error; err != nil {
		t.Fatalf("create account playlist item: %v", err)
	}

	w := deleteAccount(t, token, "wrong-password")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("wrong password: expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(musicDir); err != nil {
		t.Fatalf("media was removed after rejected deletion: %v", err)
	}

	w = deleteAccount(t, token, "password123")
	if w.Code != http.StatusOK {
		t.Fatalf("delete account: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var userCount int64
	if err := database.DB.Unscoped().Model(&domain.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
		t.Fatalf("count deleted user: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("user rows after deletion = %d, want 0", userCount)
	}
	var musicCount int64
	if err := database.DB.Unscoped().Model(&domain.Music{}).Where("id = ?", musicID).Count(&musicCount).Error; err != nil {
		t.Fatalf("count account music: %v", err)
	}
	if musicCount != 0 {
		t.Fatalf("owned music rows after deletion = %d, want 0", musicCount)
	}
	for _, check := range []struct {
		label string
		model any
		query string
		value uint
	}{
		{label: "playlist", model: &domain.Playlist{}, query: "id = ?", value: playlist.ID},
		{label: "playlist item", model: &domain.PlaylistItem{}, query: "playlist_id = ?", value: playlist.ID},
		{label: "media file", model: &domain.MediaFile{}, query: "music_id = ?", value: musicID},
		{label: "artist credit", model: &domain.MusicArtistCredit{}, query: "music_id = ?", value: musicID},
		{label: "album membership", model: &domain.MusicAlbumMembership{}, query: "music_id = ?", value: musicID},
		{label: "genre facet", model: &domain.MusicGenreFacet{}, query: "music_id = ?", value: musicID},
		{label: "preset classification", model: &domain.MusicPresetClassification{}, query: "music_id = ?", value: musicID},
		{label: "preset scores", model: &domain.MusicPresetScore{}, query: "music_id = ?", value: musicID},
		{label: "analysis job", model: &domain.MusicAnalysisJob{}, query: "music_id = ?", value: musicID},
	} {
		var count int64
		if err := database.DB.Unscoped().Model(check.model).Where(check.query, check.value).Count(&count).Error; err != nil {
			t.Fatalf("count account %s rows: %v", check.label, err)
		}
		if count != 0 {
			t.Fatalf("account %s rows after deletion = %d, want 0", check.label, count)
		}
	}
	if _, err := os.Stat(musicDir); !os.IsNotExist(err) {
		t.Fatalf("owned music directory still exists or stat failed: %v", err)
	}

	w = httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("deleted account token: expected 401, got %d: %s", w.Code, w.Body.String())
	}

	registerBody := `{"username":"delete-account-user","email":"delete-account-user@test.com","password":"password123"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/users/register", strings.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("re-register deleted account: expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func deleteAccount(t *testing.T, token, password string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	body := `{"password":"` + password + `"}`
	req, _ := http.NewRequest("DELETE", "/api/v1/users/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	return w
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
