#!/usr/bin/env bash
# =============================================================================
# NOVUS Installer — Quick Install Script
# =============================================================================
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/SGC-NOVUS/novus-installer/main/install.sh | sudo bash
#
# Or with GitHub PAT:
#   curl -fsSL ... | sudo NOVUS_INSTALLER_GITHUB_PAT="github_pat_..." bash
# =============================================================================
set -euo pipefail

RED='\033[1;31m'
GREEN='\033[1;32m'
CYAN='\033[1;36m'
NC='\033[0m'

log()  { printf "${CYAN}[~]${NC} %s\n" "$*"; }
ok()   { printf "${GREEN}[+]${NC} %s\n" "$*"; }
err()  { printf "${RED}[!]${NC} %s\n" "$*" >&2; }

# ── Pre-flight ─────────────────────────────────────────────────────────────
if [[ "$(id -u)" -ne 0 ]]; then
  err "This script must be run as root (or with sudo)."
  exit 1
fi

ARCH="amd64"
case "$(uname -m)" in
  aarch64|arm64) ARCH="arm64" ;;
esac

log "NOVUS Installer — bootstrapping..."

# ── Check Go ───────────────────────────────────────────────────────────────
if command -v go >/dev/null 2>&1; then
  ok "Go detected: $(go version)"
else
  log "Installing Go..."
  GO_VERSION="1.24.0"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
  export PATH=$PATH:/usr/local/go/bin
  echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
  ok "Go $(go version) installed."
fi

# ── Build installer ────────────────────────────────────────────────────────
INSTALL_DIR="/tmp/novus-installer-build"
rm -rf "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

log "Cloning novus-installer..."
git clone --depth 1 https://github.com/SGC-NOVUS/installer.git .

log "Building..."
go build -trimpath -ldflags="-s -w" -o novus-installer ./cmd/installer
ok "Build complete: $(du -h novus-installer | cut -f1)"

# ── Run ────────────────────────────────────────────────────────────────────
log "Starting installer..."
log ""
log "  The installer will open a web interface on port 8080."
log "  Open the URL printed below in your browser."
log ""

if [[ -n "${NOVUS_INSTALLER_GITHUB_PAT:-}" ]]; then
  export NOVUS_INSTALLER_GITHUB_PAT
fi

exec ./novus-installer
