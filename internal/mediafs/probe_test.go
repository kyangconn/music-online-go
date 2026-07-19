package mediafs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRootSpecRejectsProbeEscape(t *testing.T) {
	root := t.TempDir()
	for _, probeFile := range []string{"../outside", filepath.Join(root, "absolute")} {
		err := ValidateRootSpec(RootSpec{Path: root, Kind: KindNFS, ProbeFile: probeFile})
		if err == nil {
			t.Fatalf("probe path %q should not escape the media root", probeFile)
		}
	}
}

func TestSystemProberReadsSentinel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".music-online-probe"), []byte("ok"), 0600); err != nil {
		t.Fatalf("write probe file: %v", err)
	}
	result := (SystemProber{}).Probe(context.Background(), RootSpec{
		Path: root, Kind: KindAuto, ProbeFile: ".music-online-probe",
	})
	if !result.Available() {
		t.Fatalf("readable sentinel should make storage available, got %+v", result)
	}
	if result.Code != "available" && result.Code != "mount_identity_unavailable" {
		t.Fatalf("unexpected successful probe code %q", result.Code)
	}
}

func TestSystemProberReportsMissingMount(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-mounted")
	result := (SystemProber{}).Probe(context.Background(), RootSpec{Path: root, Kind: KindNFS})
	if result.Status != StatusOffline || result.Code != "mount_missing" || !result.Retryable {
		t.Fatalf("missing NFS root should be a retryable mount failure, got %+v", result)
	}
}

func TestSystemProberReportsMissingSentinel(t *testing.T) {
	result := (SystemProber{}).Probe(context.Background(), RootSpec{
		Path: t.TempDir(), Kind: KindAuto, ProbeFile: ".music-online-probe",
	})
	if result.Status != StatusOffline || result.Code != "probe_file_missing" || !result.Retryable {
		t.Fatalf("missing sentinel should have a structured retryable result, got %+v", result)
	}
}

func TestMountMismatchDistinguishesNetworkKinds(t *testing.T) {
	if message := mountMismatch(RootSpec{Kind: KindNFS}, mountIdentity{Filesystem: "ext4"}); message == "" {
		t.Fatal("an NFS root backed by a local filesystem should be rejected")
	}
	if message := mountMismatch(RootSpec{Kind: KindSMB}, mountIdentity{Filesystem: "cifs"}); message != "" {
		t.Fatalf("a CIFS mount should satisfy an SMB root: %s", message)
	}
}
