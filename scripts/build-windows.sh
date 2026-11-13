#!/usr/bin/env bash
# Builds the Windows app into build/windows/: "CLI Tollgate.exe", the engine
# and engine-commit.txt. Runs on Linux, macOS or Git Bash on Windows (no CGO).
#
# Env:
#   VERSION  marketing version (default 0.0.0-dev)
#   BUILD    monotonically increasing build number; 0 disables auto-update
#   ARCH     amd64 (default) or arm64
# The engine is (re)built with scripts/build-engine.sh windows unless
# build/engine/windows/tollgate-engine.exe already exists and SKIP_ENGINE=1.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-0.0.0-dev}"
BUILD="${BUILD:-0}"
ARCH="${ARCH:-amd64}"
OUT="$ROOT/build/windows"
ENGINE_DIR="$ROOT/build/engine/windows"

if [ "${SKIP_ENGINE:-0}" != 1 ] || [ ! -f "$ENGINE_DIR/tollgate-engine.exe" ]; then
  ENGINE_ARCH="$ARCH" "$ROOT/scripts/build-engine.sh" windows
fi

# Windows numeric versions: major.minor.patch.build
NUMERIC="$(echo "$VERSION" | sed -E 's/^([0-9]+)\.([0-9]+)\.([0-9]+).*/\1.\2.\3/')"
[[ "$NUMERIC" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || NUMERIC="0.0.0"
WINVER="$NUMERIC.$BUILD"

cd "$ROOT/windows"
rm -f rsrc_windows_*.syso
go run github.com/tc-hib/go-winres@v0.3.3 make \
  --in winres/winres.json --out rsrc --arch "$ARCH" \
  --product-version "$WINVER" --file-version "$WINVER"

rm -rf "$OUT"
mkdir -p "$OUT"
CGO_ENABLED=0 GOOS=windows GOARCH="$ARCH" go build -trimpath -buildvcs=false \
  -ldflags "-s -w -H windowsgui -X main.version=$VERSION -X main.build=$BUILD" \
  -o "$OUT/CLI Tollgate.exe" .
rm -f rsrc_windows_*.syso

cp "$ENGINE_DIR/tollgate-engine.exe" "$OUT/"
cp "$ENGINE_DIR/engine-commit.txt" "$OUT/"
cp "$ROOT/engine/LICENSE" "$OUT/CLIProxyAPI-LICENSE.txt" 2>/dev/null || true
ls -la "$OUT"
