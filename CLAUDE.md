# Working on Aria

Aria is a small interpreted language: a hand-written scanner and Pratt parser, a name
resolver, and a tree-walking evaluator, all in Go.

**Read [docs/architecture.md](docs/architecture.md) before changing anything in
`internal/`.** It covers how the pipeline fits together and, more importantly, *why* the
decisions that could reasonably have gone another way went the way they did. Several of
them look arbitrary until you know what they prevent.

## Layout

```
aria.go                CLI: run and repl
internal/source        file contents, byte offsets, line index
internal/token         Kind + Span
internal/scanner       text -> tokens
internal/parser        tokens -> AST
internal/ast           node types, Bad, Walk
internal/resolver      name binding, mutability, scope checking
internal/value         runtime values, equality, display
internal/interp        evaluator
internal/stdlib        the standard library, written in Aria, go:embed'd
internal/diag          diagnostics
testdata/semantics     the characterization suite
examples/              runnable example programs, with .out goldens
scripts/               the suite's runner
```

## Before calling anything done

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...
golangci-lint run ./...
bash scripts/characterize.sh verify
bash scripts/check-readme.sh
bash scripts/check-examples.sh
```

CI runs `golangci-lint` at a pinned version, and it checks things `go vet` does not —
`ineffassign` and `predeclared` have both failed a branch that passed everything else.
Install it with `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1`
to match.

The characterization suite is the important one. `go test` covers the Go code; the suite
covers *the language*, and it is the thing that catches a change you did not intend to
make.

After touching the scanner or parser, also run the fuzzers for 30 seconds each. They are
cheap and they have both found real bugs:

```bash
go test ./internal/scanner/ -run=Fuzz -fuzz=FuzzScan -fuzztime=30s
go test ./internal/parser/  -run=Fuzz -fuzz=FuzzParse -fuzztime=30s
```

## The characterization suite is the arbiter

`testdata/semantics/` holds 226 cases, each a `.ari` file with a `.out` golden recording
exactly what the interpreter prints and what it exits with. Files prefixed with `_` are
fixtures another case imports, and have no golden of their own.

**If a golden changes, you changed the language.** That is either the point of your change
or a bug, and the two look identical in a diff. So:

- Decide *before* re-recording whether each change is intended.
- If it is, say why in the commit message and update
  [docs/architecture.md](docs/architecture.md) if it reflects a design decision.
- If you cannot explain a change, it is a bug. This is how several real bugs were caught.

```bash
scripts/characterize.sh verify              # diff against the goldens
scripts/characterize.sh verify testdata/semantics/operators   # scope it
scripts/characterize.sh record              # re-record, deliberately
ARIA_BIN=/path/to/other scripts/characterize.sh verify   # compare two implementations
```

Both scripts build the interpreter themselves if there is no binary in the repo root, so
nothing needs building first and no artifact is left in the tree afterwards.

Two properties of the runner worth knowing: it runs each case **seven times** and flags
any case whose output varies, and re-recording **merges into** an existing
`NONDETERMINISTIC` marker rather than overwriting it — detection is sampled, so a flaky
case can come back stable by luck and silently become a fixed expectation.

Adding a language feature means adding a case. The suite is only as good as its coverage,
and every gap in it has cost something: the empty dictionary literal `[=>]` was missing,
so a parser rewrite dropped it and only the standard library noticed.

## Rules that came out of this codebase

These are not style preferences. Each one is a bug class that actually bit.

**Parsing never returns nil.** A failed parse yields `ast.Bad`, never a nil `Node`. When
`nil` meant both "no value" and "an error happened", a nil dereference reached production
and a statement that legitimately produced nothing silently discarded the rest of its
block.

**The scanner may never fail to advance.** Every path through `Scan` consumes at least one
byte. This is why the token stream terminates on any input; a peek that recorded a read
once let the cursor walk backwards and re-scan the same token forever.

**Peeking is pure.** A lookahead function must not touch cursor state. That is the exact
bug above, in one line.

**Diagnostics are values, not globals.** Pass a `*diag.Bag`. Package-level error state
made the packages unusable as a library and untestable in parallel.

**Collections are immutable.** Every operation returns a new value. An operator that
mutates an operand is a bug even when it looks like an optimisation — the old `+` grew
into a shared backing array and corrupted results computed earlier.

**Display and equality are different operations.** `String`/`Inspect` for presentation,
`Equal`/`KeyOf` for meaning. Using a formatted string as an equality primitive made `1`
and `"1"` the same dictionary key and made lookup depend on Go's map iteration order.

**Watch for Go's typed-nil trap.** A nil `*ast.Identifier` widened to a `Node` interface is
**not** `nil`. `ast.Children` uses per-type helpers for exactly this reason; a generic
`if c != nil` there crashed on every function without a declared return type.

**Result types come from operand types, never operand values.** `Int / Int` is `Int`. A
rule that inspected values meant a declared `-> Int` could fail for inputs its author
never tried.

## When changing the language deliberately

1. Add or update a case in `testdata/semantics/`.
2. Make the change.
3. Run `verify` and read every diff. Each one should be explainable.
4. `record`, and mention the behavior change in the commit.
5. Update [docs/architecture.md](docs/architecture.md) if it is a design decision, and the
   README if it is user-visible.

The README's code blocks are executable claims about the language, so they get run too:

```bash
scripts/check-readme.sh        # -v to see each block's output
```

Keep them runnable. A block that is prose, not code, should not be fenced as `swift` — the
checker will try to run it.

`examples/` is checked the same way, against a `.out` golden per program rather than an
error scan, because an example can print the wrong answer without ever printing `error:`.

```bash
scripts/check-examples.sh          # diff against the goldens
scripts/check-examples.sh record   # re-record, deliberately
```

Adding a language feature is a good reason to add an example. Keep them deterministic:
no `Math.random`, no `Time.now`, and no walking a dictionary without sorting its keys,
since dictionaries have no order. The runner repeats each one and reports any whose
output varies instead of recording a lucky run.

## Pull requests

Keep the body to two parts: a sentence or two on why the change exists, then a
`## What changed` list of what it does. That is the whole convention.

The reasoning goes in the commit messages, where it stays attached to the lines it
explains instead of to a branch that may get force-pushed out from under it. A PR body
that restates the commits goes stale immediately and gets read twice by nobody.

End at the content. No generated-with footer.

## Releasing

Tags are `vX.Y.Z`. The `v` is not decoration: the module proxy only recognises a
version tagged that way, and because every tag through 0.5.0 was unprefixed,
`go install github.com/fadion/aria@latest` resolves to a 2017 pseudo-version — the
newest commit it can see with no version attached to it. The README tells people to
run that command, so the next tag needs the prefix for it to be true.

`const version` in `aria.go` is what the binary reports, and the release workflow
refuses a tag that disagrees with it. Bump it in the same commit you tag.

```bash
git tag v1.0.0 && git push origin v1.0.0
```

That builds six archives, checksums them, and publishes a release. A pull request
touching `.github/workflows/release.yml` runs the same build without publishing, so
the packaging can be checked before a tag depends on it.

## Notes

- Go 1.26. Module path `github.com/fadion/aria`, no dependencies beyond `fatih/color` and
  `urfave/cli` for the CLI.
- The standard library is Aria source under `internal/stdlib/*.ari`, embedded with
  `go:embed`. It is real code and gets parsed and resolved like anything else — running
  the resolver over it found two functions that had never been callable.
- The resolver computes slot indices the evaluator does not use yet. See the known gaps at
  the end of [docs/architecture.md](docs/architecture.md).
