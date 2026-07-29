package service

import (
	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/mediametadata"
)

// scannedCover remains an alias while the scanner still lives in this package.
// The scanner extraction in the next commit removes this compatibility layer.
type scannedCover = mediametadata.Cover

func readScannedAudioMetadata(path string, maxTagBytes, maxCoverBytes int64) (*domain.CreateMusicRequest, *scannedCover, error) {
	return mediametadata.Read(path, maxTagBytes, maxCoverBytes)
}

func saveScannedCover(musicID uint, picture *scannedCover, uploadDir string, maxCoverBytes int64) (string, error) {
	return mediametadata.SaveCover(musicID, picture, uploadDir, maxCoverBytes)
}
