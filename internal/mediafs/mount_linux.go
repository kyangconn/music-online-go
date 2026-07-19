//go:build linux

package mediafs

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func inspectPlatformMount(path string) mountIdentity {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return mountIdentity{WarningCode: "mount_identity_unavailable", WarningMessage: "storage is readable but its mount identity could not be inspected"}
	}
	defer func() { _ = file.Close() }()

	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return mountIdentity{Err: err}
	}
	bestLength := -1
	best := mountIdentity{}
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		separator := -1
		for index, field := range fields {
			if field == "-" {
				separator = index
				break
			}
		}
		if len(fields) < 6 || separator < 0 || separator+2 >= len(fields) {
			continue
		}
		mountPoint := unescapeMountInfo(fields[4])
		if !pathContains(mountPoint, cleanPath) || len(mountPoint) <= bestLength {
			continue
		}
		bestLength = len(mountPoint)
		best = mountIdentity{Filesystem: fields[separator+1], Source: unescapeMountInfo(fields[separator+2])}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return mountIdentity{Err: scanErr}
	}
	if bestLength < 0 {
		return mountIdentity{WarningCode: "mount_identity_unavailable", WarningMessage: "storage is readable but no matching mount entry was found"}
	}
	return best
}

func unescapeMountInfo(value string) string {
	// mountinfo escapes whitespace and backslashes as three-digit octal values.
	var builder strings.Builder
	for index := 0; index < len(value); {
		if value[index] == '\\' && index+3 < len(value) {
			if decoded, err := strconv.ParseUint(value[index+1:index+4], 8, 8); err == nil {
				builder.WriteByte(byte(decoded))
				index += 4
				continue
			}
		}
		builder.WriteByte(value[index])
		index++
	}
	return builder.String()
}

func classifyPlatformError(err error) (string, string, bool, bool) {
	switch {
	case errors.Is(err, syscall.ENODEV), errors.Is(err, syscall.ENXIO):
		return "mount_missing", "storage root is not mounted or its device is unavailable", true, true
	case errors.Is(err, syscall.ESTALE):
		return "stale_handle", "the network filesystem returned a stale file handle", true, true
	case errors.Is(err, syscall.ETIMEDOUT):
		return "io_timeout", "the storage operation timed out", true, true
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH),
		errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, syscall.ENOTCONN):
		return "network_unreachable", "the network storage endpoint is unreachable", true, true
	case errors.Is(err, syscall.EIO):
		return "read_failed", "the filesystem returned an input/output error", true, true
	default:
		return "", "", false, false
	}
}
