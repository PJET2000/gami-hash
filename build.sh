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

echo "building gami-hash $VERSION"
# -H=windowsgui: double-click opens no console window (CLI still works from
# cmd/PowerShell; output is re-attached to the parent console).
build windows amd64 "gami-hash-$VERSION-windows-amd64.exe" "-H=windowsgui"
build windows arm64 "gami-hash-$VERSION-windows-arm64.exe" "-H=windowsgui"
build linux   amd64 "gami-hash-$VERSION-linux-amd64"
build linux   arm64 "gami-hash-$VERSION-linux-arm64"
build darwin  amd64 "gami-hash-$VERSION-macos-intel"
build darwin  arm64 "gami-hash-$VERSION-macos-applesilicon"

# Linux and macOS downloads ship as archives: browsers strip the executable
# permission from bare binaries, archives preserve it. Fixed timestamps and
# ordering keep the archives reproducible too. Windows .exe works as-is.
STAMP="2026-01-01 00:00:00 UTC"
package() { # file
  case "$1" in
    *linux*)
      tar --sort=name --owner=0 --group=0 --numeric-owner \
          --mtime="$STAMP" -C dist -cf - "$1" | gzip -n > "dist/$1.tar.gz"
      rm "dist/$1"
      ;;
    *macos*)
      (cd dist && touch -d "$STAMP" "$1" && zip -X -q "$1.zip" "$1")
      rm "dist/$1"
      ;;
  esac
}
for f in dist/gami-hash-*; do package "$(basename "$f")"; done

(cd dist && sha256sum -- * > "SHA256SUMS-$VERSION.txt")
echo
echo "published hashes (give these to institutions):"
cat "dist/SHA256SUMS-$VERSION.txt"
