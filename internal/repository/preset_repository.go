package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPresetClassificationNotFound = errors.New("preset classification not found")
var ErrPresetAnalysisMismatch = errors.New("audio analysis does not match current music content")

type PresetRepository interface {
	FindByMusicIDs(ctx context.Context, musicIDs []uint) (map[uint]*domain.MusicPresetClassification, error)
	ListSummaries(ctx context.Context) ([]domain.PresetSummary, error)
	Reclassify(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error)
	ReclassifyWithAudio(ctx context.Context, musicID uint, analysis *domain.MusicAudioAnalysis) (*domain.MusicPresetClassification, error)
	SetManualPreset(ctx context.Context, musicID, administratorID uint, preset string) (*domain.MusicPresetClassification, error)
	ClearManualPreset(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error)
	SetManualPresets(ctx context.Context, musicIDs []uint, administratorID uint, preset *string) (map[uint]*domain.MusicPresetClassification, error)
}

type presetRepository struct {
	db     *gorm.DB
	policy domain.PresetRulePolicy
}

func NewPresetRepository(db *gorm.DB, policy domain.PresetRulePolicy) PresetRepository {
	return &presetRepository{db: db, policy: policy}
}

func (r *presetRepository) FindByMusicIDs(ctx context.Context, musicIDs []uint) (map[uint]*domain.MusicPresetClassification, error) {
	result := make(map[uint]*domain.MusicPresetClassification)
	if len(musicIDs) == 0 {
		return result, nil
	}
	var classifications []*domain.MusicPresetClassification
	if err := r.db.WithContext(ctx).Where("music_id IN ?", musicIDs).Find(&classifications).Error; err != nil {
		return nil, fmt.Errorf("list preset classifications: %w", err)
	}
	if len(classifications) == 0 {
		return result, nil
	}
	var scores []domain.MusicPresetScore
	if err := r.db.WithContext(ctx).Where("music_id IN ?", musicIDs).
		Order("music_id ASC, preset_id ASC").Find(&scores).Error; err != nil {
		return nil, fmt.Errorf("list preset scores: %w", err)
	}
	for _, classification := range classifications {
		classification.Scores = []domain.MusicPresetScore{}
		result[classification.MusicID] = classification
	}
	for _, score := range scores {
		if classification := result[score.MusicID]; classification != nil {
			classification.Scores = append(classification.Scores, score)
		}
	}
	return result, nil
}

func (r *presetRepository) ListSummaries(ctx context.Context) ([]domain.PresetSummary, error) {
	summaries := make(map[string]*domain.PresetSummary, len(domain.PresetIDs()))
	for _, preset := range domain.PresetIDs() {
		summaries[preset] = &domain.PresetSummary{PresetID: preset}
	}
	type aggregateRow struct {
		PresetID string
		Count    int64
	}
	var classified []aggregateRow
	if err := r.db.WithContext(ctx).Model(&domain.MusicPresetClassification{}).
		Select("COALESCE(manual_preset, automatic_preset) AS preset_id, COUNT(*) AS count").
		Where("COALESCE(manual_preset, automatic_preset) <> ''").
		Group("COALESCE(manual_preset, automatic_preset)").Scan(&classified).Error; err != nil {
		return nil, fmt.Errorf("count preset tracks: %w", err)
	}
	for _, row := range classified {
		if summary := summaries[row.PresetID]; summary != nil {
			summary.TrackCount = row.Count
		}
	}
	var review []aggregateRow
	if err := r.db.WithContext(ctx).Model(&domain.MusicPresetClassification{}).
		Select("primary_preset AS preset_id, COUNT(*) AS count").
		Where("manual_preset IS NULL AND status = ? AND primary_preset <> ''", domain.PresetStatusNeedsReview).
		Group("primary_preset").Scan(&review).Error; err != nil {
		return nil, fmt.Errorf("count preset review tracks: %w", err)
	}
	for _, row := range review {
		if summary := summaries[row.PresetID]; summary != nil {
			summary.NeedsReviewCount = row.Count
		}
	}
	result := make([]domain.PresetSummary, 0, len(summaries))
	for _, preset := range domain.PresetIDs() {
		result = append(result, *summaries[preset])
	}
	return result, nil
}

func (r *presetRepository) Reclassify(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error) {
	return r.reclassify(ctx, musicID, nil)
}

func (r *presetRepository) ReclassifyWithAudio(
	ctx context.Context,
	musicID uint,
	analysis *domain.MusicAudioAnalysis,
) (*domain.MusicPresetClassification, error) {
	if analysis == nil || analysis.ID == 0 || analysis.Status != domain.AnalysisStatusSucceeded {
		return nil, ErrPresetAnalysisMismatch
	}
	return r.reclassify(ctx, musicID, analysis)
}

func (r *presetRepository) reclassify(
	ctx context.Context,
	musicID uint,
	explicitAnalysis *domain.MusicAudioAnalysis,
) (*domain.MusicPresetClassification, error) {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var music domain.Music
		if err := tx.First(&music, musicID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMusicNotFound
		} else if err != nil {
			return err
		}
		analysis := explicitAnalysis
		if analysis == nil {
			var err error
			analysis, err = latestMusicAudioAnalysis(tx, &music)
			if err != nil {
				return err
			}
		}
		if analysis != nil && (music.FileHash == "" || !strings.EqualFold(music.FileHash, analysis.FileHash)) {
			return ErrPresetAnalysisMismatch
		}
		audioEvidence, err := domain.DecodePresetAudioEvidence(analysis)
		if err != nil {
			return err
		}
		return replaceMusicPresetProjectionWithAudio(tx, &music, audioEvidence, r.policy)
	}); err != nil {
		return nil, err
	}
	return r.findOne(ctx, musicID)
}

func (r *presetRepository) SetManualPreset(ctx context.Context, musicID, administratorID uint, preset string) (*domain.MusicPresetClassification, error) {
	values, err := r.SetManualPresets(ctx, []uint{musicID}, administratorID, &preset)
	if err != nil {
		return nil, err
	}
	return values[musicID], nil
}

func (r *presetRepository) ClearManualPreset(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error) {
	values, err := r.SetManualPresets(ctx, []uint{musicID}, 0, nil)
	if err != nil {
		return nil, err
	}
	return values[musicID], nil
}

func (r *presetRepository) SetManualPresets(
	ctx context.Context,
	musicIDs []uint,
	administratorID uint,
	preset *string,
) (map[uint]*domain.MusicPresetClassification, error) {
	if len(musicIDs) == 0 || len(musicIDs) > domain.MaxPresetBatchSize {
		return nil, fmt.Errorf("manual preset batch must contain 1 to %d tracks", domain.MaxPresetBatchSize)
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var musics []*domain.Music
		if err := tx.Where("id IN ?", musicIDs).Find(&musics).Error; err != nil {
			return err
		}
		if len(musics) != len(musicIDs) {
			return ErrMusicNotFound
		}
		var classifiedIDs []uint
		if err := tx.Model(&domain.MusicPresetClassification{}).Where("music_id IN ?", musicIDs).
			Pluck("music_id", &classifiedIDs).Error; err != nil {
			return err
		}
		classified := make(map[uint]struct{}, len(classifiedIDs))
		for _, id := range classifiedIDs {
			classified[id] = struct{}{}
		}
		for _, music := range musics {
			if _, exists := classified[music.ID]; exists {
				continue
			}
			analysis, err := latestMusicAudioAnalysis(tx, music)
			if err != nil {
				return err
			}
			evidence, err := domain.DecodePresetAudioEvidence(analysis)
			if err != nil {
				return err
			}
			if err := replaceMusicPresetProjectionWithAudio(tx, music, evidence, r.policy); err != nil {
				return err
			}
		}
		updates := map[string]any{"manual_preset": nil, "manual_updated_by": nil, "manual_updated_at": nil}
		if preset != nil {
			now := time.Now().UTC()
			updates = map[string]any{"manual_preset": *preset, "manual_updated_by": administratorID, "manual_updated_at": now}
		}
		result := tx.Model(&domain.MusicPresetClassification{}).Where("music_id IN ?", musicIDs).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("update manual preset batch: %w", result.Error)
		}
		if result.RowsAffected != int64(len(musicIDs)) {
			return ErrPresetClassificationNotFound
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return r.FindByMusicIDs(ctx, musicIDs)
}

func (r *presetRepository) findOne(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error) {
	values, err := r.FindByMusicIDs(ctx, []uint{musicID})
	if err != nil {
		return nil, err
	}
	classification := values[musicID]
	if classification == nil {
		return nil, ErrPresetClassificationNotFound
	}
	return classification, nil
}

// replaceMusicPresetProjection updates only automatic columns on conflict.
// The manual override columns are deliberately absent from AssignmentColumns.
func replaceMusicPresetProjection(tx *gorm.DB, music *domain.Music, policy domain.PresetRulePolicy) error {
	return replaceMusicPresetProjectionWithAudio(tx, music, nil, policy)
}

func replaceMusicPresetProjectionWithAudio(
	tx *gorm.DB,
	music *domain.Music,
	audio *domain.PresetAudioEvidence,
	policy domain.PresetRulePolicy,
) error {
	classification, scores := domain.BuildMusicPresetProjectionWithAudio(music, audio, policy)
	if classification == nil {
		return nil
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "music_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"primary_preset", "automatic_preset", "confidence", "status", "rule_version",
			"metadata_revision", "audio_analysis_id", "evidence_summary", "evaluated_at", "updated_at",
		}),
	}).Create(classification).Error; err != nil {
		return fmt.Errorf("upsert preset classification: %w", err)
	}
	if err := tx.Where("music_id = ?", music.ID).Delete(&domain.MusicPresetScore{}).Error; err != nil {
		return fmt.Errorf("replace preset scores: %w", err)
	}
	if len(scores) > 0 {
		if err := tx.Create(&scores).Error; err != nil {
			return fmt.Errorf("create preset scores: %w", err)
		}
	}
	return nil
}

func latestMusicAudioAnalysis(tx *gorm.DB, music *domain.Music) (*domain.MusicAudioAnalysis, error) {
	if music == nil || music.ID == 0 || music.FileHash == "" {
		return nil, nil
	}
	var job domain.MusicAnalysisJob
	result := tx.Preload("Analysis").Where(
		"music_id = ? AND kind = ? AND status = ? AND analysis_id IS NOT NULL AND LOWER(file_hash) = ?",
		music.ID, domain.AnalysisJobKindAudio, domain.AnalysisStatusSucceeded, strings.ToLower(music.FileHash),
	).Order("id DESC").Limit(1).Find(&job)
	if result.Error != nil {
		return nil, fmt.Errorf("find latest audio analysis for classification: %w", result.Error)
	}
	if result.RowsAffected == 0 || job.Analysis == nil || job.Analysis.Status != domain.AnalysisStatusSucceeded {
		return nil, nil
	}
	return job.Analysis, nil
}

func deleteMusicPresetProjection(tx *gorm.DB, musicIDs []uint) error {
	if len(musicIDs) == 0 {
		return nil
	}
	if err := tx.Where("music_id IN ?", musicIDs).Delete(&domain.MusicPresetScore{}).Error; err != nil {
		return fmt.Errorf("delete music preset scores: %w", err)
	}
	if err := tx.Where("music_id IN ?", musicIDs).Delete(&domain.MusicPresetClassification{}).Error; err != nil {
		return fmt.Errorf("delete music preset classifications: %w", err)
	}
	return nil
}
