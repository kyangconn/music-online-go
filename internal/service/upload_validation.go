package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kyangconn/music-online-go/internal/config"
	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
)

const (
	multipartOverheadBytes = 1 << 20
	signatureReadSize      = 512
)

var allowedAudioExts = map[string]struct{}{
	".aac":  {},
	".aif":  {},
	".aiff": {},
	".ape":  {},
	".flac": {},
	".m4a":  {},
	".mp3":  {},
	".ogg":  {},
	".opus": {},
	".wav":  {},
	".wma":  {},
}

var allowedCoverExts = map[string]struct{}{
	".gif":  {},
	".jpeg": {},
	".jpg":  {},
	".png":  {},
	".webp": {},
}

var allowedCoverMIMEs = map[string]struct{}{
	"image/gif":  {},
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

type UploadPolicy struct {
	MaxAudioSizeBytes int64    `json:"max_audio_size_bytes"`
	MaxCoverSizeBytes int64    `json:"max_cover_size_bytes"`
	MaxAudioSizeMB    int      `json:"max_audio_size_mb"`
	MaxCoverSizeMB    int      `json:"max_cover_size_mb"`
	AudioExtensions   []string `json:"audio_extensions"`
	CoverExtensions   []string `json:"cover_extensions"`
}

func UploadPolicyFromServerConfig(serverConfig config.ServerConfig) UploadPolicy {
	maxAudioMB := serverConfig.MaxAudioSizeMB
	maxCoverMB := serverConfig.MaxCoverSizeMB
	return UploadPolicy{
		MaxAudioSizeBytes: maxSizeBytes(maxAudioMB),
		MaxCoverSizeBytes: maxSizeBytes(maxCoverMB),
		MaxAudioSizeMB:    maxAudioMB,
		MaxCoverSizeMB:    maxCoverMB,
		AudioExtensions:   sortedExtensions(allowedAudioExts),
		CoverExtensions:   sortedExtensions(allowedCoverExts),
	}
}

func validateUploadedAudioFileWithLimit(header *multipart.FileHeader, maxBytes int64) error {
	if header == nil {
		return nil
	}
	if err := validateSize(header, "audio", maxBytes); err != nil {
		return err
	}
	if err := validateExtension(header.Filename, "audio", allowedAudioExts); err != nil {
		return err
	}
	if err := validateAudioHeaderMIME(header); err != nil {
		return err
	}
	signature, err := readSignature(header)
	if err != nil {
		return fmt.Errorf("%w: failed to read audio header", ErrInvalidMediaFile)
	}
	if !isSupportedAudioSignature(signature) {
		return fmt.Errorf("%w: audio content is not a supported audio format", ErrInvalidMediaFile)
	}
	return nil
}

func validateUploadedCoverFileWithLimit(header *multipart.FileHeader, maxBytes int64) error {
	if header == nil {
		return nil
	}
	if err := validateSize(header, "cover", maxBytes); err != nil {
		return err
	}
	if err := validateExtension(header.Filename, "cover", allowedCoverExts); err != nil {
		return err
	}
	if err := validateHeaderMIME(header, "cover", "image/"); err != nil {
		return err
	}
	signature, err := readSignature(header)
	if err != nil {
		return fmt.Errorf("%w: failed to read cover header", ErrInvalidMediaFile)
	}
	if !isSupportedCoverSignature(signature) {
		return fmt.Errorf("%w: cover content is not a supported image format", ErrInvalidMediaFile)
	}
	return nil
}

func validateSize(header *multipart.FileHeader, label string, maxBytes int64) error {
	if header.Size <= 0 {
		return fmt.Errorf("%w: %s file is empty", ErrInvalidMediaFile, label)
	}
	if header.Size > maxBytes {
		return fmt.Errorf("%w: %s file is too large, max %s", ErrInvalidMediaFile, label, formatUploadSize(maxBytes))
	}
	return nil
}

func validateExtension(filename string, label string, allowed map[string]struct{}) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if _, ok := allowed[ext]; !ok {
		return fmt.Errorf("%w: unsupported %s file extension %q", ErrInvalidMediaFile, label, ext)
	}
	return nil
}

func validateAudioHeaderMIME(header *multipart.FileHeader) error {
	contentType := cleanHeaderContentType(header.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		return nil
	}
	if strings.HasPrefix(contentType, "audio/") || contentType == "application/ogg" || contentType == "video/mp4" {
		return nil
	}
	return fmt.Errorf("%w: unsupported audio MIME type %q", ErrInvalidMediaFile, contentType)
}

func validateHeaderMIME(header *multipart.FileHeader, label string, prefix string) error {
	contentType := cleanHeaderContentType(header.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		return nil
	}
	if !strings.HasPrefix(contentType, prefix) {
		return fmt.Errorf("%w: unsupported %s MIME type %q", ErrInvalidMediaFile, label, contentType)
	}
	return nil
}

func cleanHeaderContentType(value string) string {
	contentType := strings.ToLower(value)
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	return strings.TrimSpace(contentType)
}

func readSignature(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && !errors.Is(cerr, io.EOF) && !errors.Is(cerr, io.ErrUnexpectedEOF) {
			pklog.Errorf("Failed to close signature reader: %v", cerr)
		}
	}()

	buf := make([]byte, signatureReadSize)
	n, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buf[:n], nil
}

func isSupportedAudioSignature(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	if bytes.HasPrefix(data, []byte("ID3")) || isMPEGFrame(data) || isADTSFrame(data) {
		return true
	}
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("fLaC")) {
		return true
	}
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("OggS")) {
		return true
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")) {
		return true
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("FORM")) && (bytes.Equal(data[8:12], []byte("AIFF")) || bytes.Equal(data[8:12], []byte("AIFC"))) {
		return true
	}
	if len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) {
		return true
	}
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("MAC ")) {
		return true
	}
	asfHeader := []byte{0x30, 0x26, 0xb2, 0x75, 0x8e, 0x66, 0xcf, 0x11, 0xa6, 0xd9, 0x00, 0xaa, 0x00, 0x62, 0xce, 0x6c}
	return len(data) >= len(asfHeader) && bytes.Equal(data[:len(asfHeader)], asfHeader)
}

func isSupportedCoverSignature(data []byte) bool {
	contentType := http.DetectContentType(data)
	if _, ok := allowedCoverMIMEs[contentType]; ok {
		return true
	}
	return len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
}

func isMPEGFrame(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0
}

func isADTSFrame(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xff && data[1]&0xf0 == 0xf0
}

func maxSizeBytes(configuredMB int) int64 {
	return int64(configuredMB) * 1024 * 1024
}

func formatUploadSize(bytes int64) string {
	if bytes%(1024*1024) == 0 {
		return fmt.Sprintf("%d MB", bytes/(1024*1024))
	}
	return fmt.Sprintf("%d bytes", bytes)
}

func sortedExtensions(exts map[string]struct{}) []string {
	values := make([]string, 0, len(exts))
	for ext := range exts {
		values = append(values, ext)
	}
	sort.Strings(values)
	return values
}
