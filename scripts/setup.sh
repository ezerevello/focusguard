#!/usr/bin/env bash
# Plug-and-play setup of FocusGuard for Linux.
# - Compiles the binary (requires Go).
# - Installs it into /usr/local/bin.
# - Registers it as a systemd service that runs at boot (as root, because
#   editing /etc/hosts requires superuser privileges).
#
# Usage:
#   ./scripts/setup.sh              installs and starts the service
#   ./scripts/setup.sh --uninstall  uninstalls everything and cleans up /etc/hosts

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="/usr/local/bin/focusguard"
SERVICE_PATH="/etc/systemd/system/focusguard.service"
PORT=7878

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
info() { printf '  -> %s\n' "$1"; }
err()  { printf '  !! %s\n' "$1" >&2; }

uninstall() {
  bold "Uninstalling FocusGuard..."
  if systemctl is-active --quiet focusguard 2>/dev/null; then
    sudo systemctl stop focusguard
  fi
  sudo systemctl disable focusguard 2>/dev/null || true
  sudo rm -f "$SERVICE_PATH"
  sudo systemctl daemon-reload
  sudo rm -f "$BIN_PATH"

  # Cleans up any FocusGuard block left in /etc/hosts.
  if grep -q "FocusGuard START" /etc/hosts 2>/dev/null; then
    info "Cleaning up /etc/hosts..."
    sudo sed -i '/# === FocusGuard START/,/# === FocusGuard END ===/d' /etc/hosts
  fi

  info "Done. Data in ~/.focusguard was left untouched in case you want to reinstall."
  exit 0
}

if [[ "${1:-}" == "--uninstall" ]]; then
  uninstall
fi

bold "FocusGuard — setup for Linux"

if [[ "$(uname -s)" != "Linux" ]]; then
  err "This script is for Linux. For Windows use scripts/setup.ps1"
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  err "This system does not use systemd. You will have to run the binary manually"
  err "(see README.md, 'Without systemd' section)."
fi

# 1. Go
if ! command -v go >/dev/null 2>&1; then
  info "Go is not installed. Attempting to install it with apt..."
  if command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -qq && sudo apt-get install -y golang-go
  else
    err "Could not install Go automatically. Please install it manually from https://go.dev/dl/"
    err "and run this script again."
    exit 1
  fi
fi
info "Go found: $(go version)"

# 2. Build
bold "Compiling..."
cd "$REPO_ROOT"
go build -o /tmp/focusguard-build ./cmd/focusguard
info "Build OK"

# 3. Install binary
sudo install -m 0755 /tmp/focusguard-build "$BIN_PATH"
info "Binary installed into $BIN_PATH"

# 4. Systemd service
sudo cp "$REPO_ROOT/systemd/focusguard.service" "$SERVICE_PATH"
sudo systemctl daemon-reload
sudo systemctl enable --now focusguard
info "Systemd service enabled and started"

sleep 1
if systemctl is-active --quiet focusguard; then
  bold "Done. FocusGuard is running."
else
  err "The service failed to start. Check: sudo journalctl -u focusguard -n 50"
  exit 1
fi

info "Opening http://localhost:$PORT ..."
xdg-open "http://localhost:$PORT" >/dev/null 2>&1 || true

cat <<EOF

$(bold "Web UI:")      http://localhost:$PORT
$(bold "Logs:")        sudo journalctl -u focusguard -f
$(bold "Stop:")        sudo systemctl stop focusguard
$(bold "Uninstall:")   ./scripts/setup.sh --uninstall

EOF
