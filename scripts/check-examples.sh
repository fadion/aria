#!/usr/bin/env bash
# Run every example and compare its output to the golden beside it.
#
# The examples are programs people read to learn the language, so one that has
# stopped running teaches the wrong thing. This is the same idea as
# check-readme.sh, with goldens instead of an error scan, because an example can
# fail by printing the wrong answer without ever printing "error:".
#
#   scripts/check-examples.sh          compare against the goldens
#   scripts/check-examples.sh record   regenerate them, deliberately
#   scripts/check-examples.sh -v       also print each example's output
#
# Only examples/*.ari are run. examples/lib/ holds files meant to be imported,
# which have no output of their own and no golden.
#
# Each case runs more than once and any case whose output varies is reported
# rather than recorded. Dictionaries have no order, so an example that walks one
# without sorting first is nondeterministic, and a golden recorded from a single
# lucky run would fail on somebody else's machine.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIR="$ROOT/examples"
RUNS="${ARIA_RUNS:-3}"
TIMEOUT="${ARIA_TIMEOUT:-10}"

MODE="verify"
VERBOSE=""
for arg in "$@"; do
  case "$arg" in
    record) MODE="record" ;;
    -v)     VERBOSE="-v" ;;
    *)      echo "usage: $(basename "$0") [record] [-v]" >&2; exit 2 ;;
  esac
done

# ARIA_BIN wins, then a binary already sitting in the repo root, and otherwise
# build one into a temp directory, so this runs from a clean checkout with no
# setup and leaves no artifact behind.
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

# Run one example once, from the examples directory so relative imports resolve.
# </dev/null matters: an example calling prompt() would otherwise read this
# loop's own input and swallow the remaining examples.
run_once() {
  (cd "$DIR" && timeout "$TIMEOUT" "$BIN" run "$1" </dev/null 2>&1)
  printf 'exit: %d\n' "$?"
}

total=0; ok=0; failed=0; flaky=0; recorded=0
while IFS= read -r file; do
  total=$((total + 1))
  name="$(basename "$file" .ari)"
  golden="$DIR/$name.out"

  out="$(run_once "$name.ari")"

  # Repeat, and treat any variation as a failure rather than recording one of
  # the answers as the expected one.
  varies=""
  for _ in $(seq 2 "$RUNS"); do
    again="$(run_once "$name.ari")"
    [ "$again" != "$out" ] && varies="yes" && break
  done

  if [ "$VERBOSE" = "-v" ]; then
    printf -- '--- %s\n%s\n' "$name" "$out"
  fi

  if [ -n "$varies" ]; then
    flaky=$((flaky + 1))
    printf '%s: output varies between runs\n' "$name"
    printf '    a dictionary walked without sorting its keys is the usual cause\n'
    continue
  fi

  if [ "$MODE" = "record" ]; then
    printf '%s\n' "$out" > "$golden"
    recorded=$((recorded + 1))
    continue
  fi

  if [ ! -f "$golden" ]; then
    failed=$((failed + 1))
    printf '%s: no golden; run `scripts/check-examples.sh record`\n' "$name"
    continue
  fi

  if ! printf '%s\n' "$out" | diff -q - "$golden" >/dev/null 2>&1; then
    failed=$((failed + 1))
    printf '%s: output does not match its golden\n' "$name"
    printf '%s\n' "$out" | diff -u "$golden" - | sed -n '3,15p' | sed 's/^/    /'
    continue
  fi

  ok=$((ok + 1))
done < <(find "$DIR" -maxdepth 1 -name '*.ari' | sort)

echo
if [ "$MODE" = "record" ]; then
  printf '%d examples: %d recorded, %d skipped as nondeterministic\n' \
    "$total" "$recorded" "$flaky"
else
  printf '%d examples: %d ok, %d failed, %d nondeterministic\n' \
    "$total" "$ok" "$failed" "$flaky"
fi

[ $((failed + flaky)) -gt 0 ] && exit 1
exit 0
