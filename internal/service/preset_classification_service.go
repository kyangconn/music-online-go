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
	ErrInvalidPresetBatch     = errors.New("invalid preset batch")
)

type PresetClassificationService interface {
	ListPresets(ctx context.Context) ([]domain.PresetSummary, error)
	Reclassify(ctx context.Context, musicID uint) (*domain.PresetClassificationResponse, error)
	SetManualPreset(ctx context.Context, musicID, administratorID uint, preset string) (*domain.PresetClassificationResponse, error)
	ClearManualPreset(ctx context.Context, musicID uint) (*domain.PresetClassificationResponse, error)
	SetManualPresets(ctx context.Context, administratorID uint, request domain.BatchPresetOverrideRequest) (*domain.BatchPresetOverrideResponse, error)
}

type presetClassificationService struct {
	repo    repository.PresetRepository
	enabled bool
}

func NewPresetClassificationService(repo repository.PresetRepository, enabled bool) PresetClassificationService {
	return &presetClassificationService{repo: repo, enabled: enabled}
}

func (s *presetClassificationService) ListPresets(ctx context.Context) ([]domain.PresetSummary, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	return s.repo.ListSummaries(ctx)
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

func (s *presetClassificationService) SetManualPresets(
	ctx context.Context,
	administratorID uint,
	request domain.BatchPresetOverrideRequest,
) (*domain.BatchPresetOverrideResponse, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if len(request.MusicIDs) == 0 || len(request.MusicIDs) > domain.MaxPresetBatchSize {
		return nil, fmt.Errorf("%w: manual preset batch must contain 1 to %d tracks", ErrInvalidPresetBatch, domain.MaxPresetBatchSize)
	}
	if request.Preset != nil && !domain.IsPresetID(*request.Preset) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPreset, *request.Preset)
	}
	seen := make(map[uint]struct{}, len(request.MusicIDs))
	for _, musicID := range request.MusicIDs {
		if musicID == 0 {
			return nil, fmt.Errorf("%w: music IDs must be positive", ErrInvalidPresetBatch)
		}
		if _, duplicate := seen[musicID]; duplicate {
			return nil, fmt.Errorf("%w: music ID %d appears more than once", ErrInvalidPresetBatch, musicID)
		}
		seen[musicID] = struct{}{}
	}
	values, err := s.repo.SetManualPresets(ctx, request.MusicIDs, administratorID, request.Preset)
	if err != nil {
		return nil, err
	}
	response := &domain.BatchPresetOverrideResponse{
		Updated: len(request.MusicIDs), Classifications: make([]*domain.PresetClassificationResponse, 0, len(request.MusicIDs)),
	}
	for _, musicID := range request.MusicIDs {
		classification := values[musicID]
		if classification == nil {
			return nil, repository.ErrPresetClassificationNotFound
		}
		response.Classifications = append(response.Classifications, classification.ToResponse())
	}
	return response, nil
}

func (s *presetClassificationService) guard() error {
	if !s.enabled || s.repo == nil {
		return ErrClassificationDisabled
	}
	return nil
}
