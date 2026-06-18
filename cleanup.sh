#!/usr/bin/env bash
# =============================================================================
# NOVUS-OS — VPS Cleanup Script
# =============================================================================
# Полностью удаляет все следы предыдущих установок NOVUS-OS.
# Запустите на VPS:
#   curl -fsSL https://raw.githubusercontent.com/SGC-NOVUS/installer/main/cleanup.sh | sudo bash
# =============================================================================
set -euo pipefail

RED='\033[1;31m'
GREEN='\033[1;32m'
CYAN='\033[1;36m'
NC='\033[0m'

log()  { printf "${CYAN}[cleanup]${NC} %s\n" "$*"; }
ok()   { printf "${GREEN}[+]${NC} %s\n" "$*"; }
skip() { printf "    (skipped — not found)\n"; }

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root." >&2
  exit 1
fi

log "Stopping services..."
systemctl stop nginx 2>/dev/null || true
systemctl stop mariadb 2>/dev/null || true
systemctl stop novus-agent 2>/dev/null || true
systemctl stop php8.5-fpm 2>/dev/null || true
systemctl stop php-fpm 2>/dev/null || true
pkill -f novus-installer 2>/dev/null || true
ok "Services stopped"

log "Removing NOVUS-OS directories..."
rm -rf /var/www/novus 2>/dev/null && ok "/var/www/novus" || skip
rm -rf /etc/novus 2>/dev/null && ok "/etc/novus" || skip
rm -rf /opt/novus 2>/dev/null && ok "/opt/novus" || skip
rm -rf /opt/sgc-agent 2>/dev/null && ok "/opt/sgc-agent" || skip

log "Removing binaries..."
rm -f /usr/local/bin/novus-installer 2>/dev/null && ok "novus-installer" || skip
rm -f /usr/local/bin/novus-agent 2>/dev/null && ok "novus-agent" || skip

log "Removing systemd units..."
rm -f /etc/systemd/system/novus-agent.service 2>/dev/null && ok "novus-agent.service" || skip
rm -f /etc/systemd/system/sgc-agent.service 2>/dev/null && ok "sgc-agent.service" || skip
systemctl daemon-reload 2>/dev/null || true

log "Removing nginx configs..."
rm -f /etc/nginx/sites-enabled/novus-installer.conf 2>/dev/null && ok "nginx: installer" || skip
rm -f /etc/nginx/sites-available/novus-installer.conf 2>/dev/null && ok "nginx: available" || skip
rm -f /www/server/panel/vhost/nginx/loader-wildcard.*.conf 2>/dev/null && ok "nginx: loader" || skip

log "Removing cron..."
rm -f /etc/cron.d/novus-panel 2>/dev/null && ok "cron.d/novus-panel" || skip
systemctl reload cron 2>/dev/null || systemctl reload crond 2>/dev/null || true

log "Removing APT repos (broken PPAs)..."
rm -f /etc/apt/sources.list.d/ondrej*.list 2>/dev/null && ok "PPA: ondrej" || skip
rm -f /etc/apt/sources.list.d/sury-php.list 2>/dev/null && ok "PPA: sury-php" || skip
rm -f /etc/apt/sources.list.d/mariadb*.list 2>/dev/null && ok "PPA: mariadb" || skip

log "Cleaning temp files..."
rm -rf /tmp/novus-installer-build 2>/dev/null && ok "/tmp/novus-installer-build" || skip
rm -f /tmp/panel.zip /tmp/novus-agent 2>/dev/null && ok "/tmp/panel.zip + agent" || skip

log "Purging packages (optional)..."
if command -v apt-get >/dev/null 2>&1; then
  apt-get autoremove --purge -y nginx mariadb-server php8.5-fpm php-fpm 2>/dev/null || true
  ok "Packages purged"
fi

log ""
log "=============================================="
log " CLEANUP COMPLETE"
log "=============================================="
log ""
log " VPS is ready for fresh NOVUS-OS installation."
log " Run: curl -fsSL https://raw.githubusercontent.com/SGC-NOVUS/installer/main/install.sh | sudo bash"
log "=============================================="
