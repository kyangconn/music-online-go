// Package mediafs probes administrator-registered media storage without
// attempting to mount, repair, or authenticate the underlying filesystem.
package mediafs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	StatusUnknown  = "unknown"
	StatusOnline   = "online"
	StatusDegraded = "degraded"
	StatusOffline  = "offline"

	KindManaged = "managed"
	KindAuto    = "auto"
	KindLocal   = "local"
	KindNFS     = "nfs"
	KindSMB     = "smb"

	PathSemanticsAuto            = "auto"
	PathSemanticsCaseSensitive   = "case_sensitive"
	PathSemanticsCaseInsensitive = "case_insensitive"
)

type RootSpec struct {
	Path               string
	Kind               string
	ExpectedFilesystem string
	ProbeFile          string
	PathSemantics      string
}

type Result struct {
	Status      string
	Code        string
	Message     string
	Filesystem  string
	MountSource string
	Retryable   bool
	CheckedAt   time.Time
}

func (r Result) Available() bool {
	return r.Status == StatusOnline || r.Status == StatusDegraded
}

// IsTransientError is used for the narrow race where a mounted source fails
// after path resolution but before the HTTP handler opens or stats the file.
func IsTransientError(err error) bool {
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if errors.Is(err, fs.ErrPermission) {
		return true
	}
	_, _, retryable, ok := classifyPlatformError(err)
	return ok && retryable
}

// IsRetryableError excludes permanent policy failures such as permissions. It
// is used by the scanner when deciding whether replaying a whole pass can help.
func IsRetryableError(err error) bool {
	if err == nil || errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return false
	}
	_, _, retryable, ok := classifyPlatformError(err)
	return ok && retryable
}

type Prober interface {
	Probe(ctx context.Context, spec RootSpec) Result
}

type SystemProber struct{}

func NewSystemProber() Prober {
	return SystemProber{}
}

func ValidateRootSpec(spec RootSpec) error {
	path := strings.TrimSpace(spec.Path)
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("path must be absolute inside the server or container")
	}
	if len(path) > 1000 {
		return errors.New("path exceeds 1000 bytes")
	}
	cleanPath := filepath.Clean(path)
	if samePath(cleanPath, filepath.Dir(cleanPath)) && (runtime.GOOS != "windows" || !strings.HasPrefix(cleanPath, `\\`)) {
		return errors.New("filesystem root cannot be registered")
	}
	switch normalizeKind(spec.Kind) {
	case KindManaged, KindAuto, KindLocal, KindNFS, KindSMB:
	default:
		return fmt.Errorf("unsupported storage kind %q", spec.Kind)
	}
	switch normalizePathSemantics(spec.PathSemantics) {
	case PathSemanticsAuto, PathSemanticsCaseSensitive, PathSemanticsCaseInsensitive:
	default:
		return fmt.Errorf("unsupported path semantics %q", spec.PathSemantics)
	}
	expectedFilesystem := strings.TrimSpace(spec.ExpectedFilesystem)
	if len(expectedFilesystem) > 64 {
		return errors.New("expected filesystem exceeds 64 bytes")
	}
	for _, char := range expectedFilesystem {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') &&
			char != '-' && char != '_' && char != '.' && char != '+' {
			return errors.New("expected filesystem contains an invalid character")
		}
	}
	if len(strings.TrimSpace(spec.ProbeFile)) > 500 {
		return errors.New("probe_file exceeds 500 bytes")
	}
	if _, err := resolveProbePath(cleanPath, spec.ProbeFile); err != nil {
		return err
	}
	return nil
}

func (SystemProber) Probe(ctx context.Context, spec RootSpec) Result {
	result := Result{Status: StatusUnknown, Code: "not_checked", CheckedAt: time.Now().UTC()}
	if err := ctx.Err(); err != nil {
		return failedResult(result, "probe_cancelled", "storage probe was cancelled", true)
	}
	if err := ValidateRootSpec(spec); err != nil {
		return failedResult(result, "invalid_root", err.Error(), false)
	}

	root := filepath.Clean(spec.Path)
	info, err := os.Lstat(root)
	if err != nil {
		return classifyError(result, err, true)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return failedResult(result, "not_directory", "storage root is not a real directory", false)
	}

	identity := inspectPlatformMount(root)
	result.Filesystem = identity.Filesystem
	result.MountSource = identity.Source
	if identity.Err != nil {
		return classifyError(result, identity.Err, true)
	}
	if mismatch := mountMismatch(spec, identity); mismatch != "" {
		return failedResult(result, "mount_mismatch", mismatch, true)
	}

	probePath, err := resolveProbePath(root, spec.ProbeFile)
	if err != nil {
		return failedResult(result, "invalid_probe_file", err.Error(), false)
	}
	if strings.TrimSpace(spec.ProbeFile) != "" {
		probeInfo, statErr := os.Lstat(probePath)
		if statErr != nil {
			codeResult := classifyError(result, statErr, false)
			if errors.Is(statErr, fs.ErrNotExist) {
				codeResult.Code = "probe_file_missing"
				codeResult.Message = "the configured storage probe file is missing"
			}
			return codeResult
		}
		if probeInfo.Mode()&os.ModeSymlink != 0 || !probeInfo.Mode().IsRegular() {
			return failedResult(result, "invalid_probe_file", "the storage probe must be a regular non-symlink file", false)
		}
	}
	if err := readProbeTarget(probePath, strings.TrimSpace(spec.ProbeFile) == ""); err != nil {
		codeResult := classifyError(result, err, strings.TrimSpace(spec.ProbeFile) == "")
		if errors.Is(err, fs.ErrNotExist) && strings.TrimSpace(spec.ProbeFile) != "" {
			codeResult.Code = "probe_file_missing"
			codeResult.Message = "the configured storage probe file is missing"
			codeResult.Retryable = true
		}
		return codeResult
	}

	if identity.WarningCode != "" {
		result.Status = StatusDegraded
		result.Code = identity.WarningCode
		result.Message = identity.WarningMessage
		return result
	}
	result.Status = StatusOnline
	result.Code = "available"
	result.Message = "storage is available"
	return result
}

type mountIdentity struct {
	Filesystem     string
	Source         string
	WarningCode    string
	WarningMessage string
	Err            error
}

func mountMismatch(spec RootSpec, identity mountIdentity) string {
	actual := strings.ToLower(strings.TrimSpace(identity.Filesystem))
	expected := strings.ToLower(strings.TrimSpace(spec.ExpectedFilesystem))
	if expected != "" && actual != "" && actual != "remote" && actual != expected {
		return fmt.Sprintf("mounted filesystem is %s, expected %s", actual, expected)
	}
	if actual == "" {
		return ""
	}
	switch normalizeKind(spec.Kind) {
	case KindNFS:
		if actual != "nfs" && actual != "nfs4" && actual != "remote" {
			return fmt.Sprintf("storage is configured as NFS but the mounted filesystem is %s", actual)
		}
	case KindSMB:
		if actual != "cifs" && actual != "smbfs" && actual != "smb3" && actual != "remote" {
			return fmt.Sprintf("storage is configured as SMB but the mounted filesystem is %s", actual)
		}
	}
	return ""
}

func resolveProbePath(root, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return root, nil
	}
	if filepath.IsAbs(value) {
		return "", errors.New("probe_file must be relative to the storage root")
	}
	relative := filepath.Clean(filepath.FromSlash(value))
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("probe_file must stay inside the storage root")
	}
	candidate := filepath.Join(root, relative)
	if !pathContains(root, candidate) {
		return "", errors.New("probe_file must stay inside the storage root")
	}
	return candidate, nil
}

func readProbeTarget(path string, directory bool) error {
	// Filesystem calls can block in the kernel for a hard-mounted NFS volume.
	// Spawning a timeout goroutine would only hide (and leak) that blocked call,
	// so mount options remain the administrator's timeout/recovery boundary.
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if directory {
		_, err = file.Readdirnames(1)
		if errors.Is(err, io.EOF) { // An empty but readable directory is healthy.
			return nil
		}
		return err
	}
	buffer := make([]byte, 1)
	_, err = file.Read(buffer)
	if errors.Is(err, io.EOF) { // An empty sentinel still proves the mount is readable.
		return nil
	}
	return err
}

func classifyError(base Result, err error, root bool) Result {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		code := "path_missing"
		message := "storage path is missing"
		if root {
			code = "mount_missing"
			message = "storage root is not mounted or does not exist"
		}
		return failedResult(base, code, message, true)
	case errors.Is(err, fs.ErrPermission):
		return failedResult(base, "permission_denied", "storage cannot be read with the current process permissions", false)
	default:
		if code, message, retryable, ok := classifyPlatformError(err); ok {
			return failedResult(base, code, message, retryable)
		}
		return failedResult(base, "read_failed", "storage could not be read", true)
	}
}

func failedResult(base Result, code, message string, retryable bool) Result {
	base.Status = StatusOffline
	base.Code = code
	base.Message = message
	base.Retryable = retryable
	return base
}

func normalizeKind(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return KindAuto
	}
	return value
}

func normalizePathSemantics(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return PathSemanticsAuto
	}
	return value
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbs, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftAbs, rightAbs)
	}
	return leftAbs == rightAbs
}

func pathContains(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}
