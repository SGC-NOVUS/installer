package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEffectiveRestoreKeyMaterial(t *testing.T) {
	if got := (SetupRequest{Restore: RestoreConfig{KeyMaterial: "  raw-key  "}}).effectiveRestoreKeyMaterial(); got != "raw-key" {
		t.Errorf("key material: got %q", got)
	}
	got := (SetupRequest{Restore: RestoreConfig{RecoveryPhrase: "  Alpha   Bravo  "}}).effectiveRestoreKeyMaterial()
	if got != "alpha bravo" {
		t.Errorf("recovery phrase normalize: got %q", got)
	}
	if got := (SetupRequest{}).effectiveRestoreKeyMaterial(); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

func TestEffectiveCloudflareKMSAPIToken(t *testing.T) {
	if got := (SetupRequest{CloudflareKMS: CloudflareKMSConfig{APIToken: "kms-tok"}}).effectiveCloudflareKMSAPIToken(); got != "kms-tok" {
		t.Errorf("kms token: got %q", got)
	}
	if got := (SetupRequest{CloudflareAPIToken: "cf-tok"}).effectiveCloudflareKMSAPIToken(); got != "cf-tok" {
		t.Errorf("fallback cf token: got %q", got)
	}
}

func TestHasPanelRuntimeConfiguration(t *testing.T) {
	if (SetupRequest{}).hasPanelRuntimeConfiguration() {
		t.Error("empty request should have no runtime config")
	}
	if !(SetupRequest{SecurityEntrance: SecurityEntranceConfig{Enabled: true}}).hasPanelRuntimeConfiguration() {
		t.Error("security entrance should count as runtime config")
	}
}

func TestResolveInstallerMasterKey(t *testing.T) {
	manual := SetupRequest{MasterKeyMode: "manual", MasterKey: "abc123"}
	if got, err := resolveInstallerMasterKey(manual); err != nil || got != "abc123" {
		t.Errorf("manual key: got %q err %v", got, err)
	}

	if _, err := resolveInstallerMasterKey(SetupRequest{MasterKeyMode: "manual"}); err == nil {
		t.Error("manual mode without key should error")
	}

	auto, err := resolveInstallerMasterKey(SetupRequest{MasterKeyMode: "automatic"})
	if err != nil {
		t.Fatalf("automatic key: %v", err)
	}
	if len(auto) != 64 {
		t.Errorf("automatic key should be 64 hex chars, got %d", len(auto))
	}
}

func TestResolveCloudflareSharedSecret(t *testing.T) {
	if got, err := resolveCloudflareSharedSecret(SetupRequest{}); err != nil || got != "" {
		t.Errorf("non-kms backend: got %q err %v", got, err)
	}
	secret, err := resolveCloudflareSharedSecret(SetupRequest{MasterKeyBackend: "tier2"})
	if err != nil {
		t.Fatalf("kms secret: %v", err)
	}
	if len(secret) != 64 {
		t.Errorf("kms secret should be 64 hex chars, got %d", len(secret))
	}
}

func TestRestoreSourceLabel(t *testing.T) {
	if got := restoreSourceLabel(SetupRequest{Restore: RestoreConfig{BackupURL: "http://x/y"}}); got != "http://x/y" {
		t.Errorf("url label: got %q", got)
	}
	if got := restoreSourceLabel(SetupRequest{Restore: RestoreConfig{BackupPayload: "data"}}); got != "inline_payload" {
		t.Errorf("inline label: got %q", got)
	}
	if got := restoreSourceLabel(SetupRequest{}); got != "" {
		t.Errorf("empty label: got %q", got)
	}
}

func TestGenerateLaravelAppKey(t *testing.T) {
	key, err := generateLaravelAppKey()
	if err != nil {
		t.Fatalf("app key: %v", err)
	}
	if !strings.HasPrefix(key, "base64:") {
		t.Errorf("app key should be base64-prefixed, got %q", key)
	}
}

func TestResolveRestorePayloadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.bin")
	if err := os.WriteFile(path, []byte("  payload-data  "), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	req := SetupRequest{Restore: RestoreConfig{SourceType: "file", BackupFile: path}}
	got, err := resolveRestorePayload(req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "payload-data" {
		t.Errorf("got %q", got)
	}
}

func TestResolveRestorePayloadMissingSource(t *testing.T) {
	if _, err := resolveRestorePayload(SetupRequest{}); err == nil {
		t.Error("expected error for missing restore source")
	}
}

func TestBuildInstallStepsHaveNames(t *testing.T) {
	runner := &Runner{dryRun: true}
	steps := runner.buildInstallSteps(baselineValidRequest(), "panel.example.com", platformProfile{ID: "ubuntu", VersionID: "24.04"})
	if len(steps) == 0 {
		t.Fatal("expected install steps")
	}
	for i, step := range steps {
		if strings.TrimSpace(step.Name) == "" {
			t.Errorf("step %d has empty name", i)
		}
		if step.Run == nil {
			t.Errorf("step %q has nil Run", step.Name)
		}
	}
}

func TestWriteBootstrapManifestDryRun(t *testing.T) {
	sink := &recordingSink{}
	runner := NewRunner(sink, RunnerOptions{DryRun: true, AuditDisabled: true})
	req := baselineValidRequest()
	req.InstallMode = "restore"
	req.Restore = RestoreConfig{SourceType: "inline", BackupPayload: "data", KeyMaterial: "k"}
	if err := runner.writeBootstrapManifest(context.Background(), req, "panel.example.com"); err != nil {
		t.Fatalf("dry-run manifest should not error: %v", err)
	}
}
