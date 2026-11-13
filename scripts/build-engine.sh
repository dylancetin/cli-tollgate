#!/usr/bin/env bash
# Builds the bundled proxy engine into build/engine/.
#
# Usage: scripts/build-engine.sh [darwin|windows|linux]   (default: darwin)
#   darwin   build/engine/tollgate-engine (universal when lipo is available)
#   windows  build/engine/windows/tollgate-engine.exe   (ENGINE_ARCH, default amd64)
#   linux    build/engine/linux/tollgate-engine         (for the Windows app's dev mode)
#
# Source: the CLIProxyAPI fork, taken from (first match wins)
#   1. $ENGINE_SRC
#   2. ./engine  (git submodule)
#   3. ~/workspace/CLIProxyAPI
# Only the fork's committed HEAD is built (exported to a temporary copy), so
# uncommitted changes in the fork's working tree never end up in the app.
#
set -euo pipefail

TARGET="${1:-darwin}"
case "$TARGET" in darwin|windows|linux) ;; *) echo "usage: $0 [darwin|windows|linux]" >&2; exit 2 ;; esac

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/build/engine"

SRC="${ENGINE_SRC:-}"
if [ -z "$SRC" ]; then
  if git -C "$ROOT/engine" rev-parse --git-dir >/dev/null 2>&1; then
    SRC="$ROOT/engine"
  else
    SRC="$HOME/workspace/CLIProxyAPI"
  fi
fi
git -C "$SRC" rev-parse --git-dir >/dev/null 2>&1 || { echo "error: no CLIProxyAPI checkout at $SRC" >&2; exit 1; }
if [ -n "$(git -C "$SRC" status --porcelain)" ]; then
  echo "note: $SRC has uncommitted changes; building its committed HEAD only" >&2
fi

COMMIT="$(git -C "$SRC" rev-parse HEAD)"
VERSION="${ENGINE_VERSION:-tollgate-${COMMIT:0:12}}"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w -X 'main.Version=$VERSION' -X 'main.Commit=$COMMIT' -X 'main.BuildDate=$DATE'"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
git -C "$SRC" archive --format=tar HEAD | tar -x -C "$WORK"

if [ "$TARGET" != darwin ]; then
  ARCH="${ENGINE_ARCH:-amd64}"
  DEST="$OUT/$TARGET"
  EXT=""; [ "$TARGET" = windows ] && EXT=".exe"
  mkdir -p "$DEST"
  echo "building engine $COMMIT for $TARGET/$ARCH"
  (cd "$WORK" && CGO_ENABLED=0 GOOS=$TARGET GOARCH=$ARCH \
    go build -trimpath -buildvcs=false -ldflags="$LDFLAGS" -o "$DEST/tollgate-engine$EXT" ./cmd/server)
  echo "$COMMIT" > "$DEST/engine-commit.txt"
  echo "engine: $DEST/tollgate-engine$EXT"
  exit 0
fi

mkdir -p "$OUT"
for arch in arm64 amd64; do
  echo "building engine $COMMIT for darwin/$arch"
  (cd "$WORK" && CGO_ENABLED=0 GOOS=darwin GOARCH=$arch \
    go build -trimpath -buildvcs=false -ldflags="$LDFLAGS" -o "$OUT/tollgate-engine-$arch" ./cmd/server)
done
echo "$COMMIT" > "$OUT/engine-commit.txt"

if command -v lipo >/dev/null 2>&1; then
  lipo -create -output "$OUT/tollgate-engine" "$OUT/tollgate-engine-arm64" "$OUT/tollgate-engine-amd64"
  rm "$OUT/tollgate-engine-arm64" "$OUT/tollgate-engine-amd64"
  # lipo invalidates the linker's ad-hoc signature; re-sign so it runs on Apple silicon.
  codesign --force --sign - "$OUT/tollgate-engine"
  echo "universal engine: $OUT/tollgate-engine"
else
  echo "lipo not found; left per-arch binaries in $OUT"
fi
