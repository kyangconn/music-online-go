// Package service music_duplicates.go - 音乐查重与元数据补全
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
)

func (s *musicService) CheckDuplicates(
	ctx context.Context,
	userID uint,
	role string,
	req *domain.MusicDuplicateCheckRequest,
) (*domain.MusicDuplicateCheckResponse, error) {
	incoming, err := normalizedMusicMetadata(req.Metadata())
	if err != nil {
		return nil, err
	}
	response := &domain.MusicDuplicateCheckResponse{
		MetadataMatches:   []*domain.MusicResponse{},
		SuggestedMetadata: incoming,
	}

	var exact *domain.Music
	fileHash := strings.ToLower(strings.TrimSpace(req.FileHash))
	if fileHash != "" {
		match, err := s.repo.FindByFileHash(ctx, fileHash)
		if err != nil && !errors.Is(err, repository.ErrMusicNotFound) {
			return nil, err
		}
		exact = match
		if exact != nil {
			response.ExactMatch = s.toResponse(exact)
			if canManageMusic(exact, userID, role) {
				response.Enrichment = buildMetadataEnrichment(exact, incoming)
			}
		}
	}

	matches, err := s.repo.FindByStableMetadataIDs(ctx, incoming.MusicBrainzRecordingID, incoming.MusicBrainzTrackID, 5)
	if err != nil {
		return nil, err
	}
	textMatches, err := s.repo.FindByTitleAndArtist(ctx, incoming.Title, incoming.Artist, 5)
	if err != nil {
		return nil, err
	}
	seenMatches := make(map[uint]struct{}, len(matches)+len(textMatches))
	for _, match := range append(matches, textMatches...) {
		if exact != nil && match.ID == exact.ID {
			continue
		}
		if _, exists := seenMatches[match.ID]; exists {
			continue
		}
		seenMatches[match.ID] = struct{}{}
		response.MetadataMatches = append(response.MetadataMatches, s.toResponse(match))
	}

	response.SuggestedMetadata = buildSuggestedMetadata(incoming, exact, matches)
	return response, nil
}
func buildMetadataEnrichment(existing *domain.Music, incoming domain.MusicMetadata) *domain.UpdateMusicRequest {
	patch := &domain.UpdateMusicRequest{}
	changed := false
	if artistListCanBeEnriched(existing.Artist, existing.Artists, incoming.Artists) {
		patch.Artists = stringListPointer(incoming.Artists)
		changed = true
	}
	if existing.Album == "" && incoming.Album != "" {
		patch.Album = &incoming.Album
		changed = true
	}
	if existing.AlbumArtist == "" && incoming.AlbumArtist != "" {
		patch.AlbumArtist = &incoming.AlbumArtist
		changed = true
	}
	if artistListCanBeEnriched(existing.AlbumArtist, existing.AlbumArtists, incoming.AlbumArtists) {
		patch.AlbumArtists = stringListPointer(incoming.AlbumArtists)
		changed = true
	}
	if existing.Year == 0 && incoming.Year > 0 {
		patch.Year = &incoming.Year
		changed = true
	}
	if existing.TrackNumber == 0 && incoming.TrackNumber > 0 {
		patch.TrackNumber = &incoming.TrackNumber
		changed = true
	}
	if existing.TrackTotal == 0 && incoming.TrackTotal > 0 {
		patch.TrackTotal = &incoming.TrackTotal
		changed = true
	}
	if existing.DiscNumber == 0 && incoming.DiscNumber > 0 {
		patch.DiscNumber = &incoming.DiscNumber
		changed = true
	}
	if existing.DiscTotal == 0 && incoming.DiscTotal > 0 {
		patch.DiscTotal = &incoming.DiscTotal
		changed = true
	}
	if existing.ReleaseDate == "" && incoming.ReleaseDate != "" {
		patch.ReleaseDate = &incoming.ReleaseDate
		changed = true
	}
	if existing.OriginalReleaseDate == "" && incoming.OriginalReleaseDate != "" {
		patch.OriginalReleaseDate = &incoming.OriginalReleaseDate
		changed = true
	}
	if existing.Genre == "" && len(existing.Genres) == 0 && len(incoming.Genres) > 0 {
		patch.Genres = stringListPointer(incoming.Genres)
		changed = true
	}
	if existing.Comment == "" && incoming.Comment != "" {
		patch.Comment = &incoming.Comment
		changed = true
	}
	if len(existing.ISRCs) == 0 && len(incoming.ISRCs) > 0 {
		patch.ISRCs = stringListPointer(incoming.ISRCs)
		changed = true
	}
	if existing.Duration == 0 && incoming.Duration > 0 {
		patch.Duration = &incoming.Duration
		changed = true
	}
	if existing.MusicBrainzRecordingID == "" && incoming.MusicBrainzRecordingID != "" {
		patch.MusicBrainzRecordingID = &incoming.MusicBrainzRecordingID
		changed = true
	}
	if existing.MusicBrainzTrackID == "" && incoming.MusicBrainzTrackID != "" {
		patch.MusicBrainzTrackID = &incoming.MusicBrainzTrackID
		changed = true
	}
	if existing.MusicBrainzReleaseID == "" && incoming.MusicBrainzReleaseID != "" {
		patch.MusicBrainzReleaseID = &incoming.MusicBrainzReleaseID
		changed = true
	}
	if existing.MusicBrainzReleaseGroupID == "" && incoming.MusicBrainzReleaseGroupID != "" {
		patch.MusicBrainzReleaseGroupID = &incoming.MusicBrainzReleaseGroupID
		changed = true
	}
	if len(existing.MusicBrainzArtistIDs) == 0 && len(incoming.MusicBrainzArtistIDs) > 0 {
		patch.MusicBrainzArtistIDs = stringListPointer(incoming.MusicBrainzArtistIDs)
		changed = true
	}
	if len(existing.MusicBrainzAlbumArtistIDs) == 0 && len(incoming.MusicBrainzAlbumArtistIDs) > 0 {
		patch.MusicBrainzAlbumArtistIDs = stringListPointer(incoming.MusicBrainzAlbumArtistIDs)
		changed = true
	}
	if !changed {
		return nil
	}
	return patch
}

func buildSuggestedMetadata(incoming domain.MusicMetadata, exact *domain.Music, matches []*domain.Music) domain.MusicMetadata {
	best := exact
	for _, candidate := range matches {
		if best == nil || metadataCompleteness(candidate) > metadataCompleteness(best) {
			best = candidate
		}
	}
	if best == nil {
		return incoming
	}
	if incoming.Album == "" {
		incoming.Album = best.Album
	}
	if len(incoming.Artists) == 0 ||
		(len(incoming.Artists) == 1 && strings.EqualFold(incoming.Artists[0], incoming.Artist) && len(best.Artists) > 1) {
		incoming.Artists = append(domain.StringList{}, best.Artists...)
	}
	if incoming.AlbumArtist == "" {
		incoming.AlbumArtist = best.AlbumArtist
	}
	if len(incoming.AlbumArtists) == 0 ||
		(len(incoming.AlbumArtists) == 1 && strings.EqualFold(incoming.AlbumArtists[0], incoming.AlbumArtist) && len(best.AlbumArtists) > 1) {
		incoming.AlbumArtists = append(domain.StringList{}, best.AlbumArtists...)
	}
	if incoming.Year == 0 {
		incoming.Year = best.Year
	}
	if incoming.TrackNumber == 0 {
		incoming.TrackNumber = best.TrackNumber
	}
	if incoming.TrackTotal == 0 {
		incoming.TrackTotal = best.TrackTotal
	}
	if incoming.DiscNumber == 0 {
		incoming.DiscNumber = best.DiscNumber
	}
	if incoming.DiscTotal == 0 {
		incoming.DiscTotal = best.DiscTotal
	}
	if incoming.ReleaseDate == "" {
		incoming.ReleaseDate = best.ReleaseDate
	}
	if incoming.OriginalReleaseDate == "" {
		incoming.OriginalReleaseDate = best.OriginalReleaseDate
	}
	if incoming.Genre == "" && len(incoming.Genres) == 0 {
		incoming.Genre = best.Genre
		incoming.Genres = append(domain.StringList{}, best.Genres...)
	}
	if incoming.Comment == "" {
		incoming.Comment = best.Comment
	}
	if len(incoming.ISRCs) == 0 {
		incoming.ISRCs = append(domain.StringList{}, best.ISRCs...)
	}
	if incoming.Duration == 0 {
		incoming.Duration = best.Duration
	}
	if incoming.MusicBrainzRecordingID == "" {
		incoming.MusicBrainzRecordingID = best.MusicBrainzRecordingID
	}
	if incoming.MusicBrainzTrackID == "" {
		incoming.MusicBrainzTrackID = best.MusicBrainzTrackID
	}
	if incoming.MusicBrainzReleaseID == "" {
		incoming.MusicBrainzReleaseID = best.MusicBrainzReleaseID
	}
	if incoming.MusicBrainzReleaseGroupID == "" {
		incoming.MusicBrainzReleaseGroupID = best.MusicBrainzReleaseGroupID
	}
	if len(incoming.MusicBrainzArtistIDs) == 0 {
		incoming.MusicBrainzArtistIDs = append(domain.StringList{}, best.MusicBrainzArtistIDs...)
	}
	if len(incoming.MusicBrainzAlbumArtistIDs) == 0 {
		incoming.MusicBrainzAlbumArtistIDs = append(domain.StringList{}, best.MusicBrainzAlbumArtistIDs...)
	}
	return incoming
}

func metadataCompleteness(music *domain.Music) int {
	score := 0
	if music.Album != "" {
		score++
	}
	if len(music.Artists) > 1 {
		score++
	}
	if music.AlbumArtist != "" || len(music.AlbumArtists) > 0 {
		score++
	}
	if music.Year > 0 {
		score++
	}
	if music.TrackNumber > 0 {
		score++
	}
	if music.TrackTotal > 0 {
		score++
	}
	if music.DiscNumber > 0 || music.DiscTotal > 0 {
		score++
	}
	if music.ReleaseDate != "" || music.OriginalReleaseDate != "" {
		score++
	}
	if music.Genre != "" || len(music.Genres) > 0 {
		score++
	}
	if music.Comment != "" || len(music.ISRCs) > 0 {
		score++
	}
	if music.Duration > 0 {
		score++
	}
	if music.MusicBrainzRecordingID != "" || music.MusicBrainzTrackID != "" {
		score += 2
	}
	if music.MusicBrainzReleaseID != "" || music.MusicBrainzReleaseGroupID != "" {
		score++
	}
	if len(music.MusicBrainzArtistIDs) > 0 || len(music.MusicBrainzAlbumArtistIDs) > 0 {
		score++
	}
	return score
}

func normalizedMusicMetadata(metadata domain.MusicMetadata) (domain.MusicMetadata, error) {
	music := &domain.Music{}
	request := &domain.CreateMusicRequest{
		Title:                     metadata.Title,
		Artist:                    metadata.Artist,
		Artists:                   metadata.Artists,
		Album:                     metadata.Album,
		AlbumArtist:               metadata.AlbumArtist,
		AlbumArtists:              metadata.AlbumArtists,
		Year:                      metadata.Year,
		TrackNumber:               metadata.TrackNumber,
		TrackTotal:                metadata.TrackTotal,
		DiscNumber:                metadata.DiscNumber,
		DiscTotal:                 metadata.DiscTotal,
		ReleaseDate:               metadata.ReleaseDate,
		OriginalReleaseDate:       metadata.OriginalReleaseDate,
		Genre:                     metadata.Genre,
		Genres:                    metadata.Genres,
		Comment:                   metadata.Comment,
		ISRCs:                     metadata.ISRCs,
		Duration:                  metadata.Duration,
		MusicBrainzRecordingID:    metadata.MusicBrainzRecordingID,
		MusicBrainzTrackID:        metadata.MusicBrainzTrackID,
		MusicBrainzReleaseID:      metadata.MusicBrainzReleaseID,
		MusicBrainzReleaseGroupID: metadata.MusicBrainzReleaseGroupID,
		MusicBrainzArtistIDs:      metadata.MusicBrainzArtistIDs,
		MusicBrainzAlbumArtistIDs: metadata.MusicBrainzAlbumArtistIDs,
	}
	if err := applyCreateMusicMetadata(music, request); err != nil {
		return domain.MusicMetadata{}, err
	}
	return musicMetadataFromMusic(music), nil
}

func artistListCanBeEnriched(credited string, existing, incoming domain.StringList) bool {
	if len(incoming) == 0 {
		return false
	}
	if len(existing) == 0 {
		return true
	}
	return len(existing) == 1 && strings.EqualFold(existing[0], credited) && len(incoming) > 1
}

func stringListPointer(values domain.StringList) *domain.StringList {
	copy := append(domain.StringList{}, values...)
	return &copy
}
