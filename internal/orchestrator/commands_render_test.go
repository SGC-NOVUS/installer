package orchestrator

import (
	"strings"
	"testing"
)

func TestSystemAndStackCommands(t *testing.T) {
	if !strings.Contains(systemDependenciesCommand(), "apt-get install") {
		t.Error("system deps should install packages")
	}
	stack := stackInstallCommand()
	for _, want := range []string{"nginx", "mariadb-server", "php8.5-fpm", "certbot"} {
		if !strings.Contains(stack, want) {
			t.Errorf("stack install missing %q", want)
		}
	}
}

func TestRepositoriesCommandPerPlatform(t *testing.T) {
	debian := repositoriesCommand(platformProfile{ID: "debian", VersionID: "12"})
	if !strings.Contains(debian, "sury.org") {
		t.Error("debian repo should use sury.org")
	}
	ubuntu := repositoriesCommand(platformProfile{ID: "ubuntu", VersionID: "24.04"})
	if !strings.Contains(ubuntu, "ondrej/php") {
		t.Error("ubuntu repo should use ondrej ppa")
	}
}

func TestMariaDBConfigurationCommandEscapes(t *testing.T) {
	req := SetupRequest{DBRootPassword: "r'oot", DBPanelPassword: "p'anel"}
	cmd := mariaDBConfigurationCommand(req)
	if !strings.Contains(cmd, "CREATE DATABASE IF NOT EXISTS") {
		t.Error("should create databases")
	}
	// Implementation uses printf to write SQL to temp file, then pipes to mariadb/mysql.
	if !strings.Contains(cmd, "novus_db_setup.sql") {
		t.Error("should use temp SQL file approach")
	}
	if !strings.Contains(cmd, "mariadb <") && !strings.Contains(cmd, "mysql <") {
		t.Error("should pipe SQL file to mariadb or mysql")
	}
}

func TestAgentInstallCommand(t *testing.T) {
	cmd := agentInstallCommand("https://example.com/novus-agent")
	if !strings.Contains(cmd, "curl -fL") {
		t.Error("should download via curl")
	}
	if !strings.Contains(cmd, "/usr/local/bin/novus-agent") {
		t.Error("should install to /usr/local/bin")
	}
}

func TestNginxAndSSLCommandModes(t *testing.T) {
	base := SetupRequest{Domain: "panel.example.com"}

	le := base
	le.SSLMode = "letsencrypt"
	if cmd := nginxAndSSLCommand(le, "panel.example.com", "admin@example.com"); !strings.Contains(cmd, "certbot --nginx") {
		t.Error("letsencrypt should run certbot --nginx")
	}

	cf := base
	cf.SSLMode = "cloudflare"
	cf.CloudflareAPIToken = "cf-token"
	if cmd := nginxAndSSLCommand(cf, "panel.example.com", "admin@example.com"); !strings.Contains(cmd, "dns-cloudflare") {
		t.Error("cloudflare should use dns-cloudflare")
	}

	custom := base
	custom.SSLMode = "custom"
	custom.CustomCertificate = "CERT"
	custom.CustomPrivateKey = "KEY"
	if cmd := nginxAndSSLCommand(custom, "panel.example.com", "admin@example.com"); !strings.Contains(cmd, "ssl_certificate") {
		t.Error("custom should write TLS vhost")
	}

	// IP host without custom SSL downgrades to plain HTTP (no certbot).
	ip := base
	ip.SSLMode = "letsencrypt"
	if cmd := nginxAndSSLCommand(ip, "192.168.1.10", "admin@example.com"); strings.Contains(cmd, "certbot") {
		t.Error("IP host should not invoke certbot")
	}
}

func TestPanelCommands(t *testing.T) {
	if !strings.Contains(panelDeploymentCommand("https://example.com/panel.zip", ""), "unzip") {
		t.Error("deployment should unzip archive")
	}
	if !strings.Contains(panelBridgeCommand(), "novus:migrate-nodes") {
		t.Error("bridge should reference novus:migrate-nodes")
	}
	if !strings.Contains(panelBridgeCommand(), "sudo -u www-data") {
		t.Error("bridge should run composer/artisan as www-data")
	}
	if !strings.Contains(panelSetupStripCommand(), "SetupController.php") {
		t.Error("strip should remove setup controller")
	}
}

func TestRenderNginxVHost(t *testing.T) {
	vhost := renderNginxVHost("panel.example.com")
	if !strings.Contains(vhost, "server_name panel.example.com") {
		t.Error("missing server_name")
	}
	if !strings.Contains(vhost, "php8.5-fpm.sock") {
		t.Error("missing php-fpm socket")
	}
}

func TestRenderNginxTLSVHost(t *testing.T) {
	vhost := renderNginxTLSVHost("panel.example.com", "/c.pem", "/k.pem")
	if !strings.Contains(vhost, "ssl_certificate /c.pem;") {
		t.Error("missing certificate path")
	}
	if !strings.Contains(vhost, "ssl_certificate_key /k.pem;") {
		t.Error("missing key path")
	}
	if !strings.Contains(vhost, "return 301 https://") {
		t.Error("missing http->https redirect")
	}
}

func TestQuoteEnvValueEscaping(t *testing.T) {
	got := quoteEnvValue("a\"b\\c\nd")
	if !strings.Contains(got, "\\\"") || !strings.Contains(got, "\\\\") || !strings.Contains(got, "\\n") {
		t.Errorf("quoteEnvValue did not escape correctly: %q", got)
	}
}
