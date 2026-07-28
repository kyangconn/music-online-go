package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
)

func openBrowseRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "browse.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open browse database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get browse database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close browse database: %v", err)
		}
	})
	if err := db.AutoMigrate(
		&domain.User{}, &domain.Music{}, &domain.UserMusicLike{}, &domain.MusicArtistCredit{}, &domain.MusicAlbumMembership{},
		&domain.MusicGenreFacet{}, &domain.Playlist{}, &domain.PlaylistItem{}, &domain.MediaFile{},
		&domain.MusicPresetClassification{}, &domain.MusicPresetScore{},
	); err != nil {
		t.Fatalf("migrate browse database: %v", err)
	}
	return db
}

func TestMusicRepositoryLoadsEngagementInBatch(t *testing.T) {
	db := openBrowseRepositoryTestDB(t)
	repo := NewMusicRepository(db)
	ctx := context.Background()
	tracks := []*domain.Music{{Title: "One", Artist: "Artist"}, {Title: "Two", Artist: "Artist"}}
	for _, track := range tracks {
		if err := repo.Create(ctx, track); err != nil {
			t.Fatalf("create track: %v", err)
		}
	}
	for _, like := range []domain.UserMusicLike{
		{UserID: 7, MusicID: tracks[0].ID},
		{UserID: 8, MusicID: tracks[0].ID},
		{UserID: 8, MusicID: tracks[1].ID},
	} {
		if err := db.Create(&like).Error; err != nil {
			t.Fatalf("create like: %v", err)
		}
	}
	currentUserID := uint(7)
	engagement, err := repo.ListEngagementByMusicIDs(ctx, []uint{tracks[0].ID, tracks[1].ID}, &currentUserID)
	if err != nil {
		t.Fatalf("load engagement: %v", err)
	}
	if engagement[tracks[0].ID].LikeCount != 2 || !engagement[tracks[0].ID].IsLiked ||
		engagement[tracks[1].ID].LikeCount != 1 || engagement[tracks[1].ID].IsLiked {
		t.Fatalf("engagement = %+v", engagement)
	}
}

func TestMusicRepositoryReplacesBrowseProjectionWithCanonicalMetadata(t *testing.T) {
	db := openBrowseRepositoryTestDB(t)
	repo := NewMusicRepository(db)
	ctx := context.Background()
	music := &domain.Music{
		Title: "Track", Artist: "Original Artist", Artists: domain.StringList{"Original Artist"},
		Album: "Original Album", AlbumArtist: "Original Artist", Genres: domain.StringList{"Ambient / Electronic"},
	}
	if err := repo.Create(ctx, music); err != nil {
		t.Fatalf("create music: %v", err)
	}
	originalAlbumKey := domain.BuildMusicBrowseProjection(music).AlbumMembership.GroupKey

	music.Artist = "Replacement Artist"
	music.Artists = domain.StringList{"Replacement Artist"}
	music.Album = "Replacement Album"
	music.AlbumArtist = "Replacement Artist"
	music.Genres = domain.StringList{"Trance"}
	if err := repo.Update(ctx, music); err != nil {
		t.Fatalf("update music: %v", err)
	}

	var oldRows int64
	if err := db.Model(&domain.MusicAlbumMembership{}).Where("group_key = ?", originalAlbumKey).Count(&oldRows).Error; err != nil {
		t.Fatalf("count old projection: %v", err)
	}
	if oldRows != 0 {
		t.Fatalf("old album projection survived metadata replacement: %d", oldRows)
	}
	params := &domain.MusicSearchParams{Genre: "trance", Album: "replacement album", Artist: "replacement artist", Page: 1, PageSize: 10}
	items, total, err := repo.Search(ctx, params)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != music.ID {
		t.Fatalf("search by replacement facets: total=%d items=%+v err=%v", total, items, err)
	}
	params.Genre = "ambient"
	if _, total, err = repo.Search(ctx, params); err != nil || total != 0 {
		t.Fatalf("old genre still matched: total=%d err=%v", total, err)
	}
}

func TestBrowseRepositoryGroupsStableIdentitiesAndOrdersAlbumTracks(t *testing.T) {
	db := openBrowseRepositoryTestDB(t)
	musicRepo := NewMusicRepository(db)
	browseRepo := NewBrowseRepository(db)
	ctx := context.Background()
	artistID := "123e4567-e89b-42d3-a456-426614174010"
	releaseID := "123e4567-e89b-42d3-a456-426614174011"
	tracks := []*domain.Music{
		{Title: "Second", Artist: "ARTIST", Artists: domain.StringList{"ARTIST"}, MusicBrainzArtistIDs: domain.StringList{artistID},
			Album: "Release", AlbumArtist: "Artist", MusicBrainzReleaseID: releaseID, TrackNumber: 2, DiscNumber: 1, Duration: 120},
		{Title: "First", Artist: "Artist", Artists: domain.StringList{"Artist"}, MusicBrainzArtistIDs: domain.StringList{artistID},
			Album: "release", AlbumArtist: "Artist", MusicBrainzReleaseID: releaseID, TrackNumber: 1, DiscNumber: 1, Duration: 100, Img: "cover.jpg"},
	}
	for _, track := range tracks {
		if err := musicRepo.Create(ctx, track); err != nil {
			t.Fatalf("create track: %v", err)
		}
	}

	artists, total, err := browseRepo.ListArtists(ctx, domain.BrowseArtistParams{Page: 1, PageSize: 10})
	if err != nil || total != 1 || len(artists) != 1 || artists[0].TrackCount != 2 || artists[0].AlbumCount != 1 {
		t.Fatalf("artist aggregation: total=%d artists=%+v err=%v", total, artists, err)
	}
	albums, total, err := browseRepo.ListAlbums(ctx, domain.BrowseAlbumParams{Page: 1, PageSize: 10})
	if err != nil || total != 1 || len(albums) != 1 || albums[0].TrackCount != 2 || albums[0].TotalDuration != 220 || albums[0].CoverMusicID == nil {
		t.Fatalf("album aggregation: total=%d albums=%+v err=%v", total, albums, err)
	}
	filters, err := musicRepo.ListFilterOptions(ctx)
	if err != nil || len(filters.Artists) != 1 || len(filters.Albums) != 1 {
		t.Fatalf("normalized filter options = %+v, err=%v", filters, err)
	}

	albumKey := albums[0].Key
	ordered, total, err := musicRepo.Search(ctx, &domain.MusicSearchParams{AlbumKey: albumKey, Page: 1, PageSize: 10})
	if err != nil || total != 2 || len(ordered) != 2 || ordered[0].Title != "First" || ordered[1].Title != "Second" {
		t.Fatalf("album track order: total=%d tracks=%+v err=%v", total, ordered, err)
	}
}

func TestPlaylistRepositoryEnforcesOwnershipIdempotencyAndOrder(t *testing.T) {
	db := openBrowseRepositoryTestDB(t)
	musicRepo := NewMusicRepository(db)
	playlistRepo := NewPlaylistRepository(db)
	ctx := context.Background()
	for _, user := range []domain.User{
		{ID: 7, Username: "playlist-owner", Email: "playlist-owner@example.com", Password: "unused"},
		{ID: 8, Username: "playlist-other", Email: "playlist-other@example.com", Password: "unused"},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("create playlist user: %v", err)
		}
	}
	tracks := []*domain.Music{{Title: "One", Artist: "Artist"}, {Title: "Two", Artist: "Artist"}}
	for _, track := range tracks {
		if err := musicRepo.Create(ctx, track); err != nil {
			t.Fatalf("create track: %v", err)
		}
	}
	playlist := &domain.Playlist{UserID: 7, Name: "Private", Revision: 1}
	if err := playlistRepo.Create(ctx, playlist); err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	for _, musicID := range []uint{tracks[0].ID, tracks[1].ID, tracks[0].ID} {
		if err := playlistRepo.AddItem(ctx, playlist.ID, 7, musicID); err != nil {
			t.Fatalf("add playlist item: %v", err)
		}
	}
	items, err := playlistRepo.ListItems(ctx, playlist.ID, 7)
	if err != nil || len(items) != 2 || items[0].MusicID != tracks[0].ID || items[1].MusicID != tracks[1].ID {
		t.Fatalf("playlist items = %+v, err=%v", items, err)
	}
	if err := playlistRepo.ReorderItems(ctx, playlist.ID, 7, []uint{tracks[1].ID, tracks[0].ID}); err != nil {
		t.Fatalf("reorder playlist: %v", err)
	}
	items, _ = playlistRepo.ListItems(ctx, playlist.ID, 7)
	if items[0].MusicID != tracks[1].ID || items[1].MusicID != tracks[0].ID {
		t.Fatalf("reordered playlist items = %+v", items)
	}
	if err := playlistRepo.ReorderItems(ctx, playlist.ID, 7, []uint{tracks[0].ID}); !errors.Is(err, ErrPlaylistItemsMismatch) {
		t.Fatalf("incomplete reorder error = %v", err)
	}
	if _, err := playlistRepo.ListItems(ctx, playlist.ID, 8); !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("other user read error = %v", err)
	}

	if err := musicRepo.Delete(ctx, tracks[1].ID); err != nil {
		t.Fatalf("delete queued music: %v", err)
	}
	items, err = playlistRepo.ListItems(ctx, playlist.ID, 7)
	if err != nil || len(items) != 1 || items[0].MusicID != tracks[0].ID || items[0].Position != 0 {
		t.Fatalf("playlist after music deletion = %+v, err=%v", items, err)
	}
}

func TestPresetRepositoryKeepsManualOverrideAcrossAutomaticReclassification(t *testing.T) {
	db := openBrowseRepositoryTestDB(t)
	policy := domain.DefaultPresetRulePolicy()
	musicRepo := NewMusicRepository(db, policy)
	presetRepo := NewPresetRepository(db, policy)
	ctx := context.Background()
	music := &domain.Music{
		Title: "Cross-style track", Artist: "Artist", Genres: domain.StringList{"Dubstep"}, MetadataRevision: 1,
	}
	if err := musicRepo.Create(ctx, music); err != nil {
		t.Fatalf("create classified music: %v", err)
	}

	classifications, err := presetRepo.FindByMusicIDs(ctx, []uint{music.ID})
	if err != nil {
		t.Fatalf("find initial classification: %v", err)
	}
	initial := classifications[music.ID]
	if initial == nil || initial.AutomaticPreset != domain.PresetBassImpact || len(initial.Scores) != 4 {
		t.Fatalf("initial classification = %+v", initial)
	}

	manual, err := presetRepo.SetManualPreset(ctx, music.ID, 99, domain.PresetCalmFlow)
	if err != nil || manual.ManualPreset == nil || *manual.ManualPreset != domain.PresetCalmFlow {
		t.Fatalf("manual classification = %+v, err=%v", manual, err)
	}
	music.Genres = domain.StringList{"Trance"}
	music.MetadataRevision++
	if err := musicRepo.Update(ctx, music); err != nil {
		t.Fatalf("reclassify updated music: %v", err)
	}
	updated, err := presetRepo.FindByMusicIDs(ctx, []uint{music.ID})
	if err != nil {
		t.Fatalf("find updated classification: %v", err)
	}
	classification := updated[music.ID]
	if classification.AutomaticPreset != domain.PresetCosmicDrift || classification.ManualPreset == nil ||
		*classification.ManualPreset != domain.PresetCalmFlow {
		t.Fatalf("updated classification = %+v", classification)
	}

	rows, total, err := musicRepo.Search(ctx, &domain.MusicSearchParams{
		Preset: domain.PresetCalmFlow, Page: 1, PageSize: 10,
	})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != music.ID {
		t.Fatalf("manual preset filter: total=%d rows=%+v err=%v", total, rows, err)
	}
	if _, err := presetRepo.ClearManualPreset(ctx, music.ID); err != nil {
		t.Fatalf("clear manual preset: %v", err)
	}
	rows, total, err = musicRepo.Search(ctx, &domain.MusicSearchParams{
		Preset: domain.PresetCosmicDrift, PresetStatus: domain.PresetStatusClassified, Page: 1, PageSize: 10,
	})
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("automatic preset filter: total=%d rows=%+v err=%v", total, rows, err)
	}
}
