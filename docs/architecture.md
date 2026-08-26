# Aria — Architecture and Design Decisions

How the interpreter is put together, and why the parts that could reasonably have gone
another way went the way they did.

This is a working document. When a design decision changes, change it here too.

## The pipeline

```
source text
   │
   ├─ internal/source     file contents, byte offsets, line index
   ├─ internal/token      Kind + Span, no strings
   ├─ internal/scanner    text  -> tokens
   ├─ internal/parser     tokens -> AST          (internal/ast)
   ├─ internal/resolver   AST   -> name bindings
   └─ internal/interp     AST + bindings -> values  (internal/value)
```

Each phase completes before the next begins. Nothing is evaluated until the whole file
has parsed *and* resolved, so an undefined name inside a branch that never runs is still
reported, and it is reported before the program has had any effect.

`internal/diag` carries diagnostics across all of them. `internal/stdlib` holds the
standard library, written in Aria and embedded with `go:embed`.

## Positions are byte offsets

Everything refers to source text by byte offset — a `source.Span` of two `int32` — and
line/column is derived only when a diagnostic is rendered.

Offsets are cheap to store, exact, and cannot drift out of sync with the text. Tokens
carry no strings at all: the text is recovered by slicing the source when it is actually
needed, which keeps scanning allocation-free.

## The scanner cannot go backwards

The scanner reads an in-memory byte slice through a single integer cursor. There is no
buffer, no unread, and no way to move backwards. Every path through `Scan` advances the
cursor by at least one byte before returning.

This is not a convention, it is the reason the token stream is guaranteed to terminate on
any input at all. The previous lexer *could* rewind, and a peek that recorded a read let
`UnreadRune` walk the cursor back over input it had already consumed — so a file
containing invalid UTF-8 re-scanned the same token forever, at about 47 million tokens a
second, until memory ran out.

`FuzzScan` asserts the property directly: a scan may never produce more tokens than the
input has bytes.

## Parsing never returns nil

A failed parse yields an `ast.Bad` node covering the offending span, never a nil `Node`.
The tree is therefore always well-formed and nothing downstream has to nil-check a child.

`nil` used to mean both "no value" and "an error happened", which is what let a nil
dereference reach production: dotted access on a failed left side called a method on a
nil interface. A fuzzer found it in under a second.

A `Bad` node always comes with a diagnostic already reported, so anything walking the
tree stays quiet about it rather than adding a second message for the same mistake.

### The cursor invariant

Every parse method is entered with the cursor on the **first** token of the construct it
parses, and returns with the cursor on the **last** token of that construct — never past
it. Advancing to the next construct is the caller's job.

`at`, `accept`, `expectPeek` and `advance` are the only places the cursor moves, so the
invariant is checkable by reading four helpers rather than by tracing every call site.

### Reporting and recovery are separate

`errorAt` only reports. A caller that wants to resynchronise calls `recover` explicitly.
Fusing them meant that *reporting an error* silently advanced the token stream at every
call site.

The recovery set is derived from what can actually begin a construct. The previous one
was a hand-written list that had gone stale — `var` was added to the language and never
added to the list, so any parse error swallowed every following `var` statement to end of
file.

## Operator precedence

One table, in `internal/parser/precedence.go`, loosest to tightest:

| Level | Operators | Assoc |
|-------|-----------|-------|
| 1 | `=` `+=` `-=` `*=` `/=` | right |
| 2 | `\|>` | left |
| 3 | `->` | right |
| 4 | `? :` | right |
| 5 | `\|\|` | left |
| 6 | `&&` | left |
| 7 | `==` `!=` `<` `<=` `>` `>=` | left |
| 8 | `\|` | left |
| 9 | `&` | left |
| 10 | `..` | left |
| 11 | `<<` `>>` | left |
| 12 | `+` `-` | left |
| 13 | `*` `/` `%` | left |
| 14 | `**` | **right** |
| 15 | prefix `!` `~` `-` | — |
| 16 | `(` call, `[` index, `.` access | left |
| 17 | `is` `as` | left |

Three of these are worth explaining, because each was wrong before and each is easy to
get wrong again.

**`&&` binds tighter than `||`.** They shared one level, so `false && false || true`
grouped as `false && (false || true)` and evaluated to `false` where every other language
gives `true`. A silent wrong answer with no error attached.

**Bitwise binds tighter than comparison** (levels 7–9). This is Python's ordering and the
inverse of C's, and it is easy to write down backwards. `6 & 3 == 3` groups as
`(6 & 3) == 3`. Under C's ordering it groups as `6 & (3 == 3)`, which in Aria is a type
error rather than a subtle bug — but a type error on valid-looking code is still bad.

**`**` is right-associative and outranks a prefix operator on its left**, but its own
right operand may still be a prefix expression. This is Python's rule:

```
-2 ** 2   ->  -(2 ** 2)  =  -4
2 ** -1   ->  2 ** (-1)  =   0
```

Precedence alone cannot express that, so `prefixBindingPower` sits just below `**`
rather than above it.

## The resolver

Between parsing and evaluation, a pass binds every name to its declaration. It exists for
three reasons.

**Shadowing works by construction.** Declaring looks only at the *current* scope, so an
outer binding of the same name is hidden rather than being a redeclaration. Walking parent
scopes to check for redeclaration is what made `let x` inside a function fail whenever any
enclosing scope had an `x`.

**Mutability belongs to the binding, not the name.** A `Binding` carries whether it may be
rebound. Tracking it in one process-wide map keyed by bare name meant a `let counter`
inside one function stopped an unrelated top-level `var counter` from being reassigned,
for the rest of the process.

**Names are checked before anything runs.** "Identifier not found" moved from a runtime
surprise to a diagnostic. Running the resolver over the standard library immediately found
two functions that had never been callable — `Enum.random` called a `rand` that does not
exist, and `Enum.reduce` annotated its array parameter as `Function`.

Parameters and loop variables bind like `let`. That matters for immutability below: a
function that could rebind its parameter could mutate a collection its caller owns.

The resolver records a slot index and scope depth for every binding. The evaluator does
not use them yet — it walks a scope chain of name maps — but they are there for when
lookup becomes worth optimising.

## Values

### Display and equality are different things

`Equal` and `KeyOf` decide meaning. `String` and `Inspect` decide presentation. The old
runtime used one `Inspect` method as both, so dictionary lookup, switch matching and array
comparison all compared formatted strings. That made `1` and `"1"` the same dictionary
key, made lookup O(n), and made which of two colliding keys won depend on Go's map
iteration order — the characterization suite was nondeterministic for exactly this reason.

`String` renders a value directly; `Inspect` renders it as it appears inside a collection,
where a string needs its quotes to stay distinguishable from an atom:

```
println("hi")     // hi
println(["hi"])   // ["hi"]
```

An empty array prints `[]` and an empty dictionary `[=>]`, so the two are not confusable.

### Collections are immutable

Every operation on an array or dictionary returns a new value. Nothing is mutated in
place, ever.

This is why `a + [x]` cannot corrupt `a`, and why merging two dictionaries cannot modify
either operand — both of which the old runtime did, the first by growing into a shared
backing array's spare capacity and the second by folding the left operand into the right
and returning it.

At the language level, `x[] = v` and `x[i] = v` are **rebinding**, not mutation. They
require a `var`, and the resolver enforces that at compile time. One rule, and it is the
rule that already existed for names.

## Numbers

**`Int / Int` is `Int`, truncating toward zero.** Operand *types* decide the result type,
never operand values. The old rule returned an `Int` when the division happened to come
out exact and a `Float` otherwise, which meant a function declared `-> Int` could fail at
runtime for inputs its author never tried. `2 ** -1` truncates to `0` for the same reason
`1 / 2` does.

Real division needs a `Float` operand: `10 / 4.0`.

**Floats print without fixed decimals**, and keep a `.0` when integral so a `Float` stays
visibly a `Float`: `1.5`, `3.0`, `1e-05`. The old `%f` turned `1.5` into `1.500000` and
`1e-5` into `0.000010`, losing the value entirely.

## Strings index by rune

`"héllo"[1]` is `é`. The old runtime was inconsistent about this: anything built on
iteration decoded runes correctly while raw subscripting indexed bytes, so
`String.reverse` produced mojibake for non-ASCII input while `String.count` was right.

## Errors

**Parse errors accumulate; the first runtime error halts.**

A parse error leaves the rest of the file perfectly parseable, so seeing all of them in
one run is useful. A runtime error means the program state is already wrong, so continuing
produces messages about the confusion rather than about the problem.

The evaluator unwinds with a Go panic caught at the top of `Run`. That is what makes
halting exact and keeps every eval path free of error plumbing.

Diagnostics are values held by whoever is compiling — not package-level state. That lets
two compilations run at once, lets tests run in parallel without leaking errors into each
other, and makes the packages usable as a library. They render with the offending line and
a caret:

```
main.ari:3:11: error: unterminated string
  let s = "oops
          ^
```

A failed run exits non-zero. The original exited `0` on every parse and runtime error
alike, which made failure undetectable from a script.

### A runtime error carries the file its span indexes into

A `source.Span` is a pair of byte offsets and nothing else, so it only means something
against the file it was taken from. One program routinely evaluates code from three: the
file being run, whatever it imports, and the standard library. Rendering every span
against the file being run drew the importer's text at the importee's offsets — an
unrelated line with a caret under nothing — and for the standard library the offset
usually landed past the end of the user's file, which produced a line number that did not
exist and no source line at all.

So `interp.Error` carries a `*source.File` alongside its span, and the interpreter keeps a
stack of call frames. A `*Function` records the file its body was written in; calling it
pushes a frame naming that file, and `fail` locates the error in the innermost frame's
file. Errors reported *at* a call — arity, parameter types, return type — are faults in
the caller, so they go through `failIn` with the caller's file explicitly.

### Recursion is bounded

The parser bounds nesting at 250 so that deep input fails with a diagnostic instead of
exhausting the stack. The evaluator now does the same at `maxCallDepth`, 3000 frames: past
that a call fails as an ordinary Aria error. Without it, runaway recursion grew the
goroutine stack to Go's 1 GB ceiling and killed the process with a traceback, which is not
something the author of an Aria program can act on.

## The characterization suite

`testdata/semantics/` holds 132 cases. Each `.ari` file has a `.out` golden recording
exactly what the interpreter prints and what it exits with.

```bash
scripts/characterize.sh verify    # diff current behavior against the goldens
scripts/characterize.sh record    # regenerate them
```

This is the regression suite for language behavior, and it is how the rewrite was carried
out: the goldens were first recorded against the original interpreter, and every single
difference the new pipeline produced had to be explained before it was accepted.

Two things about the runner are deliberate:

**It runs each case seven times** and marks any case whose output varies as
`NONDETERMINISTIC`. That is how the dictionary-key bug was found — nobody was looking for
it. No case is marked today.

**Nondeterminism is sticky on re-record.** Detection is sampled, so a case known to vary
can come back stable on any given run; overwriting the marker then would silently turn a
documented variation into a fixed expectation. Record merges into an existing marker
instead.

`ARIA_BIN` points the runner at a different binary, which is how two implementations get
compared case by case. With it unset the runner uses a binary in the repo root if there is
one, and otherwise builds a throwaway — so the suite runs from a clean checkout with no
setup, and leaves nothing behind.

## Testing

- Unit tests per package.
- `FuzzScan` and `FuzzParse` assert the structural invariants — termination, spans in
  bounds, no nil nodes, and that rendering a diagnostic never panics. Both found real
  bugs, one of them within a second of first being run.
- The characterization suite covers language behavior end to end.
- `scripts/check-readme.sh` runs every README code block, since those are claims about
  what the language does. It found a precedence table describing the pre-rewrite
  ordering, a dictionary literal in a replaced syntax, and two examples that had never
  worked.

The two fuzz targets are cheap to run and worth running after any change to the front end:

```bash
go test ./internal/scanner/ -run=Fuzz -fuzz=FuzzScan -fuzztime=30s
go test ./internal/parser/  -run=Fuzz -fuzz=FuzzParse -fuzztime=30s
```

## Known gaps

- **Import resolution.** The resolver cannot see into an imported file, so a program that
  imports anything has undefined-name checking disabled for the whole file. Resolving
  imports would remove that exception.
- **Slot-based scopes.** The resolver computes slot indices and scope sizes that the
  evaluator ignores, walking a chain of name maps instead. Switching would make lookup an
  array index.
- **Empty bodies.** `if`/`for`/`func` with an empty body are accepted and evaluate to
  `nil`. The original rejected them. Nothing decided this either way; it fell out of
  blocks being ordinary node lists.
