#!/usr/bin/env bash
# Packages a built app into dist/: a .zip (used by Sparkle) and a .dmg (for first install).
# Usage: scripts/package.sh <path/to/CLI Tollgate.app> <version>
set -euo pipefail

APP="$1"
VERSION="$2"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"
NAME="CLI-Tollgate-$VERSION"

rm -rf "$DIST"
mkdir -p "$DIST"

# Xcode already signed everything ad-hoc (including the embedded engine); just verify.
codesign --verify --deep --strict --verbose=2 "$APP"

ditto -c -k --sequesterRsrc --keepParent "$APP" "$DIST/$NAME.zip"

STAGE="$(mktemp -d)"
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
hdiutil create -volname "CLI Tollgate" -srcfolder "$STAGE" -ov -format UDZO "$DIST/$NAME.dmg"
rm -rf "$STAGE"

ls -la "$DIST"
