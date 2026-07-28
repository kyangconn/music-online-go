package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
)

var (
	ErrClassificationDisabled = errors.New("preset classification is disabled")
	ErrInvalidPreset          = errors.New("invalid preset")
)

type PresetClassificationService interface {
	Reclassify(ctx context.Context, musicID uint) (*domain.PresetClassificationResponse, error)
	SetManualPreset(ctx context.Context, musicID, administratorID uint, preset string) (*domain.PresetClassificationResponse, error)
	ClearManualPreset(ctx context.Context, musicID uint) (*domain.PresetClassificationResponse, error)
}

type presetClassificationService struct {
	repo    repository.PresetRepository
	enabled bool
}

func NewPresetClassificationService(repo repository.PresetRepository, enabled bool) PresetClassificationService {
	return &presetClassificationService{repo: repo, enabled: enabled}
}

func (s *presetClassificationService) Reclassify(ctx context.Context, musicID uint) (*domain.PresetClassificationResponse, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	classification, err := s.repo.Reclassify(ctx, musicID)
	if err != nil {
		return nil, err
	}
	return classification.ToResponse(), nil
}

func (s *presetClassificationService) SetManualPreset(
	ctx context.Context,
	musicID, administratorID uint,
	preset string,
) (*domain.PresetClassificationResponse, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if !domain.IsPresetID(preset) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPreset, preset)
	}
	classification, err := s.repo.SetManualPreset(ctx, musicID, administratorID, preset)
	if err != nil {
		return nil, err
	}
	return classification.ToResponse(), nil
}

func (s *presetClassificationService) ClearManualPreset(ctx context.Context, musicID uint) (*domain.PresetClassificationResponse, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	classification, err := s.repo.ClearManualPreset(ctx, musicID)
	if err != nil {
		return nil, err
	}
	return classification.ToResponse(), nil
}

func (s *presetClassificationService) guard() error {
	if !s.enabled || s.repo == nil {
		return ErrClassificationDisabled
	}
	return nil
}
