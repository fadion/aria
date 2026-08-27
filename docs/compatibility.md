# Compatibility

What a version number promises, and what it does not.

From 1.0 on, the short version is: **a working program keeps working.** If it ran on 1.0
and used only what the README documents, it runs the same way on every 1.x after it. The
rest of this file is the fine print on what "working", "the same way" and "documents" mean,
because each of them has an edge worth naming before somebody finds it the hard way.

## Version numbers

Releases are tagged `vX.Y.Z`. The `v` is not decoration: Go's module proxy only recognises
a version tagged that way.

| | |
|---|---|
| **Major** | A program that ran on the previous release may stop running, or run differently. |
| **Minor** | New things. Everything that worked still works. |
| **Patch** | Fixes. Nothing new, nothing removed. |

Before 1.0, minor releases broke things freely, which is what 0.x is for. Rejecting empty
bodies, removing `<` on collections and making integer overflow an error all landed that
way. After 1.0 each of those would have needed a major.

## What is covered

The promise applies to a program that runs without error and uses only documented features.
For such a program, 1.x guarantees:

1. **Syntax.** Anything the README describes keeps parsing, and keeps meaning what it meant.
2. **Semantics.** What the program computes, and what it prints when it prints a value.
3. **The standard library.** Documented functions keep their names, their parameters and
   what they answer, tagged results included.
4. **Exit status.** `0` when the program ran to completion, `1` when it did not, and `2`
   for a usage mistake such as an unknown command or an unknown flag.
5. **The command line.** `run`, `check`, `repl`, `-e` and `-` keep working and keep meaning
   what they mean.

Two of those are narrower than they look, so they get their own notes below.

## What is not covered

### Diagnostic wording and position

The text of an error is not part of the promise, nor the line and column it points at.

This one matters most, because the characterization suite records every message exactly and
a diff makes a reworded diagnostic look identical to a behavior change. The suite is a
regression-detector for the people working on the interpreter, not a contract with the
people using it. If message wording were covered, improving a confusing error would need a
major release, which is a good way to guarantee the errors stay confusing.

Do not match on message text. `check`'s exit status tells you whether a file is good.

### Which error a broken program gets

A program that already fails may fail differently: a different message, a different
position, or at a different stage. Moving a check from the evaluator into the resolver
turns a runtime failure into one reported before the program runs, which is an improvement
that changes when the failure arrives and what came out on stdout first.

### Programs that are errors today

Making something legal that used to be rejected is additive, and lands in a minor. Nothing
that worked stops working, because the thing in question did not work.

That covers every syntax decision currently deferred. Nullable and union type
annotations, record and dictionary destructuring by name, and named call arguments are each
a parse or resolve error today, so none of them needs a major to arrive.

### Dictionary order

Dictionaries have no order. `Dict.keys`, `Dict.values`, `Dict.toPairs` and iteration in a
`for` may answer in a different order between two runs of the same binary, let alone
between releases. A program that depends on it is already broken; nothing here will fix it,
and a release may change it.

### `Math.random`

The sequence. It is not seeded reproducibly and is not meant to be.

### Limits

Parse nesting and call depth are bounded so that pathological input fails with a
diagnostic rather than a stack overflow. The exact numbers may move, in either direction.

### Anything under `internal/`

Go enforces this, since those packages cannot be imported from outside the module, and
that is also the point. The scanner, parser, resolver, evaluator and value types are implementation.
There is no Go API promise, and the pkg.go.dev listing is not one.

### Performance

Speed, allocation counts and memory use are not covered. A program may get faster or slower
in any release. Aria is a tree-walking interpreter and was never trying to be fast.

### The REPL transcript

The banner, the prompts, and what `:vars`, `:modules` and `:help` print. The REPL is for a
person at a terminal, not for a script to drive.

## Why this is enforceable

Most projects cannot make a compatibility promise honestly, because they have no way to
notice breaking it. Aria can:

- **226 characterization cases** recording exactly what the interpreter prints and the code
  it exits with, run seven times each to catch anything that varies.
- **141 README code blocks**, executed, because the documentation is where the promise is
  written down and an example that stopped working is a broken promise.
- **20 examples**, each with a golden.

A change that breaks a documented program cannot land quietly. It shows up as a diff, and
the rule is already that an unexplainable diff is a bug.

The flip side is the reason the diagnostics carve-out exists: the suite also pins message
text, so **a moved golden is not by itself a compatibility break.** Deciding which it is
still takes a person reading the diff.

## Removing something

Deprecate in a minor, remove in the next major, and say so in both sets of release notes. A
deprecated feature keeps working for the whole of the major it was deprecated in.

## If a working program stops working

That is a bug, whether or not this file technically allowed it. Open an issue with the
program and the two versions. A promise nobody can report against is not worth much.
