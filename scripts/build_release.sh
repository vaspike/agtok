#!/usr/bin/env bash
set -euo pipefail

# Build release tarballs for macOS and Linux (amd64/arm64).
# - Inside tar: binary is named exactly 'agtok' (no version suffix)
# - Tar filename contains version: agtok-v<version>-<os>-<arch>.tar.gz

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Resolve version (priority: $VERSION env -> internal/version/version.go -> date)
VERSION="${VERSION:-}"
if [[ -z "$VERSION" && -f internal/version/version.go ]]; then
  # Extract the literal default value from the file (var Version = "x.y.z")
  VERSION=$(sed -nE 's/^var[[:space:]]+Version[[:space:]]*=[[:space:]]*"([^"]+)"/\1/p' internal/version/version.go | head -n1 || true)
fi
if [[ -z "$VERSION" || "$VERSION" == "dev" ]]; then
  VERSION=$(date +%Y.%m.%d)
fi

echo "Building agtok version: $VERSION"

mkdir -p "$ROOT_DIR/dist" "$ROOT_DIR/bin"

TARGETS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
)

# Use a local go cache to avoid permission problems on CI/machines
: "${GOCACHE:=$ROOT_DIR/.gocache}"

for target in "${TARGETS[@]}"; do
  IFS=/ read -r GOOS GOARCH <<< "$target"
  echo "- Building for $GOOS/$GOARCH ..."

  # Build binary (TUI tag by default), inject version via ldflags
  OUT_BIN="$ROOT_DIR/bin/agtok-$GOOS-$GOARCH"
  env CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" GOCACHE="$GOCACHE" \
    go build -tags tui -trimpath \
      -ldflags "-s -w -X tks/internal/version.Version=$VERSION" \
      -o "$OUT_BIN" ./cmd/agtok

  chmod +x "$OUT_BIN"

  # Package: tar must contain a single file named 'agtok'
  STAGE_DIR="$ROOT_DIR/dist/stage-$GOOS-$GOARCH"
  rm -rf "$STAGE_DIR" && mkdir -p "$STAGE_DIR"
  cp "$OUT_BIN" "$STAGE_DIR/agtok"

  TAR_PATH="$ROOT_DIR/dist/agtok-v$VERSION-$GOOS-$GOARCH.tar.gz"
  tar -C "$STAGE_DIR" -czf "$TAR_PATH" agtok
  echo "  -> $TAR_PATH"

  rm -rf "$STAGE_DIR"
done

echo "Done. Artifacts in: $ROOT_DIR/dist"

