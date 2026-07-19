//go:build windows

package mediafs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func inspectPlatformMount(path string) mountIdentity {
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, `\\`) || strings.HasPrefix(filepath.ToSlash(clean), "//") {
		return mountIdentity{
			Filesystem:     "remote",
			Source:         "UNC",
			WarningCode:    "remote_protocol_unverified",
			WarningMessage: "storage is readable, but Windows does not expose the UNC filesystem protocol to this process",
		}
	}
	volume := filepath.VolumeName(clean)
	if volume == "" {
		return mountIdentity{WarningCode: "mount_identity_unavailable", WarningMessage: "storage is readable but its Windows volume type is unknown"}
	}
	root := volume + `\`
	rootPointer, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return mountIdentity{Err: err}
	}
	switch windows.GetDriveType(rootPointer) {
	case windows.DRIVE_REMOTE:
		return mountIdentity{
			Filesystem:     "remote",
			Source:         volume,
			WarningCode:    "mapped_drive_session_scoped",
			WarningMessage: "mapped network drives are session-scoped and their protocol is not verifiable here; a UNC path is more reliable for a Windows service",
		}
	case windows.DRIVE_NO_ROOT_DIR:
		return mountIdentity{Err: os.ErrNotExist}
	default:
		return mountIdentity{Filesystem: "local", Source: volume}
	}
}

func classifyPlatformError(err error) (string, string, bool, bool) {
	switch {
	case errors.Is(err, windows.ERROR_SEM_TIMEOUT), errors.Is(err, windows.WSAETIMEDOUT):
		return "io_timeout", "the storage operation timed out", true, true
	case errors.Is(err, windows.ERROR_BAD_NETPATH), errors.Is(err, windows.ERROR_BAD_NET_NAME),
		errors.Is(err, windows.ERROR_NETNAME_DELETED), errors.Is(err, windows.ERROR_NETWORK_UNREACHABLE),
		errors.Is(err, windows.ERROR_NOT_CONNECTED), errors.Is(err, windows.WSAEHOSTUNREACH),
		errors.Is(err, windows.WSAENETUNREACH):
		return "network_unreachable", "the network storage endpoint is unreachable", true, true
	case errors.Is(err, windows.ERROR_UNEXP_NET_ERR):
		return "read_failed", "the network filesystem returned an input/output error", true, true
	default:
		return "", "", false, false
	}
}
