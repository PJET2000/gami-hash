#!/bin/sh
# Reproducible release build for all supported platforms.
#
#   ./build.sh v1.0.0
#
# Anyone with the same Go toolchain (see go.mod / VERIFY.md) can run this and
# get bit-for-bit identical binaries — that is the point: institutions can
# verify that a published binary really came from this source.
set -eu

VERSION="${1:?usage: ./build.sh vX.Y.Z}"
LDFLAGS_COMMON="-s -w -buildid= -X github.com/PJET2000/gami-hash/internal/engine.Version=$VERSION"

export CGO_ENABLED=0   # pure Go: static binaries, no C toolchain influence
export GOFLAGS="-trimpath -buildvcs=false"

rm -rf dist
mkdir -p dist

build() { # GOOS GOARCH output [extra ldflags]
  echo "  $1/$2 -> dist/$3"
  GOOS="$1" GOARCH="$2" go build -ldflags "$LDFLAGS_COMMON ${4:-}" -o "dist/$3" .
}

# Download names are for people, not for build systems: the Windows file the
# archivist sees is "GAMI-Hashing-Tool-1.2.0.exe", not a platform triple.
VERNUM="${VERSION#v}"

# Archives carry the commit date as file timestamp: deterministic for anyone
# who checks out the same tag (reproducibility), and a date that makes sense
# to the person extracting the file (not some fixed epoch).
STAMP="$(git log -1 --format=%ci 2>/dev/null || echo "2026-01-01 00:00:00 +0000")"

echo "building gami-hash $VERSION"
# -H=windowsgui: double-click opens no console window (CLI still works from
# cmd/PowerShell; output is re-attached to the parent console).
build windows amd64 "GAMI-Hashing-Tool-$VERNUM.exe" "-H=windowsgui"
build windows arm64 "GAMI-Hashing-Tool-$VERNUM-windows-arm64.exe" "-H=windowsgui"

# Linux and macOS downloads ship as archives: browsers strip the executable
# permission from bare binaries, archives preserve it. Inside is one cleanly
# named program file without version clutter.
pack_linux() { # GOARCH archive-name
  build linux "$1" "gami-hash"
  tar --sort=name --owner=0 --group=0 --numeric-owner \
      --mtime="$STAMP" -C dist -cf - "gami-hash" | gzip -n > "dist/$2"
  rm dist/gami-hash
}
pack_macos() { # GOARCH archive-name
  build darwin "$1" "GAMI-Hashing-Tool"
  (cd dist && touch -d "$STAMP" "GAMI-Hashing-Tool" && zip -X -q "$2" "GAMI-Hashing-Tool")
  rm dist/GAMI-Hashing-Tool
}
pack_linux amd64 "GAMI-Hashing-Tool-$VERNUM-linux.tar.gz"
pack_linux arm64 "GAMI-Hashing-Tool-$VERNUM-linux-arm64.tar.gz"
pack_macos arm64 "GAMI-Hashing-Tool-$VERNUM-macos.zip"
pack_macos amd64 "GAMI-Hashing-Tool-$VERNUM-macos-intel.zip"

(cd dist && sha256sum -- * > "SHA256SUMS-$VERSION.txt")
echo
echo "published hashes (give these to institutions):"
cat "dist/SHA256SUMS-$VERSION.txt"
