#!/usr/bin/env bash
# =============================================================================
# NOVUS Installer — Quick Install Script (PRIVATE REPO)
# =============================================================================
# Since SGC-NOVUS/installer is a PRIVATE repository, you must provide
# a GitHub PAT with repo-read scope. Two methods:
#
# Method A (env var):
#   export NOVUS_INSTALLER_GITHUB_PAT="github_pat_..."
#   curl -fsSL -H "Authorization: token ${NOVUS_INSTALLER_GITHUB_PAT}" \
#     https://raw.githubusercontent.com/SGC-NOVUS/installer/main/install.sh | sudo -E bash
#
# Method B (download + run):
#   curl -fsSL -H "Authorization: token GITHUB_PAT" \
#     https://raw.githubusercontent.com/SGC-NOVUS/installer/main/install.sh -o install.sh
#   sudo bash install.sh  (will prompt for PAT)
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

# NOVUS_INSTALLER_GITHUB_PAT can be set via env or piped via curl header.
# If not set, prompt interactively.
GITHUB_PAT="${NOVUS_INSTALLER_GITHUB_PAT:-}"
if [[ -z "$GITHUB_PAT" ]]; then
  read -rsp "Enter GitHub PAT for SGC-NOVUS/installer (private repo): " GITHUB_PAT
  echo ""
fi
if [[ -z "$GITHUB_PAT" ]]; then
  err "GitHub PAT is required to access private repository SGC-NOVUS/installer."
  exit 1
fi
export NOVUS_INSTALLER_GITHUB_PAT="$GITHUB_PAT"

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

log "Cloning installer from private repo..."
git clone --depth 1 "https://oauth2:${GITHUB_PAT}@github.com/SGC-NOVUS/installer.git" .

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
