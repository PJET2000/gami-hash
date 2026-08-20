# GAMI Hashing Tool (`gami-hash`)

A small, trustworthy tool that institutions run locally to produce checksums
for their collections: it walks a folder tree, computes SHA-256 for every
file, and writes **one CSV manifest**. Nothing else.

This is the only software GAMI ever asks an institution to install — so it is
built to require no support calls and to be verifiable by the institution's IT.

## What it does

- You pick a folder and an output location, press start, watch a progress
  bar, and get a done message with the path to the result file.
- Per file it records: `relative_path, filename, size_bytes, sha256, mtime_utc`
  — technical metadata only, no accession numbers, no institution-specific
  logic. The relative path is what GAMI matches against the institution's
  own metadata later.
- Output is UTF-8 (with BOM, so Excel displays umlauts correctly), RFC-4180
  CSV, one line per file, deterministic order.

## What it does not do

- It never writes into the scanned folder (this is enforced, not just policy —
  choosing an output location inside the scanned folder is refused).
- It never modifies, moves, or deletes any file. Files are opened read-only.
- It makes **no network connections whatsoever**. There is no networking code
  in the program: `go list -deps` shows no `net*` packages
  (see [docs/WHAT-THIS-TOOL-DOES.md](docs/WHAT-THIS-TOOL-DOES.md)).
- No installation, no admin rights, no configuration, no telemetry.

## Robustness (built for the archive reality)

- **Resumable**: multi-terabyte runs take hours. Interrupt any time (cancel
  button, Ctrl-C, crash, power loss) — restart the program, pick the same
  folder and output file, and it continues where it stopped. A torn CSV line
  from a hard crash is repaired automatically.
- **Errors never abort the run**: unreadable files, permission problems and
  vanished files are recorded in an error log next to the output file
  (`…_errors.log`) and skipped.
- **Handles**: paths longer than 260 characters, UNC/network drives, umlauts,
  decomposed Unicode (NFD), CJK, emoji, filenames in legacy encodings
  (recorded percent-encoded so the CSV stays valid UTF-8; the raw name goes
  to the error log), and control characters in names.
- Symlinks/junctions are never followed (loop- and escape-safe); they are
  noted in the error log.
- **Parallelism**: defaults to 2 workers — deliberately low, because parallel
  reads make spinning disks *slower* (seeking). On SSDs, override with
  `-workers N` (CLI) or the `GAMI_HASH_WORKERS` environment variable (GUI).

## Usage

**GUI (for archivists):** double-click the binary. A five-step wizard runs:
explanation → pick folder → pick output location (defaults to the desktop)
→ start → progress → done, with an "open folder" button that reveals the
result file. German on German systems, English otherwise. If an external
drive comes back under a different drive letter, the resume dialog explains
it and continues correctly. A step-by-step guide for archivists is in
[docs/ANLEITUNG-FUER-ARCHIVARE.md](docs/ANLEITUNG-FUER-ARCHIVARE.md).

**CLI (for IT staff / scripting):**

```
gami-hash -root FOLDER -output FILE.csv [-workers N] [-fresh] [-quiet]
```

Exit codes: `0` done · `1` fatal error · `2` done but some files unreadable
(see error log) · `130` interrupted (resumable).

## Files it writes

| File | When | Purpose |
|---|---|---|
| `<output>.csv` | always | the manifest (the only deliverable) |
| `<output>_errors.log` | only if problems occurred | one line per skipped/unreadable file |
| `<output>.csv.part.json` | during a run | marks an unfinished run for resume; removed on success; safe to delete |

## Building / verifying

Binaries are reproducible: the same source + the same Go toolchain produce
bit-for-bit identical files on any machine. See [VERIFY.md](VERIFY.md).

```
./build.sh v1.0.0     # builds Windows/macOS/Linux + SHA256SUMS
go test -race ./...   # test suite
```

Until a code-signing certificate is in place, Windows SmartScreen will warn on
first run; institutions verify the published SHA-256 instead (instructions in
the IT one-pager).

## Alternatives for institutions (context)

This tool is option 4 of 4: (1) shipping drives to GAMI is being phased out;
(2) institutions with existing checksums (BagIt, fixity reports) just send
those; (3) institutions with IT staff can use the one-page PowerShell script
in [reference/Get-FileHashes.ps1](reference/Get-FileHashes.ps1), which
produces the identical CSV format and doubles as an independent
cross-check of this tool.
