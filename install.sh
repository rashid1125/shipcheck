#!/usr/bin/env bash
# LaunchSafe quick-install script
# Usage:  curl -fsSL https://raw.githubusercontent.com/rashid1125/launchsafe/main/install.sh | bash
set -euo pipefail

REPO="rashid1125/launchsafe"
BINARY="launchsafe"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── detect OS and arch ───────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# ── get latest release tag from GitHub API ───────────────────────────────────
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

if [ -z "$TAG" ]; then
  echo "Could not determine latest version. Check https://github.com/${REPO}/releases" >&2
  exit 1
fi
echo "Latest version: $TAG"

# ── build download URL ────────────────────────────────────────────────────────
ASSET="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

# ── download and install ──────────────────────────────────────────────────────
TMP=$(mktemp)
echo "Downloading ${URL} ..."
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"

# Try to install system-wide; fall back to ~/bin
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/${BINARY}"
  echo "Installed to ${INSTALL_DIR}/${BINARY}"
else
  mkdir -p "$HOME/bin"
  mv "$TMP" "$HOME/bin/${BINARY}"
  echo "Installed to $HOME/bin/${BINARY}"
  echo "Make sure $HOME/bin is in your PATH:"
  echo "  export PATH=\"\$HOME/bin:\$PATH\""
fi

echo ""
${INSTALL_DIR}/${BINARY} version 2>/dev/null || "$HOME/bin/${BINARY}" version
