#!/usr/bin/env bash
# ShipCheck quick-install script
# Usage:  curl -fsSL https://raw.githubusercontent.com/rashid1125/shipcheck/main/install.sh | bash
set -euo pipefail

REPO="rashid1125/shipcheck"
BINARY="shipcheck"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Temp files cleaned up on any exit.
TMP_BIN=""
TMP_SUMS=""
cleanup() {
  [ -n "$TMP_BIN" ]  && rm -f "$TMP_BIN"
  [ -n "$TMP_SUMS" ] && rm -f "$TMP_SUMS"
}
trap cleanup EXIT

# ── detect OS and arch ────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# ── resolve latest tag ────────────────────────────────────────────────────────
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' | head -1 \
      | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

if [ -z "$TAG" ]; then
  echo "Could not determine latest version. Check https://github.com/${REPO}/releases" >&2
  exit 1
fi
echo "Latest version: $TAG"

# ── download binary and checksums ─────────────────────────────────────────────
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
ASSET="${BINARY}-${OS}-${ARCH}"

TMP_BIN=$(mktemp)
TMP_SUMS=$(mktemp)

echo "Downloading ${ASSET}..."
curl -fsSL "${BASE_URL}/${ASSET}" -o "$TMP_BIN"
echo "Downloading checksums.txt..."
curl -fsSL "${BASE_URL}/checksums.txt" -o "$TMP_SUMS"

# ── verify checksum ───────────────────────────────────────────────────────────
echo "Verifying checksum..."
# Build a single-line checksums file for just this asset.
EXPECTED=$(grep "${ASSET}$" "$TMP_SUMS" || true)
if [ -z "$EXPECTED" ]; then
  echo "ERROR: ${ASSET} not found in checksums.txt — aborting." >&2
  exit 1
fi

# sha256sum on Linux; shasum on macOS.
if command -v sha256sum >/dev/null 2>&1; then
  echo "${EXPECTED%  *}  $TMP_BIN" | sha256sum -c - >/dev/null 2>&1 \
    || { echo "ERROR: checksum mismatch — binary may be corrupted or tampered." >&2; exit 1; }
elif command -v shasum >/dev/null 2>&1; then
  echo "${EXPECTED%  *}  $TMP_BIN" | shasum -a 256 -c - >/dev/null 2>&1 \
    || { echo "ERROR: checksum mismatch — binary may be corrupted or tampered." >&2; exit 1; }
else
  echo "WARNING: no sha256sum/shasum found; skipping checksum verification." >&2
fi
echo "Checksum OK."

# ── install ───────────────────────────────────────────────────────────────────
chmod +x "$TMP_BIN"
DEST="${INSTALL_DIR}/${BINARY}"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_BIN" "$DEST"
  TMP_BIN=""  # already moved; don't remove in cleanup
  echo "Installed to ${DEST}"
else
  mkdir -p "$HOME/bin"
  DEST="$HOME/bin/${BINARY}"
  mv "$TMP_BIN" "$DEST"
  TMP_BIN=""
  echo "Installed to ${DEST}"
  echo "Ensure \$HOME/bin is in your PATH:"
  echo "  export PATH=\"\$HOME/bin:\$PATH\""
fi

echo ""
"$DEST" version
