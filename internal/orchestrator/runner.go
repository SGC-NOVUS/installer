package orchestrator

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	osReleasePath          = "/etc/os-release"
	defaultOSDBName        = "novus_os"
	defaultIDDBName        = "novus_id"
	defaultSDDBName        = "novus_sd"
	defaultSystemDBUser    = "novus_panel"
	defaultIdentityDBUser  = "novus_id"
	defaultAgentAMD64URL   = "https://github.com/SGC-NOVUS/agent/releases/latest/download/novus-agent-linux-amd64"
	defaultAgentARM64URL   = "https://github.com/SGC-NOVUS/agent/releases/latest/download/novus-agent-linux-arm64"
	agentBinaryURLEnv      = "NOVUS_INSTALLER_AGENT_BINARY_URL"
	// Panel source: PRIVATE repository SGC-NOVUS/panel-core (NOT the empty public SGC-NOVUS/panel).
	// Requires GitHub PAT with repo-read scope, set via NOVUS_INSTALLER_PANEL_RELEASE_URL
	// or the env vars NOVUS_INSTALLER_GITHUB_PAT + NOVUS_INSTALLER_PANEL_RELEASE_URL.
	// In the future, panels will be SourceGuard-encrypted and published to the public SGC-NOVUS/panel repo.
	defaultPanelReleaseURL = ""  // Must be explicitly set via env — no default, we refuse to download from empty public repo.
	panelReleaseURLEnv     = "NOVUS_INSTALLER_PANEL_RELEASE_URL"
	panelCoreRepo          = "SGC-NOVUS/panel-core"
	panelCoreArchivePath   = "panel.zip"
	panelGithubPATEnv      = "NOVUS_INSTALLER_GITHUB_PAT"
	panelInstallRoot       = "/var/www/novus"
	panelPublicRoot        = "/var/www/novus/public"
	panelEnvPath           = "/var/www/novus/.env"
	panelSecretsDir        = "/etc/novus/secrets"
	panelSecretsEnvPath    = "/etc/novus/secrets/panel-novus.env"
	masterKeyFilePath      = "/etc/novus/secrets/master.key"
	pendingMasterKeyPath   = "/etc/novus/secrets/master.key.next"
	cloudflareSharedSecret = "/etc/novus/secrets/cloudflare_kms_shared_secret"
	nginxSitePath          = "/etc/nginx/sites-available/novus-installer.conf"
	nginxSiteEnabledPath   = "/etc/nginx/sites-enabled/novus-installer.conf"
	manifestDir            = "/etc/novus"
	sslAssetDir            = "/etc/novus/ssl"
	manifestPath           = "/etc/novus/manifest.json"
	panelArchivePath       = "/tmp/panel.zip"
	agentDownloadPath      = "/tmp/novus-agent"
	cloudflareCredsPath    = "/root/.secrets/certbot/cloudflare.ini"
	localHealthCheckURL    = "http://127.0.0.1/"
	defaultDryRunDelay     = time.Second
	healthCheckTimeout     = 5 * time.Second
	minPanelArchiveBytes   = int64(10 * 1024)
	minAgentBinaryBytes    = int64(1024)
)

var ErrAlreadyRunning = errors.New("install_already_running")

var securityEntranceReservedSegments = map[string]struct{}{
	"api":                   {},
	"assets":                {},
	"auth":                  {},
	"build":                 {},
	"favicon.ico":           {},
	"g":                     {},
	"index.php":             {},
	"login":                 {},
	"p":                     {},
	"public":                {},
	"setup":                 {},
	"apple-touch-icon.png":  {},
	"safari-pinned-tab.svg": {},
}

var integrationRequiredFields = map[string][]string{
	"telegram":              {"telegram_bot_token", "telegram_admin_chat_id", "telegram_client_secret"},
	"discord_notifications": {"discord_bot_token", "discord_admin_id", "discord_client_secret"},
	"google":                {"google_client_id", "google_client_secret"},
	"github":                {"github_client_id", "github_client_secret"},
	"github_cli":            {"github_pat"},
	"gemini":                {"gemini_api_key"},
	"discord":               {"discord_client_id", "discord_client_secret"},
	"cloudflare":            {"cloudflare_api_token", "cloudflare_account_id"},
	"steam":                 {"steam_web_api_key"},
	"smtp":                  {"smtp_host", "smtp_port", "smtp_user", "smtp_password", "smtp_from_name", "smtp_from_email"},
	"novus_agent":           {"installer_url", "installer_auth_header"},
}

type StatusMessage struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

type OutputSink interface {
	io.Writer
	EmitStatus(StatusMessage) error
}

type RunnerOptions struct {
	DevMode     bool
	DryRun      bool
	DryRunDelay time.Duration
	// AuditWriter overrides the destination for the JSON-line audit trail. When
	// nil the runner appends to defaultInstallAuditLogPath (falling back to
	// stderr if that path cannot be opened). Primarily used by tests.
	AuditWriter io.Writer
	// AuditDisabled turns the audit trail into a no-op.
	AuditDisabled bool
}

type SecurityEntranceConfig struct {
	Enabled       bool
	Path          string
	Port          int
	WindowSeconds int
	MaxAttempts   int
	BlockSeconds  int
}

type RestoreConfig struct {
	SourceType     string
	BackupURL      string
	BackupFile     string
	BackupPayload  string
	KeyMaterial    string
	RecoveryPhrase string
	ImportMode     string
}

type CloudflareKMSConfig struct {
	Enabled        bool
	APIToken       string
	AccountID      string
	ScriptName     string
	NamespaceTitle string
	ZoneID         string
	RoutePattern   string
	WorkerURL      string
}

type IntegrationConfig struct {
	Key     string
	Enabled bool
	Fields  map[string]string
}

type SetupRequest struct {
	Domain             string
	InstallMode        string
	UseLetsEncrypt     bool
	SSLMode            string
	CloudflareAPIToken string
	CustomCertificate  string
	CustomPrivateKey   string
	AdminEmail         string
	AdminUsername      string
	AdminPassword      string
	DBRootPassword     string
	DBPanelPassword    string
	MasterKeyMode      string
	MasterKey          string
	MasterKeyBackend   string
	GitHubPAT          string `json:"github_pat"` // GitHub PAT: backend receives as "github_pat" (via env) or "GitHubPAT" (from web form)
	GitHubPATAlt       string `json:"GitHubPAT"`   // Web form sends this casing — will be merged in Validate()
	SecurityEntrance   SecurityEntranceConfig
	Restore            RestoreConfig
	CloudflareKMS      CloudflareKMSConfig
	TelegramEnabled    bool
	TelegramBotToken   string
	TelegramAdminID    string
	DiscordEnabled     bool
	DiscordBotToken    string
	DiscordAdminID     string
	Integrations       []IntegrationConfig
}

type Step struct {
	Name string
	Run  func(context.Context, SetupRequest, *Runner) error
	// Rollback is an optional compensating action invoked when a later step
	// fails. It is only registered for steps that create installer-owned
	// artifacts that are safe to remove on abort (never secrets, databases, or
	// the master key). Rollback must be idempotent.
	Rollback func(context.Context, SetupRequest, *Runner) error
}

type Runner struct {
	output          OutputSink
	devMode         bool
	dryRun          bool
	dryRunDelay     time.Duration
	agentBinaryURL  string
	panelReleaseURL string
	audit           *InstallAuditLogger

	mu      sync.Mutex
	running bool
}

func NewRunner(output OutputSink, options RunnerOptions) *Runner {
	dryRunDelay := options.DryRunDelay
	if dryRunDelay <= 0 {
		dryRunDelay = defaultDryRunDelay
	}

	auditWriter := options.AuditWriter
	if auditWriter == nil {
		auditWriter = resolveInstallAuditWriter(defaultInstallAuditLogPath)
	}
	auditEnabled := !options.AuditDisabled

	return &Runner{
		output:          output,
		devMode:         options.DevMode,
		dryRun:          options.DryRun,
		dryRunDelay:     dryRunDelay,
		agentBinaryURL:  resolveAgentBinaryURL(),
		panelReleaseURL: resolvePanelReleaseURL(),
		audit:           NewInstallAuditLogger(auditWriter, auditEnabled),
	}
}

func (r *Runner) Start(ctx context.Context, request SetupRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrAlreadyRunning
	}
	r.running = true
	r.mu.Unlock()

	go r.run(ctx, request)
	return nil
}

func (r *Runner) run(ctx context.Context, request SetupRequest) {
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	domain, err := normalizeDomain(request.Domain)
	if err != nil {
		r.emitStatus(StatusMessage{Type: "error", Text: err.Error()})
		r.writeLine(fmt.Sprintf("\x1b[31m[FAILED]\x1b[0m %s\r\n", err.Error()))
		log.Printf("novus-installer invalid domain: %v", err)
		return
	}

	platform, err := detectPlatformProfile()
	if err != nil {
		r.emitStatus(StatusMessage{Type: "error", Text: err.Error()})
		r.writeLine(fmt.Sprintf("\x1b[31m[FAILED]\x1b[0m %s\r\n", err.Error()))
		log.Printf("novus-installer platform detection failed: %v", err)
		return
	}

	// Resolve GitHub PAT from all sources before passing to install steps.
	request.resolveGitHubPAT()

	finishURL := buildFinishURL(domain, request.effectiveSSLMode())
	steps := r.buildInstallSteps(request, domain, platform)

	r.emitStatus(StatusMessage{Type: "step", Text: "Подготовка install pipeline"})
	r.writeLine("\x1b[35m[NOVUS]\x1b[0m Installer orchestration started.\r\n")
	r.writeLine(fmt.Sprintf("\x1b[2mTarget domain: %s | Admin email: %s | OS: %s %s\x1b[0m\r\n", domain, sanitizeField(request.AdminEmail), platform.ID, platform.VersionID))

	r.audit.Log(InstallAuditEvent{Event: "install_start", Total: len(steps)})

	completed := make([]Step, 0, len(steps))
	for index, step := range steps {
		r.emitStatus(StatusMessage{Type: "step", Text: step.Name})
		r.writeLine(fmt.Sprintf("\r\n\x1b[34m==> [%d/%d] %s\x1b[0m\r\n", index+1, len(steps), step.Name))
		r.audit.Log(InstallAuditEvent{Event: "step_start", Step: step.Name, Index: index + 1, Total: len(steps)})

		started := time.Now()
		if err := step.Run(ctx, request, r); err != nil {
			r.audit.Log(InstallAuditEvent{Event: "step_failed", Step: step.Name, Index: index + 1, Total: len(steps), Outcome: "error", Reason: err.Error(), DurationMs: time.Since(started).Milliseconds()})
			r.emitStatus(StatusMessage{Type: "error", Text: fmt.Sprintf("%s: %s", step.Name, err.Error())})
			r.writeLine(fmt.Sprintf("\x1b[31m[FAILED]\x1b[0m %s\r\n", err.Error()))
			log.Printf("novus-installer orchestrator failed: %v", err)
			r.rollback(ctx, request, completed)
			r.audit.Log(InstallAuditEvent{Event: "install_aborted", Step: step.Name, Index: index + 1, Total: len(steps), Reason: err.Error()})
			return
		}
		if err := ctx.Err(); err != nil {
			r.audit.Log(InstallAuditEvent{Event: "step_canceled", Step: step.Name, Index: index + 1, Total: len(steps), Outcome: "canceled", Reason: err.Error(), DurationMs: time.Since(started).Milliseconds()})
			r.emitStatus(StatusMessage{Type: "error", Text: fmt.Sprintf("%s: %v", step.Name, err)})
			r.writeLine(fmt.Sprintf("\x1b[31m[FAILED]\x1b[0m orchestration canceled: %v\r\n", err))
			log.Printf("novus-installer orchestrator canceled: %v", err)
			r.rollback(ctx, request, completed)
			r.audit.Log(InstallAuditEvent{Event: "install_aborted", Step: step.Name, Index: index + 1, Total: len(steps), Reason: err.Error()})
			return
		}

		r.audit.Log(InstallAuditEvent{Event: "step_success", Step: step.Name, Index: index + 1, Total: len(steps), Outcome: "ok", DurationMs: time.Since(started).Milliseconds()})
		completed = append(completed, step)
	}

	r.audit.Log(InstallAuditEvent{Event: "install_complete", Total: len(steps), Outcome: "ok"})
	r.emitStatus(StatusMessage{Type: "finish", Text: "Установка успешно завершена", URL: finishURL})
	r.writeLine("\r\n\x1b[32m[SUCCESS]\x1b[0m Installation steps completed successfully.\r\n")
	log.Printf("novus-installer orchestrator finished successfully")
	r.maybeSelfDestruct()
}

// rollback invokes the compensating action of every completed step that defines
// one, in reverse order. Rollback is best-effort: a failing compensator is
// logged and the sweep continues so the remaining artifacts are still cleaned.
func (r *Runner) rollback(ctx context.Context, request SetupRequest, completed []Step) {
	hasRollback := false
	for _, step := range completed {
		if step.Rollback != nil {
			hasRollback = true
			break
		}
	}
	if !hasRollback && len(completed) == 0 {
		return
	}

	r.audit.Log(InstallAuditEvent{Event: "rollback_start"})
	r.emitStatus(StatusMessage{Type: "step", Text: "Полный откат установки — удаление всех артефактов"})
	r.writeLine("\r\n\x1b[33m==> Полный откат установки\x1b[0m\r\n")

	// 1. Per-step rollback handlers (reverse order).
	for i := len(completed) - 1; i >= 0; i-- {
		step := completed[i]
		if step.Rollback == nil {
			continue
		}
		r.audit.Log(InstallAuditEvent{Event: "rollback_step_start", Step: step.Name})
		started := time.Now()
		if err := step.Rollback(ctx, request, r); err != nil {
			r.audit.Log(InstallAuditEvent{Event: "rollback_step_failed", Step: step.Name, Outcome: "error", Reason: err.Error(), DurationMs: time.Since(started).Milliseconds()})
			r.writeLine(fmt.Sprintf("\x1b[31m[ROLLBACK FAILED]\x1b[0m %s: %s\r\n", step.Name, err.Error()))
			log.Printf("novus-installer rollback failed for %q: %v", step.Name, err)
			continue
		}
		r.audit.Log(InstallAuditEvent{Event: "rollback_step_success", Step: step.Name, Outcome: "ok", DurationMs: time.Since(started).Milliseconds()})
		r.writeLine(fmt.Sprintf("\x1b[33m[ROLLBACK]\x1b[0m %s\r\n", step.Name))
	}

	// 2. Full cleanup: remove ALL installer artifacts unconditionally.
	r.writeLine("\x1b[33m[ROLLBACK]\x1b[0m Полная очистка артефактов установки...\r\n")
	if err := r.fullCleanup(ctx); err != nil {
		r.writeLine(fmt.Sprintf("\x1b[31m[CLEANUP FAILED]\x1b[0m %s\r\n", err.Error()))
		log.Printf("novus-installer full cleanup failed: %v", err)
	} else {
		r.writeLine("\x1b[33m[ROLLBACK]\x1b[0m Все артефакты удалены.\r\n")
	}

	r.audit.Log(InstallAuditEvent{Event: "rollback_complete"})
}

// fullCleanup removes all installer-created artifacts: panel root, DB data,
// secrets, nginx configs, binary, manifest, etc. Runs on install failure.
func (r *Runner) fullCleanup(ctx context.Context) error {
	if r.dryRun {
		r.writeLine("[DRY-RUN] Would remove all NOVUS-OS artifacts.\r\n")
		return nil
	}

	// Stop services first.
	for _, svc := range []string{"novus-agent", "php8.5-fpm", "php-fpm", "nginx"} {
		_ = r.runPTYCommand(ctx, "systemctl stop "+svc+" 2>/dev/null || true")
	}

	// Wipe paths.
	for _, path := range []string{
		panelInstallRoot,    // /var/www/novus
		panelSecretsDir,     // /etc/novus/secrets
		manifestDir,         // /etc/novus
		sslAssetDir,         // /etc/novus/ssl
		nginxSitePath,       // nginx site config
		nginxSiteEnabledPath,
		panelArchivePath,    // /tmp/panel.zip
		agentDownloadPath,   // /tmp/novus-agent
	} {
		if err := os.RemoveAll(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("fullCleanup: remove %s: %v", path, err)
		}
	}

	// Drop databases (if MariaDB is still running).
	dropSQL := fmt.Sprintf(
		"DROP DATABASE IF EXISTS %s; DROP DATABASE IF EXISTS %s; DROP DATABASE IF EXISTS %s; DROP USER IF EXISTS '%s'@'localhost'; DROP USER IF EXISTS '%s'@'localhost';",
		defaultOSDBName, defaultIDDBName, defaultSDDBName,
		defaultSystemDBUser, defaultIdentityDBUser,
	)
	_ = r.runPTYCommand(ctx, "(mariadb -u root -e "+shellQuote(dropSQL)+" 2>/dev/null || mysql -u root -e "+shellQuote(dropSQL)+" 2>/dev/null || true)")

	// Remove binary.
	_ = os.Remove("/usr/local/bin/novus-agent")
	_ = os.Remove("/usr/local/bin/novus-installer")
	_ = os.Remove("/usr/local/bin/sgc-agent")

	// Reload systemd.
	_ = os.Remove("/etc/systemd/system/novus-agent.service")
	_ = os.Remove("/etc/systemd/system/sgc-agent.service")
	_ = r.runPTYCommand(ctx, "systemctl daemon-reload 2>/dev/null || true")

	return nil
}

// removeInstallerArtifacts deletes installer-owned files created during this
// run. It is a no-op in dry-run mode and ignores already-absent paths so it can
// be invoked safely from compensating rollback handlers.
func (r *Runner) removeInstallerArtifacts(paths ...string) error {
	if r.dryRun {
		return nil
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (req SetupRequest) Validate() error {
	checks := map[string]string{
		"Domain":          req.Domain,
		"AdminEmail":      req.AdminEmail,
		"AdminPassword":   req.AdminPassword,
		"DBRootPassword":  req.DBRootPassword,
		"DBPanelPassword": req.DBPanelPassword,
	}

	for field, value := range checks {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("setup_request_invalid:%s_required", strings.ToLower(field))
		}
	}

	if err := validateInstallHost(req.Domain); err != nil {
		return err
	}
	if err := validateAdminEmail(req.AdminEmail); err != nil {
		return err
	}
	if err := validatePasswordStrength("admin_password", req.AdminPassword); err != nil {
		return err
	}
	if err := validatePasswordStrength("db_root_password", req.DBRootPassword); err != nil {
		return err
	}
	if err := validatePasswordStrength("db_panel_password", req.DBPanelPassword); err != nil {
		return err
	}

	switch req.effectiveInstallMode() {
	case "new_install", "restore":
		// Supported install surfaces.
	default:
		return fmt.Errorf("setup_request_invalid:install_mode_invalid")
	}

	switch req.effectiveSSLMode() {
	case "letsencrypt":
		// No additional payload fields required.
	case "cloudflare":
		if strings.TrimSpace(req.CloudflareAPIToken) == "" {
			return fmt.Errorf("setup_request_invalid:cloudflare_api_token_required")
		}
	case "custom":
		if strings.TrimSpace(req.CustomCertificate) == "" {
			return fmt.Errorf("setup_request_invalid:custom_certificate_required")
		}
		if strings.TrimSpace(req.CustomPrivateKey) == "" {
			return fmt.Errorf("setup_request_invalid:custom_private_key_required")
		}
		if err := validateCustomTLSMaterial(req.CustomCertificate, req.CustomPrivateKey); err != nil {
			return err
		}
	default:
		return fmt.Errorf("setup_request_invalid:ssl_mode_invalid")
	}

	switch req.effectiveMasterKeyMode() {
	case "automatic":
		// Generated during environment rendering.
	case "manual":
		if strings.TrimSpace(req.MasterKey) == "" {
			return fmt.Errorf("setup_request_invalid:master_key_required")
		}
	default:
		return fmt.Errorf("setup_request_invalid:master_key_mode_invalid")
	}

	if req.SecurityEntrance.Enabled {
		if err := validateSecurityEntranceConfig(req.SecurityEntrance); err != nil {
			return err
		}
	}

	if req.effectiveInstallMode() == "restore" {
		if req.effectiveRestoreSourceType() == "" {
			return fmt.Errorf("setup_request_invalid:restore_source_required")
		}
		if strings.TrimSpace(req.effectiveRestoreKeyMaterial()) == "" {
			return fmt.Errorf("setup_request_invalid:restore_key_material_required")
		}
	}

	if req.effectiveMasterKeyBackend() == "tier2_cloudflare_zero_disk_kms" {
		if strings.TrimSpace(req.effectiveCloudflareKMSAPIToken()) == "" {
			return fmt.Errorf("setup_request_invalid:cloudflare_kms_api_token_required")
		}
		if strings.TrimSpace(req.CloudflareKMS.AccountID) == "" {
			return fmt.Errorf("setup_request_invalid:cloudflare_kms_account_id_required")
		}
		if strings.TrimSpace(req.CloudflareKMS.WorkerURL) == "" {
			return fmt.Errorf("setup_request_invalid:cloudflare_kms_worker_url_required")
		}
	}

	// GitHub PAT: validate format if present; warn if missing (deferred to panel deploy step).
	if strings.TrimSpace(req.GitHubPAT) != "" {
		pat := strings.TrimSpace(req.GitHubPAT)
		if !strings.HasPrefix(pat, "github_pat_") && !strings.HasPrefix(pat, "ghp_") {
			return fmt.Errorf("setup_request_invalid:github_pat_format_invalid")
		}
	}

	for _, integration := range req.normalizedIntegrations() {
		if !integration.Enabled {
			continue
		}
		required, ok := integrationRequiredFields[integration.Key]
		if !ok {
			return fmt.Errorf("setup_request_invalid:integration_provider_invalid:%s", integration.Key)
		}
		for _, field := range required {
			if strings.TrimSpace(integration.Fields[field]) == "" {
				return fmt.Errorf("setup_request_invalid:%s_required", field)
			}
		}
	}

	return nil
}

func (req SetupRequest) effectiveInstallMode() string {
	if strings.EqualFold(strings.TrimSpace(req.InstallMode), "restore") {
		return "restore"
	}

	return "new_install"
}

// resolveGitHubPAT merges GitHub PAT from all sources: direct field, alt casing,
// github_cli integration, and environment variable. The result is stored in req.GitHubPAT.
func (req *SetupRequest) resolveGitHubPAT() {
	if strings.TrimSpace(req.GitHubPAT) != "" {
		return
	}
	if pat := strings.TrimSpace(req.GitHubPATAlt); pat != "" {
		req.GitHubPAT = pat
		return
	}
	// Check integrations (normalized — handles any casing from web form).
	for _, integration := range req.normalizedIntegrations() {
		if integration.Key == "github_cli" && integration.Enabled {
			if pat := strings.TrimSpace(integration.Fields["github_pat"]); pat != "" {
				req.GitHubPAT = pat
				return
			}
		}
	}
	if pat := strings.TrimSpace(os.Getenv(panelGithubPATEnv)); pat != "" {
		req.GitHubPAT = pat
	}
}

func (req SetupRequest) effectiveSSLMode() string {
	mode := strings.ToLower(strings.TrimSpace(req.SSLMode))
	switch mode {
	case "letsencrypt", "cloudflare", "custom":
		return mode
	}

	if strings.TrimSpace(req.CloudflareAPIToken) != "" {
		return "cloudflare"
	}
	if strings.TrimSpace(req.CustomCertificate) != "" || strings.TrimSpace(req.CustomPrivateKey) != "" {
		return "custom"
	}
	if req.UseLetsEncrypt {
		return "letsencrypt"
	}

	return "letsencrypt"
}

func (req SetupRequest) effectiveMasterKeyMode() string {
	mode := strings.ToLower(strings.TrimSpace(req.MasterKeyMode))
	if mode == "manual" || mode == "import" {
		return "manual"
	}

	return "automatic"
}

func (req SetupRequest) effectiveMasterKeyBackend() string {
	switch strings.ToLower(strings.TrimSpace(req.MasterKeyBackend)) {
	case "tier2", "cloudflare", "cloudflare_kms", "tier2_cloudflare_zero_disk_kms":
		return "tier2_cloudflare_zero_disk_kms"
	case "tier3", "tpm2", "hardware_tpm", "tier3_tpm2_hardware_sealing":
		return "tier3_tpm2_hardware_sealing"
	default:
		if req.CloudflareKMS.Enabled {
			return "tier2_cloudflare_zero_disk_kms"
		}
		return "tier1_hybrid_auto_unseal"
	}
}

func (req SetupRequest) effectiveRestoreSourceType() string {
	mode := strings.ToLower(strings.TrimSpace(req.Restore.SourceType))
	switch mode {
	case "url", "file", "inline":
		return mode
	}

	if strings.TrimSpace(req.Restore.BackupPayload) != "" {
		return "inline"
	}
	if strings.TrimSpace(req.Restore.BackupFile) != "" {
		return "file"
	}
	if strings.TrimSpace(req.Restore.BackupURL) != "" {
		return "url"
	}

	return ""
}

func (req SetupRequest) effectiveRestoreImportMode() string {
	mode := strings.ToLower(strings.TrimSpace(req.Restore.ImportMode))
	if mode == "skip_existing" {
		return "skip_existing"
	}

	if mode == "report_only" {
		return "report_only"
	}

	return "overwrite"
}

func (req SetupRequest) effectiveRestoreKeyMaterial() string {
	if value := strings.TrimSpace(req.Restore.KeyMaterial); value != "" {
		return value
	}

	return normalizeWhitespace(strings.ToLower(strings.TrimSpace(req.Restore.RecoveryPhrase)))
}

func (req SetupRequest) effectiveCloudflareKMSAPIToken() string {
	if value := strings.TrimSpace(req.CloudflareKMS.APIToken); value != "" {
		return value
	}

	return strings.TrimSpace(req.CloudflareAPIToken)
}

func (req SetupRequest) normalizedIntegrations() []IntegrationConfig {
	if len(req.Integrations) > 0 {
		seen := make(map[string]struct{}, len(req.Integrations))
		integrations := make([]IntegrationConfig, 0, len(req.Integrations))
		for _, item := range req.Integrations {
			key := strings.ToLower(strings.TrimSpace(item.Key))
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			fields := make(map[string]string, len(item.Fields))
			for fieldKey, fieldValue := range item.Fields {
				fields[strings.ToLower(strings.TrimSpace(fieldKey))] = strings.TrimSpace(fieldValue)
			}
			integrations = append(integrations, IntegrationConfig{
				Key:     key,
				Enabled: item.Enabled,
				Fields:  fields,
			})
		}
		return integrations
	}

	legacy := make([]IntegrationConfig, 0, 2)
	if req.TelegramEnabled || strings.TrimSpace(req.TelegramBotToken) != "" || strings.TrimSpace(req.TelegramAdminID) != "" {
		legacy = append(legacy, IntegrationConfig{
			Key:     "telegram",
			Enabled: req.TelegramEnabled || strings.TrimSpace(req.TelegramBotToken) != "" || strings.TrimSpace(req.TelegramAdminID) != "",
			Fields: map[string]string{
				"telegram_bot_token":     strings.TrimSpace(req.TelegramBotToken),
				"telegram_admin_chat_id": strings.TrimSpace(req.TelegramAdminID),
			},
		})
	}
	if req.DiscordEnabled || strings.TrimSpace(req.DiscordBotToken) != "" || strings.TrimSpace(req.DiscordAdminID) != "" {
		legacy = append(legacy, IntegrationConfig{
			Key:     "discord_notifications",
			Enabled: req.DiscordEnabled || strings.TrimSpace(req.DiscordBotToken) != "" || strings.TrimSpace(req.DiscordAdminID) != "",
			Fields: map[string]string{
				"discord_bot_token": strings.TrimSpace(req.DiscordBotToken),
				"discord_admin_id":  strings.TrimSpace(req.DiscordAdminID),
			},
		})
	}

	return legacy
}

func (req SetupRequest) hasPanelRuntimeConfiguration() bool {
	return req.SecurityEntrance.Enabled || len(req.normalizedIntegrations()) > 0
}

func validateSecurityEntranceConfig(cfg SecurityEntranceConfig) error {
	path := strings.Trim(strings.ToLower(cfg.Path), " /")
	if path == "" {
		return fmt.Errorf("setup_request_invalid:security_entrance_path_required")
	}
	if _, reserved := securityEntranceReservedSegments[path]; reserved {
		return fmt.Errorf("setup_request_invalid:security_entrance_path_reserved")
	}
	if len(path) < 3 || len(path) > 64 {
		return fmt.Errorf("setup_request_invalid:security_entrance_path_invalid")
	}
	for _, symbol := range path {
		if (symbol < 'a' || symbol > 'z') && (symbol < '0' || symbol > '9') && symbol != '-' && symbol != '_' {
			return fmt.Errorf("setup_request_invalid:security_entrance_path_invalid")
		}
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("setup_request_invalid:security_entrance_port_invalid")
	}
	if cfg.WindowSeconds < 0 || cfg.WindowSeconds > 86400 {
		return fmt.Errorf("setup_request_invalid:security_entrance_window_invalid")
	}
	if cfg.MaxAttempts < 0 || cfg.MaxAttempts > 1000 {
		return fmt.Errorf("setup_request_invalid:security_entrance_attempts_invalid")
	}
	if cfg.BlockSeconds < 0 || cfg.BlockSeconds > 604800 {
		return fmt.Errorf("setup_request_invalid:security_entrance_block_invalid")
	}

	return nil
}

func (r *Runner) buildInstallSteps(request SetupRequest, domain string, platform platformProfile) []Step {
	steps := []Step{
		{
			Name: "Очистка кэша APT и удаление несовместимых репозиториев",
			Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, "rm -f /etc/apt/sources.list.d/ondrej*.list /etc/apt/sources.list.d/sury-php.list /etc/apt/sources.list.d/mariadb*.list 2>/dev/null; apt-get update -qq 2>/dev/null || apt-get update -o Acquire::AllowInsecureRepositories=true -o Acquire::AllowDowngradeToInsecureRepositories=true || true")
			},
		},
		{
			Name: "Системные зависимости",
			Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, systemDependenciesCommand())
			},
		},
		{
			Name: "Репозитории PHP 8.5 и MariaDB",
			Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, repositoriesCommand(platform))
			},
		},
		{
			Name: "Установка стека Nginx, MariaDB и PHP 8.5",
			Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, stackInstallCommand())
			},
		},
		{
			Name: "Настройка MariaDB",
			Run: func(ctx context.Context, req SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, mariaDBConfigurationCommand(req))
			},
		},
		{
			Name: "Установка и настройка сервисов (Redis, Supervisor, Fail2Ban, UFW, Cron)",
			Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, servicesSetupCommand())
			},
		},
		{
			Name: "Установка novus-agent",
			Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, agentInstallCommand(runner.agentBinaryURL))
			},
		},
		{
			Name: "Конфигурация Nginx и SSL",
			Run: func(ctx context.Context, req SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, nginxAndSSLCommand(req, domain, req.AdminEmail))
			},
			Rollback: func(_ context.Context, _ SetupRequest, runner *Runner) error {
				return runner.removeInstallerArtifacts(nginxSiteEnabledPath, nginxSitePath)
			},
		},
		{
			Name: "Развертывание кода Панели",
			Run: func(ctx context.Context, req SetupRequest, runner *Runner) error {
				// Resolve PAT one final time from ALL sources before deploying.
				pat := strings.TrimSpace(req.GitHubPAT)
				if pat == "" {
					pat = strings.TrimSpace(os.Getenv(panelGithubPATEnv))
				}
				if pat == "" {
					// Try integrations as last resort.
					for _, integration := range req.normalizedIntegrations() {
						if integration.Key == "github_cli" && integration.Enabled {
							pat = strings.TrimSpace(integration.Fields["github_pat"])
							break
						}
					}
				}
				req.GitHubPAT = pat

				panelURL := runner.panelReleaseURL
				if strings.TrimSpace(panelURL) == "" {
					// Build URL using the resolved PAT.
					ref := strings.TrimSpace(os.Getenv("NOVUS_INSTALLER_PANEL_CORE_REF"))
					if ref == "" {
						ref = "main"
					}
					panelURL = fmt.Sprintf("https://api.github.com/repos/%s/zipball/%s", panelCoreRepo, url.PathEscape(ref))
				}
				if strings.TrimSpace(panelURL) == "" {
					return fmt.Errorf("panel_release_url_missing: set NOVUS_INSTALLER_PANEL_RELEASE_URL or provide github_pat for private SGC-NOVUS/panel-core access")
				}
				return runner.runPTYCommand(ctx, panelDeploymentCommand(panelURL, req.GitHubPAT))
			},
		},
		{
			Name: "Генерация секретов и .env",
			Run: func(ctx context.Context, req SetupRequest, runner *Runner) error {
				return runner.generatePanelEnvironment(ctx, req, domain)
			},
		},
		{
			Name: "Инициализация БД (Bridge Layer)",
			Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
				return runner.runPTYCommand(ctx, panelBridgeCommand())
			},
		},
	}

	if request.effectiveInstallMode() == "restore" {
		steps = append(steps, Step{
			Name: "Восстановление конфигурации панели",
			Run: func(ctx context.Context, req SetupRequest, runner *Runner) error {
				return runner.applyRestoreBundle(ctx, req)
			},
		})
	}

	if request.hasPanelRuntimeConfiguration() {
		steps = append(steps, Step{
			Name: "Настройка Security Entrance и интеграций",
			Run: func(ctx context.Context, req SetupRequest, runner *Runner) error {
				return runner.applyPanelConfiguration(ctx, req)
			},
		})
	}

	if request.effectiveMasterKeyBackend() == "tier2_cloudflare_zero_disk_kms" {
		steps = append(steps, Step{
			Name: "Развёртывание Cloudflare KMS",
			Run: func(ctx context.Context, req SetupRequest, runner *Runner) error {
				return runner.deployCloudflareKMS(ctx, req)
			},
		})
	}

	steps = append(steps, Step{
		Name: "Удаление setup-артефактов панели",
		Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
			return runner.runPTYCommand(ctx, panelSetupStripCommand())
		},
	})

	steps = append(steps,
		Step{
			Name: "Canonical Bootstrap Manifest",
			Run: func(ctx context.Context, req SetupRequest, runner *Runner) error {
				return runner.writeBootstrapManifest(ctx, req, domain)
			},
			Rollback: func(_ context.Context, _ SetupRequest, runner *Runner) error {
				return runner.removeInstallerArtifacts(manifestPath)
			},
		},
		Step{
			Name: "Проверка работоспособности системы",
			Run: func(ctx context.Context, _ SetupRequest, runner *Runner) error {
				return runner.runSystemHealthCheck(ctx, domain)
			},
		},
	)

	return steps
}

func (r *Runner) runPTYCommand(ctx context.Context, command string) error {
	if r.dryRun {
		r.writeLine(fmt.Sprintf("\x1b[33m[DRY-RUN] Would execute: %s\x1b[0m\r\n", command))
		return sleepContext(ctx, r.dryRunDelay)
	}

	r.writeLine(fmt.Sprintf("\x1b[33m$ %s\x1b[0m\r\n", command))

	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
		"LC_ALL=C.UTF-8",
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"FORCE_COLOR=1",
		"COLUMNS=120",
		"LINES=40",
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("pty_start_failed:%s: %w", command, err)
	}

	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(r.output, ptmx)
		copyDone <- copyErr
	}()

	waitErr := cmd.Wait()
	_ = ptmx.Close()
	copyErr := <-copyDone

	if ignorePTYReadError(copyErr) {
		copyErr = nil
	}
	if waitErr != nil {
		return fmt.Errorf("command_failed:%s: %w", command, waitErr)
	}
	if copyErr != nil {
		return fmt.Errorf("pty_stream_failed:%s: %w", command, copyErr)
	}

	return nil
}

func (r *Runner) runLocalAction(ctx context.Context, description string, action func() error) error {
	if r.dryRun {
		r.writeLine(fmt.Sprintf("\x1b[33m[DRY-RUN] Would execute: %s\x1b[0m\r\n", description))
		return sleepContext(ctx, r.dryRunDelay)
	}

	r.writeLine(fmt.Sprintf("\x1b[33m$ %s\x1b[0m\r\n", description))
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := action(); err != nil {
		return fmt.Errorf("action_failed:%s: %w", description, err)
	}

	return nil
}

func (r *Runner) emitStatus(message StatusMessage) {
	if err := r.output.EmitStatus(message); err != nil {
		log.Printf("novus-installer status emit failed: %v", err)
	}
}

func (r *Runner) writeLine(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	_, _ = io.WriteString(r.output, line)
}

func systemDependenciesCommand() string {
	return strings.Join([]string{
		// Fix any previously-interrupted dpkg state.
		"dpkg --configure -a 2>/dev/null || true",
		// Remove stale dpkg locks from aborted runs (safe fallback).
		"rm -f /var/lib/dpkg/lock-frontend /var/lib/apt/lists/lock /var/cache/apt/archives/lock 2>/dev/null || true",
		"apt-get update -qq 2>/dev/null || apt-get update -o Acquire::AllowInsecureRepositories=true -o Acquire::AllowDowngradeToInsecureRepositories=true 2>/dev/null || true",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y software-properties-common curl wget git unzip ufw || true",
	}, " && ")
}

func repositoriesCommand(platform platformProfile) string {
	// Official repos for all key components — OS-version independent.
	// Each repo setup is non-fatal (|| true) so the installer never blocks.
	addPHPRepo := "LC_ALL=C.UTF-8 add-apt-repository -y ppa:ondrej/php 2>/dev/null || true"
	addMariaDBRepo := "curl -sS https://downloads.mariadb.com/MariaDB/mariadb_repo_setup | bash 2>/dev/null || true"
	addRedisRepo := "rm -f /usr/share/keyrings/redis-archive-keyring.gpg 2>/dev/null; curl -fsSL https://packages.redis.io/gpg | gpg --dearmor --yes -o /usr/share/keyrings/redis-archive-keyring.gpg 2>/dev/null; echo \"deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(. /etc/os-release 2>/dev/null && echo ${VERSION_CODENAME:-noble}) main\" > /etc/apt/sources.list.d/redis.list 2>/dev/null || true"

	if platform.ID == "debian" {
		return strings.Join([]string{
			"apt-get install -y ca-certificates apt-transport-https gnupg2",
			// PHP from sury.org (Debian)
			"(curl -fsSL https://packages.sury.org/php/apt.gpg -o /usr/share/keyrings/sury-php.gpg 2>/dev/null || true)",
			"(. /etc/os-release 2>/dev/null; echo \"deb [signed-by=/usr/share/keyrings/sury-php.gpg] https://packages.sury.org/php/ ${VERSION_CODENAME:-bookworm} main\" > /etc/apt/sources.list.d/sury-php.list 2>/dev/null || true)",
			"(" + addMariaDBRepo + " 2>/dev/null || true)",
			"(" + addRedisRepo + " 2>/dev/null || true)",
			"apt-get update -qq 2>/dev/null || apt-get update -o Acquire::AllowInsecureRepositories=true || true",
		}, " && ")
	}

	return strings.Join([]string{
		// Clean any previously-broken repo files.
		"rm -f /etc/apt/sources.list.d/ondrej*.list /etc/apt/sources.list.d/ondrej*.sources /etc/apt/sources.list.d/mariadb*.list /etc/apt/sources.list.d/redis.list 2>/dev/null",
		// PHP 8.5 (ondrej PPA — covers all Ubuntu LTS)
		"(" + addPHPRepo + ")",
		// MariaDB (official repo — covers all distros)
		"(" + addMariaDBRepo + ")",
		// Redis (official repo — covers all distros)
		"(" + addRedisRepo + ")",
		// Refresh package lists
		"apt-get update -qq 2>/dev/null || apt-get update -o Acquire::AllowInsecureRepositories=true || true",
	}, " && ")
}

func stackInstallCommand() string {
	return strings.Join([]string{
		"apt-get update",
		// Full stack: Nginx + MariaDB + PHP 8.5 + all extensions + Redis + tools + security.
		"DEBIAN_FRONTEND=noninteractive apt-get install -y " +
			"nginx mariadb-server " +
			"php8.5-fpm php8.5-cli php8.5-mysql php8.5-mbstring php8.5-xml php8.5-curl php8.5-zip " +
			"php8.5-bcmath php8.5-intl php8.5-gd php8.5-redis php8.5-grpc " +
			"redis-server supervisor fail2ban ufw " +
			"certbot python3-certbot-nginx python3-certbot-dns-cloudflare " +
			"curl wget git unzip jq openssl ca-certificates gnupg openssh-server " +
			"|| DEBIAN_FRONTEND=noninteractive apt-get install -y " +
			"nginx mariadb-server php8.5-fpm php8.5-cli php8.5-mysql php8.5-mbstring php8.5-xml php8.5-curl php8.5-zip " +
			"php8.5-bcmath php8.5-intl php8.5-gd php8.5-redis php8.5-grpc " +
			"redis-server supervisor fail2ban ufw " +
			"certbot python3-certbot-nginx python3-certbot-dns-cloudflare " +
			"curl wget git unzip jq openssl ca-certificates gnupg openssh-server",
		// Enable base services.
		"systemctl enable nginx mariadb 2>/dev/null || true",
		"systemctl enable php8.5-fpm 2>/dev/null || true",
		"systemctl start nginx mariadb 2>/dev/null || true",
		"systemctl start php8.5-fpm 2>/dev/null || true",
	}, " && ")
}

func mariaDBConfigurationCommand(req SetupRequest) string {
	// Write SQL to temp file using printf (no here-doc escaping issues).
	safeRoot := escapeSQLLiteral(req.DBRootPassword)
	safePanel := escapeSQLLiteral(req.DBPanelPassword)
	sql := fmt.Sprintf(
		"ALTER USER 'root'@'localhost' IDENTIFIED BY '%s'; CREATE DATABASE IF NOT EXISTS %s; CREATE DATABASE IF NOT EXISTS %s; CREATE DATABASE IF NOT EXISTS %s; CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON %s.* TO '%s'@'localhost'; GRANT ALL PRIVILEGES ON %s.* TO '%s'@'localhost'; GRANT ALL PRIVILEGES ON %s.* TO '%s'@'localhost'; FLUSH PRIVILEGES;",
		safeRoot,
		defaultOSDBName, defaultIDDBName, defaultSDDBName,
		defaultSystemDBUser, safePanel,
		defaultIdentityDBUser, safePanel,
		defaultOSDBName, defaultSystemDBUser,
		defaultIDDBName, defaultIdentityDBUser,
		defaultSDDBName, defaultSystemDBUser,
	)

	// Use printf to write SQL to file — avoids all here-doc and pipe escaping issues.
	return fmt.Sprintf(
		`(printf %%s %s > /tmp/novus_db_setup.sql && (mariadb < /tmp/novus_db_setup.sql 2>/dev/null || mysql < /tmp/novus_db_setup.sql 2>/dev/null || true) && rm -f /tmp/novus_db_setup.sql)`,
		shellQuote(sql),
	)
}

func agentInstallCommand(agentBinaryURL string) string {
	return strings.Join([]string{
		"curl -fL --retry 3 --connect-timeout 15 " + shellQuote(agentBinaryURL) + " -o " + shellQuote(agentDownloadPath),
		"[ -s " + shellQuote(agentDownloadPath) + " ]",
		"test \"$(stat -c%s " + shellQuote(agentDownloadPath) + ")\" -ge " + strconv.FormatInt(minAgentBinaryBytes, 10),
		"install -m 0755 " + shellQuote(agentDownloadPath) + " /usr/local/bin/novus-agent",
	}, " && ")
}

func servicesSetupCommand() string {
	return strings.Join([]string{
		// Redis
		"systemctl enable --now redis-server 2>/dev/null || true",
		// Supervisor
		"systemctl enable --now supervisor 2>/dev/null || true",
		// Fail2Ban
		"systemctl enable --now fail2ban 2>/dev/null || true",
		// UFW — default deny incoming, allow essential ports
		"ufw --force disable 2>/dev/null || true",
		"ufw default deny incoming 2>/dev/null || true",
		"ufw default allow outgoing 2>/dev/null || true",
		"ufw allow 22/tcp 2>/dev/null || true",
		"ufw allow 80/tcp 2>/dev/null || true",
		"ufw allow 443/tcp 2>/dev/null || true",
		"ufw allow 8080/tcp 2>/dev/null || true",
		"ufw allow 9443/tcp 2>/dev/null || true",
		"ufw --force enable 2>/dev/null || true",
		// Cron jobs (from panel templates)
		"if [ -f /var/www/novus/deploy/templates/novus-panel.cron ]; then " +
			"sed -e 's|{{PHP_BIN}}|/usr/bin/php8.5|g' -e 's|{{PANEL_DIR}}|/var/www/novus|g' -e 's|^\\* \\* \\* \\* \\* www |* * * * * www |' " +
			"/var/www/novus/deploy/templates/novus-panel.cron > /etc/cron.d/novus-panel 2>/dev/null; " +
			"chmod 644 /etc/cron.d/novus-panel 2>/dev/null; " +
			"systemctl reload cron 2>/dev/null || systemctl reload crond 2>/dev/null || true; " +
			"fi",
	}, " && ")
}

func nginxAndSSLCommand(req SetupRequest, domain string, email string) string {
	commands := []string{
		"mkdir -p " + shellQuote(panelPublicRoot),
	}

	effectiveSSLMode := req.effectiveSSLMode()
	if isIPAddressHost(domain) && effectiveSSLMode != "custom" {
		effectiveSSLMode = "none"
	}

	switch effectiveSSLMode {
	case "cloudflare":
		commands = append(commands,
			writeFileCommand(nginxSitePath, renderNginxVHost(domain)),
			"ln -sf "+shellQuote(nginxSitePath)+" "+shellQuote(nginxSiteEnabledPath),
			"rm -f /etc/nginx/sites-enabled/default",
			"nginx -t",
			"systemctl reload nginx || systemctl restart nginx",
			"install -d -m 0700 "+shellQuote("/root/.secrets/certbot"),
			writeFileCommand(cloudflareCredsPath, "dns_cloudflare_api_token = "+strings.TrimSpace(req.CloudflareAPIToken)+"\n"),
			"chmod 600 "+shellQuote(cloudflareCredsPath),
			"certbot certonly --dns-cloudflare --dns-cloudflare-credentials "+shellQuote(cloudflareCredsPath)+" -d "+shellQuote(domain)+" -m "+shellQuote(strings.TrimSpace(email))+" --non-interactive --agree-tos",
			writeFileCommand(nginxSitePath, renderNginxTLSVHost(domain, letsEncryptFullchainPath(domain), letsEncryptPrivkeyPath(domain))),
			"nginx -t",
			"systemctl reload nginx || systemctl restart nginx",
		)
	case "custom":
		certPath := customCertificatePath(domain)
		keyPath := customPrivateKeyPath(domain)
		commands = append(commands,
			"install -d -m 0700 "+shellQuote(sslAssetDir),
			writeFileCommand(certPath, strings.TrimSpace(req.CustomCertificate)+"\n"),
			writeFileCommand(keyPath, strings.TrimSpace(req.CustomPrivateKey)+"\n"),
			"chmod 600 "+shellQuote(certPath)+" "+shellQuote(keyPath),
			writeFileCommand(nginxSitePath, renderNginxTLSVHost(domain, certPath, keyPath)),
			"ln -sf "+shellQuote(nginxSitePath)+" "+shellQuote(nginxSiteEnabledPath),
			"rm -f /etc/nginx/sites-enabled/default",
			"nginx -t",
			"systemctl reload nginx || systemctl restart nginx",
		)
	default:
		commands = append(commands,
			writeFileCommand(nginxSitePath, renderNginxVHost(domain)),
			"ln -sf "+shellQuote(nginxSitePath)+" "+shellQuote(nginxSiteEnabledPath),
			"rm -f /etc/nginx/sites-enabled/default",
			"nginx -t",
			"systemctl reload nginx || systemctl restart nginx",
		)
		if effectiveSSLMode == "letsencrypt" {
			commands = append(commands, "certbot --nginx -d "+shellQuote(domain)+" -m "+shellQuote(strings.TrimSpace(email))+" --non-interactive --agree-tos")
		}
	}

	return strings.Join(commands, " && ")
}

func isIPAddressHost(host string) bool {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return false
	}

	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err == nil {
			trimmed = parsed.Hostname()
		}
	}

	trimmed = strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")
	return net.ParseIP(trimmed) != nil
}

func buildFinishURL(domain string, sslMode string) string {
	scheme := "https"
	if isIPAddressHost(domain) && strings.TrimSpace(sslMode) != "custom" {
		scheme = "http"
	}

	return scheme + "://" + domain
	}

func panelDeploymentCommand(panelReleaseURL string, githubPAT string) string {
	// If downloading from api.github.com (private repo), test PAT validity first.
	pat := strings.TrimSpace(githubPAT)
	if pat == "" {
		pat = strings.TrimSpace(os.Getenv(panelGithubPATEnv))
	}

	curlCmd := "curl -fL --retry 3 --connect-timeout 15 " + shellQuote(panelReleaseURL) + " -o " + shellQuote(panelArchivePath)
	if pat != "" && strings.Contains(panelReleaseURL, "api.github.com") {
		curlCmd = "curl -fL --retry 3 --connect-timeout 15 -H " +
			shellQuote("Authorization: token "+pat) + " " +
			shellQuote(panelReleaseURL) + " -o " + shellQuote(panelArchivePath)
	}

	return strings.Join([]string{
		"mkdir -p " + shellQuote(panelInstallRoot),
		curlCmd,
		"[ -s " + shellQuote(panelArchivePath) + " ]",
		"test \"$(stat -c%s " + shellQuote(panelArchivePath) + ")\" -ge " + strconv.FormatInt(minPanelArchiveBytes, 10),
		"unzip -tq " + shellQuote(panelArchivePath),
		// GitHub zipball extracts into a subdirectory (repo-branch/). Move contents up.
		"unzip -oq " + shellQuote(panelArchivePath) + " -d /tmp/novus_panel_extract",
		"EXTRACT_DIR=$(ls -d /tmp/novus_panel_extract/*/ 2>/dev/null | head -1)",
		"[ -n \"$EXTRACT_DIR\" ] && rsync -a \"$EXTRACT_DIR\" " + shellQuote(panelInstallRoot) + "/ || unzip -oq " + shellQuote(panelArchivePath) + " -d " + shellQuote(panelInstallRoot),
		"rm -rf /tmp/novus_panel_extract",
		"chown -R www-data:www-data " + shellQuote(panelInstallRoot),
	}, " && ")
}

func panelBridgeCommand() string {
	return strings.Join([]string{
		"cd " + shellQuote(panelInstallRoot),
		// Install PHP dependencies first — required for artisan to work.
		"composer install --no-dev --prefer-dist --no-interaction --optimize-autoloader 2>/dev/null || composer install --no-dev --no-interaction 2>/dev/null || true",
		"sudo -u www-data php artisan migrate --force",
		"sudo -u www-data php artisan novus:setup-foundation",
	}, " && ")
}

func panelSetupStripCommand() string {
	return strings.Join([]string{
		"cd " + shellQuote(panelInstallRoot),
		"rm -f resources/views/setup.php",
		"rm -f resources/js/setup.js",
		"rm -rf resources/js/setup",
		"if [ -f routes/api.php ]; then sed -i '/use App\\\\Controllers\\\\Api\\\\SetupController;/d;/\\/api\\/setup\\//d' routes/api.php; fi",
		"rm -f app/Controllers/Api/SetupController.php",
		"if [ -d app/Services/Setup ]; then find app/Services/Setup -type f ! -name 'SetupStateService.php' -delete; find app/Services/Setup -type d -empty -delete; fi",
		"rm -f app/Services/Integrations/SetupProviderVerificationClient.php",
		"rm -f tools/setup_foundation.php",
		"rm -f tools/migrations/setup_foundation.php",
		"if [ -d database/migrations ]; then find database/migrations -maxdepth 1 -type f -name '*setup*' -delete; fi",
		"if [ -d tests/Unit/Setup ]; then rm -rf tests/Unit/Setup; fi",
		"if [ -d tests/Feature/Setup ]; then rm -rf tests/Feature/Setup; fi",
		"chown -R www-data:www-data " + shellQuote(panelInstallRoot),
	}, " && ")
}

func (r *Runner) generatePanelEnvironment(ctx context.Context, req SetupRequest, domain string) error {
	appKey, err := generateLaravelAppKey()
	if err != nil {
		return err
	}
	masterKey, err := resolveInstallerMasterKey(req)
	if err != nil {
		return err
	}
	sharedSecret, err := resolveCloudflareSharedSecret(req)
	if err != nil {
		return err
	}
	envContents := renderPanelEnvironment(req, buildFinishURL(domain, req.effectiveSSLMode()), appKey, masterKey, sharedSecret)
	description := "generate panel env, runtime secrets, and write bootstrap files"

	return r.runLocalAction(ctx, description, func() error {
		uid, gid, err := lookupUserIDs("www-data")
		if err != nil {
			return err
		}
		if err := ensureOwnedDirectory(panelInstallRoot, 0o755, uid, gid); err != nil {
			return err
		}
		if err := ensureOwnedDirectory(panelSecretsDir, 0o750, uid, gid); err != nil {
			return err
		}
		if err := writeOwnedFile(panelEnvPath, []byte(envContents), 0o640, uid, gid); err != nil {
			return err
		}
		if err := writeOwnedFile(panelSecretsEnvPath, []byte(envContents), 0o640, uid, gid); err != nil {
			return err
		}
		if err := writeOwnedFile(masterKeyFilePath, []byte(masterKey+"\n"), 0o600, uid, gid); err != nil {
			return err
		}
		if err := writeOwnedFile(pendingMasterKeyPath, []byte{}, 0o600, uid, gid); err != nil {
			return err
		}
		if sharedSecret != "" {
			if err := writeOwnedFile(cloudflareSharedSecret, []byte(sharedSecret+"\n"), 0o600, uid, gid); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Runner) writeBootstrapManifest(ctx context.Context, req SetupRequest, domain string) error {
	manifest := bootstrapManifest{
		SchemaVersion:    "1.1",
		InstallMode:      req.effectiveInstallMode(),
		PanelVersion:     "latest",
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		DBTarget:         "localhost",
		TargetDomain:     domain,
		SSLMode:          req.effectiveSSLMode(),
		MasterKeyBackend: req.effectiveMasterKeyBackend(),
		RestoreSource:    req.effectiveRestoreSourceType(),
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest_json_encode_failed:%w", err)
	}
	payload = append(payload, '\n')

	return r.runLocalAction(ctx, "write "+manifestPath, func() error {
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, payload, 0o600); err != nil {
			return err
		}
		return os.Chmod(manifestPath, 0o600)
	})
}

func (r *Runner) applyRestoreBundle(ctx context.Context, req SetupRequest) error {
	if req.effectiveInstallMode() != "restore" {
		return nil
	}

	payload, err := resolveRestorePayload(req)
	if err != nil {
		return err
	}

	config := map[string]string{
		"payload":      payload,
		"passphrase":   req.effectiveRestoreKeyMaterial(),
		"import_mode":  req.effectiveRestoreImportMode(),
		"source_type":  req.effectiveRestoreSourceType(),
		"source_label": restoreSourceLabel(req),
	}

	script := `<?php
declare(strict_types=1);

chdir('/var/www/novus');
require '/var/www/novus/bootstrap/app.php';

$raw = @file_get_contents($argv[1] ?? '');
$input = json_decode((string) $raw, true);
if (!is_array($input)) {
	fwrite(STDERR, "restore_payload_invalid\n");
	exit(1);
}

$payload = trim((string) ($input['payload'] ?? ''));
$passphrase = trim((string) ($input['passphrase'] ?? ''));
$mode = trim((string) ($input['import_mode'] ?? 'overwrite'));
if ($payload === '' || $passphrase === '') {
	fwrite(STDERR, "restore_arguments_missing\n");
	exit(1);
}

$previewMode = $mode;
if ($previewMode === 'report_only') {
	$previewMode = 'overwrite';
}

$extract = static function (string $rawPayload): string {
	$rawPayload = trim($rawPayload);
	if ($rawPayload === '') {
		throw new RuntimeException('restore_payload_empty');
	}

	$decoded = json_decode($rawPayload, true);
	if (is_array($decoded)) {
		$isEnvelope = ($decoded['v'] ?? null) === 1
			&& isset($decoded['salt'])
			&& isset($decoded['iv'])
			&& isset($decoded['tag'])
			&& isset($decoded['ct']);
		if ($isEnvelope) {
			return $rawPayload;
		}

		$candidate = $decoded['settings_payload'] ?? null;
		if (is_string($candidate) && trim($candidate) !== '') {
			return trim($candidate);
		}

		$candidateEnvelope = $decoded['settings_envelope'] ?? null;
		if (is_array($candidateEnvelope)) {
			$json = json_encode($candidateEnvelope, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES);
			if ($json !== false) {
				return $json;
			}
		}
	}

	throw new RuntimeException('restore_bundle_unsupported');
};

try {
	$envelope = $extract($payload);
	$container = app();
	$settings = $container->resolve(\App\Services\Settings\SettingsService::class);
	$preview = $settings->previewImportAll($envelope, $passphrase, $previewMode);
	fwrite(STDOUT, json_encode(['preview' => $preview], JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) . PHP_EOL);
	if (($preview['ok'] ?? false) !== true) {
		throw new RuntimeException('restore_preview_failed:' . (string) ($preview['error'] ?? 'unknown'));
	}
	if ($mode === 'report_only') {
		fwrite(STDOUT, json_encode(['import' => ['ok' => true, 'skipped' => true, 'reason' => 'report_only']], JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) . PHP_EOL);
		exit(0);
	}
	$result = $settings->importAll($envelope, $passphrase, $mode);
	fwrite(STDOUT, json_encode(['import' => $result], JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) . PHP_EOL);
	if (($result['ok'] ?? false) !== true) {
		throw new RuntimeException('restore_import_failed:' . (string) ($result['error'] ?? 'unknown'));
	}
} catch (Throwable $e) {
	fwrite(STDERR, $e->getMessage() . PHP_EOL);
	exit(1);
}
`

	return r.runPanelBootstrapJSONTask(ctx, "restore panel settings bundle", config, script)
}

func (r *Runner) applyPanelConfiguration(ctx context.Context, req SetupRequest) error {
	if !req.hasPanelRuntimeConfiguration() {
		return nil
	}

	config := map[string]any{
		"security_entrance": map[string]any{
			"enabled":        req.SecurityEntrance.Enabled,
			"path":           strings.Trim(req.SecurityEntrance.Path, " /"),
			"port":           req.SecurityEntrance.Port,
			"window_seconds": req.SecurityEntrance.WindowSeconds,
			"max_attempts":   req.SecurityEntrance.MaxAttempts,
			"block_seconds":  req.SecurityEntrance.BlockSeconds,
		},
		"integrations": req.normalizedIntegrations(),
	}

	script := `<?php
declare(strict_types=1);

chdir('/var/www/novus');
require '/var/www/novus/bootstrap/app.php';

$raw = @file_get_contents($argv[1] ?? '');
$input = json_decode((string) $raw, true);
if (!is_array($input)) {
	fwrite(STDERR, "panel_configuration_payload_invalid\n");
	exit(1);
}

$container = app();
$settings = $container->resolve(\App\Services\Settings\SettingsService::class);

$set = static function ($result, string $field): void {
	if (!is_array($result) || (($result['ok'] ?? false) !== true && !array_key_exists('created', $result))) {
		$message = is_array($result) ? (string) ($result['error'] ?? 'unknown') : 'unknown';
		throw new RuntimeException('panel_setting_save_failed:' . $field . ':' . $message);
	}
};

try {
	$securityEntrance = is_array($input['security_entrance'] ?? null) ? $input['security_entrance'] : [];
	if (($securityEntrance['enabled'] ?? false) === true) {
		$service = $container->resolve(\App\Services\Security\SecurityEntranceService::class);
		$saved = $service->save([
			'path' => (string) ($securityEntrance['path'] ?? ''),
			'port' => (int) ($securityEntrance['port'] ?? 0),
			'window_seconds' => (int) ($securityEntrance['window_seconds'] ?? 300),
			'max_attempts' => (int) ($securityEntrance['max_attempts'] ?? 8),
			'block_seconds' => (int) ($securityEntrance['block_seconds'] ?? 900),
		]);
		fwrite(STDOUT, json_encode(['security_entrance' => $saved], JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) . PHP_EOL);
	}

	$providers = is_array($input['integrations'] ?? null) ? $input['integrations'] : [];
	$providerService = $container->resolve(\App\Services\Integrations\IntegrationProviderService::class);
	foreach ($providers as $provider) {
		if (!is_array($provider) || (($provider['enabled'] ?? false) !== true)) {
			continue;
		}
		$key = strtolower(trim((string) ($provider['key'] ?? '')));
		$fields = is_array($provider['fields'] ?? null) ? $provider['fields'] : [];

		if ($key === 'discord_notifications') {
			$set($settings->set('integrations', 'discord_bot_token', (string) ($fields['discord_bot_token'] ?? ''), true, 'Installer-managed Discord bot token.'), 'discord_bot_token');
			$set($settings->set('integrations', 'discord_admin_id', (string) ($fields['discord_admin_id'] ?? ''), false, 'Installer-managed Discord admin/channel id.'), 'discord_admin_id');
			fwrite(STDOUT, json_encode(['integration' => ['provider' => $key, 'saved' => true]], JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) . PHP_EOL);
			continue;
		}

		$saved = $providerService->saveProvider($key, $fields, ['enabled' => true]);
		if (($saved['ok'] ?? false) !== true) {
			throw new RuntimeException('integration_save_failed:' . $key . ':' . (string) ($saved['error'] ?? 'unknown'));
		}
		fwrite(STDOUT, json_encode(['integration' => $saved], JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) . PHP_EOL);
	}
} catch (Throwable $e) {
	fwrite(STDERR, $e->getMessage() . PHP_EOL);
	exit(1);
}
`

	return r.runPanelBootstrapJSONTask(ctx, "apply panel settings and integration catalog", config, script)
}

func (r *Runner) deployCloudflareKMS(ctx context.Context, req SetupRequest) error {
	config := map[string]string{
		"api_token":       req.effectiveCloudflareKMSAPIToken(),
		"account_id":      strings.TrimSpace(req.CloudflareKMS.AccountID),
		"script_name":     strings.TrimSpace(req.CloudflareKMS.ScriptName),
		"namespace_title": strings.TrimSpace(req.CloudflareKMS.NamespaceTitle),
		"zone_id":         strings.TrimSpace(req.CloudflareKMS.ZoneID),
		"route_pattern":   strings.TrimSpace(req.CloudflareKMS.RoutePattern),
		"worker_url":      strings.TrimSpace(req.CloudflareKMS.WorkerURL),
		"shared_secret":   strings.TrimSpace(readOptionalFile(cloudflareSharedSecret)),
		"master_key_hex":  strings.TrimSpace(readOptionalFile(masterKeyFilePath)),
	}

	script := `<?php
declare(strict_types=1);

chdir('/var/www/novus');
require '/var/www/novus/bootstrap/app.php';

$raw = @file_get_contents($argv[1] ?? '');
$input = json_decode((string) $raw, true);
if (!is_array($input)) {
	fwrite(STDERR, "cloudflare_kms_payload_invalid\n");
	exit(1);
}

try {
	$service = app()->resolve(\App\Services\Security\CloudflareKmsDeployerService::class);
	$result = $service->deploy($input);
	fwrite(STDOUT, json_encode(['cloudflare_kms' => $result], JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) . PHP_EOL);
} catch (Throwable $e) {
	fwrite(STDERR, $e->getMessage() . PHP_EOL);
	exit(1);
}
`

	return r.runPanelBootstrapJSONTask(ctx, "deploy Cloudflare KMS backend", config, script)
}

func (r *Runner) runPanelBootstrapJSONTask(ctx context.Context, description string, payload any, script string) error {
	if r.dryRun {
		r.writeLine(fmt.Sprintf("\x1b[33m[DRY-RUN] Would execute: %s\x1b[0m\r\n", description))
		return sleepContext(ctx, r.dryRunDelay)
	}

	return r.runLocalAction(ctx, description, func() error {
		encodedPayload, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("panel_task_payload_encode_failed:%w", err)
		}

		// The PHP task runs as www-data via sudo, so the secret-bearing temp
		// files must be readable by that user while staying private to others.
		taskUID, taskGID, err := lookupUserIDs("www-data")
		if err != nil {
			return fmt.Errorf("panel_task_owner_lookup_failed:%w", err)
		}

		phpFile, err := os.CreateTemp("/tmp", "novus-installer-*.php")
		if err != nil {
			return fmt.Errorf("panel_task_php_tempfile_failed:%w", err)
		}
		phpPath := phpFile.Name()
		defer os.Remove(phpPath)
		// Restrict permissions before writing: these files may carry secrets
		// (master key, KMS config) and /tmp is world-readable.
		if err := phpFile.Chmod(0o600); err != nil {
			_ = phpFile.Close()
			return fmt.Errorf("panel_task_php_chmod_failed:%w", err)
		}
		if err := phpFile.Chown(taskUID, taskGID); err != nil {
			_ = phpFile.Close()
			return fmt.Errorf("panel_task_php_chown_failed:%w", err)
		}
		if _, err := phpFile.WriteString(script); err != nil {
			_ = phpFile.Close()
			return fmt.Errorf("panel_task_php_write_failed:%w", err)
		}
		if err := phpFile.Close(); err != nil {
			return fmt.Errorf("panel_task_php_close_failed:%w", err)
		}

		jsonFile, err := os.CreateTemp("/tmp", "novus-installer-*.json")
		if err != nil {
			return fmt.Errorf("panel_task_json_tempfile_failed:%w", err)
		}
		jsonPath := jsonFile.Name()
		defer os.Remove(jsonPath)
		if err := jsonFile.Chmod(0o600); err != nil {
			_ = jsonFile.Close()
			return fmt.Errorf("panel_task_json_chmod_failed:%w", err)
		}
		if err := jsonFile.Chown(taskUID, taskGID); err != nil {
			_ = jsonFile.Close()
			return fmt.Errorf("panel_task_json_chown_failed:%w", err)
		}
		if _, err := jsonFile.Write(encodedPayload); err != nil {
			_ = jsonFile.Close()
			return fmt.Errorf("panel_task_json_write_failed:%w", err)
		}
		if err := jsonFile.Close(); err != nil {
			return fmt.Errorf("panel_task_json_close_failed:%w", err)
		}

		cmd := exec.CommandContext(ctx, "sudo", "-u", "www-data", "php", phpPath, jsonPath)
		cmd.Dir = panelInstallRoot
		cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
		output, err := cmd.CombinedOutput()
		if len(output) > 0 {
			r.writeLine(string(output))
		}
		if err != nil {
			return fmt.Errorf("panel_task_failed:%s: %w", description, err)
		}

		return nil
	})
}

func (r *Runner) runSystemHealthCheck(ctx context.Context, domain string) error {
	description := fmt.Sprintf("http GET %s with Host: %s", localHealthCheckURL, domain)

	return r.runLocalAction(ctx, description, func() error {
		client := &http.Client{
			Timeout: healthCheckTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, localHealthCheckURL, nil)
		if err != nil {
			return fmt.Errorf("health_check_request_build_failed:%w", err)
		}
		req.Host = domain

		resp, err := client.Do(req)
		if err != nil {
			if isTimeoutError(err) {
				return fmt.Errorf("health_check_timeout:%w", err)
			}
			return fmt.Errorf("health_check_request_failed:%w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK, http.StatusMovedPermanently, http.StatusFound, http.StatusUnauthorized:
			return nil
		case http.StatusInternalServerError, http.StatusBadGateway:
			return fmt.Errorf("health_check_failed:status_%d", resp.StatusCode)
		default:
			return fmt.Errorf("health_check_unexpected_status:%d", resp.StatusCode)
		}
	})
}

func (r *Runner) maybeSelfDestruct() {
	if r.devMode || r.dryRun {
		return
	}

	executablePath, err := os.Executable()
	if err != nil {
		log.Printf("novus-installer self-destruct skipped: executable path unavailable: %v", err)
		return
	}

	if err := os.Remove(executablePath); err != nil {
		log.Printf("novus-installer self-destruct failed: %v", err)
		return
	}

	log.Printf("novus-installer self-destruct removed executable: %s", executablePath)
}

// nginxErrorPagesBlock renders NOVUS-branded static error pages for ANY HTTP
// status code. 404/502/50x have dedicated pages; every other code falls back to
// the universal error.html, into which nginx injects the real status code via
// sub_filter (the page's inline JS then resolves a localized title/description).
// These pages live in panel public/ and are served by nginx even when PHP-FPM is
// down. __CODE__ is the placeholder substituted with $status.
const nginxErrorPagesBlock = `
	# NOVUS OS — branded error pages for any status code.
	error_page 404 /404.html;
	error_page 502 /502.html;
	error_page 500 503 504 /50x.html;
	error_page 400 401 402 403 405 406 407 408 409 410 411 412 413 414 415 416 417 418 421 422 423 424 425 426 428 429 431 451 501 505 506 507 508 510 511 /error.html;

	location = /404.html {
		internal;
		add_header Cache-Control "no-store" always;
	}

	location = /502.html {
		internal;
		add_header Cache-Control "no-store" always;
	}

	location = /50x.html {
		internal;
		add_header Cache-Control "no-store" always;
	}

	location = /error.html {
		internal;
		sub_filter '__CODE__' $status;
		sub_filter_once off;
		add_header Cache-Control "no-store" always;
	}
`

func renderNginxVHost(domain string) string {
	return fmt.Sprintf(`server {
	listen 80;
	listen [::]:80;
	server_name %s;
	root %s;
	index index.php index.html;

	location / {
		try_files $uri $uri/ /index.php?$query_string;
	}

	location ~ \.php$ {
		include snippets/fastcgi-php.conf;
		fastcgi_pass unix:/run/php/php8.5-fpm.sock;
		fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
		include fastcgi_params;
	}

	location ~ /\.(?!well-known).* {
		deny all;
	}
%s}`,
		domain,
		panelPublicRoot,
		nginxErrorPagesBlock,
	)
}

func renderNginxTLSVHost(domain string, certificatePath string, privateKeyPath string) string {
	return fmt.Sprintf(`server {
	listen 80;
	listen [::]:80;
	server_name %s;
	return 301 https://$host$request_uri;
}

server {
	listen 443 ssl http2;
	listen [::]:443 ssl http2;
	server_name %s;
	root %s;
	index index.php index.html;

	ssl_certificate %s;
	ssl_certificate_key %s;

	location / {
		try_files $uri $uri/ /index.php?$query_string;
	}

	location ~ \.php$ {
		include snippets/fastcgi-php.conf;
		fastcgi_pass unix:/run/php/php8.5-fpm.sock;
		fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
		include fastcgi_params;
	}

	location ~ /\.(?!well-known).* {
		deny all;
	}
%s}`,
		domain,
		domain,
		panelPublicRoot,
		certificatePath,
		privateKeyPath,
		nginxErrorPagesBlock,
	)
}

func letsEncryptFullchainPath(domain string) string {
	return "/etc/letsencrypt/live/" + strings.TrimSpace(domain) + "/fullchain.pem"
}

func letsEncryptPrivkeyPath(domain string) string {
	return "/etc/letsencrypt/live/" + strings.TrimSpace(domain) + "/privkey.pem"
}

func customCertificatePath(domain string) string {
	return sslAssetDir + "/" + strings.TrimSpace(domain) + ".crt"
}

func customPrivateKeyPath(domain string) string {
	return sslAssetDir + "/" + strings.TrimSpace(domain) + ".key"
}

func resolveAgentBinaryURL() string {
	if value := strings.TrimSpace(os.Getenv(agentBinaryURLEnv)); value != "" {
		return value
	}

	switch runtime.GOARCH {
	case "arm64":
		return defaultAgentARM64URL
	default:
		return defaultAgentAMD64URL
	}
}

func resolvePanelReleaseURL() string {
	// Explicit override always wins (e.g. custom hosting, local file:// or pre-signed URL).
	if value := strings.TrimSpace(os.Getenv(panelReleaseURLEnv)); value != "" {
		return value
	}

	// No default public URL — SGC-NOVUS/panel has NO releases.
	// Build a private GitHub archive download URL using PAT if available.
	pat := strings.TrimSpace(os.Getenv(panelGithubPATEnv))
	if pat != "" {
		// GitHub API: GET /repos/{owner}/{repo}/zipball/{ref}
		// ref defaults to main; set via NOVUS_INSTALLER_PANEL_CORE_REF
		ref := strings.TrimSpace(os.Getenv("NOVUS_INSTALLER_PANEL_CORE_REF"))
		if ref == "" {
			ref = "main"
		}
		// We use the API endpoint which redirects to codeload.github.com.
		// curl can follow redirects with -L, and we attach the PAT via header.
		return fmt.Sprintf("https://api.github.com/repos/%s/zipball/%s", panelCoreRepo, url.PathEscape(ref))
	}

	// Last resort: if env override explicitly set to a custom URL, use it. Otherwise fail.
	return ""
}

func detectPlatformProfile() (platformProfile, error) {
	file, err := os.Open(osReleasePath)
	if err != nil {
		return platformProfile{}, fmt.Errorf("platform_detect_failed:%w", err)
	}
	defer file.Close()

	profile := platformProfile{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		value = strings.Trim(value, "\"'")
		switch strings.TrimSpace(key) {
		case "ID":
			profile.ID = strings.ToLower(strings.TrimSpace(value))
		case "VERSION_ID":
			profile.VersionID = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return platformProfile{}, fmt.Errorf("platform_detect_failed:%w", err)
	}
	if profile.ID == "" {
		return platformProfile{}, fmt.Errorf("platform_detect_failed:missing_os_id")
	}

	return profile, nil
}

func normalizeDomain(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("domain_required")
	}

	parseTarget := trimmed
	if !strings.Contains(parseTarget, "://") {
		parseTarget = "https://" + parseTarget
	}

	parsed, err := url.Parse(parseTarget)
	if err != nil {
		return "", fmt.Errorf("domain_invalid:%w", err)
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", fmt.Errorf("domain_invalid:host_missing")
	}

	return host, nil
}

func renderPanelEnvironment(req SetupRequest, appURL string, appKey string, masterKey string, sharedSecret string) string {
	integrations := integrationFieldLookup(req.normalizedIntegrations())

	lines := []string{
		"APP_NAME=\"SGC-NOVUS OS\"",
		"APP_ENV=production",
		"APP_DEBUG=false",
		"APP_INSTALLED=1",
		"SETUP_ENABLED=0",
		"SETUP_FORCE_ENABLED=0",
		"APP_URL=" + quoteEnvValue(appURL),
		"APP_KEY=" + quoteEnvValue(appKey),
		"CORS_ALLOWED_ORIGIN=" + quoteEnvValue(appURL),
		"PANEL_PUBLIC_BASE_URL=" + quoteEnvValue(appURL),
		"TELEGRAM_AUTH_ORIGIN=" + quoteEnvValue(appURL),
		"SECRETS_MASTER_KEY_FILE=" + quoteEnvValue(masterKeyFilePath),
		"SECRETS_PENDING_MASTER_KEY_FILE=" + quoteEnvValue(pendingMasterKeyPath),
		"DB_HOST=127.0.0.1",
		"DB_PORT=3306",
		"DB_DATABASE=" + quoteEnvValue(defaultOSDBName),
		"DB_NAME=" + quoteEnvValue(defaultOSDBName),
		"DB_USERNAME=" + quoteEnvValue(defaultSystemDBUser),
		"DB_USER=" + quoteEnvValue(defaultSystemDBUser),
		"DB_PASSWORD=" + quoteEnvValue(req.DBPanelPassword),
		"OS_DB_HOST=127.0.0.1",
		"OS_DB_PORT=3306",
		"OS_DB_DATABASE=" + quoteEnvValue(defaultOSDBName),
		"OS_DB_USERNAME=" + quoteEnvValue(defaultSystemDBUser),
		"OS_DB_PASSWORD=" + quoteEnvValue(req.DBPanelPassword),
		"ID_DB_HOST=127.0.0.1",
		"ID_DB_PORT=3306",
		"ID_DB_DATABASE=" + quoteEnvValue(defaultIDDBName),
		"ID_DB_USERNAME=" + quoteEnvValue(defaultIdentityDBUser),
		"ID_DB_PASSWORD=" + quoteEnvValue(req.DBPanelPassword),
		"SD_DB_HOST=127.0.0.1",
		"SD_DB_PORT=3306",
		"SD_DB_DATABASE=" + quoteEnvValue(defaultSDDBName),
		"SD_DB_USERNAME=" + quoteEnvValue(defaultSystemDBUser),
		"SD_DB_PASSWORD=" + quoteEnvValue(req.DBPanelPassword),
		"NOVUS_MASTER_KEY_BACKEND=" + quoteEnvValue(req.effectiveMasterKeyBackend()),
		"NOVUS_MASTER_KEY_RUNTIME_PATH=" + quoteEnvValue("/dev/shm/novus_master.key"),
		"NOVUS_SSL_MODE=" + quoteEnvValue(req.effectiveSSLMode()),
		"NOVUS_MASTER_KEY_MODE=" + quoteEnvValue(req.effectiveMasterKeyMode()),
		"NOVUS_MASTER_KEY=" + quoteEnvValue(masterKey),
		"TELEGRAM_ENABLED=" + quoteEnvValue(strconv.FormatBool(integrationEnabled(req.normalizedIntegrations(), "telegram"))),
		"TELEGRAM_BOT_TOKEN=" + quoteEnvValue(firstNonEmpty(integrationField(integrations, "telegram", "telegram_bot_token"), strings.TrimSpace(req.TelegramBotToken))),
		"TELEGRAM_CHAT_ID=" + quoteEnvValue(firstNonEmpty(integrationField(integrations, "telegram", "telegram_admin_chat_id"), strings.TrimSpace(req.TelegramAdminID))),
		"TELEGRAM_ADMIN_ID=" + quoteEnvValue(firstNonEmpty(integrationField(integrations, "telegram", "telegram_admin_chat_id"), strings.TrimSpace(req.TelegramAdminID))),
		"TELEGRAM_CLIENT_ID=" + quoteEnvValue(integrationField(integrations, "telegram", "telegram_client_id")),
		"TELEGRAM_CLIENT_SECRET=" + quoteEnvValue(integrationField(integrations, "telegram", "telegram_client_secret")),
		"DISCORD_ENABLED=" + quoteEnvValue(strconv.FormatBool(integrationEnabled(req.normalizedIntegrations(), "discord_notifications"))),
		"DISCORD_BOT_TOKEN=" + quoteEnvValue(firstNonEmpty(integrationField(integrations, "discord_notifications", "discord_bot_token"), strings.TrimSpace(req.DiscordBotToken))),
		"DISCORD_ADMIN_ID=" + quoteEnvValue(firstNonEmpty(integrationField(integrations, "discord_notifications", "discord_admin_id"), strings.TrimSpace(req.DiscordAdminID))),
		"SGC_DISCORD_CLIENT_ID=" + quoteEnvValue(integrationField(integrations, "discord", "discord_client_id")),
		"SGC_DISCORD_CLIENT_SECRET=" + quoteEnvValue(firstNonEmpty(integrationField(integrations, "discord", "discord_client_secret"), integrationField(integrations, "discord_notifications", "discord_client_secret"))),
		"SGC_GOOGLE_CLIENT_ID=" + quoteEnvValue(integrationField(integrations, "google", "google_client_id")),
		"SGC_GOOGLE_CLIENT_SECRET=" + quoteEnvValue(integrationField(integrations, "google", "google_client_secret")),
		"SGC_GITHUB_CLIENT_ID=" + quoteEnvValue(integrationField(integrations, "github", "github_client_id")),
		"SGC_GITHUB_CLIENT_SECRET=" + quoteEnvValue(integrationField(integrations, "github", "github_client_secret")),
		"STEAM_WEB_API_KEY=" + quoteEnvValue(integrationField(integrations, "steam", "steam_web_api_key")),
		"NOVUS_CLOUDFLARE_KMS_WORKER_URL=" + quoteEnvValue(strings.TrimSpace(req.CloudflareKMS.WorkerURL)),
		"NOVUS_CLOUDFLARE_KMS_SHARED_SECRET_FILE=" + quoteEnvValue(cloudflareSharedSecret),
	}

	if req.effectiveMasterKeyBackend() == "tier2_cloudflare_zero_disk_kms" && sharedSecret != "" {
		lines = append(lines, "NOVUS_CLOUDFLARE_KMS_SHARED_SECRET="+quoteEnvValue(sharedSecret))
	}

	return strings.Join(lines, "\n") + "\n"
}

func writeFileCommand(path string, contents string) string {
	return "printf %s " + shellQuote(contents) + " > " + shellQuote(path)
}

func generateLaravelAppKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("app_key_generate_failed:%w", err)
	}

	return "base64:" + base64.StdEncoding.EncodeToString(raw), nil
}

func quoteEnvValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)
	return `"` + replacer.Replace(value) + `"`
}

func lookupUserIDs(username string) (int, int, error) {
	entry, err := user.Lookup(username)
	if err != nil {
		return 0, 0, fmt.Errorf("user_lookup_failed:%s: %w", username, err)
	}
	uid, err := strconv.Atoi(entry.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("user_uid_invalid:%s: %w", username, err)
	}
	gid, err := strconv.Atoi(entry.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("user_gid_invalid:%s: %w", username, err)
	}

	return uid, gid, nil
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	return errors.Is(err, context.DeadlineExceeded)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

var domainLabelPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// validateInstallHost enforces that the install target is either a valid DNS
// hostname or an IP address. The domain is interpolated into shell commands,
// nginx config and filesystem paths, so rejecting metacharacters here closes
// the shell/path/config-injection surface at the boundary.
func validateInstallHost(value string) error {
	host := strings.TrimSpace(value)
	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")

	if host == "" {
		return fmt.Errorf("setup_request_invalid:domain_required")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if len(host) > 253 {
		return fmt.Errorf("setup_request_invalid:domain_invalid")
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return fmt.Errorf("setup_request_invalid:domain_invalid")
	}
	for _, label := range labels {
		if !domainLabelPattern.MatchString(label) {
			return fmt.Errorf("setup_request_invalid:domain_invalid")
		}
	}
	return nil
}

// validateAdminEmail enforces RFC 5322 addr-spec form and rejects shell/control
// characters before the value reaches certbot/panel bootstrap commands.
func validateAdminEmail(value string) error {
	email := strings.TrimSpace(value)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return fmt.Errorf("setup_request_invalid:admin_email_invalid")
	}
	if strings.ContainsAny(email, " \t\r\n'\"`$;&|<>()\\") {
		return fmt.Errorf("setup_request_invalid:admin_email_invalid")
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || !strings.Contains(email[at+1:], ".") {
		return fmt.Errorf("setup_request_invalid:admin_email_invalid")
	}
	return nil
}

// validatePasswordStrength enforces a minimum strength policy for credentials
// that are later embedded into bootstrap SQL and panel provisioning. The policy
// requires at least 12 characters spanning three of four character classes
// (lowercase, uppercase, digit, symbol) and rejects control characters that
// could corrupt downstream command rendering.
func validatePasswordStrength(label string, value string) error {
	if strings.ContainsAny(value, "\x00\r\n") {
		return fmt.Errorf("setup_request_invalid:%s_invalid", label)
	}
	if len([]rune(value)) < 12 {
		return fmt.Errorf("setup_request_invalid:%s_too_weak", label)
	}

	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSymbol = true
		}
	}

	classes := 0
	for _, present := range []bool{hasLower, hasUpper, hasDigit, hasSymbol} {
		if present {
			classes++
		}
	}
	if classes < 3 {
		return fmt.Errorf("setup_request_invalid:%s_too_weak", label)
	}
	return nil
}

// validateCustomTLSMaterial verifies that the operator-supplied certificate and
// private key are well-formed PEM, parse as a usable x509 keypair, and that the
// leaf certificate is not already expired. This blocks malformed or mismatched
// material before it is written to disk and referenced by the nginx vhost.
func validateCustomTLSMaterial(certPEM string, keyPEM string) error {
	cert := []byte(strings.TrimSpace(certPEM) + "\n")
	key := []byte(strings.TrimSpace(keyPEM) + "\n")

	block, _ := pem.Decode(cert)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("setup_request_invalid:custom_certificate_invalid")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("setup_request_invalid:custom_certificate_invalid")
	}

	keyBlock, _ := pem.Decode(key)
	if keyBlock == nil || !strings.Contains(keyBlock.Type, "PRIVATE KEY") {
		return fmt.Errorf("setup_request_invalid:custom_private_key_invalid")
	}

	if _, err := tls.X509KeyPair(cert, key); err != nil {
		return fmt.Errorf("setup_request_invalid:custom_certificate_key_mismatch")
	}

	if now := time.Now(); now.After(leaf.NotAfter) {
		return fmt.Errorf("setup_request_invalid:custom_certificate_expired")
	}
	return nil
}

func escapeSQLLiteral(value string) string {
	// MySQL/MariaDB treat backslash as an escape character inside string
	// literals, so it must be doubled before single quotes are doubled to
	// prevent the closing quote from being neutralised (SQL injection).
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "'", "''")
	return value
}

// sanitizePasswordForShell removes shell-problematic characters from
// passwords that will be interpolated into command-line arguments.
// This is a safety net — the real fix is to pass SQL via stdin (here-doc).
func sanitizePasswordForShell(value string) string {
	// Safe alphanumeric + basic punctuation
	valid := strings.Builder{}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '-' || r == '_' || r == '!' || r == '#' || r == '%' ||
			r == '&' || r == '+' || r == '/' || r == '=' || r == '?' || r == '@' || r == '^' {
			valid.WriteRune(r)
		}
	}
	out := valid.String()
	if len(out) < 8 {
		out = "NOVUS_pw_" + strings.Repeat("x", 8-len(out))
	}
	return out
}

func resolveInstallerMasterKey(req SetupRequest) (string, error) {
	if req.effectiveMasterKeyMode() == "manual" {
		value := strings.TrimSpace(req.MasterKey)
		if value == "" {
			return "", fmt.Errorf("master_key_required")
		}
		return value, nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("master_key_generate_failed:%w", err)
	}

	return fmt.Sprintf("%x", raw), nil
}

func resolveCloudflareSharedSecret(req SetupRequest) (string, error) {
	if req.effectiveMasterKeyBackend() != "tier2_cloudflare_zero_disk_kms" {
		return "", nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("cloudflare_kms_shared_secret_generate_failed:%w", err)
	}

	return fmt.Sprintf("%x", raw), nil
}

func ensureOwnedDirectory(path string, perm os.FileMode, uid int, gid int) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return err
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}

func writeOwnedFile(path string, content []byte, perm os.FileMode, uid int, gid int) error {
	if err := os.WriteFile(path, content, perm); err != nil {
		return err
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}

func resolveRestorePayload(req SetupRequest) (string, error) {
	switch req.effectiveRestoreSourceType() {
	case "inline":
		return decodeInlinePayload(req.Restore.BackupPayload)
	case "file":
		payload, err := os.ReadFile(strings.TrimSpace(req.Restore.BackupFile))
		if err != nil {
			return "", fmt.Errorf("restore_backup_file_read_failed:%w", err)
		}
		return strings.TrimSpace(string(payload)), nil
	case "url":
		client := &http.Client{Timeout: 15 * time.Second}
		response, err := client.Get(strings.TrimSpace(req.Restore.BackupURL))
		if err != nil {
			return "", fmt.Errorf("restore_backup_url_read_failed:%w", err)
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return "", fmt.Errorf("restore_backup_url_invalid_status:%d", response.StatusCode)
		}
		payload, err := io.ReadAll(response.Body)
		if err != nil {
			return "", fmt.Errorf("restore_backup_url_read_failed:%w", err)
		}
		return strings.TrimSpace(string(payload)), nil
	default:
		return "", fmt.Errorf("restore_source_required")
	}
}

func restoreSourceLabel(req SetupRequest) string {
	switch req.effectiveRestoreSourceType() {
	case "url":
		return strings.TrimSpace(req.Restore.BackupURL)
	case "file":
		return strings.TrimSpace(req.Restore.BackupFile)
	case "inline":
		return "inline_payload"
	default:
		return ""
	}
}

func decodeInlinePayload(payload string) (string, error) {
	value := strings.TrimSpace(payload)
	if value == "" {
		return "", fmt.Errorf("restore_backup_payload_empty")
	}
	if !strings.HasPrefix(value, "data:") {
		return value, nil
	}
	comma := strings.Index(value, ",")
	if comma < 0 {
		return "", fmt.Errorf("restore_inline_payload_invalid")
	}
	meta := strings.ToLower(value[:comma])
	body := value[comma+1:]
	if strings.Contains(meta, ";base64") {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return "", fmt.Errorf("restore_inline_payload_invalid:%w", err)
		}
		return string(decoded), nil
	}
	decoded, err := url.QueryUnescape(body)
	if err != nil {
		return "", fmt.Errorf("restore_inline_payload_invalid:%w", err)
	}
	return decoded, nil
}

func integrationFieldLookup(integrations []IntegrationConfig) map[string]map[string]string {
	lookup := make(map[string]map[string]string, len(integrations))
	for _, integration := range integrations {
		fields := make(map[string]string, len(integration.Fields))
		for key, value := range integration.Fields {
			fields[key] = value
		}
		lookup[integration.Key] = fields
	}
	return lookup
}

func integrationField(lookup map[string]map[string]string, providerKey string, fieldKey string) string {
	provider, ok := lookup[providerKey]
	if !ok {
		return ""
	}
	return strings.TrimSpace(provider[fieldKey])
}

func integrationEnabled(integrations []IntegrationConfig, providerKey string) bool {
	for _, integration := range integrations {
		if integration.Key == providerKey {
			return integration.Enabled
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func readOptionalFile(path string) string {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(payload)
}

func sanitizeField(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "(not provided)"
	}

	return trimmed
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func ignorePTYReadError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EIO) {
		return true
	}

	return strings.Contains(strings.ToLower(err.Error()), "input/output error")
}

type platformProfile struct {
	ID        string
	VersionID string
}

type bootstrapManifest struct {
	SchemaVersion    string `json:"schema_version"`
	InstallMode      string `json:"install_mode"`
	PanelVersion     string `json:"panel_version"`
	Timestamp        string `json:"timestamp"`
	DBTarget         string `json:"db_target"`
	TargetDomain     string `json:"target_domain"`
	SSLMode          string `json:"ssl_mode"`
	MasterKeyBackend string `json:"master_key_backend"`
	RestoreSource    string `json:"restore_source,omitempty"`
}
