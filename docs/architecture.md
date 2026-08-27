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

### When a newline ends an expression

A newline is a statement terminator, which is what lets Aria do without semicolons — and it
used to be a hard one everywhere except inside `[...]` and call parentheses, so a long
expression had to fit on one line. That is worst for pipelines, which is where the pressure
to wrap is highest.

Two rules, and both are needed because the two shapes read differently:

**A line that ends with an operator continues.** The operator has no right side yet, so the
newline cannot be terminating anything. Unambiguous by construction.

**A line that begins with an infix operator continues, unless that operator could also begin
an expression.** `-`, `(` and `[` could — they are negation, a call and a subscript — so a
line starting with one of those is a new statement, exactly as it was. Everything else is
unambiguous: no Aria expression starts with `|>`, `.`, `??` or `+`.

```swift
data
  |> Enum.filter((x) -> x > 0)
  |> Enum.map((x) -> x * 2)
```

Deciding this needs the parser to look past a run of newlines, which is the one place it
reads further than one token ahead. It has to: the decision must be made *before* anything
is consumed, since consuming and then deciding not to continue would eat the separator the
enclosing block needs. The extra tokens are buffered, so the scanner is still read once,
forwards, and the cursor invariant below holds.

**A parenthesised parameter list may span lines**; a bare one may not. Inside `(` a newline
is layout, and outside it is still the end of the signature. The loop used to terminate on a
newline unconditionally, so a wrapped signature was a parse error that blamed the closing
`end`, several lines from the actual problem.

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
| 1 | `=` `+=` `-=` `*=` `/=` `%=` `**=` | right |
| 2 | `\|>` | left |
| 3 | `->` | right |
| 4 | `? :` | right |
| 5 | `\|\|` | left |
| 6 | `??` | left |
| 7 | `&&` | left |
| 8 | `==` `!=` `<` `<=` `>` `>=` | left |
| 9 | `\|` | left |
| 10 | `^` | left |
| 11 | `&` | left |
| 12 | `..` | left |
| 13 | `<<` `>>` | left |
| 14 | `+` `-` | left |
| 15 | `*` `/` `%` | left |
| 16 | `**` | **right** |
| 17 | prefix `!` `~` `-` | — |
| 18 | `(` call, `[` index, `.` and `?.` access | left |
| 19 | `is` `as` | left |

Three of these are worth explaining, because each was wrong before and each is easy to
get wrong again.

**`&&` binds tighter than `||`.** They shared one level, so `false && false || true`
grouped as `false && (false || true)` and evaluated to `false` where every other language
gives `true`. A silent wrong answer with no error attached.

**Bitwise binds tighter than comparison** (levels 8–11). This is Python's ordering and the
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

**Top-level functions hoist.** Every top-level `let` whose value is a function literal is
declared before anything is resolved, so two functions that call each other can both live
at the top level. Wrapping them in a module was the only way to write that, which is a
strange thing to have to do for two free functions.

The line is deliberate on both sides.

*Only function literals.* A hoisted name is in scope during its own initializer by
construction, which would defeat the `pending` check that catches `let x = x`. A function
literal is the one value that can be built without evaluating anything first, so hoisting
it early builds the same value — anything else would mean hoisting whatever has to run to
produce it.

*Only the top level.* A function body opens a scope of its own, and nothing in it hoists.

The evaluator hoists to match, in `Interp.Run` and again for an imported file. Hoisting in
only one of the two would let a name resolve and then not be there at runtime — `eval`'s
Identifier case, the one whose comment says the two passes cannot disagree.

**Type annotations are checked here, not at runtime.** Aria has hints, and almost nothing
used to look at them before the program ran.

`is` was a string comparison against `Type().String()` with the right-hand side never
resolved, so `5 is Banana` was a permanently-false test rather than a typo. The set of type
names is fixed and known, so `is`, `as`, parameter hints and return hints are all checked
against it.

A parameter's default is checked against its own annotation. `checkParamType` ran only on
arguments actually passed, so `func (n: Int = "oops")` bound the default unchecked and the
hint was a lie for every caller that omitted the argument. A literal default is checked
here; anything else is checked when it is evaluated.

Module members are checked for the modules this pass can see — the standard library, whose
members are known before the user's program is parsed, and anything declared in the same
file. `resolver.moduleAccess` used to skip this on the grounds that matching members across
files is the evaluator's job, which is true only for a module in an imported file. Moving
"not found" from runtime to a diagnostic is the resolver's stated reason for existing, and
a mistyped `Enum.sizze` is the same class of mistake as a mistyped variable.

**`Any` is a name a hint may use.** Without it, the way to say "accepts anything" was to say
nothing, so an unannotated parameter was ambiguous between that and "not yet annotated".
`x is Any` is true, `x as Any` is `x`, and `: Any` accepts everything.

**Nullable and union annotations are deliberately not here yet.** A parameter that
legitimately takes `Int` or `nil` still cannot be annotated, which is why most of the
standard library leaves its optional parameters bare. Both need real grammar in the
annotation position — `Int?`, `Int | String` — and that is a type-syntax decision worth
making once, alongside user-defined types, rather than improvised into a bug fix. `Any` is
the honest way to say it in the meantime.

**It also checks what only a tree walk can see.** Three mistakes are knowable before the
program starts, and each was silent or late:

- `break` and `continue` outside a loop, and `return` outside a function. `Interp.Run`
  stops its node loop on any control signal, so a stray `break` at top level discarded the
  rest of the file with no diagnostic and exit code `0`. A function body resets the loop
  count, so a `break` inside a function that happens to be defined in a loop is an error
  too — the call is not the loop's body.
- `_` anywhere but a switch case value or an append target. It mapped to nil in `eval`, so
  `let x = _` was accepted and quietly meant nothing.
- More than two `for` loop variables. The evaluator checked this inside the per-iteration
  closure, so `for a, b, c in []` never reached the check at all.

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

### Equality and keying may not contradict each other

Separate operations, but not independent ones: two values that report equal have to be
interchangeable as dictionary keys. `Equal` said an `Atom` and the `String` of the same
text were equal while `KeyOf` tagged them `TAtom` and `TString`, so `d[:a]` and `d["a"]`
reached different entries in a dictionary whose keys the language claimed were equal.

An `Atom` now keys as the `String` of its text. The other direction — making `:a != "a"`,
as Elixir does — was available and would also have been coherent, but the README already
says atoms "can generally be interchanged" and shows a dictionary written with atoms being
read with a string. Interchangeable is what the language claims, so keying is what moved.
Only the key is shared; the dictionary stores the key value it was given, so a dictionary
written with atoms still prints with atoms.

### Sorting orders with `<`

There was no sort anywhere in the language: `value.SortedKeys` existed in Go and was
unexported to Aria, so a program had no way to order a collection at all.

`Enum.sort` and `Enum.sortBy` order with the language's own `<` — numbers among numbers,
text among text. A pair `<` cannot compare is an error, naming the two types, rather than
an invented total order across every type. Ranking `Int` against `String` would be the same
kind of meaning-nobody-would-guess that got `<` removed from collections below.

`sortBy` takes a key function rather than a comparator. A comparator has to answer in three
values, and the language has no convention for that; a key composes with the same `<`.

### Collections have no order

`<` and `>` on an `Array` or `Dictionary` compared lengths, and `<=` and `>=` were not
defined on them at all — four comparison operators meaning two different things on one
type. The meaning they had is also not the one they read as: `[1, 2, 3] < [9]` was false,
because it asked whether 3 < 1.

All four are now undefined on collections. Defining the missing two the same way would
have made the set consistent at the cost of keeping an operator whose meaning nobody
reading it would guess, and would foreclose a real element-wise ordering later. Length has
a spelling that says what it is: `Enum.size(a) < Enum.size(b)`.

### A module is a value, and `.` is an operator

Modules used to live outside the value system, in a registry on the interpreter rather
than in any scope. The resolver let a bare module name through as "not a binding" and the
evaluator then failed on it, so `let E = Enum` reported `'Enum' is not defined` — with the
two passes disagreeing about scoping in exactly the place `eval` claims they cannot.

A module is now bound in the scope it is declared in, like anything else. `typeof(Enum)`
is `Module`, and a module can be bound, passed and returned.

`.` was a special form over two identifiers, which is why it never chained: `cfg.db.host`
had `cfg.db` on its left and was rejected as a module name, and so were `f().a` and
`a[0].k`. It is an operator over an arbitrary expression now — evaluate the left side,
then dispatch on what it is — which *removes* the two-branch special case in
`evalModuleAccess` rather than adding a feature.

That surfaced a collision the two namespaces had been hiding: `String` is both a
conversion builtin and a standard library module, and as a value the name has to mean one
thing. It means both. A module that shadows a builtin of its own name carries it, so
`String("x")` converts, `String.join(...)` reads a member, and `let S = String` gets a
value that still does both.

### `|>` takes a bare name, and `_` marks the slot

The right side of a pipe used to have to be a function call, and the piped value always
landed first. So `4 |> double()` worked while `4 |> double` did not — an empty argument
list on a function that takes an argument reads as a mistake — and anything whose subject
is not its first parameter fell out of a pipeline entirely.

A right side that is not a call is applied to the piped value. A `_` among a piped call's
arguments marks the slot the piped value goes into; the resolver rejects a second one,
since it would have nothing to receive. That is the third position where `_` means
something, alongside a switch case value and an append target.

The argument list is still built at evaluation, not by rewriting the tree. The original
prepended in place, so a piped call inside a loop grew its own argument list on every
iteration.

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

**Integer arithmetic fails rather than wraps.** `+`, `-`, `*`, `**`, `<<`, `/` and unary
`-` raise a runtime error when the result does not fit in an `Int`, instead of silently
landing on a wrapped value.

Aria's `Int` is one fixed-width signed integer. There is no unsigned counterpart, no way to
name the width, and no operator that means "wrap deliberately" — so a program has no way to
observe or intend a wrap, and a wrapped result is simply a wrong answer that looks like a
computed one. That is the failure mode this codebase keeps removing: a plausible value that
surfaces a long way from its cause. The cost is one comparison per operation, which a
tree-walking interpreter does not notice.

`**` follows from the same rule as `/`: it is computed in integer arithmetic, by squaring.
Routing it through `math.Pow` lost precision above 2^53 and then converted out of range,
which on amd64 produced `MinInt64` — so `3 ** 40` was a negative number.

**`floor`, `ceil`, `round` and `trunc` answer with an `Int`, and raise when the exact
answer does not fit in one.** The result type comes from what the function is for, not from
the operand's value, which is the division rule again: `Math.floor(2.5)` is `2`, an `Int`,
because an index or a count is what a floor is usually wanted for. What can depend on the
value is whether the answer exists — `Math.floor(1e300)` has no `Int` answer, exactly as
`9223372036854775807 + 1` has none, and both raise rather than land on `MinInt64`.

Returning a `Float` instead, as Go's `math.Floor` does, would also have been representable
everywhere; it was not taken because it makes the common use — `a[Math.floor(x)]` — need a
conversion at every call site to avoid a type the caller never wanted.

**A shift count of 64 or more is an error**, rather than Go's answer of shifting every bit
out and yielding `0`. `1 << 100` is not `0`; it is a question about a 64-bit integer that
has no answer.

**Floats follow IEEE 754** and are left alone: overflow reaches `Inf`, which is a value the
format defines. The one exception is `Float % 0.0`, which returned `NaN` while `Int / 0`,
`Int % 0` and `Float / 0.0` all raised. A `NaN` propagates through arithmetic and compares
false against everything including itself, so it is the same "surfaces far from its cause"
shape. It raises now, like its siblings.

## What the standard library asks the runtime for

The library is written in Aria, and that is the point — but a function whose answer the
runtime already has should ask for it rather than compute it again in interpreted code.

`String.count` and `Enum.size` were interpreted loops over data whose length the runtime
knows. `String.slice` walked the whole string to take a window out of it. Both were then
called from inside per-character loops in `split`, `replace`, `contains?`, `starts?`,
`ends?` and `trim` — `replace` called `slice` twice per character — so each of those was
quadratic or worse. On 7 KB of text, `String.split` took 11 seconds; doubling the input to
14 KB took 40.

So `runtime_len`, `runtime_slice`, `runtime_index_of`, `runtime_last_index_of`,
`runtime_split`, `runtime_replace`, `runtime_join`, `runtime_repeat` and `runtime_reverse`
are builtins, and the library functions are one call each. Everything stays rune-indexed,
matching the subscript rule. Same input: 11 seconds becomes 20 milliseconds, and 144 KB —
which did not finish in ten minutes — takes 84.

Accumulation was the other half of it. `sliced += v` on an immutable string allocates a
whole new string per character, so building a result a character at a time is quadratic on
its own. `join`, `repeat` and `reverse` build in Go now.

**Still quadratic: appending to an array in a loop.** `Array.Append` copies, deliberately —
growing in place is what let a later append write into an earlier result's spare capacity —
so `out[] = v` inside a loop is O(n²). Every accumulate-into-an-array function in the
library has that shape. Fixing it needs a way to know an array's tail is unshared, which is
a change to the value representation rather than to the library.

## String literals

Two forms. `"..."` processes escapes and may not span lines; `` `...` `` processes none and
may. The raw form is one answer to two gaps: there was no way to write a string over more
than one line, and every backslash in a regex passed to `String.match?` had to be doubled.
Carriage returns are dropped from a raw literal, as Go does, so the same source means the
same string on a checkout with CRLF line endings.

`"..."` also interpolates: `#{expr}` holds a whole expression, and the value renders the
way `println` renders it — `String`, not `Inspect` — so `"#{[1, 2]}"` is `[1, 2]` and a
string in a hole does not come out quoted. `#` is only special before a `{`, and `\#` opts
out. A raw literal processes nothing, interpolation included.

The scanner hands an interpolated literal over as pieces — `StringStart`, the hole's own
tokens, `StringPart`..., `StringEnd` — so the expressions parse with the grammar the
parser already has and their spans point at where they are written, which is what a
diagnostic inside a hole needs. Aria has no other use for braces, so a `}` while an
interpolation is open always closes it; nesting needs a counter and nothing more.

It lands in the tree as `ast.Interpolation` rather than desugaring to `+` with `String(...)`
calls around each hole. Two reasons: `+` does not coerce and the `String` conversion
refuses a collection, so a desugaring would make `"#{[1, 2]}"` an error where `println`
prints it; and naming `String` in a desugaring means anything shadowing that name silently
changes what every interpolated string in scope does.

`\xNN`, `\uNNNN` and `\u{N...}` write a rune by codepoint, which for a language that
indexes strings by rune was a conspicuous thing to be missing. **`\xNN` is a codepoint, not
a byte**, even though two hex digits can only reach 0xFF: Aria strings are UTF-8 and index
by rune, so a raw high byte would be a way to build a string the rest of the language
cannot read. The scanner validates the digits and the codepoint where they are written, so
a bad escape is a diagnostic rather than a surprising string later.

## Strings index by rune

`"héllo"[1]` is `é`. The old runtime was inconsistent about this: anything built on
iteration decoded runes correctly while raw subscripting indexed bytes, so
`String.reverse` produced mojibake for non-ASCII input while `String.count` was right.

## Loops cost what they use

`for` is an expression: it evaluates to an array of every iteration's value. `loop` built
that array unconditionally, whether anything read it or not, so a two-million-iteration
side-effect loop peaked at 98.9 MB of results discarded the moment it ended.

**A `for` whose value nobody reads collects nothing.** The parser knows: a `for` that is
not the last node of its block cannot be that block's value. The last node is left alone,
because a block evaluates to it and its own caller may well want it. Same loop, 14.4 MB.

**`while` and `until` evaluate to `nil`.** There is no per-iteration value worth
collecting, which is the whole reason they exist alongside `for`: the infinite `for` plus
`break` was the substitute, and that is exactly the shape with the memory problem. `until`
is `while` with its condition negated — one node, one flag — so the two share every rule.

**`break N` leaves N enclosing loops**, and plain `break` is `break 1`. Breaking out of two
loops needed a flag variable, which needs a `var`, which fights the immutability the README
spends a section on. A count needs no new token, where a label would; it reads poorly past
two, but two is the case that comes up. The resolver counts the loops a `break` is inside,
so `break 3` inside two is a diagnostic rather than an unwind out of the enclosing function.

## Nil is threaded, not tested for truth

Reading a missing array index or dictionary key yields `nil` rather than failing, so
threading nil is an ordinary thing to be doing in Aria — and there was nothing to do it
with. `||` cannot substitute because it coerces: `0 || 5` is `true`, not `0`.

`a ?? b` yields `a` unless it is `nil`, and short-circuits, so the default is not evaluated
when the left side is there. It tests for `nil` specifically rather than for truthiness,
which is the whole point of having it alongside `||`.

`a?.b` yields `nil` as soon as a link is `nil`, and answers `nil` for a missing key too —
subscripting already answers `nil` for the same read, and `?.` is how a caller says it
expects one.

The scanner absorbs a trailing `?` into a name, since `empty?` is one token — but not when
a dot follows, or `cfg?.db` would read as a name called `cfg?`. That makes `empty?.x`
unwritable as a member access on the result of `empty?`; parenthesise it. That is the side
of the ambiguity worth losing, since the other side is every safe navigation there is.

**`do ... end` is an expression** with its own scope, yielding its last value. Everything in
Aria is expression-valued except a block, which was the one place the claim was not true.
`ast.Block` and `evalBlock` already did exactly this; it needed a prefix parse and nothing
else.

## Ranges and slicing

`..` builds an array. It is a value like any other, so `Array(1..5)`, `Enum.map(1..5, f)`
and `println(1..5)` all work the way they always have.

**Except in the two positions where the endpoints are bounds rather than data.** A range
written directly as a `for` enumerable counts, and one written directly as a subscript
index slices. Neither materialises anything: `for i in 1..10000000` used to build ten
million `value.Int` boxes before the first iteration ran, for a loop that needs one at a
time.

Doing it at those two syntactic positions, rather than by introducing a `Range` value, is
the conservative half of the change. A `Range` type would be more elegant and would make
`let r = 1..1000000` cheap too, but it would have to behave like an `Array` in every place
one is accepted — `typeof`, `is`, `+`, `==`, indexing, `Enum`, `Dict`, display — and each
one it did not reach would be a regression. Nothing here changes what `..` means.

The endpoints are evaluated exactly once either way. A range whose ends are not both `Int`s
falls through to the ordinary array value, built from the operands already evaluated rather
than by evaluating them again.

**A slice is inclusive**, because `..` is inclusive everywhere else in Aria: `a[1..3]` is
three elements. Negative endpoints count from the end, as a scalar index does. A descending
range gives the elements in that order, which is what the range itself would have held.

**A slice clamps.** Reading a scalar index out of bounds already yields `nil` rather than
raising, so a range that half-overlaps the collection gives the overlapping part and one
that misses entirely gives nothing.

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

`testdata/semantics/` holds 189 cases. Each `.ari` file has a `.out` golden recording
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
