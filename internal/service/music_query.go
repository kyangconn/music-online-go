// Package service music_query.go - 音乐查询服务
package service

import (
	"context"

	"github.com/kyangconn/music-online-go/internal/domain"
)

func (s *musicService) GetByID(ctx context.Context, id uint, currentUserID *uint) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := s.toResponse(music)
	if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *musicService) GetByIDs(ctx context.Context, ids []uint, currentUserID *uint) ([]*domain.MusicResponse, error) {
	musics, err := s.repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return s.toEnrichedResponses(ctx, musics, currentUserID)
}

func (s *musicService) Search(ctx context.Context, params *domain.MusicSearchParams, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.Search(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	responses, err := s.toEnrichedResponses(ctx, musics, currentUserID)
	if err != nil {
		return nil, 0, err
	}

	return responses, total, nil
}

func (s *musicService) FindByMusicBrainzRecordingID(ctx context.Context, recordingID string, currentUserID *uint) (*domain.MusicResponse, error) {
	normalized, err := normalizeMBID("musicbrainz_recording_id", recordingID)
	if err != nil {
		return nil, err
	}
	music, err := s.repo.FindByMusicBrainzRecordingID(ctx, normalized)
	if err != nil {
		return nil, err
	}
	resp := s.toResponse(music)
	if err := s.enrichMusicResponse(ctx, resp, currentUserID); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *musicService) FindMetadataCandidates(ctx context.Context, metadata domain.MusicMetadata, currentUserID *uint) ([]*domain.MusicResponse, bool, error) {
	probe, err := normalizedMusicMetadata(metadata)
	if err != nil {
		return nil, false, err
	}

	stableMatches, err := s.repo.FindByStableMetadataIDs(ctx, probe.MusicBrainzRecordingID, probe.MusicBrainzTrackID, 5)
	if err != nil {
		return nil, false, err
	}
	if len(stableMatches) > 0 {
		responses, err := s.toEnrichedResponses(ctx, stableMatches, currentUserID)
		return responses, true, err
	}

	textMatches, err := s.repo.FindByTitleAndArtist(ctx, probe.Title, probe.Artist, 5)
	if err != nil {
		return nil, false, err
	}
	responses, err := s.toEnrichedResponses(ctx, textMatches, currentUserID)
	return responses, false, err
}

func (s *musicService) ListFilterOptions(ctx context.Context) (*domain.MusicFilterOptions, error) {
	return s.repo.ListFilterOptions(ctx)
}

func (s *musicService) CountWithMetadata(ctx context.Context) (int64, error) {
	return s.repo.CountWithMetadata(ctx)
}
func (s *musicService) ListByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.ListByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses, err := s.toEnrichedResponses(ctx, musics, currentUserID)
	if err != nil {
		return nil, 0, err
	}

	return responses, total, nil
}
func (s *musicService) ListLikedByUserID(ctx context.Context, userID uint, page, pageSize int, currentUserID *uint) ([]*domain.MusicResponse, int64, error) {
	musics, total, err := s.repo.ListLikedByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses, err := s.toEnrichedResponses(ctx, musics, currentUserID)
	if err != nil {
		return nil, 0, err
	}

	return responses, total, nil
}

// toEnrichedResponses 批量转换并填充音乐响应列表。
// 统一 Search / ListByUserID / ListLikedByUserID 的「ToResponse + enrich」循环，避免重复逻辑。
func (s *musicService) toEnrichedResponses(ctx context.Context, musics []*domain.Music, currentUserID *uint) ([]*domain.MusicResponse, error) {
	ids := make([]uint, 0, len(musics))
	for _, music := range musics {
		ids = append(ids, music.ID)
	}
	engagement, err := s.repo.ListEngagementByMusicIDs(ctx, ids, currentUserID)
	if err != nil {
		return nil, err
	}
	classifications, err := s.presetRepo.FindByMusicIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	analysisJobs, err := s.analysisRepo.LatestAudioJobsByMusicIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	responses := make([]*domain.MusicResponse, 0, len(musics))
	for _, m := range musics {
		resp := s.toResponse(m)
		values := engagement[m.ID]
		resp.LikeCount = values.LikeCount
		resp.IsLiked = values.IsLiked
		resp.PresetClassification = classifications[m.ID].ToResponse()
		resp.AudioAnalysis = analysisJobs[m.ID].ToSummary()
		responses = append(responses, resp)
	}
	return responses, nil
}

// enrichMusicResponse populates one detail response. List responses use the
// repository's batch path to keep playlist and library reads bounded.
func (s *musicService) enrichMusicResponse(ctx context.Context, resp *domain.MusicResponse, currentUserID *uint) error {
	count, err := s.repo.CountLikes(ctx, resp.ID)
	if err != nil {
		return err
	}
	resp.LikeCount = count

	if currentUserID != nil {
		liked, err := s.repo.IsLiked(ctx, *currentUserID, resp.ID)
		if err != nil {
			return err
		}
		resp.IsLiked = liked
	} else {
		resp.IsLiked = false
	}
	classifications, err := s.presetRepo.FindByMusicIDs(ctx, []uint{resp.ID})
	if err != nil {
		return err
	}
	resp.PresetClassification = classifications[resp.ID].ToResponse()
	jobs, err := s.analysisRepo.LatestAudioJobsByMusicIDs(ctx, []uint{resp.ID})
	if err != nil {
		return err
	}
	resp.AudioAnalysis = jobs[resp.ID].ToSummary()
	return nil
}
