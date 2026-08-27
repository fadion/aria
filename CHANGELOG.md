# Changelog

Notable changes per release. [docs/compatibility.md](docs/compatibility.md) says
what a version number promises; the short version is that from 1.0 on, a working
program keeps working, and a breaking change costs a major.

**Breaking** marks a change that can stop an existing program from running or
change what it computes. Diagnostic wording is not covered by the promise and is
not tracked here.

## 1.0.0

The first release with a compatibility promise attached. Everything below landed
between 0.6.0 and 1.0.0.

### Language

- **Records.** `record Point x: Int end` gives structured data a type. Fields are
  a parameter list, so constructing one is an ordinary call with the same arity
  check, type hints and defaults. `typeof` answers the record's own name, and two
  records with the same fields are still different types.
- **Recoverable errors, in two shapes.** Tagged `[:ok, v]` / `[:error, why]`
  results for outcomes a caller should expect, and `try`/`rescue` for faults,
  including ones the runtime raised. The library draws the line at data versus
  misuse.
- **Destructuring.** `let [a, b] = pair`, with `_` as a hole, `...name` for the
  rest, and nesting. In a `switch` arm, `let name` captures what matched.
- **`switch` guards, type cases and range cases.** `case 1..9 when n % 2 == 0`,
  and `case is Int`.
- **Files, arguments, environment and a clock.** `File`, `OS` and `Time`, where
  `prompt` had been the only way out of the process.
- **Imports are one compilation.** An imported file is resolved with the file that
  imports it, so a mistake in one is reported before anything runs, in the file
  it was made in. `import "f" as Name` namespaces what it brings in, and cycles
  terminate.
- **String interpolation** with `#{}`, raw strings in backticks, and `\u{...}`
  escapes.
- **`while`, `until`, and `break N`.** A `for` collects a value per iteration;
  these are for when that array is not wanted.
- **`??`, `?.`, and `do ... end` as an expression.**
- **Lazy ranges in a `for`, and slicing** with `a[1..3]`.
- **Line continuation.** An expression spans lines when a line ends with an
  operator or begins with one, and a parenthesised parameter list can span lines.
- **Top-level functions hoist**, so two of them can call each other without a
  module.
- **Modules are values**, and `.` is an operator over expressions rather than a
  form over two names.
- **`|>` takes a bare name**, and `_` marks the slot when the piped value does
  not belong first.
- **Type annotations are checked before the program runs**, including a default
  against its own parameter's hint.

### Breaking

- **Integer arithmetic fails rather than wraps.** An operation whose result does
  not fit in a 64-bit signed integer is an error, not a plausible-looking
  negative number. `Float % 0.0` raises.
- **Collections have no order**, so `<`, `<=`, `>` and `>=` are no longer defined
  on arrays and dictionaries. Compare sizes explicitly with `Enum.size` or
  `Dict.size`.
- **An atom keys as the string of its text**, so `:a` and `"a"` are the same
  dictionary key and compare equal.
- **A comma-separated `case` list is alternatives**, and a pattern is an array
  literal. The same syntax used to mean both, chosen by the runtime type of the
  subject.
- **An empty body is an error** in `if`, `else`, `for`, `while`, **`until`**,
  `func`, `do`, `try`, `rescue` and a `switch` arm. A comment is not a node, so a
  body holding only a comment is empty; write `nil` for a deliberate no-op.
  `module` and `record` still take an empty body.
- **`Record` names any record.** It was an accepted type name that no value
  satisfied, so a hint written with it resolved and then rejected every argument.
- **A module or record in an aliased import is an error.** An alias makes the
  unit's scope a module, and a module body holds only `let`, so there was nowhere
  for the declaration to go and it went nowhere silently. Import such a file
  without `as`.
- **`String.first("")` and `String.last("")` answer `nil`** instead of failing
  with the interpreter's own return-type error. They match `Enum.first` and
  `Enum.last` now.
- **Stray control keywords are rejected**, along with `_` out of position and
  too many `for` loop variables.
- **A usage mistake exits 2.** An unknown command or flag printed help and exited
  `0`, so a typo in a script read as success.

### Standard library

- `Result`: `ok`, `error`, `ok?`, `error?`, `unwrap`, `expect`, `reason`,
  `attempt`.
- `File`, `OS` and `Time`.
- `String.code` and `String.fromCode`, between a character and its codepoint.
- Filled the `String`, `Math`, `Enum` and `Dict` coverage gaps; `Enum.sort`
  orders with the language's own `<`, and `sortBy` takes a key function.
- Fixed `String.trim`, `String.join` on non-strings, and `Math.floor`/`ceil` on
  negatives.
- The runtime gained the primitives the library had been reimplementing in Aria,
  which were quadratic inside the loops that called them.

### Tooling

- `aria check` parses and resolves without running, for CI or an editor.
- `aria -e` evaluates a one-liner, and `aria run -` reads standard input.
- The REPL reads whole constructs rather than lines, with `:load`, `:vars`,
  `:modules` and `:help`.
- A runtime error is located in the file its span indexes into, which had been
  the importing file.

### Documentation and testing

- [docs/compatibility.md](docs/compatibility.md), [docs/architecture.md](docs/architecture.md),
  [SECURITY.md](SECURITY.md), and twenty runnable [examples](examples/).
- 226 characterization cases, 141 executed README code blocks, and 20 examples
  with goldens, all run in CI on three platforms alongside the fuzzers.

## 0.6.0 and earlier

Not tracked here. 0.x broke things freely, which is what 0.x is for.

The tags through 0.5.0 carry no `v` prefix, and the module proxy does not
recognise those, so `go install github.com/fadion/aria@latest` resolved to a 2017
pseudo-version, the newest commit with no version attached to it, until v0.6.0
became the first prefixed tag.
