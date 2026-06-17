package orchestrator

import (
	"strings"
	"testing"
	"time"
)

func baselineValidRequest() SetupRequest {
	return SetupRequest{
		Domain:          "panel.example.com",
		AdminEmail:      "admin@example.com",
		AdminPassword:   "Str0ng!Passw0rd",
		DBRootPassword:  "R00t!Passw0rd##",
		DBPanelPassword: "Pan3l!Passw0rd##",
		InstallMode:     "new_install",
		SSLMode:         "letsencrypt",
		MasterKeyMode:   "automatic",
	}
}

func TestValidateAcceptsBaseline(t *testing.T) {
	if err := baselineValidRequest().Validate(); err != nil {
		t.Fatalf("baseline request should be valid, got %v", err)
	}
}

func TestValidateRejectsMissingFields(t *testing.T) {
	mutators := map[string]func(*SetupRequest){
		"domain":            func(r *SetupRequest) { r.Domain = "" },
		"admin_email":       func(r *SetupRequest) { r.AdminEmail = "" },
		"admin_password":    func(r *SetupRequest) { r.AdminPassword = "" },
		"db_root_password":  func(r *SetupRequest) { r.DBRootPassword = "" },
		"db_panel_password": func(r *SetupRequest) { r.DBPanelPassword = "" },
	}
	for name, mutate := range mutators {
		req := baselineValidRequest()
		mutate(&req)
		if err := req.Validate(); err == nil {
			t.Errorf("expected invalid when %s missing", name)
		}
	}
}

func TestValidateRejectsWeakPassword(t *testing.T) {
	req := baselineValidRequest()
	req.AdminPassword = "weak"
	if err := req.Validate(); err == nil {
		t.Error("expected invalid for weak admin password")
	}
}

func TestValidateCloudflareRequiresToken(t *testing.T) {
	req := baselineValidRequest()
	req.SSLMode = "cloudflare"
	if err := req.Validate(); err == nil {
		t.Error("expected invalid: cloudflare without token")
	}
	req.CloudflareAPIToken = "cf-token"
	if err := req.Validate(); err != nil {
		t.Errorf("expected valid with cloudflare token, got %v", err)
	}
}

func TestValidateCustomSSLRequiresMaterial(t *testing.T) {
	req := baselineValidRequest()
	req.SSLMode = "custom"
	if err := req.Validate(); err == nil {
		t.Error("expected invalid: custom ssl without certificate")
	}
	cert, key := generateTestKeypair(t, 365*24*time.Hour)
	req.CustomCertificate = cert
	req.CustomPrivateKey = key
	if err := req.Validate(); err != nil {
		t.Errorf("expected valid with proper material, got %v", err)
	}
}

func TestValidateManualMasterKeyRequired(t *testing.T) {
	req := baselineValidRequest()
	req.MasterKeyMode = "manual"
	if err := req.Validate(); err == nil {
		t.Error("expected invalid: manual master key without value")
	}
	req.MasterKey = "deadbeef"
	if err := req.Validate(); err != nil {
		t.Errorf("expected valid with manual master key, got %v", err)
	}
}

func TestValidateSecurityEntranceConfig(t *testing.T) {
	valid := SecurityEntranceConfig{
		Enabled:       true,
		Path:          "secret-gate",
		Port:          8443,
		WindowSeconds: 60,
		MaxAttempts:   5,
		BlockSeconds:  300,
	}
	if err := validateSecurityEntranceConfig(valid); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}

	invalidPaths := []string{"", "ab", strings.Repeat("a", 65), "bad path", "UPPER!"}
	for _, p := range invalidPaths {
		cfg := valid
		cfg.Path = p
		if err := validateSecurityEntranceConfig(cfg); err == nil {
			t.Errorf("expected invalid path %q", p)
		}
	}

	outOfRange := []SecurityEntranceConfig{
		func() SecurityEntranceConfig { c := valid; c.Port = 70000; return c }(),
		func() SecurityEntranceConfig { c := valid; c.WindowSeconds = 100000; return c }(),
		func() SecurityEntranceConfig { c := valid; c.MaxAttempts = 5000; return c }(),
		func() SecurityEntranceConfig { c := valid; c.BlockSeconds = 999999999; return c }(),
	}
	for i, cfg := range outOfRange {
		if err := validateSecurityEntranceConfig(cfg); err == nil {
			t.Errorf("expected out-of-range invalid (case %d)", i)
		}
	}
}

func TestValidateRejectsInvalidSecurityEntrance(t *testing.T) {
	req := baselineValidRequest()
	req.SecurityEntrance = SecurityEntranceConfig{Enabled: true, Path: ""}
	if err := req.Validate(); err == nil {
		t.Error("expected invalid security entrance to fail Validate")
	}
}
