#!/usr/bin/env bash
# Characterization harness for Aria.
#
# Records what the CURRENT interpreter actually does for every case in
# testdata/semantics, bugs included, so a rewrite has an oracle to diff against.
#
#   scripts/characterize.sh record   regenerate every .out golden
#   scripts/characterize.sh verify   compare current behavior to the goldens
#   scripts/characterize.sh <path>   restrict either mode to one dir or file
#
# ARIA_BIN overrides the interpreter under test (default: ./aria[.exe]).

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORPUS="$ROOT/testdata/semantics"
TIMEOUT="${ARIA_TIMEOUT:-3}"
RUNS="${ARIA_RUNS:-7}"   # repeat each case; map iteration order is randomized, so a
                         # low sample count lets nondeterminism agree by chance

# ARIA_BIN wins, then a binary already sitting in the repo root, and otherwise
# build one into a temp directory. Building on demand means the suite runs from
# a clean checkout with no setup, and leaves no artifact behind afterwards.
BUILT=""
cleanup() { [ -n "$BUILT" ] && rm -rf "$BUILT"; }
trap cleanup EXIT

BIN="${ARIA_BIN:-}"
if [ -z "$BIN" ]; then
  for c in "$ROOT/aria.exe" "$ROOT/aria"; do [ -x "$c" ] && BIN="$c" && break; done
fi
if [ -z "$BIN" ]; then
  BUILT="$(mktemp -d)"
  BIN="$BUILT/aria"
  if ! (cd "$ROOT" && go build -o "$BIN" .); then
    echo "error: could not build the interpreter" >&2
    exit 2
  fi
fi
if [ ! -x "$BIN" ]; then
  echo "error: '$BIN' is not executable" >&2
  exit 2
fi

MODE="${1:-verify}"
case "$MODE" in
  record|verify) TARGET="${2:-$CORPUS}" ;;
  *)             TARGET="$MODE"; MODE="verify" ;;
esac

# Run one case once; emit combined output plus an exit trailer.
# A Go panic trace carries goroutine addresses and absolute build paths, both of
# which differ per run and per machine. Keep the panic message; drop the trace.
normalize() {
  awk '
    /^goroutine [0-9]+ \[running\]:/ { print "[goroutine stack elided]"; skip = 1; next }
    skip { next }
    /^\[signal /                     { print "[signal details elided]"; next }
    { print }
  '
}

run_once() {
  local file="$1" dir out code
  dir="$(dirname "$file")"
  out="$(cd "$dir" && timeout "$TIMEOUT" "$BIN" run "$(basename "$file")" </dev/null 2>&1)"
  code=$?
  printf '%s\n' "$out" | normalize
  if [ "$code" -eq 124 ]; then
    printf -- '--- exit: TIMEOUT after %ss\n' "$TIMEOUT"
  else
    printf -- '--- exit: %s\n' "$code"
  fi
}

# Run a case RUNS times. Identical every time -> that output. Otherwise the
# case is nondeterministic, and we record the fact plus each distinct variant
# so the rewrite still has something stable to diff against.
observe() {
  local file="$1" first="" cur="" varies=0 i
  local tmp; tmp="$(mktemp -d)"
  for i in $(seq 1 "$RUNS"); do
    cur="$(run_once "$file")"
    printf '%s\n' "$cur" > "$tmp/$i"
    if [ "$i" -eq 1 ]; then first="$cur"
    elif [ "$cur" != "$first" ]; then varies=1
    fi
  done

  if [ "$varies" -eq 0 ]; then
    printf '%s\n' "$first"
  else
    printf -- '--- NONDETERMINISTIC: output varies across runs\n'
    sort -u "$tmp"/* | sed 's/^/| /'
  fi
  rm -rf "$tmp"
}

# For a case recorded as NONDETERMINISTIC, report whether every line this run
# produced was already among the recorded variants. New output means a real
# change; a subset just means the sampling landed differently.
within_variants() {
  local actual="$1" golden="$2" line
  while IFS= read -r line; do
    case "$line" in
      '--- NONDETERMINISTIC'*) continue ;;
      '| '*) continue ;;
    esac
    grep -qxF "| $line" "$golden" || return 1
  done <<< "$actual"
  return 0
}

mapfile -t CASES < <(find "$TARGET" -name '*.ari' ! -name '_*' | sort)
[ "${#CASES[@]}" -eq 0 ] && { echo "no cases under $TARGET" >&2; exit 2; }

pass=0; fail=0; wrote=0; nondet=0
for case in "${CASES[@]}"; do
  golden="${case%.ari}.out"
  actual="$(observe "$case")"
  rel="${case#"$ROOT"/}"

  case "$actual" in *"NONDETERMINISTIC"*) nondet=$((nondet+1)) ;; esac

  if [ "$MODE" = record ]; then
    # Nondeterminism is sticky. Detection is sampled, so a case known to vary
    # can come back stable on any given run; overwriting the marker then would
    # silently turn a documented variation into a fixed expectation, and the
    # next run would report a spurious change. Merge instead.
    if [ -f "$golden" ] && grep -q '^--- NONDETERMINISTIC' "$golden" \
       && ! printf '%s' "$actual" | grep -q '^--- NONDETERMINISTIC'; then
      { grep '^--- NONDETERMINISTIC' "$golden"
        { grep '^| ' "$golden"; printf '%s\n' "$actual" | sed 's/^/| /'; } | sort -u
      } > "$golden.tmp" && mv "$golden.tmp" "$golden"
      wrote=$((wrote+1))
      nondet=$((nondet+1))
      printf 'merged    %s (known nondeterministic; run was stable)\n' "$rel"
      continue
    fi
    printf '%s\n' "$actual" > "$golden"
    wrote=$((wrote+1))
    printf 'recorded  %s\n' "$rel"
  else
    if [ ! -f "$golden" ]; then
      printf 'MISSING   %s (no golden; run: scripts/characterize.sh record)\n' "$rel"
      fail=$((fail+1))
    elif [ "$actual" = "$(cat "$golden")" ]; then
      pass=$((pass+1))
    elif grep -q '^--- NONDETERMINISTIC' "$golden" && within_variants "$actual" "$golden"; then
      # The golden records a case whose output varies. Detection is sampled, so
      # a run can legitimately come back stable, or land on a different subset
      # of the variants. Either is a pass as long as nothing NEW appeared.
      pass=$((pass+1))
      nondet=$((nondet+1))
    else
      printf 'CHANGED   %s\n' "$rel"
      diff -u "$golden" <(printf '%s\n' "$actual") | sed '1,2d;s/^/    /'
      fail=$((fail+1))
    fi
  fi
done

echo
if [ "$MODE" = record ]; then
  printf 'recorded %d case(s); %d nondeterministic\n' "$wrote" "$nondet"
else
  printf '%d passed, %d changed, %d nondeterministic\n' "$pass" "$fail" "$nondet"
  [ "$fail" -gt 0 ] && exit 1
fi
exit 0
