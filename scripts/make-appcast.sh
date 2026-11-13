#!/usr/bin/env bash
# Signs the update zip with the Sparkle EdDSA (Ed25519) key and writes dist/appcast.xml.
# Env:
#   SPARKLE_PRIVATE_KEY   PEM-encoded Ed25519 private key (or SPARKLE_PRIVATE_KEY_FILE)
#   VERSION, BUILD        marketing version and monotonically increasing build number
#   REPO                  owner/name on GitHub (defaults to $GITHUB_REPOSITORY)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"
REPO="${REPO:-${GITHUB_REPOSITORY:-dylancetin/cli-tollgate}}"
ZIP="$DIST/CLI-Tollgate-$VERSION.zip"
TAG="v$VERSION"

# Ed25519 one-shot signing needs OpenSSL 3 (macOS ships LibreSSL as `openssl`).
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

SIGNATURE="$("$OPENSSL" pkeyutl -sign -rawin -inkey "$KEY_FILE" -in "$ZIP" | base64 | tr -d '\n')"
LENGTH="$(wc -c < "$ZIP" | tr -d ' ')"
PUBDATE="$(LC_ALL=C date -u '+%a, %d %b %Y %H:%M:%S +0000')"
URL="https://github.com/$REPO/releases/download/$TAG/$(basename "$ZIP")"

cat > "$DIST/appcast.xml" <<XML
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>CLI Tollgate</title>
    <item>
      <title>Version $VERSION</title>
      <pubDate>$PUBDATE</pubDate>
      <sparkle:version>$BUILD</sparkle:version>
      <sparkle:shortVersionString>$VERSION</sparkle:shortVersionString>
      <sparkle:minimumSystemVersion>13.0</sparkle:minimumSystemVersion>
      <sparkle:releaseNotesLink>https://github.com/$REPO/releases/tag/$TAG</sparkle:releaseNotesLink>
      <enclosure url="$URL" length="$LENGTH" type="application/octet-stream" sparkle:edSignature="$SIGNATURE"/>
    </item>
  </channel>
</rss>
XML

echo "wrote $DIST/appcast.xml"
