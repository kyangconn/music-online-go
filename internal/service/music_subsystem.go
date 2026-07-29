package service

import (
	"github.com/kyangconn/music-online-go/internal/config"
	"github.com/kyangconn/music-online-go/internal/mediafs"
	"github.com/kyangconn/music-online-go/internal/repository"
)

// MusicSubsystemRepositories names the persistence boundaries that jointly
// implement music storage, scanning, classification and derived analysis.
// Keeping this graph explicit makes omissions visible at the composition root.
type MusicSubsystemRepositories struct {
	Music        repository.MusicRepository
	Preset       repository.PresetRepository
	Analysis     repository.MusicAnalysisRepository
	MediaLibrary repository.MediaLibraryRepository
}

// MusicSubsystem is returned only after the scanner/analysis dependency cycle
// has been completed. Callers never receive a service that still needs setter
// injection before it is safe to use.
type MusicSubsystem struct {
	Music        MusicService
	MediaLibrary MediaLibraryService
	Analysis     MusicAnalysisService
}

// NewMusicSubsystem wires the production media prober and a validated config
// snapshot into the complete music subsystem.
func NewMusicSubsystem(repos MusicSubsystemRepositories, cfg config.Config) MusicSubsystem {
	return newMusicSubsystemWithProber(repos, cfg, mediafs.NewSystemProber())
}

func newMusicSubsystemWithProber(repos MusicSubsystemRepositories, cfg config.Config, prober mediafs.Prober) MusicSubsystem {
	library := newMediaLibraryService(repos.MediaLibrary, repos.Music, cfg.Library, cfg.Server, prober)
	analysis := NewMusicAnalysisService(repos.Analysis, repos.Preset, library, cfg.Classification)

	// The incomplete concrete value is private and cannot be observed until the
	// return below; this is the only assignment needed to close the real domain
	// cycle (analysis resolves media paths, scanner schedules analysis).
	library.analyzer = analysis
	music := NewMusicService(repos.Music, library, cfg, repos.Preset, repos.Analysis, analysis)

	return MusicSubsystem{Music: music, MediaLibrary: library, Analysis: analysis}
}
