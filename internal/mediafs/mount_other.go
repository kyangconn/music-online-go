//go:build !linux && !windows

package mediafs

func inspectPlatformMount(_ string) mountIdentity {
	return mountIdentity{
		WarningCode:    "mount_identity_unavailable",
		WarningMessage: "storage is readable but mount identity inspection is not supported on this platform",
	}
}

func classifyPlatformError(_ error) (string, string, bool, bool) {
	return "", "", false, false
}
