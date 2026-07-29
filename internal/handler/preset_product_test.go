package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/database"
)

func TestPresetBrowseAndTransactionalBatchReview(t *testing.T) {
	ownerToken := registerAndLogin(t, "preset-batch-owner")
	firstID := createMusicFromJSON(t, ownerToken, `{
		"title":"Preset review one","artist":"Classifier","genres":["Ambient; Progressive House"]
	}`)
	secondID := createMusicFromJSON(t, ownerToken, `{
		"title":"Preset review two","artist":"Classifier","genres":["Ambient; Progressive House"]
	}`)

	reviewIDs := listMusicIDs(t, "/api/v1/musics?preset_status=needs_review&page_size=100", "")
	if !containsMusicID(reviewIDs, firstID) || !containsMusicID(reviewIDs, secondID) {
		t.Fatalf("review queue did not include new ambiguous tracks: %v", reviewIDs)
	}

	presets := httptest.NewRecorder()
	testRouter.ServeHTTP(presets, httptest.NewRequest(http.MethodGet, "/api/v1/presets", nil))
	if presets.Code != http.StatusOK {
		t.Fatalf("list presets: %d %s", presets.Code, presets.Body.String())
	}
	var presetResponse struct {
		Data struct {
			Items []domain.PresetSummary `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(presets.Body.Bytes(), &presetResponse); err != nil {
		t.Fatalf("decode preset summaries: %v", err)
	}
	if len(presetResponse.Data.Items) != len(domain.PresetIDs()) {
		t.Fatalf("preset summaries = %+v", presetResponse.Data.Items)
	}
	var cosmicReviewCount int64
	for _, summary := range presetResponse.Data.Items {
		if summary.PresetID == domain.PresetCosmicDrift {
			cosmicReviewCount = summary.NeedsReviewCount
		}
	}
	if cosmicReviewCount < 2 {
		t.Fatalf("cosmic review count = %d, want at least two", cosmicReviewCount)
	}

	batchPath := "/api/v1/users/admin/classifications/manual-batch"
	forbidden := performPresetBatch(t, ownerToken, batchPath,
		fmt.Sprintf(`{"music_ids":[%d,%d],"preset":"kinetic_pulse"}`, firstID, secondID))
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin batch override: %d %s", forbidden.Code, forbidden.Body.String())
	}

	_, administratorID := registerAndLoginWithID(t, "preset-batch-admin")
	if err := database.DB.Model(&domain.User{}).Where("id = ?", administratorID).Update("role", "admin").Error; err != nil {
		t.Fatalf("promote batch administrator: %v", err)
	}
	administratorToken := loginUser(t, "preset-batch-admin")

	duplicate := performPresetBatch(t, administratorToken, batchPath,
		fmt.Sprintf(`{"music_ids":[%d,%d],"preset":"calm_flow"}`, firstID, firstID))
	if duplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate batch IDs: %d %s", duplicate.Code, duplicate.Body.String())
	}
	tooLarge := performPresetBatch(t, administratorToken, batchPath,
		`{"music_ids":[`+strings.Repeat("1,", 20_000)+`1],"preset":"calm_flow"}`)
	if tooLarge.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized batch: %d %s", tooLarge.Code, tooLarge.Body.String())
	}

	updated := performPresetBatch(t, administratorToken, batchPath,
		fmt.Sprintf(`{"music_ids":[%d,%d],"preset":"kinetic_pulse"}`, firstID, secondID))
	if updated.Code != http.StatusOK {
		t.Fatalf("batch override: %d %s", updated.Code, updated.Body.String())
	}
	var updatedResponse struct {
		Data domain.BatchPresetOverrideResponse `json:"data"`
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &updatedResponse); err != nil {
		t.Fatalf("decode batch override: %v", err)
	}
	if updatedResponse.Data.Updated != 2 || len(updatedResponse.Data.Classifications) != 2 {
		t.Fatalf("batch override response = %+v", updatedResponse.Data)
	}
	for _, classification := range updatedResponse.Data.Classifications {
		if classification.ManualPreset == nil || *classification.ManualPreset != domain.PresetKineticPulse {
			t.Fatalf("batch classification = %+v", classification)
		}
	}

	reviewIDs = listMusicIDs(t, "/api/v1/musics?preset_status=needs_review&page_size=100", "")
	if containsMusicID(reviewIDs, firstID) || containsMusicID(reviewIDs, secondID) {
		t.Fatalf("manually resolved tracks remained in review queue: %v", reviewIDs)
	}

	missingID := uint(1_000_000_000)
	rolledBack := performPresetBatch(t, administratorToken, batchPath,
		fmt.Sprintf(`{"music_ids":[%d,%d],"preset":"bass_impact"}`, firstID, missingID))
	if rolledBack.Code != http.StatusNotFound {
		t.Fatalf("batch with missing track: %d %s", rolledBack.Code, rolledBack.Body.String())
	}
	var stored domain.MusicPresetClassification
	if err := database.DB.First(&stored, "music_id = ?", firstID).Error; err != nil {
		t.Fatalf("load classification after rollback: %v", err)
	}
	if stored.ManualPreset == nil || *stored.ManualPreset != domain.PresetKineticPulse {
		t.Fatalf("failed batch was partially applied: %+v", stored)
	}

	cleared := performPresetBatch(t, administratorToken, batchPath,
		fmt.Sprintf(`{"music_ids":[%d,%d],"preset":null}`, firstID, secondID))
	if cleared.Code != http.StatusOK {
		t.Fatalf("batch clear: %d %s", cleared.Code, cleared.Body.String())
	}
	reviewIDs = listMusicIDs(t, "/api/v1/musics?preset_status=needs_review&page_size=100", "")
	if !containsMusicID(reviewIDs, firstID) || !containsMusicID(reviewIDs, secondID) {
		t.Fatalf("cleared tracks did not return to review queue: %v", reviewIDs)
	}
}

func performPresetBatch(t *testing.T, token, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(response, request)
	return response
}

func listMusicIDs(t *testing.T, path, token string) []uint {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	testRouter.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list music: %d %s", response.Code, response.Body.String())
	}
	var decoded struct {
		Data struct {
			Items []struct {
				ID uint `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode music list: %v", err)
	}
	ids := make([]uint, 0, len(decoded.Data.Items))
	for _, item := range decoded.Data.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func containsMusicID(values []uint, expected uint) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
