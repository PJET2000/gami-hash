# GAMI Hashing Tool — Information for IT staff / Informationen für die IT

*One page. English first, Deutsch unten.*

## What it is

A single portable executable (~3 MB) that computes SHA-256 checksums for
every file under a folder chosen by the user and writes them to one CSV file.
It is used to prepare collection handovers to GAMI without shipping the files
themselves.

## What it does — completely

1. **Reads** every regular file under the user-chosen folder (opened
   read-only).
2. **Writes** exactly one CSV file to the user-chosen output location —
   which the program refuses to place inside the scanned folder — plus, if
   needed, an error log and a temporary resume marker next to it.
3. Shows a progress bar and, at the end, an "Open folder" button that
   reveals the result file in the system file manager. That is all.

## What it does not do

- **No network activity.** The program contains no networking code at all —
  not merely "doesn't phone home": the network stack is not linked into the
  binary. Verify: watch it with any firewall/Process Monitor, or build from
  source and run `go list -deps .` (no `net*` packages appear).
- **No writes into the scanned folder.** Enforced in code; additionally every
  file is opened with read-only flags. You can run it against a read-only
  share or a write-protected drive.
- **No installation, no admin rights, no services, no registry changes, no
  drivers.** Delete the file and it is gone.
- **No file contents leave the machine.** The output CSV contains only:
  relative path, filename, size, SHA-256, modification time.

## Verifying the binary

Until code signing is in place, Windows SmartScreen may warn on first run
("unknown publisher"). Verify the download instead:

```powershell
Get-FileHash .\gami-hash-v1.0.0-windows-amd64.exe -Algorithm SHA256
```

Compare the result with the hash published by GAMI (delivered separately —
by mail from your GAMI contact and on the GAMI release page). The build is
reproducible: anyone can build the same binary from the public source and
obtain the identical hash (see `VERIFY.md` in the repository).

## Resource use

Reads files sequentially with 2 parallel workers by default (safe for
spinning disks and network shares). CPU: one to two cores while hashing.
Memory: typically well under 200 MB. Runtime: roughly the time needed to read
the data once (a 4 TB collection on a USB disk ≈ one working day; the run can
be interrupted and resumed at any time).

---

# Deutsch

## Was es ist

Eine einzelne portable Programmdatei (~3 MB), die für jede Datei unterhalb
eines vom Benutzer gewählten Ordners SHA-256-Prüfsummen berechnet und in eine
CSV-Datei schreibt. Sie dient der Vorbereitung von Bestandsübergaben an GAMI,
ohne die Dateien selbst zu versenden.

## Was es tut — vollständig

1. **Liest** jede reguläre Datei unterhalb des gewählten Ordners (nur lesend
   geöffnet).
2. **Schreibt** genau eine CSV-Datei an den gewählten Speicherort — den das
   Programm nicht innerhalb des erfassten Ordners zulässt — sowie bei Bedarf
   ein Fehlerprotokoll und eine temporäre Fortsetzungs-Markierung daneben.
3. Zeigt einen Fortschrittsbalken. Auf Wunsch („Ordner öffnen" im
   Abschlussdialog) öffnet es den Dateimanager am Speicherort der
   Ergebnisdatei. Das ist alles.

## Was es nicht tut

- **Keine Netzwerkaktivität.** Das Programm enthält überhaupt keinen
  Netzwerkcode — nicht nur „telefoniert nicht nach Hause": der Netzwerk-Stack
  ist in die Programmdatei gar nicht eingebunden. Überprüfbar per Firewall/
  Process Monitor oder durch Übersetzen aus dem Quellcode (`go list -deps .`
  zeigt keine `net*`-Pakete).
- **Keine Schreibzugriffe in den erfassten Ordner.** Im Code erzwungen;
  zusätzlich wird jede Datei nur lesend geöffnet. Das Programm kann gegen
  schreibgeschützte Freigaben oder Laufwerke laufen.
- **Keine Installation, keine Admin-Rechte, keine Dienste, keine
  Registry-Einträge, keine Treiber.** Datei löschen — Programm weg.
- **Keine Dateiinhalte verlassen den Rechner.** Die CSV enthält nur:
  relativen Pfad, Dateinamen, Größe, SHA-256, Änderungsdatum.

## Überprüfung der Programmdatei

Bis zur Code-Signierung kann Windows SmartScreen beim ersten Start warnen
(„unbekannter Herausgeber"). Prüfen Sie stattdessen den Download:

```powershell
Get-FileHash .\gami-hash-v1.0.0-windows-amd64.exe -Algorithm SHA256
```

Vergleichen Sie das Ergebnis mit der von GAMI veröffentlichten Prüfsumme
(separat zugestellt — per E-Mail von Ihrem GAMI-Kontakt und auf der
GAMI-Release-Seite). Der Build ist reproduzierbar: Jeder kann aus dem
öffentlichen Quellcode dieselbe Programmdatei mit identischer Prüfsumme
erzeugen (siehe `VERIFY.md` im Repository).

## Ressourcenverbrauch

Liest Dateien sequenziell mit standardmäßig 2 parallelen Workern (schonend
für Magnetplatten und Netzlaufwerke). CPU: ein bis zwei Kerne während des
Hashens. Arbeitsspeicher: typischerweise deutlich unter 200 MB. Laufzeit:
etwa die Zeit, die einmaliges Lesen der Daten benötigt (4 TB an USB-Platte ≈
ein Arbeitstag; der Lauf kann jederzeit unterbrochen und fortgesetzt werden).
