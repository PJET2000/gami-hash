# Verifying a release binary (reproducible builds)

Every release binary can be rebuilt bit-for-bit from source. If your build
hash matches the published hash, the binary provably contains exactly this
source code — nothing added, nothing removed.

## Requirements

- The Go toolchain version pinned in [go.mod](go.mod) (the `toolchain` line;
  Go downloads it automatically if your installed Go is newer than 1.21).
- Any OS — cross-compilation is built in; the result is identical regardless
  of the machine or directory you build in.

## Steps

```sh
git clone <repository-url>
cd gami-hash
git checkout <release-tag>          # e.g. v1.0.0
./build.sh v1.0.0                   # or the version you checked out
cat dist/SHA256SUMS-v1.0.0.txt      # compare against the published file
```

On Windows, instead of `build.sh` run the equivalent commands:

```powershell
$env:CGO_ENABLED = "0"; $env:GOFLAGS = "-trimpath -buildvcs=false"
$env:GOOS = "windows"; $env:GOARCH = "amd64"
go build -ldflags "-s -w -buildid= -X github.com/PJET2000/gami-hash/internal/engine.Version=v1.0.0 -H=windowsgui" -o gami-hash-v1.0.0-windows-amd64.exe .
Get-FileHash .\gami-hash-v1.0.0-windows-amd64.exe -Algorithm SHA256
```

## Why this works

- `CGO_ENABLED=0`: pure Go, no C compiler or system libraries involved.
- `-trimpath`: no build-directory paths embedded.
- `-buildid=` and `-buildvcs=false`: no per-build or per-checkout metadata.
- The Go toolchain itself is pinned; Go's builds are deterministic given the
  same toolchain, source, and flags.

The release CI workflow (`.github/workflows/release.yml`) performs a
double-build from two different directories and fails if the hashes differ,
so every published release has already proven its reproducibility once.
