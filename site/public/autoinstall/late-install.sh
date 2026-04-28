#!/usr/bin/env bash
# late-install.sh — convocate host provisioning script.
#
# Fetched and executed by the Ubuntu autoinstall late-commands.
# Runs inside the installed target system (not the live ISO).
#
# What it does:
#   1. Installs Go 1.26.x
#   2. Clones the convocate repository
#   3. Builds all three binaries (convocate, convocate-host, convocate-agent)
#   4. Installs them to /usr/local/bin
#   5. Runs convocate-host install (creates convocate user, enables Docker,
#      configures UFW, sets timezone, enables unattended-upgrades)
#
# After this script completes the host is ready for either:
#   convocate-host init-shell --host <this-host>
#   convocate-host init-agent --host <this-host>
#
# Neither is run automatically — the shell must exist before agents can
# be deployed, and the cryptographic peering requires operator action.
set -euo pipefail

GO_VERSION="1.26.2"
REPO_URL="https://github.com/asymmetric-effort/convocate.git"
BUILD_DIR="/tmp/convocate-build"

log() { echo "[late-install] $*"; }

# ── Install Go ─────────────────────────────────────────────────────────
log "Installing Go ${GO_VERSION}..."
ARCH=$(dpkg --print-architecture)
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" \
  -o /tmp/go.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tar.gz
rm -f /tmp/go.tar.gz
export PATH="/usr/local/go/bin:${PATH}"
log "Go $(go version) installed."

# ── Clone and build ────────────────────────────────────────────────────
log "Cloning convocate..."
rm -rf "${BUILD_DIR}"
git clone --depth=1 "${REPO_URL}" "${BUILD_DIR}"

log "Building convocate binaries..."
cd "${BUILD_DIR}"
make build

# ── Install binaries ───────────────────────────────────────────────────
log "Installing binaries to /usr/local/bin..."
install -m 0755 build/convocate       /usr/local/bin/convocate
install -m 0755 build/convocate-host  /usr/local/bin/convocate-host
install -m 0755 build/convocate-agent /usr/local/bin/convocate-agent

# ── Run convocate-host install ─────────────────────────────────────────
# This is idempotent: creates convocate user (UID 1337), enables Docker,
# configures UFW, sets timezone UTC, enables unattended-upgrades.
# Most of these are already done by the autoinstall packages/late-commands
# but running this ensures the host matches convocate-host expectations
# exactly.
log "Running convocate-host install..."
convocate-host install

# ── Persist Go on PATH ─────────────────────────────────────────────────
if ! grep -q '/usr/local/go/bin' /etc/profile.d/go.sh 2>/dev/null; then
  echo 'export PATH="/usr/local/go/bin:${PATH}"' > /etc/profile.d/go.sh
  chmod 0644 /etc/profile.d/go.sh
fi

# ── Clean up ───────────────────────────────────────────────────────────
rm -rf "${BUILD_DIR}"

log "Done.  Host is ready for:"
log "  convocate-host init-shell --host <this-host>"
log "  convocate-host init-agent --host <this-host>"
