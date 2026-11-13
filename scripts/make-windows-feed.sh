#!/usr/bin/env bash
# Signs the Windows installer with the update Ed25519 key (the same key Sparkle
# uses for macOS) and writes dist/latest-windows.json for the app's updater.
# Env:
#   SPARKLE_PRIVATE_KEY   PEM-encoded Ed25519 private key (or SPARKLE_PRIVATE_KEY_FILE)
#   VERSION, BUILD        marketing version and monotonically increasing build number
#   REPO                  owner/name on GitHub (defaults to $GITHUB_REPOSITORY)
#   DIST                  folder holding CLI-Tollgate-Setup-$VERSION.exe (default dist/)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="${DIST:-$ROOT/dist}"
REPO="${REPO:-${GITHUB_REPOSITORY:-dylancetin/cli-tollgate}}"
SETUP="$DIST/CLI-Tollgate-Setup-$VERSION.exe"
TAG="v$VERSION"
test -f "$SETUP" || { echo "missing $SETUP" >&2; exit 1; }

OPENSSL=openssl
if ! "$OPENSSL" version 2>/dev/null | grep -q '^OpenSSL 3'; then
  OPENSSL="$(brew --prefix openssl@3)/bin/openssl"
fi

KEY_FILE="${SPARKLE_PRIVATE_KEY_FILE:-}"
if [ -z "$KEY_FILE" ]; then
  KEY_FILE="$(mktemp)"
  trap 'rm -f "$KEY_FILE"' EXIT
  printf '%s\n' "$SPARKLE_PRIVATE_KEY" > "$KEY_FILE"
fi

SIGNATURE="$("$OPENSSL" pkeyutl -sign -rawin -inkey "$KEY_FILE" -in "$SETUP" | base64 | tr -d '\n')"
LENGTH="$(wc -c < "$SETUP" | tr -d ' ')"
if command -v sha256sum >/dev/null 2>&1; then
  SHA256="$(sha256sum "$SETUP" | cut -d' ' -f1)"
else
  SHA256="$(shasum -a 256 "$SETUP" | cut -d' ' -f1)"
fi

cat > "$DIST/latest-windows.json" <<JSON
{
  "version": "$VERSION",
  "build": $BUILD,
  "url": "https://github.com/$REPO/releases/download/$TAG/$(basename "$SETUP")",
  "length": $LENGTH,
  "sha256": "$SHA256",
  "signature": "$SIGNATURE",
  "notesUrl": "https://github.com/$REPO/releases/tag/$TAG"
}
JSON
echo "wrote $DIST/latest-windows.json"
