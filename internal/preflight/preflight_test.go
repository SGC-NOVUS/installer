package preflight

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOSReleaseDisplayName(t *testing.T) {
	cases := []struct {
		name    string
		release OSRelease
		want    string
	}{
		{"pretty", OSRelease{PrettyName: "Ubuntu 24.04.4 LTS"}, "Ubuntu 24.04.4 LTS"},
		{"name+version", OSRelease{Name: "Ubuntu", Version: "24.04"}, "Ubuntu 24.04"},
		{"id+versionid", OSRelease{ID: "debian", VersionID: "12"}, "debian 12"},
		{"name-only", OSRelease{Name: "Ubuntu"}, "Ubuntu"},
		{"empty", OSRelease{}, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.release.DisplayName(); got != tc.want {
				t.Errorf("DisplayName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateOS(t *testing.T) {
	valid := []OSRelease{
		{ID: "ubuntu", VersionID: "22.04"},
		{ID: "ubuntu", VersionID: "24.04"},
		{ID: "Ubuntu", VersionID: "24.04"},
		{ID: "debian", VersionID: "12"},
	}
	for _, release := range valid {
		if err := validateOS(release); err != nil {
			t.Errorf("expected %+v valid, got %v", release, err)
		}
	}

	invalid := []OSRelease{
		{ID: "ubuntu", VersionID: "20.04"},
		{ID: "fedora", VersionID: "40"},
		{ID: "debian", VersionID: "11"},
		{ID: "", VersionID: ""},
	}
	for _, release := range invalid {
		if err := validateOS(release); err == nil {
			t.Errorf("expected %+v invalid", release)
		}
	}
}

func TestTotalRAMString(t *testing.T) {
	if got := (&Result{TotalRAMBytes: 0}).TotalRAMString(); got != "unknown" {
		t.Errorf("zero RAM = %q, want unknown", got)
	}
	if got := (&Result{TotalRAMBytes: 4 * 1024 * 1024 * 1024}).TotalRAMString(); got != "4.00 GiB" {
		t.Errorf("4GiB = %q, want 4.00 GiB", got)
	}
}

func TestParseOSReleaseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "os-release")
	content := "NAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\nID=ubuntu\nPRETTY_NAME=\"Ubuntu 24.04.4 LTS\"\n# comment\n\nVERSION=\"24.04.4 LTS (Noble Numbat)\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write os-release: %v", err)
	}

	release, err := parseOSReleaseFile(path)
	if err != nil {
		t.Fatalf("parseOSReleaseFile: %v", err)
	}
	if release.ID != "ubuntu" || release.VersionID != "24.04" {
		t.Errorf("unexpected release: %+v", release)
	}
	if release.PrettyName != "Ubuntu 24.04.4 LTS" {
		t.Errorf("pretty name = %q", release.PrettyName)
	}
}

func TestParseOSReleaseFileMissingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "os-release")
	if err := os.WriteFile(path, []byte("NAME=\"Ubuntu\"\n"), 0o600); err != nil {
		t.Fatalf("write os-release: %v", err)
	}
	if _, err := parseOSReleaseFile(path); err == nil {
		t.Error("expected error for missing ID/VERSION_ID")
	}
}

func TestParseOSReleaseFileMissing(t *testing.T) {
	if _, err := parseOSReleaseFile(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("expected error for absent file")
	}
}

func TestReadMemTotalBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meminfo")
	content := "MemTotal:        8174312 kB\nMemFree:          123456 kB\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write meminfo: %v", err)
	}

	got, err := readMemTotalBytes(path)
	if err != nil {
		t.Fatalf("readMemTotalBytes: %v", err)
	}
	if want := uint64(8174312) * 1024; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestReadMemTotalBytesInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meminfo")
	if err := os.WriteFile(path, []byte("MemAvailable: 100 kB\n"), 0o600); err != nil {
		t.Fatalf("write meminfo: %v", err)
	}
	if _, err := readMemTotalBytes(path); err == nil {
		t.Error("expected error when MemTotal absent")
	}
}

func TestEnsurePortAvailable(t *testing.T) {
	ctx := context.Background()
	// A high ephemeral port should be free in the test sandbox.
	if err := ensurePortAvailable(ctx, 0); err != nil {
		t.Errorf("port 0 (auto-assign) should be available, got %v", err)
	}
}

func TestCheckContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := checkContext(ctx); err == nil {
		t.Error("expected error for cancelled context")
	}
	if err := checkContext(context.Background()); err != nil {
		t.Errorf("expected nil for live context, got %v", err)
	}
}

func TestJoinFailures(t *testing.T) {
	err := joinFailures([]error{nil, context.Canceled, nil})
	if err == nil {
		t.Fatal("expected joined error")
	}
}
