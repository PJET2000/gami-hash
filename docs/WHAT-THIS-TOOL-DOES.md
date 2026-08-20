# GAMI Hashing Tool: information for IT staff

*One page.*

## What it is

A single portable executable (about 3 MB) that computes SHA-256 checksums
for every file under a folder chosen by the user and writes them to one CSV
file. It is used to prepare collection handovers to GAMI without shipping
the files themselves.

## What it does, completely

1. Reads every regular file under the user-chosen folder (opened read-only).
2. Writes exactly one CSV file to the user-chosen output location, which the
   program refuses to place inside the scanned folder. Next to it, when
   needed: an error log and a temporary resume marker.
3. Remembers an interrupted run in one small JSON file in the user's config
   folder (two paths, nothing else), so the next start can offer to continue
   with one click. Removed when the run completes.
4. Shows a progress bar, and at the end an "Open folder" button that reveals
   the result file in the system file manager. That is all.

## What it does not do

- No network activity. The program contains no networking code at all. This
  is stronger than "doesn't phone home": the network stack is not linked
  into the binary. Verify it with any firewall or Process Monitor, or build
  from source and run `go list -deps .` (no `net*` packages appear).
- No writes into the scanned folder. Enforced in code, and every file is
  opened with read-only flags. You can run it against a read-only share or a
  write-protected drive.
- No installation, no admin rights, no services, no registry changes, no
  drivers. Delete the file and it is gone.
- No file contents leave the machine. The output CSV contains only: relative
  path, file name, size, SHA-256, modification time.

## Verifying the binary

Until code signing is in place, Windows SmartScreen may warn on first run
("unknown publisher"). Verify the download instead:

```powershell
Get-FileHash .\GAMI-Hashing-Tool-1.2.0.exe -Algorithm SHA256
```

Compare the result with the checksum published by GAMI (delivered
separately: by mail from your GAMI contact and on the release page). The
build is reproducible: anyone can build the same binary from the public
source and get the identical hash (see `VERIFY.md` in the repository).

## Resource use

Reads files sequentially with 2 parallel workers by default (safe for
spinning disks and network shares). CPU: one to two cores while hashing.
Memory: typically well under 200 MB. Runtime: roughly the time needed to
read the data once. A 4 TB collection on a USB disk takes about a working
day, and the run can be interrupted and resumed at any time.
