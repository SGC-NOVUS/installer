package orchestrator

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// systemDependenciesCommand returns a shell one-liner that installs baseline OS
// packages required by every subsequent step (curl, git, unzip, rsync, etc.).
func systemDependenciesCommand() string {
	return strings.Join([]string{
		"dpkg --configure -a 2>/dev/null || true",
		"rm -f /var/lib/dpkg/lock-frontend /var/lib/apt/lists/lock /var/cache/apt/archives/lock 2>/dev/null || true",
		"apt-get update -qq 2>/dev/null || apt-get update -o Acquire::AllowInsecureRepositories=true -o Acquire::AllowDowngradeToInsecureRepositories=true 2>/dev/null || true",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y software-properties-common curl wget git unzip rsync ufw || true",
	}, " && ")
}

// repositoriesCommand configures APT repositories for PHP 8.5, MariaDB, and
// Redis on the detected platform (Ubuntu or Debian).
func repositoriesCommand(platform platformProfile) string {
	addPHPRepo := "LC_ALL=C.UTF-8 add-apt-repository -y ppa:ondrej/php 2>/dev/null || true"
	addMariaDBRepo := "curl -sS https://downloads.mariadb.com/MariaDB/mariadb_repo_setup | bash 2>/dev/null || true"
	addRedisRepo := "rm -f /usr/share/keyrings/redis-archive-keyring.gpg 2>/dev/null; curl -fsSL https://packages.redis.io/gpg | gpg --dearmor --yes -o /usr/share/keyrings/redis-archive-keyring.gpg 2>/dev/null; echo \"deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(. /etc/os-release 2>/dev/null && echo ${VERSION_CODENAME:-noble}) main\" > /etc/apt/sources.list.d/redis.list 2>/dev/null || true"

	if platform.ID == "debian" {
		return strings.Join([]string{
			"apt-get install -y ca-certificates apt-transport-https gnupg2",
			"(curl -fsSL https://packages.sury.org/php/apt.gpg -o /usr/share/keyrings/sury-php.gpg 2>/dev/null || true)",
			"(. /etc/os-release 2>/dev/null; echo \"deb [signed-by=/usr/share/keyrings/sury-php.gpg] https://packages.sury.org/php/ ${VERSION_CODENAME:-bookworm} main\" > /etc/apt/sources.list.d/sury-php.list 2>/dev/null || true)",
			"(" + addMariaDBRepo + " 2>/dev/null || true)",
			"(" + addRedisRepo + " 2>/dev/null || true)",
			"apt-get update -qq 2>/dev/null || apt-get update -o Acquire::AllowInsecureRepositories=true || true",
		}, " && ")
	}

	return strings.Join([]string{
		"rm -f /etc/apt/sources.list.d/ondrej*.list /etc/apt/sources.list.d/ondrej*.sources /etc/apt/sources.list.d/mariadb*.list /etc/apt/sources.list.d/redis.list 2>/dev/null",
		"(" + addPHPRepo + ")",
		"(" + addMariaDBRepo + ")",
		"(" + addRedisRepo + ")",
		"apt-get update -qq 2>/dev/null || apt-get update -o Acquire::AllowInsecureRepositories=true || true",
	}, " && ")
}

// stackInstallCommand returns a shell one-liner that installs the full NOVUS-OS
// stackInstallCommand returns a shell one-liner that installs the full NOVUS-OS
// stack. PHP package names are derived from defaultPHPVersion via phpPkg().
func stackInstallCommand() string {
	phpPackages := strings.Join([]string{
		phpPkg("fpm"), phpPkg("cli"), phpPkg("mysql"), phpPkg("mbstring"),
		phpPkg("xml"), phpPkg("curl"), phpPkg("zip"),
		phpPkg("bcmath"), phpPkg("intl"), phpPkg("gd"),
		phpPkg("redis"), phpPkg("grpc"),
	}, " ")
	phpFpm := phpPkg("fpm")

	return strings.Join([]string{
		"apt-get update",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y " +
			"nginx mariadb-server " + phpPackages + " " +
			"redis-server supervisor fail2ban ufw " +
			"certbot python3-certbot-nginx python3-certbot-dns-cloudflare " +
			"curl wget git unzip rsync jq openssl ca-certificates gnupg openssh-server " +
			"|| DEBIAN_FRONTEND=noninteractive apt-get install -y " +
			"nginx mariadb-server " + phpPackages + " " +
			"redis-server supervisor fail2ban ufw " +
			"certbot python3-certbot-nginx python3-certbot-dns-cloudflare " +
			"curl wget git unzip rsync jq openssl ca-certificates gnupg openssh-server",
		"(" +
			"curl -fsSL https://repos.influxdata.com/influxdb-archive-compat.key | gpg --dearmor -o /usr/share/keyrings/influxdb-archive-keyring.gpg 2>/dev/null && " +
			"echo \"deb [signed-by=/usr/share/keyrings/influxdb-archive-keyring.gpg] https://repos.influxdata.com/debian stable main\" " +
			"> /etc/apt/sources.list.d/influxdb.list 2>/dev/null && " +
			"apt-get update -qq 2>/dev/null && " +
			"DEBIAN_FRONTEND=noninteractive apt-get install -y influxdb2 2>/dev/null" +
			") || true",
		"systemctl enable nginx mariadb 2>/dev/null || true",
		"systemctl enable " + phpFpm + " 2>/dev/null || true",
		"systemctl start nginx mariadb 2>/dev/null || true",
		"mkdir -p /etc/novus/secrets",
		"systemctl start " + phpFpm + " 2>/dev/null || true",
		"mkdir -p /etc/systemd/system/" + phpFpm + ".service.d 2>/dev/null || true",
		"echo -e '[Service]\\nReadWritePaths=-/etc/novus/secrets' > /etc/systemd/system/" + phpFpm + ".service.d/override.conf || true",
		"systemctl daemon-reload 2>/dev/null || true",
		"systemctl reset-failed " + phpFpm + " 2>/dev/null || true",
		"systemctl restart " + phpFpm + " 2>/dev/null || true",
	}, " && ")
}

// mariaDBConfigurationCommand returns a shell one-liner that configures MariaDB:
// sets root password, creates the three system databases (novus_os, novus_id,
// novus_sd), creates panel users, and grants privileges.
func mariaDBConfigurationCommand(req SetupRequest) string {
	safeRoot := escapeSQLLiteral(req.DBRootPassword)
	safePanel := escapeSQLLiteral(req.DBPanelPassword)
	sql := fmt.Sprintf(
		"ALTER USER 'root'@'localhost' IDENTIFIED BY '%s'; CREATE DATABASE IF NOT EXISTS %s; CREATE DATABASE IF NOT EXISTS %s; CREATE DATABASE IF NOT EXISTS %s; CREATE USER IF NOT EXISTS '%s'@'127.0.0.1' IDENTIFIED BY '%s'; CREATE USER IF NOT EXISTS '%s'@'127.0.0.1' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON %s.* TO '%s'@'127.0.0.1'; GRANT ALL PRIVILEGES ON %s.* TO '%s'@'127.0.0.1'; GRANT ALL PRIVILEGES ON %s.* TO '%s'@'127.0.0.1'; FLUSH PRIVILEGES;",
		safeRoot,
		defaultOSDBName, defaultIDDBName, defaultSDDBName,
		defaultSystemDBUser, safePanel,
		defaultIdentityDBUser, safePanel,
		defaultOSDBName, defaultSystemDBUser,
		defaultIDDBName, defaultIdentityDBUser,
		defaultSDDBName, defaultSystemDBUser,
	)

	return fmt.Sprintf(
		`(printf %%s %s > /tmp/novus_db_setup.sql && (mariadb < /tmp/novus_db_setup.sql 2>/dev/null || mysql < /tmp/novus_db_setup.sql 2>/dev/null || mariadb -u root -p%s < /tmp/novus_db_setup.sql 2>/dev/null || mysql -u root -p%s < /tmp/novus_db_setup.sql 2>/dev/null || true) && rm -f /tmp/novus_db_setup.sql)`,
		shellQuote(sql),
		shellQuote(req.DBRootPassword),
		shellQuote(req.DBRootPassword),
	)
}

// agentInstallCommand returns a shell one-liner that downloads and installs the
// novus-agent binary from the given URL.
func agentInstallCommand(agentBinaryURL string) string {
	return strings.Join([]string{
		"curl -fL --retry 3 --connect-timeout 15 " + shellQuote(agentBinaryURL) + " -o " + shellQuote(agentDownloadPath),
		"[ -s " + shellQuote(agentDownloadPath) + " ]",
		"test \"$(stat -c%s " + shellQuote(agentDownloadPath) + ")\" -ge " + strconv.FormatInt(minAgentBinaryBytes, 10),
		"install -m 0755 " + shellQuote(agentDownloadPath) + " /usr/local/bin/novus-agent",
	}, " && ")
}

// servicesSetupCommand returns a shell one-liner that enables Redis,
// Supervisor, InfluxDB, Fail2Ban, configures UFW, deploys sudoers rules
// and supervisor program configs from the panel template directory.
func servicesSetupCommand() string {
	return strings.Join([]string{
		"systemctl enable --now redis-server 2>/dev/null || true",
		"systemctl enable --now supervisor 2>/dev/null || true",
		"systemctl enable --now influxdb 2>/dev/null || true",
		"systemctl enable --now fail2ban 2>/dev/null || true",
		"ufw --force disable 2>/dev/null || true",
		"ufw default deny incoming 2>/dev/null || true",
		"ufw default allow outgoing 2>/dev/null || true",
		"ufw allow 22/tcp 2>/dev/null || true",
		"ufw allow 80/tcp 2>/dev/null || true",
		"ufw allow 443/tcp 2>/dev/null || true",
		"ufw allow 8080/tcp 2>/dev/null || true",
		"ufw allow 9443/tcp 2>/dev/null || true",
		"ufw --force enable 2>/dev/null || true",
		// Sudoers rules (deployed after panel code is in place)
		"if [ -f /var/www/novus/deploy/templates/sudoers.d-sgc-vfs-io ]; then " +
			"cp /var/www/novus/deploy/templates/sudoers.d-sgc-vfs-io /etc/sudoers.d/sgc-vfs-io 2>/dev/null; " +
			"chmod 440 /etc/sudoers.d/sgc-vfs-io 2>/dev/null; " +
			"fi",
		"if [ -f /var/www/novus/deploy/templates/sudoers.d-sgc-console-root ]; then " +
			"cp /var/www/novus/deploy/templates/sudoers.d-sgc-console-root /etc/sudoers.d/sgc-console-root 2>/dev/null; " +
			"chmod 440 /etc/sudoers.d/sgc-console-root 2>/dev/null; " +
			"fi",
		// Supervisor program configs
		"if [ -f /var/www/novus/deploy/templates/novus-influxd.supervisor.conf ]; then " +
			"cp /var/www/novus/deploy/templates/novus-influxd.supervisor.conf /etc/supervisor/conf.d/novus-influxd.conf 2>/dev/null; " +
			"supervisorctl reread 2>/dev/null || true; " +
			"supervisorctl update 2>/dev/null || true; " +
			"fi",
		// Nginx wildcard template for Loader Studio
		"if [ -f /var/www/novus/deploy/templates/loader-wildcard.nginx.conf ]; then " +
			"mkdir -p /www/wwwlogs 2>/dev/null; " +
			"echo 'Loader wildcard template present (will be activated by Loader Studio setup)' > /dev/null; " +
			"fi",
	}, " && ")
}

// cronSetupCommand returns a shell one-liner that installs the panel cron jobs
// from the deployed template, substituting PHP binary and panel directory paths.
func cronSetupCommand() string {
	return strings.Join([]string{
		"if [ -f /var/www/novus/deploy/templates/novus-panel.cron ]; then " +
			"sed -e 's|{{PHP_BIN}}|/usr/bin/" + phpBin() + "|g' -e 's|{{PANEL_DIR}}|/var/www/novus|g' -e 's|^\\* \\* \\* \\* \\* www |* * * * * www |' " +
			"/var/www/novus/deploy/templates/novus-panel.cron > /etc/cron.d/novus-panel 2>/dev/null; " +
			"chmod 644 /etc/cron.d/novus-panel 2>/dev/null; " +
			"systemctl reload cron 2>/dev/null || systemctl reload crond 2>/dev/null || true; " +
			"fi",
	}, " && ")
}

// detectPlatformProfile reads /etc/os-release and returns the detected OS
// platform (ubuntu or debian) with its version codename.
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

// resolveAgentBinaryURL returns the URL for downloading the novus-agent binary.
func resolveAgentBinaryURL() string {
	if url := strings.TrimSpace(os.Getenv(agentBinaryURLEnv)); url != "" {
		return url
	}
	arch := strings.ToLower(runtime.GOARCH)
	if arch == "arm64" || arch == "aarch64" {
		return defaultAgentARM64URL
	}
	return defaultAgentAMD64URL
}

// resolvePanelReleaseURL returns the panel release archive URL.
// It prefers NOVUS_INSTALLER_PANEL_RELEASE_URL, then builds a GitHub API URL
// using PAT from NOVUS_INSTALLER_GITHUB_PAT.
func resolvePanelReleaseURL() string {
	if value := strings.TrimSpace(os.Getenv(panelReleaseURLEnv)); value != "" {
		return value
	}

	pat := strings.TrimSpace(os.Getenv(panelGithubPATEnv))
	if pat != "" {
		ref := strings.TrimSpace(os.Getenv("NOVUS_INSTALLER_PANEL_CORE_REF"))
		if ref == "" {
			ref = "main"
		}
		return fmt.Sprintf("https://api.github.com/repos/%s/zipball/%s", panelCoreRepo, url.PathEscape(ref))
	}

	return ""
}
