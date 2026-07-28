package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPresetClassificationNotFound = errors.New("preset classification not found")

type PresetRepository interface {
	FindByMusicIDs(ctx context.Context, musicIDs []uint) (map[uint]*domain.MusicPresetClassification, error)
	Reclassify(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error)
	SetManualPreset(ctx context.Context, musicID, administratorID uint, preset string) (*domain.MusicPresetClassification, error)
	ClearManualPreset(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error)
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

func (r *presetRepository) Reclassify(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error) {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var music domain.Music
		if err := tx.First(&music, musicID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMusicNotFound
		} else if err != nil {
			return err
		}
		return replaceMusicPresetProjection(tx, &music, r.policy)
	}); err != nil {
		return nil, err
	}
	return r.findOne(ctx, musicID)
}

func (r *presetRepository) SetManualPreset(ctx context.Context, musicID, administratorID uint, preset string) (*domain.MusicPresetClassification, error) {
	if err := r.ensureClassification(ctx, musicID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&domain.MusicPresetClassification{}).Where("music_id = ?", musicID).
		Updates(map[string]any{"manual_preset": preset, "manual_updated_by": administratorID, "manual_updated_at": now})
	if result.Error != nil {
		return nil, fmt.Errorf("set manual preset: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrPresetClassificationNotFound
	}
	return r.findOne(ctx, musicID)
}

func (r *presetRepository) ClearManualPreset(ctx context.Context, musicID uint) (*domain.MusicPresetClassification, error) {
	if err := r.ensureClassification(ctx, musicID); err != nil {
		return nil, err
	}
	result := r.db.WithContext(ctx).Model(&domain.MusicPresetClassification{}).Where("music_id = ?", musicID).
		Updates(map[string]any{"manual_preset": nil, "manual_updated_by": nil, "manual_updated_at": nil})
	if result.Error != nil {
		return nil, fmt.Errorf("clear manual preset: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrPresetClassificationNotFound
	}
	return r.findOne(ctx, musicID)
}

func (r *presetRepository) ensureClassification(ctx context.Context, musicID uint) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.MusicPresetClassification{}).
		Where("music_id = ?", musicID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := r.Reclassify(ctx, musicID)
	return err
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
	classification, scores := domain.BuildMusicPresetProjection(music, policy)
	if classification == nil {
		return nil
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "music_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"primary_preset", "automatic_preset", "confidence", "status", "rule_version",
			"metadata_revision", "evidence_summary", "evaluated_at", "updated_at",
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
