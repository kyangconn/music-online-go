// Package service music_upload.go - 音乐文件上传与媒体路径
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kyangconn/music-online-go/internal/domain"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
)

func (s *musicService) GetAudioPath(ctx context.Context, id uint) (string, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if music.Path == "" {
		return "", ErrMediaNotFound
	}
	if music.MediaRelativePath != "" {
		return s.storage.ResolveMusicPath(ctx, music)
	}
	return secureManagedMediaPathAt(s.serverConfig.UploadDir, music.Path)
}

func (s *musicService) GetCoverPath(ctx context.Context, id uint) (string, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if music.Img == "" {
		return "", ErrMediaNotFound
	}

	return secureManagedMediaPathAt(s.serverConfig.UploadDir, music.Img)
}

// UploadFiles 上传音频和封面文件到已有音乐记录
func (s *musicService) UploadFiles(ctx context.Context, userID uint, role string, id uint, audioHeader, coverHeader *multipart.FileHeader) (*domain.MusicResponse, error) {
	music, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManageMusic(music, userID, role) {
		return nil, ErrForbidden
	}
	if err := s.guardReadOnlyMediaSource(ctx, music, role); err != nil {
		return nil, err
	}
	if err := validateUploadedAudioFileWithLimit(audioHeader, s.uploadPolicy.MaxAudioSizeBytes); err != nil {
		return nil, err
	}
	if err := validateUploadedCoverFileWithLimit(coverHeader, s.uploadPolicy.MaxCoverSizeBytes); err != nil {
		return nil, err
	}

	uploadDir := s.serverConfig.UploadDir
	musicDir := filepath.Join(uploadDir, strconv.FormatUint(uint64(id), 10))
	if err := os.MkdirAll(musicDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	var staged []*stagedMediaFile
	cleanupStaged := func() {
		for _, file := range staged {
			file.cleanupTemp()
		}
	}
	rollbackApplied := func(cause error) error {
		var rollbackErr error
		for i := len(staged) - 1; i >= 0; i-- {
			if err := staged[i].rollback(); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		}
		if rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("failed to restore previous media files: %w", rollbackErr))
		}
		return cause
	}

	if audioHeader != nil {
		previousPath := music.Path
		tmpPath, finalPath, fileHash, err := saveUploadedMediaFile(musicDir, "audio", audioHeader, true)
		if err != nil {
			cleanupStaged()
			return nil, err
		}
		staged = append(staged, &stagedMediaFile{
			tmpPath:         tmpPath,
			finalPath:       finalPath,
			previousPath:    previousPath,
			cleanupPrevious: pathIsInManagedUploadDirAt(uploadDir, previousPath),
		})
		music.Path = finalPath
		music.FileHash = fileHash
	}

	if coverHeader != nil {
		previousPath := music.Img
		tmpPath, finalPath, _, err := saveUploadedMediaFile(musicDir, "cover", coverHeader, false)
		if err != nil {
			cleanupStaged()
			return nil, err
		}
		staged = append(staged, &stagedMediaFile{
			tmpPath:         tmpPath,
			finalPath:       finalPath,
			previousPath:    previousPath,
			cleanupPrevious: pathIsInManagedUploadDirAt(uploadDir, previousPath),
		})
		music.Img = finalPath
	}

	for _, file := range staged {
		if err := file.apply(); err != nil {
			return nil, rollbackApplied(err)
		}
	}
	if audioHeader != nil {
		info, err := os.Stat(music.Path)
		if err != nil {
			return nil, rollbackApplied(fmt.Errorf("failed to inspect finalized audio file: %w", err))
		}
		relative, err := filepath.Rel(uploadDir, music.Path)
		if err != nil || relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, rollbackApplied(ErrMediaNotFound)
		}
		relative = filepath.ToSlash(relative)
		sourceKey := mediaSourceKey(domain.ManagedMediaRootID, relative, domain.MediaPathSemanticsAuto)
		modifiedAt := info.ModTime().UTC()
		music.MediaRootID = domain.ManagedMediaRootID
		music.MediaRelativePath = relative
		music.MediaSourceKey = &sourceKey
		music.SourceFileSize = info.Size()
		music.SourceFileModTime = &modifiedAt
		music.SourceReadOnly = false
	}

	var persistErr error
	if audioHeader != nil {
		persistErr = s.storage.PersistManagedMusicSource(ctx, music)
	} else {
		persistErr = s.repo.Update(ctx, music)
	}
	if persistErr != nil {
		return nil, rollbackApplied(persistErr)
	}

	for _, file := range staged {
		file.commit()
	}
	if audioHeader != nil {
		scheduleCtx, cancelSchedule := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if err := s.analyzer.ScheduleContentAnalysis(scheduleCtx, music.ID, userID); err != nil {
			// Analysis is derived work. A full queue or unavailable analyzer must
			// never turn a successfully committed upload into an HTTP failure.
			pklog.Warnf("Music %d was uploaded but analysis could not be queued: %v", music.ID, err)
		}
		cancelSchedule()
	}

	return s.toResponse(music), nil
}

func secureManagedMediaPathAt(uploadDir, path string) (string, error) {
	if strings.TrimSpace(uploadDir) == "" {
		return "", ErrMediaNotFound
	}
	uploadRoot, err := filepath.Abs(filepath.Clean(uploadDir))
	if err != nil {
		return "", ErrMediaNotFound
	}
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil || !pathContains(uploadRoot, absPath) {
		return "", ErrMediaNotFound
	}
	relative, err := filepath.Rel(uploadRoot, absPath)
	if err != nil {
		return "", ErrMediaNotFound
	}
	return securePathWithinRoot(uploadRoot, filepath.ToSlash(relative))
}

func (s *musicService) toResponse(music *domain.Music) *domain.MusicResponse {
	return s.presenter.music(music)
}

type stagedMediaFile struct {
	tmpPath         string
	finalPath       string
	previousPath    string
	backupPath      string
	applied         bool
	cleanupPrevious bool
}

func (f *stagedMediaFile) apply() error {
	if _, err := os.Stat(f.finalPath); err == nil {
		f.backupPath = f.tmpPath + ".backup"
		if err := os.Rename(f.finalPath, f.backupPath); err != nil {
			return fmt.Errorf("failed to back up existing media file: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect existing media file: %w", err)
	}

	if err := os.Rename(f.tmpPath, f.finalPath); err != nil {
		if f.backupPath != "" {
			if restoreErr := os.Rename(f.backupPath, f.finalPath); restoreErr != nil {
				return errors.Join(
					fmt.Errorf("failed to finalize media file: %w", err),
					fmt.Errorf("failed to restore media backup: %w", restoreErr),
				)
			}
			f.backupPath = ""
		}
		return fmt.Errorf("failed to finalize media file: %w", err)
	}

	f.applied = true
	return nil
}

func (f *stagedMediaFile) rollback() error {
	var rollbackErr error
	if f.applied {
		if err := os.Remove(f.finalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove replacement %s: %w", f.finalPath, err))
		}
	}
	if f.backupPath != "" {
		if err := os.Rename(f.backupPath, f.finalPath); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore backup %s: %w", f.finalPath, err))
		}
	}
	f.cleanupTemp()
	return rollbackErr
}

func (f *stagedMediaFile) commit() {
	cleanupUploadedFiles([]string{f.backupPath})
	if f.cleanupPrevious && f.previousPath != f.finalPath {
		cleanupUploadedFiles([]string{f.previousPath})
	}
}

func pathIsInManagedUploadDirAt(uploadDir, path string) bool {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(uploadDir) == "" {
		return false
	}
	root, err := filepath.Abs(filepath.Clean(uploadDir))
	if err != nil {
		return false
	}
	candidate, err := filepath.Abs(filepath.Clean(path))
	return err == nil && pathContains(root, candidate)
}

func (f *stagedMediaFile) cleanupTemp() {
	cleanupUploadedFiles([]string{f.tmpPath})
}

// saveUploadedMediaFile saves the upload to a unique temporary file. The caller
// promotes it only after every requested media file has been validated and staged.
func saveUploadedMediaFile(dir, baseName string, header *multipart.FileHeader, calculateHash bool) (tmpPath, finalPath string, fileHash string, err error) {
	ext := filepath.Ext(header.Filename)
	finalPath = filepath.Join(dir, baseName+ext)

	src, err := header.Open()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open %s file: %w", baseName, err)
	}
	defer func() {
		if cerr := src.Close(); cerr != nil && !errors.Is(cerr, io.EOF) && !errors.Is(cerr, io.ErrUnexpectedEOF) {
			pklog.Errorf("Failed to close %s source: %v", baseName, cerr)
		}
	}()

	dst, err := os.CreateTemp(dir, "."+baseName+"-*.tmp")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create %s file: %w", baseName, err)
	}
	tmpPath = dst.Name()
	saved := false
	defer func() {
		if cerr := dst.Close(); cerr != nil {
			pklog.Errorf("Failed to close %s destination: %v", baseName, cerr)
		}
		if !saved {
			cleanupUploadedFiles([]string{tmpPath})
		}
	}()

	var writer io.Writer = dst
	hasher := sha256.New()
	if calculateHash {
		writer = io.MultiWriter(dst, hasher)
	}
	if _, err := io.Copy(writer, src); err != nil {
		return "", "", "", fmt.Errorf("failed to save %s file: %w", baseName, err)
	}
	saved = true
	if calculateHash {
		fileHash = hex.EncodeToString(hasher.Sum(nil))
	}
	return tmpPath, finalPath, fileHash, nil
}
func cleanupUploadedFiles(paths []string) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			pklog.Errorf("Failed to clean up uploaded file %s: %v", path, err)
		}
	}
}
