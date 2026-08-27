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

## Records: structured data has a home

Anything shaped like a domain object had none. A module holds only `let` and cannot be
instantiated — the README says so — and a dictionary carries data but no identity, so
`typeof` reported `Dictionary` for every one of them and `is` could not tell a point from a
config. Structured data was untyped dictionaries, checked by hand.

**A record's fields are a parameter list**, so constructing one is an ordinary call:
`Point(1, 2)`, with the same arity check, the same type hints and the same defaults every
other call has. That is why a record needs no construction syntax of its own, and a field
needs no syntax that a parameter did not already have.

*Named construction is deliberately not here.* `Point(x: 1, y: 2)` would be a change to
every call in the language, not a record feature, and it is worth making once for all calls
rather than only where records happen to need it.

**Identity is the point.** `typeof` reports the record's own name and `is` tests against it,
so `p is Point` is true and `p is Dictionary` is false. Two records with the same field
values are still different types: `Point(1, 2) == Size(1, 2)` is false, which is what having
a type is for. `value.TypeName` is the one place that answers this, and everything that
compares a type name goes through it — `typeof`, `is`, parameter hints, return hints, case
types, the reassignment type lock.

**Update returns a new record.** `p.x = 5` rebinds `p`, which is the reading `a[0] = v`
already had under the frozen-collections rule, so a record is not a new kind of thing here.
It nests through records and dictionaries alike, and a `let`-bound record cannot be written
through for the same reason a `let`-bound array cannot. Field assignment on a dictionary
falls out of the same code, and works now where it did not before.

**A missing field is a runtime error naming the record**, and an unknown field on a
declaration is caught where it is written. Records are hoisted like modules, so the order of
two declarations does not matter.

**Destructuring a record by field is not here.** It needs the same spelling dictionary
destructuring needs — binding by name rather than by position — and both are worth deciding
together. Field access plus `case is Point` covers matching in the meantime.

## `let` marks binding, everywhere

`for k, v` was the only multi-bind in the language: there was no way to take an array apart
by shape, in a binding or in a pattern.

`let [a, b] = pair` destructures, with `_` as a hole, `...name` for the rest, and nesting to
any depth. Elements after the rest count back from the end, so `[first, ...middle, last]`
works whichever way the array is long. Without a `...`, the shape has to match exactly: a
pattern that does not fit is a mistake, not a silent partial bind.

In a `switch` arm, `let name` captures what matched. That is the question the old design
sidestepped — in `case [:ok, value]`, is `value` a binding or a reference? The rule is that
a bare identifier is still a reference compared with the control, which is what it has
always been, and `let` says the other thing, using the keyword that already means exactly
that.

The alternative — a bare undeclared identifier binds — was available and is what Elixir
does, but it makes the meaning of an arm depend on what happens to be in scope: the same
arm binds in one file and compares in another, and a typo silently becomes a binding that
matches everything. `let` at the front of a binding statement and `let` on a name in a case
are then the same rule said twice, rather than two rules.

A capture is bound into a scope belonging to its arm, which the guard and the body share.
Matching writes into it as it goes, so an arm that fails to match leaves nothing behind.

**Dictionary destructuring is deliberately not here.** It needs a spelling for binding by
key — `let [:host => h] = cfg` is the obvious candidate — and that is a second syntax
decision worth making with a second use for it, rather than guessed at alongside the first.

## A case list is alternatives; a pattern is an array literal

`case 1, 2` used to mean "1 or 2" for a scalar control and "the array `[1, 2]`" for an
array control — one syntax with two meanings, chosen by the runtime type of the subject.
Both were documented in the README as features.

A comma-separated list is always a list of alternatives. A pattern is written as an array
literal, `case [1, _]`, which the parser can already tell apart, and `_` inside one is a
wildcard for that element. A pattern of a different arity is simply a different array. A
bare `_` among alternatives is still the match-anything wildcard, which is `default` said
in an arm.

That leaves `switch` able to carry the weight the README gives it — it is the language's
stated answer to the missing `else if` — with three additions:

**`when` guards.** Tested only once one of the arm's values has matched, so the
control-less form replaces an else-if chain without repeating the subject. A guard that
fails falls through to the next arm rather than to `default`.

**`case is Type`.** Aria already has `is` as an operator; this is it where there is no
left-hand side to write, which was the most common reason to reach for a chain of `typeof`
comparisons. The type name is checked at resolve time, so a typo is a diagnostic rather
than an arm that never matches.

**`case a..b`.** Membership, not equality. An `Int` range is tested by comparison, so
`case 1..1000000` costs nothing; anything else materialises, which is what `..` does outside
a case anyway.

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

### Aria has recoverable errors, in two shapes

It used to have none, and that answer was not written down anywhere, which made it read as
an oversight rather than a position. `panic` was terminal, every runtime fault unwound to
the top of `Run`, and no Aria code could be defensive.

The two shapes answer different questions, and the pairing is the usual one.

**A tagged-result convention, for outcomes a caller should expect.** A fallible operation
answers `[:ok, value]` or `[:error, reason]`. This needs no new syntax — `switch` with array
patterns and `let` capture takes one apart — and the `Result` module gives the shape a name
along with the common ways to consume it: `ok?`, `unwrap` with a fallback, `expect`,
`reason`.

**`try ... rescue e ... end`, for genuine faults.** An expression, like everything else in
Aria: it evaluates to whichever block ran.

Every runtime error is catchable, including one the runtime raised itself. The line between
"a fault" and "an expected failure" is the *caller's* to draw, not the runtime's — the same
division by zero is a bug in one program and a validation outcome in another. What is not
catchable is anything that is not a runtime error: a parse or resolve failure means the
program never started. A control signal is not a failure either, so a `return` inside a
`try` unwinds to its function as it would anywhere else.

The rescued value is a dictionary — `:message`, `:file`, `:line`, `:column` — rather than a
string. The message is what a program usually wants, but where it happened is what lets a
rescue report anything useful, and a dictionary is what dotted access already reads well on.

`Result.attempt(fn)` is the bridge: it runs a function and turns whatever it raises into an
error result.

**Where the library draws the line.** `Dict.insert`, `Dict.update` and `Dict.delete` answer
tagged results: a key that is already there, or one that is not, is an ordinary outcome, and
the only way to avoid the old panic was to duplicate the check before every call. Passing
the wrong *kind* of thing — `Math.max("a", 1)` — still raises, because that is a mistake in
the caller rather than an outcome, and `try` is what catches it. Data versus misuse is the
line.

A failure nobody catches still halts the program and exits non-zero. `try` is what makes
recovery possible, not what makes failure quiet.

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

## An empty body is a mistake; an empty container is not

The original rejected an empty body in four places, each with its own check: `if`, the
`else` half of one, `for` and `func`. The rewrite accepted them everywhere, not as a
decision but as a consequence of blocks becoming ordinary node lists — there was no longer
anywhere the emptiness was noticed. `while`, `until`, `do`, `try` and `switch` arms arrived
later and inherited the permissive reading by default.

A body with nothing in it runs nothing. It is an unfinished edit or a stub, and in either
case the program says one thing and means another. For a `for` it is worse than dead code:
the loop collects a value per iteration, so `for i in 1..1000000 end` quietly builds a
million-element array of nils. That is the silent-plausible-result failure this codebase
legislates against elsewhere, and `while` already exists for the case where the array is
not wanted, so the error has an obvious fix to point at.

So every block holding code to run rejects an empty one: `if`, `else`, `for`, `while`,
`until`, `func`, `do`, `try`, `rescue`, and a `switch` arm or `default`. Writing `nil` says
"deliberately nothing" in one more word, and says it where a reader will see it.

A `module` and a `record` are containers rather than bodies, and keep taking an empty one.
`module M end` names a namespace not filled in yet and `record R end` is a unit type whose
instances are all equal — both name something that exists, which an empty body does not.

**A comment-only body counts as empty.** Comments are not nodes, so `if x` with nothing but
`// handle it` in it has an empty block and is rejected. This is the rule's most visible
consequence: it caught three of the README's own placeholder examples. The alternative
would be tracking comment spans per block so the parser could tell "no code" from "nothing
at all", which is machinery for a distinction that helps nobody — `nil` is checkable and a
comment is not.

The check lives in the parser, at each construct's own call to `codeBlock`, so the message
names the construct. It reports to the bag directly rather than through `errorAt`, which
would set `panicking`. That flag suppresses the cascade from a parser that has lost its
place, and this one has not: it knows exactly where it is and carries on correctly, so a
second empty body further down is a separate mistake and is reported too.

Nothing is reported at end of input. A half-typed `if true` in the REPL has an empty block
for the same reason it has no `end` yet, and turning "keep typing" into an error would
break the one thing the incomplete-input signal exists for. The missing-`end` diagnostic is
the one that belongs there.

## The characterization suite

`testdata/semantics/` holds 219 cases. Each `.ari` file has a `.out` golden recording
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

## A program is one compilation, imports included

`import` used to be four problems at once. An imported file was parsed and evaluated and
never resolved, so inside one an undefined name was a runtime error, `let` immutability was
unenforced and every guarantee the resolver provides was absent. A single `import` anywhere
also turned undefined-name checking off for the whole of the *importing* file, because
there was no way to know what the import had brought in — so the resolver's headline benefit
was off for exactly the programs large enough to need it. Names had no namespace and no way
to be renamed, so two files could not both define `size`. And a stray top-level `return` in
an imported file set the signal on a sub-interpreter nobody read, so it neither propagated
nor errored.

All four come from the same thing: an imported file was a separate compilation. It is a unit
of the same one now. A loader parses the whole graph depth first, the resolver walks every
unit into one scope in the order their names have to become visible, and the same
interpreter runs them in that order. Each unit keeps its own `diag.Bag`, so a diagnostic
still renders against its own text.

**An import is a declaration, not an expression.** `if x then import "y" end` used to parse
and import conditionally at runtime, which no pass can see through — and seeing through it
is the whole point. It belongs at the top level of a file.

**`import "geometry" as Geo` namespaces what it brings in.** The unit is resolved and
evaluated in a scope of its own, which then becomes a module. A module IS a namespace, so
this needs no machinery the language does not already have — and an alias is checked like
any other module, members included.

**A cycle terminates and is not an error.** The loader records every file it has pulled in,
and a second import of one is a no-op: its names are already in scope. Two files that import
each other are one compilation either way, so there is nothing to report.

## Reaching the outside world

`prompt` was the only way an Aria program could: no files, no arguments, no environment, no
clock. A program could not read the file it was meant to process, could not be told which
file that was, and could not report how long it took.

`File`, `OS` and `Time` follow the pattern the rest of the library already does — `runtime_*`
builtins for what Aria cannot express, and an Aria surface over them — so the library stays
readable and the Go surface stays small.

**The builtins raise; the library wraps.** `runtime_read_file` fails the way anything else
in the evaluator does, and `File.read` turns that into a tagged result with `try`. That
keeps the convention in Aria, where it is written down, rather than duplicated in Go — and
it is why this came after the error model rather than before it. Adding IO first would have
baked `panic` into the one place it hurts most.

**Recognised file errors get fixed text**, not the operating system's. "no such file or
directory" against "The system cannot find the file specified." is a difference that would
otherwise leak into anything comparing output, this repository's own goldens included.

**Two clocks, because they answer different questions.** `Time.now` is milliseconds since
the Unix epoch, which is what you write down; `Time.monotonic` is nanoseconds from an
arbitrary origin, which is what you subtract. Only the second is safe to subtract, since a
wall clock can move backwards. Both are `Int`s: a `Float` would start losing nanoseconds
somewhere around 1970 plus a decade. Neither is in the characterization suite, which runs
every case seven times and would flag them.

**Sandboxing.** There is none, and that is now worth saying out loud. A program can read and
write any path the process can, `import` reaches any path it is given, and the CLI runs
whatever source it is handed. Aria is a toy language; running untrusted Aria source is
running untrusted code, and nothing here pretends otherwise.

## The tools

`run`, `check`, `repl`, and `-e` for a one-liner. `run -` reads standard input.

`check` is the pipeline stopped after resolution, which is what `Run` already did internally
before evaluating anything. It catches everything the resolver knows — undefined names,
immutable rebinding, unknown type hints, mistyped module members — without the program
having any effect, which is what CI or an editor integration wants.

**The REPL reads constructs, not lines.** A session that takes one line at a time cannot
accept a multi-line `func`, `module`, `if` or `for` at all, which rules out most of what a
REPL is for. `Session.Eval` takes a whole fragment and answers `ErrIncomplete` when more
input could complete it, so the REPL buffers until it parses.

Incompleteness is a flag on the `diag.Bag`, not a string match on the message. The parser
sets it when it reports its first diagnostic while sitting at end of input: that means it
ran out of source with a construct still open, rather than that it found something wrong.
An unterminated raw string and an unterminated block comment set it from the scanner, since
both may span lines; an unterminated `"` string does not, since it may not.

**Nothing prints for a statement that produced nothing.** Half a session used to be `nil`
from `println` calls.

**There is no `fmt`.** A formatter is a separate project rather than a missing subcommand:
it needs a decision about every layout question in the language, and where a line may break
was only settled above. `scanner.ScanComments` stays, because the fuzzer runs the scanner in
both modes and so the mode is covered code rather than speculative code. `token.IsKeyword`
went, because nothing called it and nothing covered it.

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

### `Record` names any record

`Record` was in the accepted set of type names and satisfied by nothing. `TypeName` answers
a record's own name — identity being the point of having records rather than dictionaries —
and every type check compared against that, so `Record` could only ever match a record
literally called `Record`. A hint written with it resolved cleanly and then rejected every
argument it was given.

It now means "any record", which is what `Module` already meant for free, nothing having
overridden its type name. `Point(1) is Record` is true, `Point(1) is Point` is still true,
and `Point(1) is Size` is still false: the category is added without blurring the identities
underneath it.

The three rules live in `value.Satisfies` rather than at each site that asks. There are five
— parameter hints, return hints, `is`, `case is`, and a record field — and every one of them
was special-casing `Any` on its own, so `Record` would have been a second special case in
five places.

The reassignment type lock deliberately does not go through it. That check compares two
values' names directly, because a `var` holding a `Point` must still refuse a `Size` even
though both satisfy `Record`.

### An alias cannot carry a module or a record

`import "file" as Name` works by resolving the unit into a scope of its own and turning that
scope into a module, which is why aliasing needed no machinery the language did not already
have. The catch is that a module body holds only `let`: modules do not nest and a record
cannot live in one, enforced everywhere a module is written by hand.

So a `module` or `record` declared in an aliased file has nowhere to go, and it used to go
nowhere quietly. Both halves of the implementation collected members from `ast.Let` and
`ast.Var` only, so the declaration was dropped by the resolver and by `defineModule` alike.
Neither `Lib.Shapes` nor a bare `Shapes` resolved, and the diagnostic named the alias, which
reads as a typo in the caller rather than as a problem with the import.

Making it work would mean modules that nest and records inside modules, reachable only
through an aliased import and never writable by hand, which is a language change to serve
one corner. The other direction is cheap and honest: a file that declares a module already
namespaces itself, which is the whole job of the alias, so the two are redundant. It is now
an error saying exactly that, reported against the declaration in the file that made it.

### A codepoint and a character, in both directions

Strings index by rune and a literal can name a codepoint with `\u{...}`, but nothing went
between an `Int` and a character, so a computed character was unreachable: ciphers, base
conversion, and any interpreter with an output instruction. `examples/brainfuck.ari` carried
95 characters of printable ASCII as a lookup table to print anything at all.

`String.code` answers the first rune's codepoint, and `nil` for an empty string, the way
`String.first` does. There is no codepoint to give, and an empty string is ordinary data.
`String.fromCode` raises for a number that is not a codepoint, a surrogate included, because
that is the caller's arithmetic being wrong rather than an outcome to handle. Go answers
U+FFFD for both, which would turn the mistake into a replacement character that travels
instead of an error where it happened.

The two builtin lists, `resolver.Builtins` and the evaluator's `def` calls, had nothing
tying them together, and adding these to the evaluator alone made the standard library fail
to resolve with no hint that a list elsewhere was short. `TestBuiltinListsAgree` compares
them now.

## What a release promises

[compatibility.md](compatibility.md) is the user-facing half of this file: what a version
number covers, what it deliberately does not, and why the characterization suite is what
makes the promise enforceable rather than aspirational.

The one worth knowing while working in here: **diagnostic wording and position are not
covered.** The suite records every message exactly, which makes a reworded error and a
behavior change look identical in a diff, and that is a tool for the people writing the
interpreter rather than a contract with the people using it. A golden moving is not by
itself a compatibility break.

## Known gaps

- **Slot-based scopes.** The resolver computes slot indices and scope sizes that the
  evaluator ignores, walking a chain of name maps instead. Switching would make lookup an
  array index.
