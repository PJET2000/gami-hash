#!/bin/bash
# GUI wizard flow test: runs the real binary in GUI mode against a fake
# zenity that answers dialogs per scenario and logs everything.
set -u
SCRATCH="$(cd "$(dirname "$0")" && pwd)"
WORKDIR="${TMPDIR:-/tmp}/gami-hash-tests"; mkdir -p "$WORKDIR"
BIN="${GAMI_HASH_BIN:?set GAMI_HASH_BIN to the linux binary to test}"
SHIM="$SCRATCH/shim"
chmod +x "$SHIM/zenity"
WORK="$WORKDIR/guitest"
rm -rf "$WORK"; mkdir -p "$WORK/out"
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo "  ok: $1"; }
fail() { FAIL=$((FAIL+1)); echo "FAIL: $1"; }

# Small tree (a few MB so progress emits a couple of updates)
TREE="$WORK/tree"; mkdir -p "$TREE/unter ordner"
for i in $(seq 1 300); do printf 'gui-inhalt-%04d\n' $i > "$TREE/unter ordner/akte_$i.txt"; done
truncate -s 20G "$TREE/gross.bin" # sparse; ~10-15s of hashing so the ETA (5s warm-up) reliably appears

run_gui() { # scenario log output [config-name]
  # Each scenario gets its own config dir so the one-click resume state
  # (last-run.json) never leaks between scenarios; pass the same config-name
  # for a cancel/resume pair.
  local scen="$1" log="$2" cfg="${4:-$1}"
  : > "$log"; : > "$WORK/state"; mkdir -p "$WORK/cfg-$cfg"
  env PATH="$SHIM:$PATH" LANG=de_DE.UTF-8 LC_ALL=de_DE.UTF-8 \
      XDG_CONFIG_HOME="$WORK/cfg-$cfg" \
      SHIM_SCENARIO="$scen" SHIM_LOG="$log" SHIM_STATE="$WORK/state" \
      SHIM_ROOT="$TREE" SHIM_OUTPUT="$3" \
      "$BIN"
  return $?
}

echo "== G1: happy path (welcome→folder→save→start→progress→done) =="
OUT1="$WORK/out/gui1.csv"
run_gui happy "$WORK/g1.log" "$OUT1"; RC=$?
[ $RC -eq 0 ] && ok "exit 0" || fail "exit $RC"
[ -f "$OUT1" ] && ok "manifest written via GUI" || fail "no manifest"
ROWS=$(python3 -c "import csv,sys;print(sum(1 for _ in csv.reader(open(sys.argv[1],encoding='utf-8-sig')))-1)" "$OUT1")
[ "$ROWS" = "301" ] && ok "all 301 files recorded" || fail "rows=$ROWS want 301"
grep -q -- "--question" "$WORK/g1.log" && ok "welcome question shown" || fail "no welcome dialog"
grep -q -- "--directory" "$WORK/g1.log" && ok "folder picker shown" || fail "no folder picker"
grep -q -- "--save" "$WORK/g1.log" && ok "save dialog shown" || fail "no save dialog"
grep -q -- "--progress" "$WORK/g1.log" && ok "progress dialog shown" || fail "no progress dialog"
grep -qE "PROGRESS: [0-9]+(\.[0-9]+)?$" "$WORK/g1.log" && ok "numeric progress values streamed" || fail "no numeric progress"
grep -q "PROGRESS: 100" "$WORK/g1.log" && ok "progress reached 100" || fail "progress never hit 100"
grep -q "time left" "$WORK/g1.log" && ok "ETA shown in progress text" || fail "no ETA in progress text"
grep -q "Done." "$WORK/g1.log" && ok "done message shown" || fail "no done dialog"
grep -q "Open folder" "$WORK/g1.log" && ok "done dialog offers open-folder" || fail "no open-folder button"
grep -q "checksum list" "$WORK/g1.log" && ok "welcome text present" || fail "welcome text missing"
grep -q "cannot break anything" "$WORK/g1.log" && ok "reassurance wording present" || fail "reassurance missing"
grep -q "goes to sleep" "$WORK/g1.log" && ok "sleep-mode reassurance in start dialog" || fail "sleep reassurance missing"
# GUI wording check: dialog texts should mention the output path
grep -q "gui1.csv" "$WORK/g1.log" && ok "output path shown to user" || fail "output path never shown"
[ ! -f "$OUT1.part.json" ] && ok "no checkpoint left" || fail "checkpoint left"
[ ! -f "$WORK/cfg-happy/gami-hash/last-run.json" ] && ok "last-run state cleared after success" || fail "last-run state left behind"
grep -q "—" "$WORK/g1.log" && fail "em dash found in dialog texts" || ok "no em dashes in dialogs"

echo "== G2: user cancels at welcome =="
OUT2="$WORK/out/gui2.csv"
run_gui welcome-cancel "$WORK/g2.log" "$OUT2"; RC=$?
[ $RC -eq 0 ] && ok "clean exit" || fail "exit $RC"
[ ! -f "$OUT2" ] && ok "nothing written" || fail "file written despite cancel"
grep -q -- "--directory" "$WORK/g2.log" && fail "folder picker shown after cancel" || ok "wizard stopped at welcome"

echo "== G3: cancel during progress → one-click resume at next start =="
OUT3="$WORK/out/gui3.csv"
run_gui cancel-progress "$WORK/g3.log" "$OUT3" pair3; RC=$?
[ $RC -eq 0 ] && ok "clean exit after cancel" || fail "exit $RC"
[ -f "$OUT3.part.json" ] && ok "checkpoint kept" || fail "no checkpoint"
grep -q "your progress is saved" "$WORK/g3.log" && ok "stopped message reassures" || fail "no stopped message"
grep -q "One click" "$WORK/g3.log" && ok "stopped message explains next start" || fail "no next-start hint"
[ -f "$WORK/cfg-pair3/gami-hash/last-run.json" ] && ok "last-run state saved" || fail "no last-run state"
# next start: the resume offer must come FIRST, no folder or save dialogs
run_gui resume "$WORK/g3b.log" "$OUT3" pair3; RC=$?
[ $RC -eq 0 ] && ok "resume run exits 0" || fail "resume exit $RC"
grep -q "nothing was lost" "$WORK/g3b.log" && ok "resume prompt reassures first" || fail "no reassurance in resume prompt"
grep -q "picks up exactly where it stopped" "$WORK/g3b.log" && ok "resume prompt explains continuation" || fail "no resume prompt"
grep -q -- "--extra-button" "$WORK/g3b.log" && ok "new-run offered as extra button" || fail "no extra button"
grep -q -- "--directory" "$WORK/g3b.log" && fail "folder picker shown despite one-click resume" || ok "no folder picker needed"
grep -q -- "--save" "$WORK/g3b.log" && fail "save dialog shown despite one-click resume" || ok "no save dialog needed"
ROWS=$(python3 -c "import csv,sys;print(sum(1 for _ in csv.reader(open(sys.argv[1],encoding='utf-8-sig')))-1)" "$OUT3")
[ "$ROWS" = "301" ] && ok "resume completed all 301 files" || fail "rows=$ROWS want 301"
grep -q "already been recorded in the previous run" "$WORK/g3b.log" && ok "done message mentions resumed files" || fail "no resumed-files note"
[ ! -f "$OUT3.part.json" ] && ok "checkpoint removed" || fail "checkpoint left"
[ ! -f "$WORK/cfg-pair3/gami-hash/last-run.json" ] && ok "last-run state cleared" || fail "last-run state left"

echo "== G3c: manual pick of an interrupted file still offers resume =="
OUT3C="$WORK/out/gui3c.csv"
run_gui cancel-progress "$WORK/g3c.log" "$OUT3C" pair3c
rm -f "$WORK/cfg-pair3c/gami-hash/last-run.json" # simulate: state gone, e.g. other computer
run_gui resume "$WORK/g3d.log" "$OUT3C" pair3c; RC=$?
[ $RC -eq 0 ] && ok "manual resume run exits 0" || fail "exit $RC"
grep -q "will be kept" "$WORK/g3d.log" && ok "manual resume prompt reassures" || fail "no manual resume prompt"
ROWS=$(python3 -c "import csv,sys;print(sum(1 for _ in csv.reader(open(sys.argv[1],encoding='utf-8-sig')))-1)" "$OUT3C")
[ "$ROWS" = "301" ] && ok "manual resume completed all files" || fail "rows=$ROWS want 301"

echo "== G4: output inside root is refused with a warning, then retried =="
OUT4="$WORK/out/gui4.csv"
run_gui inside-root-first "$WORK/g4.log" "$OUT4"; RC=$?
[ $RC -eq 0 ] && ok "exit 0" || fail "exit $RC"
grep -q -- "--warning" "$WORK/g4.log" && ok "warning shown for inside-root path" || fail "no warning"
[ ! -f "$TREE/boese.csv" ] && ok "nothing written into tree" || fail "wrote into tree!"
[ -f "$OUT4" ] && ok "second (valid) path used" || fail "no manifest after retry"
SAVES=$(grep -c "file10" "$WORK/state")
[ "$SAVES" -ge 2 ] && ok "save dialog re-shown after warning" || fail "save dialog not re-shown ($SAVES)"

echo "== G5: drive-letter change — resume despite different recorded root =="
OUT5="$WORK/out/gui5.csv"
run_gui cancel-progress "$WORK/g5.log" "$OUT5" pair5
[ -f "$OUT5.part.json" ] || fail "precondition: no checkpoint"
# Simulate the external drive having had a different letter/path back then.
python3 - "$OUT5.part.json" <<'EOF'
import json, sys
p = sys.argv[1]
cp = json.load(open(p))
cp["root"] = "/damals/anderer/pfad"
json.dump(cp, open(p, "w"))
EOF
run_gui resume-other-root "$WORK/g5b.log" "$OUT5" pair5; RC=$?
[ $RC -eq 0 ] && ok "resume-anyway run exits 0" || fail "exit $RC"
grep -q "Back then the path was" "$WORK/g5b.log" && ok "drive-letter explanation shown" || fail "no explanation dialog"
grep -q "different drive letter" "$WORK/g5b.log" && ok "wording mentions drive letters" || fail "wording missing"
ROWS=$(python3 -c "import csv,sys;print(sum(1 for _ in csv.reader(open(sys.argv[1],encoding='utf-8-sig')))-1)" "$OUT5")
[ "$ROWS" = "301" ] && ok "resume across root mismatch completed all files" || fail "rows=$ROWS want 301"
python3 -c "
import csv,sys
rels=[r[0] for r in csv.reader(open(sys.argv[1],encoding='utf-8-sig'))][1:]
sys.exit(0 if len(rels)==len(set(rels)) else 1)" "$OUT5" && ok "no duplicate rows" || fail "duplicates after mismatch resume"
[ ! -f "$OUT5.part.json" ] && ok "checkpoint removed" || fail "checkpoint left"

echo
echo "================================"
echo "GUI PASS: $PASS   FAIL: $FAIL"
[ $FAIL -eq 0 ]
