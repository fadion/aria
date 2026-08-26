#!/usr/bin/env bash
# Run every code block in the README through the interpreter.
#
# The README is the language's documentation, so its examples are claims about
# what the language does. This checks them. It caught a precedence table that
# still described the pre-rewrite ordering, a dictionary literal written in a
# syntax that had been replaced, and a bitwise example that had never parsed.
#
#   scripts/check-readme.sh          check them
#   scripts/check-readme.sh -v       also print each block's output
#
# Many blocks are fragments that use names defined in an earlier block, and a
# few demonstrate errors on purpose. Both are expected; anything else is a
# README that has drifted from the implementation.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
README="$ROOT/README.md"
VERBOSE="${1:-}"

WORK="$(mktemp -d)"
BUILT=""
cleanup() {
  rm -rf "$WORK"
  [ -n "$BUILT" ] && rm -rf "$BUILT"
}
trap cleanup EXIT

# ARIA_BIN wins, then a binary already sitting in the repo root, and otherwise
# build one into a temp directory, so this runs from a clean checkout with no
# setup and leaves no artifact behind.
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

# Blocks fenced as a language get extracted; bare ``` blocks are shell or
# output samples and are skipped.
awk -v dir="$WORK" '
  /^```(swift|javascript|js)$/ { inblock = 1; n++; file = sprintf("%s/%03d.ari", dir, n); start = NR; printf "" > file; next }
  /^```[[:space:]]*$/          { if (inblock) print n"\t"start >> (dir "/lines.txt"); inblock = 0; next }
  inblock                      { print >> file }
' "$README"

total=0; failed=0; fragments=0; expected=0
while IFS= read -r file; do
  total=$((total + 1))
  name="$(basename "$file" .ari)"
  line="$(awk -F'\t' -v n="$((10#$name))" '$1 == n { print $2 }' "$WORK/lines.txt" 2>/dev/null)"

  # </dev/null matters: a block calling prompt() would otherwise read the
  # loop's own input and swallow the remaining blocks.
  out="$(cd "$WORK" && timeout 10 "$BIN" run "$file" </dev/null 2>&1)"

  if [ "$VERBOSE" = "-v" ]; then
    printf -- '--- block %s (README:%s)\n%s\n' "$name" "${line:-?}" "$out"
  fi

  if ! printf '%s' "$out" | grep -q 'error:'; then
    continue
  fi

  msg="$(printf '%s' "$out" | grep 'error:' | head -1 | sed 's|.*\.ari:||')"

  # A fragment referring to names an earlier block defined, or importing a file
  # the README only describes. Neither is a drift.
  case "$msg" in
    *"is not defined"*|*"cannot read imported file"*) fragments=$((fragments + 1)); continue ;;
  esac

  # A block whose own comment says it errors is demonstrating the error.
  if grep -qiE '//.*(error|won.t|fails)' "$file"; then
    expected=$((expected + 1)); continue
  fi

  failed=$((failed + 1))
  printf 'README:%s (block %s)\n    %s\n' "${line:-?}" "$name" "$msg"
done < <(find "$WORK" -name '*.ari' | sort)

echo
printf '%d blocks: %d ok, %d fragments, %d deliberate errors, %d unexplained\n' \
  "$total" "$((total - fragments - expected - failed))" "$fragments" "$expected" "$failed"

[ "$failed" -gt 0 ] && exit 1
exit 0
