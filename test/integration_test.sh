#!/bin/bash
# Integration test for gami-hash against a synthetic "archive reality" tree.
set -u
SCRATCH="$(cd "$(dirname "$0")" && pwd)"
WORKDIR="${TMPDIR:-/tmp}/gami-hash-tests"; mkdir -p "$WORKDIR"
BIN="${GAMI_HASH_BIN:?set GAMI_HASH_BIN to the linux binary to test}"
WORK="$WORKDIR/itest"
rm -rf "$WORK"; mkdir -p "$WORK/tree" "$WORK/out"
TREE="$WORK/tree"; OUT="$WORK/out"
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo "  ok: $1"; }
fail() { FAIL=$((FAIL+1)); echo "FAIL: $1"; }

echo "== building test tree =="
# 1. Many small files with real content
for d in Bestand_A Bestand_B "Bestand C mit Leerzeichen" "Zugänge_2024"; do
  for i in $(seq 1 400); do
    mkdir -p "$TREE/$d/Karton_$((i % 20))"
    printf 'inhalt-%s-%04d\n' "$d" "$i" > "$TREE/$d/Karton_$((i % 20))/dok_$i.txt"
  done
done
# 2. Difficult names
mkdir -p "$TREE/Sonderfälle"
printf umlaut > "$TREE/Sonderfälle/Bericht_Über_Grüße_ÄÖÜß.pdf"
printf comma  > "$TREE/Sonderfälle/Liste, endgültig \"final\".csv"
printf emoji  > "$TREE/Sonderfälle/📁 Fotos 2024.dat"
printf cjk    > "$TREE/Sonderfälle/日本語ファイル.txt"
printf nl     > "$TREE/Sonderfälle/böse
zeile.txt"
printf latin1 > "$TREE/Sonderfälle/$(printf 'gru\xdf_latin1.txt')" 2>/dev/null || true
touch "$TREE/Sonderfälle/leer_0_byte.bin"
# 3. Path deeper than 260 chars
DEEP="$TREE"
for i in $(seq 1 12); do DEEP="$DEEP/sehr_langer_ordnername_für_tiefe_verschachtelung_ebene_$i"; done
mkdir -p "$DEEP"
printf tief > "$DEEP/datei_am_ende_des_sehr_langen_pfades.txt"
echo "  deepest path length: $(printf '%s' "$DEEP" | wc -c) chars"
# 4. A 1.5 GB sparse file (streams fast, tests byte-progress + big-file path)
dd if=/dev/zero of="$TREE/Bestand_A/video_gross.mov" bs=1 count=1 seek=1610612735 2>/dev/null
# 5. Unreadable file and folder
printf geheim > "$TREE/Bestand_B/gesperrt.txt"; chmod 000 "$TREE/Bestand_B/gesperrt.txt"
mkdir -p "$TREE/Bestand_B/kein_zugriff"; printf x > "$TREE/Bestand_B/kein_zugriff/verloren.txt"; chmod 000 "$TREE/Bestand_B/kein_zugriff"
# 6. Symlink (must be skipped, not followed)
ln -s /etc/hostname "$TREE/Sonderfälle/verknuepfung.lnk"

TOTAL_READABLE=$(find "$TREE" -type f -readable ! -path "*kein_zugriff*" | wc -l)
echo "  readable files: $TOTAL_READABLE"

echo "== reference hashes (sha256sum) =="
( cd "$TREE" && find . -type f -readable ! -path "*kein_zugriff*" -print0 | sort -z | xargs -0 sha256sum ) > "$WORK/reference.txt" 2>/dev/null
echo "  $(wc -l < "$WORK/reference.txt") reference rows"

echo "== T1: full run =="
"$BIN" -root "$TREE" -output "$OUT/manifest.csv" -quiet
RC=$?
[ $RC -eq 2 ] && ok "exit code 2 (completed with unreadable files)" || fail "exit code $RC, want 2"
[ -f "$OUT/manifest.csv" ] && ok "manifest written" || fail "manifest missing"
[ -f "$OUT/manifest_errors.log" ] && ok "error log written" || fail "error log missing"
[ ! -f "$OUT/manifest.csv.part.json" ] && ok "checkpoint removed after success" || fail "checkpoint left behind"

echo "== T2: manifest matches sha256sum reference =="
python3 - "$OUT/manifest.csv" "$WORK/reference.txt" <<'EOF'
import csv, sys
rows = {}
with open(sys.argv[1], encoding='utf-8-sig', newline='') as f:
    r = csv.reader(f)
    header = next(r)
    assert header == ["relative_path","filename","size_bytes","sha256","mtime_utc"], header
    for rec in r:
        rows[rec[0]] = rec[3]
def encode_specials(p):
    # Mirror the tool's rule: invalid bytes (surrogateescape'd) and control chars -> %XX
    out = []
    for c in p:
        o = ord(c)
        if 0xDC80 <= o <= 0xDCFF:
            out.append(f"%{o-0xDC00:02X}")
        elif o < 0x20 or o == 0x7F:
            out.append(f"%{o:02X}")
        else:
            out.append(c)
    return ''.join(out)
ref = {}
with open(sys.argv[2], encoding='utf-8', errors='surrogateescape') as f:
    for line in f:
        h, p = line.rstrip("\n").split("  ", 1)
        if h.startswith("\\"):  # coreutils escapes \n, \r, \\ in filenames
            h = h[1:]
            p = p.replace("\\n", "\n").replace("\\r", "\r").replace("\\\\", "\\")
        ref[encode_specials(p[2:])] = h  # strip "./"
missing = set(ref) - set(rows)
extra = set(rows) - set(ref)
wrong = [p for p in set(ref) & set(rows) if ref[p] != rows[p]]
if missing or extra:
    print("MISMATCH missing:", list(missing)[:5], "extra:", list(extra)[:5]); sys.exit(1)
if wrong:
    print("WRONG HASHES:", wrong[:5]); sys.exit(1)
print(f"  {len(rows)} rows, all hashes match reference")
EOF
[ $? -eq 0 ] && ok "all hashes match sha256sum" || fail "hash comparison failed"

echo "== T3: error log contents =="
grep -q "gesperrt.txt" "$OUT/manifest_errors.log" && ok "unreadable file logged" || fail "unreadable file not logged"
grep -q "kein_zugriff" "$OUT/manifest_errors.log" && ok "unreadable folder logged" || fail "unreadable folder not logged"
grep -q "verknuepfung.lnk" "$OUT/manifest_errors.log" && ok "symlink skip logged" || fail "symlink skip not logged"
grep -q "verloren.txt" "$OUT/manifest.csv" && fail "file inside unreadable dir appears in manifest" || ok "unreadable dir contents excluded"
grep -q "verknuepfung" "$OUT/manifest.csv" && fail "symlink was hashed" || ok "symlink not hashed"

echo "== T4: SIGKILL mid-run, then resume =="
rm -f "$OUT/kill.csv" "$OUT/kill_errors.log" "$OUT/kill.csv.part.json"
"$BIN" -root "$TREE" -output "$OUT/kill.csv" -quiet &
PID=$!
# wait until some rows exist, then kill hard
for i in $(seq 1 300); do
  ROWS=$( { wc -l < "$OUT/kill.csv"; } 2>/dev/null || echo 0)
  [ "$ROWS" -gt 200 ] && break
  sleep 0.02
done
kill -9 $PID 2>/dev/null; wait $PID 2>/dev/null
ROWS_AFTER_KILL=$(wc -l < "$OUT/kill.csv" 2>/dev/null || echo 0)
echo "  rows at kill: $ROWS_AFTER_KILL"
[ "$ROWS_AFTER_KILL" -gt 1 ] && ok "partial manifest exists after SIGKILL" || fail "no partial manifest"
[ -f "$OUT/kill.csv.part.json" ] && ok "checkpoint survives SIGKILL" || fail "checkpoint missing after SIGKILL"
# corrupt the tail like a torn write
printf 'kaputte,zeile,ohne' >> "$OUT/kill.csv"
"$BIN" -root "$TREE" -output "$OUT/kill.csv" -quiet
RC=$?
[ $RC -eq 2 ] && ok "resume completed (exit 2)" || fail "resume exit code $RC"
python3 - "$OUT/manifest.csv" "$OUT/kill.csv" <<'EOF'
import csv, sys
def load(p):
    with open(p, encoding='utf-8-sig', newline='') as f:
        r = csv.reader(f); next(r)
        return {rec[0]: (rec[2], rec[3]) for rec in r}
a, b = load(sys.argv[1]), load(sys.argv[2])
assert a == b, f"resumed manifest differs: {len(a)} vs {len(b)} rows"
# also check for duplicates
with open(sys.argv[2], encoding='utf-8-sig', newline='') as f:
    r = csv.reader(f); next(r)
    rels = [rec[0] for rec in r]
assert len(rels) == len(set(rels)), "duplicate rows after resume"
print(f"  resumed manifest identical to reference run ({len(b)} rows, no duplicates)")
EOF
[ $? -eq 0 ] && ok "resumed manifest == uninterrupted manifest" || fail "resumed manifest differs"
[ ! -f "$OUT/kill.csv.part.json" ] && ok "checkpoint removed after resume" || fail "checkpoint left after resume"

echo "== T5: SIGINT (Ctrl-C) graceful stop =="
rm -f "$OUT/int.csv" "$OUT/int_errors.log" "$OUT/int.csv.part.json"
"$BIN" -root "$TREE" -output "$OUT/int.csv" -quiet &
PID=$!
for i in $(seq 1 300); do
  ROWS=$( { wc -l < "$OUT/int.csv"; } 2>/dev/null || echo 0)
  [ "$ROWS" -gt 100 ] && break
  sleep 0.02
done
kill -INT $PID; wait $PID; RC=$?
[ $RC -eq 130 ] && ok "SIGINT exit code 130" || fail "SIGINT exit code $RC, want 130"
[ -f "$OUT/int.csv.part.json" ] && ok "checkpoint kept on interrupt" || fail "checkpoint missing on interrupt"

echo "== T6: read-only tree, nothing modified =="
find "$TREE" -path "*kein_zugriff*" -prune -o -print0 2>/dev/null | xargs -0 stat -c '%n|%s|%Y' 2>/dev/null | sort > "$WORK/before.txt"
chmod -R a-w "$TREE" 2>/dev/null
"$BIN" -root "$TREE" -output "$OUT/ro.csv" -quiet; RC=$?
chmod -R u+w "$TREE" 2>/dev/null; chmod 000 "$TREE/Bestand_B/gesperrt.txt"; chmod 000 "$TREE/Bestand_B/kein_zugriff"
find "$TREE" -path "*kein_zugriff*" -prune -o -print0 2>/dev/null | xargs -0 stat -c '%n|%s|%Y' 2>/dev/null | sort > "$WORK/after.txt"
[ $RC -eq 2 ] && ok "run on read-only tree works" || fail "read-only tree run failed rc=$RC"
diff -q "$WORK/before.txt" "$WORK/after.txt" >/dev/null && ok "tree bit-identical (sizes+mtimes)" || fail "tree was modified!"

echo "== T7: output inside root refused =="
"$BIN" -root "$TREE" -output "$TREE/boese.csv" -quiet 2>"$WORK/t7.err"; RC=$?
[ $RC -eq 1 ] && ok "refused with exit 1" || fail "exit $RC, want 1"
grep -qi "inside" "$WORK/t7.err" && ok "clear error message" || fail "unclear message: $(cat "$WORK/t7.err")"
[ ! -f "$TREE/boese.csv" ] && ok "nothing written into tree" || fail "wrote into tree!"

echo "== T8: workers override produces identical output =="
"$BIN" -root "$TREE/Sonderfälle" -output "$OUT/w1.csv" -workers 1 -quiet
"$BIN" -root "$TREE/Sonderfälle" -output "$OUT/w8.csv" -workers 8 -quiet
cmp -s "$OUT/w1.csv" "$OUT/w8.csv" && ok "1-worker and 8-worker output byte-identical" || fail "outputs differ by worker count"

echo "== T9: fresh flag ignores resume state =="
cp "$OUT/w1.csv" "$OUT/fresh.csv"
printf '{"tool_version":"x","root":"%s"}' "$TREE/Sonderfälle" > "$OUT/fresh.csv.part.json"
"$BIN" -root "$TREE/Sonderfälle" -output "$OUT/fresh.csv" -fresh -quiet
grep -c . "$OUT/fresh.csv" >/dev/null
cmp -s "$OUT/fresh.csv" "$OUT/w1.csv" && ok "-fresh rewrites from scratch, identical result" || fail "-fresh output differs"
[ ! -f "$OUT/fresh.csv.part.json" ] && ok "stale checkpoint cleaned up" || fail "checkpoint remains"

echo "== T10: version and usage =="
"$BIN" -version | grep -qE "gami-hash v[0-9]" && ok "version flag" || fail "version flag broken"
"$BIN" -root "" 2>&1 | grep -q "Usage" && ok "usage on missing args" || fail "no usage help"

echo
echo "================================"
echo "PASS: $PASS   FAIL: $FAIL"
[ $FAIL -eq 0 ]
