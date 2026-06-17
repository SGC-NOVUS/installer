package orchestrator

import (
	"runtime"
	"strings"
	"testing"
)

func TestEffectiveInstallMode(t *testing.T) {
	if (SetupRequest{InstallMode: "restore"}).effectiveInstallMode() != "restore" {
		t.Error("expected restore")
	}
	if (SetupRequest{InstallMode: "RESTORE"}).effectiveInstallMode() != "restore" {
		t.Error("expected restore (case-insensitive)")
	}
	if (SetupRequest{}).effectiveInstallMode() != "new_install" {
		t.Error("expected new_install default")
	}
}

func TestEffectiveSSLMode(t *testing.T) {
	cases := []struct {
		req  SetupRequest
		want string
	}{
		{SetupRequest{SSLMode: "letsencrypt"}, "letsencrypt"},
		{SetupRequest{SSLMode: "cloudflare"}, "cloudflare"},
		{SetupRequest{SSLMode: "custom"}, "custom"},
		{SetupRequest{CloudflareAPIToken: "tok"}, "cloudflare"},
		{SetupRequest{CustomCertificate: "cert"}, "custom"},
		{SetupRequest{CustomPrivateKey: "key"}, "custom"},
		{SetupRequest{UseLetsEncrypt: true}, "letsencrypt"},
		{SetupRequest{}, "letsencrypt"},
	}
	for i, c := range cases {
		if got := c.req.effectiveSSLMode(); got != c.want {
			t.Errorf("case %d: got %q, want %q", i, got, c.want)
		}
	}
}

func TestEffectiveMasterKeyMode(t *testing.T) {
	if (SetupRequest{MasterKeyMode: "manual"}).effectiveMasterKeyMode() != "manual" {
		t.Error("expected manual")
	}
	if (SetupRequest{MasterKeyMode: "import"}).effectiveMasterKeyMode() != "manual" {
		t.Error("expected manual for import")
	}
	if (SetupRequest{}).effectiveMasterKeyMode() != "automatic" {
		t.Error("expected automatic default")
	}
}

func TestEffectiveMasterKeyBackend(t *testing.T) {
	cases := map[string]string{
		"tier2":  "tier2_cloudflare_zero_disk_kms",
		"tpm2":   "tier3_tpm2_hardware_sealing",
		"tier3":  "tier3_tpm2_hardware_sealing",
		"":       "tier1_hybrid_auto_unseal",
		"hybrid": "tier1_hybrid_auto_unseal",
	}
	for in, want := range cases {
		if got := (SetupRequest{MasterKeyBackend: in}).effectiveMasterKeyBackend(); got != want {
			t.Errorf("backend %q: got %q, want %q", in, got, want)
		}
	}
}

func TestEffectiveRestoreSourceType(t *testing.T) {
	if (SetupRequest{Restore: RestoreConfig{SourceType: "url"}}).effectiveRestoreSourceType() != "url" {
		t.Error("explicit url")
	}
	if (SetupRequest{Restore: RestoreConfig{BackupPayload: "x"}}).effectiveRestoreSourceType() != "inline" {
		t.Error("inferred inline")
	}
	if (SetupRequest{Restore: RestoreConfig{BackupFile: "/x"}}).effectiveRestoreSourceType() != "file" {
		t.Error("inferred file")
	}
	if (SetupRequest{Restore: RestoreConfig{BackupURL: "http://x"}}).effectiveRestoreSourceType() != "url" {
		t.Error("inferred url")
	}
	if (SetupRequest{}).effectiveRestoreSourceType() != "" {
		t.Error("empty when no source")
	}
}

func TestEffectiveRestoreImportMode(t *testing.T) {
	if (SetupRequest{Restore: RestoreConfig{ImportMode: "skip_existing"}}).effectiveRestoreImportMode() != "skip_existing" {
		t.Error("skip_existing")
	}
	if (SetupRequest{Restore: RestoreConfig{ImportMode: "report_only"}}).effectiveRestoreImportMode() != "report_only" {
		t.Error("report_only")
	}
	if (SetupRequest{}).effectiveRestoreImportMode() != "overwrite" {
		t.Error("overwrite default")
	}
}

func TestIsIPAddressHost(t *testing.T) {
	truthy := []string{"192.168.1.1", "10.0.0.5", "2001:db8::1", "[2001:db8::1]", "https://192.168.1.1"}
	for _, h := range truthy {
		if !isIPAddressHost(h) {
			t.Errorf("expected %q to be IP", h)
		}
	}
	falsy := []string{"", "example.com", "panel.novus.fun", "https://example.com"}
	for _, h := range falsy {
		if isIPAddressHost(h) {
			t.Errorf("expected %q not IP", h)
		}
	}
}

func TestBuildFinishURL(t *testing.T) {
	if got := buildFinishURL("example.com", "letsencrypt"); got != "https://example.com" {
		t.Errorf("domain https: got %q", got)
	}
	if got := buildFinishURL("192.168.1.1", "none"); got != "http://192.168.1.1" {
		t.Errorf("ip http: got %q", got)
	}
	if got := buildFinishURL("192.168.1.1", "custom"); got != "https://192.168.1.1" {
		t.Errorf("ip custom https: got %q", got)
	}
}

func TestNormalizeDomain(t *testing.T) {
	got, err := normalizeDomain("  https://Panel.Example.com/path  ")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "Panel.Example.com" {
		t.Errorf("got %q", got)
	}
	if _, err := normalizeDomain("   "); err == nil {
		t.Error("expected error on empty")
	}
}

func TestSSLPathHelpers(t *testing.T) {
	if letsEncryptFullchainPath("example.com") != "/etc/letsencrypt/live/example.com/fullchain.pem" {
		t.Error("fullchain path")
	}
	if letsEncryptPrivkeyPath("example.com") != "/etc/letsencrypt/live/example.com/privkey.pem" {
		t.Error("privkey path")
	}
	if !strings.HasSuffix(customCertificatePath("example.com"), "/example.com.crt") {
		t.Error("custom cert path")
	}
	if !strings.HasSuffix(customPrivateKeyPath("example.com"), "/example.com.key") {
		t.Error("custom key path")
	}
}

func TestResolveAgentBinaryURL(t *testing.T) {
	t.Setenv(agentBinaryURLEnv, "")
	url := resolveAgentBinaryURL()
	if runtime.GOARCH == "arm64" {
		if url != defaultAgentARM64URL {
			t.Errorf("arm64 url: %q", url)
		}
	} else {
		if url != defaultAgentAMD64URL {
			t.Errorf("amd64 url: %q", url)
		}
	}

	t.Setenv(agentBinaryURLEnv, "https://custom/agent")
	if resolveAgentBinaryURL() != "https://custom/agent" {
		t.Error("env override")
	}
}

func TestQuoteEnvValue(t *testing.T) {
	got := quoteEnvValue("a\"b\\c\nd")
	if got != `"a\"b\\c\nd"` {
		t.Errorf("got %q", got)
	}
}

func TestWriteFileCommand(t *testing.T) {
	cmd := writeFileCommand("/etc/x", "hello world")
	if !strings.Contains(cmd, "printf %s") || !strings.Contains(cmd, "> '/etc/x'") {
		t.Errorf("got %q", cmd)
	}
}

func TestDecodeInlinePayload(t *testing.T) {
	if _, err := decodeInlinePayload("   "); err == nil {
		t.Error("expected empty err")
	}
	if got, _ := decodeInlinePayload("raw-data"); got != "raw-data" {
		t.Errorf("plain passthrough: %q", got)
	}
	got, err := decodeInlinePayload("data:application/json;base64,aGVsbG8=")
	if err != nil || got != "hello" {
		t.Errorf("base64 decode: %q err=%v", got, err)
	}
	got, err = decodeInlinePayload("data:text/plain,hello%20world")
	if err != nil || got != "hello world" {
		t.Errorf("urlencoded decode: %q err=%v", got, err)
	}
	if _, err := decodeInlinePayload("data:application/json;base64,!!!notbase64"); err == nil {
		t.Error("expected base64 error")
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	if normalizeWhitespace("  a   b\tc\n d ") != "a b c d" {
		t.Error("normalize whitespace")
	}
}

func TestSanitizeField(t *testing.T) {
	if sanitizeField("   ") != "(not provided)" {
		t.Error("empty placeholder")
	}
	if sanitizeField("  value  ") != "value" {
		t.Error("trim")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if firstNonEmpty("", "  ", "x", "y") != "x" {
		t.Error("first non empty")
	}
	if firstNonEmpty("", "  ") != "" {
		t.Error("all empty")
	}
}

func TestNormalizedIntegrationsDedup(t *testing.T) {
	req := SetupRequest{Integrations: []IntegrationConfig{
		{Key: "Telegram", Enabled: true, Fields: map[string]string{"Token": " abc "}},
		{Key: "telegram", Enabled: false},
		{Key: "", Enabled: true},
	}}
	out := req.normalizedIntegrations()
	if len(out) != 1 {
		t.Fatalf("expected 1 deduped integration, got %d", len(out))
	}
	if out[0].Key != "telegram" {
		t.Errorf("lowercased key: %q", out[0].Key)
	}
	if out[0].Fields["token"] != "abc" {
		t.Errorf("lowercased/trimmed field: %q", out[0].Fields["token"])
	}
}

func TestIntegrationFieldLookup(t *testing.T) {
	lookup := integrationFieldLookup([]IntegrationConfig{
		{Key: "telegram", Fields: map[string]string{"token": "abc"}},
	})
	if integrationField(lookup, "telegram", "token") != "abc" {
		t.Error("field lookup hit")
	}
	if integrationField(lookup, "telegram", "missing") != "" {
		t.Error("missing field")
	}
	if integrationField(lookup, "absent", "token") != "" {
		t.Error("absent provider")
	}
}

func TestIntegrationEnabled(t *testing.T) {
	list := []IntegrationConfig{{Key: "telegram", Enabled: true}}
	if !integrationEnabled(list, "telegram") {
		t.Error("enabled")
	}
	if integrationEnabled(list, "discord") {
		t.Error("absent disabled")
	}
}

func TestRenderNginxVHostContainsDomain(t *testing.T) {
	out := renderNginxVHost("example.com")
	if !strings.Contains(out, "example.com") {
		t.Error("vhost must contain domain")
	}
}

func TestRenderNginxTLSVHostContainsPaths(t *testing.T) {
	out := renderNginxTLSVHost("example.com", "/cert.pem", "/key.pem")
	if !strings.Contains(out, "/cert.pem") || !strings.Contains(out, "/key.pem") {
		t.Error("tls vhost must contain cert/key paths")
	}
}

func TestRenderPanelEnvironment(t *testing.T) {
	req := SetupRequest{
		DBPanelPassword: "p@ss\"word",
		Integrations: []IntegrationConfig{
			{Key: "telegram", Enabled: true, Fields: map[string]string{
				"telegram_bot_token":     "BOT:TOK",
				"telegram_admin_chat_id": "12345",
			}},
		},
	}
	env := renderPanelEnvironment(req, "https://panel.example.com", "base64:appkey", "MASTERKEY", "")

	mustContain := []string{
		"APP_ENV=production",
		"APP_INSTALLED=1",
		"SETUP_ENABLED=0",
		"SETUP_FORCE_ENABLED=0",
		`APP_URL="https://panel.example.com"`,
		"DB_PORT=3306",
		`NOVUS_MASTER_KEY="MASTERKEY"`,
		"TELEGRAM_ENABLED=\"true\"",
		`TELEGRAM_BOT_TOKEN="BOT:TOK"`,
		`TELEGRAM_CHAT_ID="12345"`,
	}
	for _, frag := range mustContain {
		if !strings.Contains(env, frag) {
			t.Errorf("env missing %q\n--- env ---\n%s", frag, env)
		}
	}

	// Password with double-quote must be escaped, never raw.
	if strings.Contains(env, `DB_PASSWORD="p@ss"word"`) {
		t.Error("DB password must be escaped, not raw")
	}
	if !strings.Contains(env, `DB_PASSWORD="p@ss\"word"`) {
		t.Error("DB password must be backslash-escaped")
	}
}

func TestRenderPanelEnvironmentCloudflareSharedSecret(t *testing.T) {
	req := SetupRequest{MasterKeyBackend: "tier2"}
	env := renderPanelEnvironment(req, "https://x", "k", "m", "SHARED")
	if !strings.Contains(env, `NOVUS_CLOUDFLARE_KMS_SHARED_SECRET="SHARED"`) {
		t.Error("expected shared secret line for tier2 backend")
	}

	// Default backend must NOT emit the inline shared secret.
	env2 := renderPanelEnvironment(SetupRequest{}, "https://x", "k", "m", "SHARED")
	if strings.Contains(env2, "NOVUS_CLOUDFLARE_KMS_SHARED_SECRET=\"SHARED\"") {
		t.Error("default backend must not emit inline shared secret")
	}
}

func TestResolvePanelReleaseURL(t *testing.T) {
	t.Setenv(panelReleaseURLEnv, "")
	if resolvePanelReleaseURL() != "" {
		t.Error("default release url must be empty (no public repo fallback)")
	}
	t.Setenv(panelReleaseURLEnv, "https://custom/panel.zip")
	if resolvePanelReleaseURL() != "https://custom/panel.zip" {
		t.Error("env override release url")
	}
}

