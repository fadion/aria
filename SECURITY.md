# Security

## Aria runs untrusted code as you

There is no sandbox, and there is not meant to be one. An Aria program reads and
writes any path the process can, sets and reads environment variables, and
`import` reaches any file it is given a path to. Running an Aria program you did
not write is running a program you did not write, with everything your account
can do.

That is a design position rather than an oversight, and it is the thing to know
before wiring the interpreter into anything that accepts source from elsewhere:
a build step, a plugin system, a web service, a CI job that runs a contributed
script. If you need to run untrusted Aria, put the sandbox around the process
yourself: a container, a jail, a user with nothing to lose. The language will not
provide one.

So the following are **not** vulnerabilities, and a report of one will be closed
with a link to this paragraph:

- An Aria program reading or writing files, including outside its directory.
- An Aria program reading the environment, or the command line.
- `import` reaching a file anywhere on disk.
- An Aria program exhausting memory or CPU. Recursion is bounded and parse
  nesting is bounded, both so that a runaway program fails with a diagnostic
  rather than a stack overflow, but neither is a resource limit.

## What is worth reporting

A way to make the **interpreter itself** misbehave on input that should have been
rejected or handled:

- A crash in Go: a panic, a nil dereference, an index out of range, a stack
  overflow. Aria source should produce an Aria diagnostic, never a Go trace. The
  scanner and parser are fuzzed continuously for this, and a case they missed is
  worth having.
- A hang or an infinite loop in the scanner, parser or resolver on some input.
  Every path through the scanner consumes a byte specifically so that the token
  stream terminates; a counterexample is a real bug.
- Memory that grows without bound while merely reading a program, before it runs.
- Anything in the release pipeline: a published archive whose contents do not
  match its source, or a checksum that does not verify.

## Reporting

Open a [private security advisory](https://github.com/fadion/aria/security/advisories/new),
which reaches the maintainer without making the report public first.

Include the input, what happened, and what you expected. A failing `.ari` file is
the most useful thing you can send, and if a fuzzer found it, the corpus entry is
better still.

This is a toy language maintained by one person in their own time. There is no
response-time commitment, no bounty, and no embargo policy beyond common sense.
What there is: a characterization suite that makes a fix provable, and a genuine
interest in any input that makes the interpreter do something other than
diagnose it.

## Supported versions

The most recent release. There are no maintenance branches, and a fix ships in
the next version rather than being backported.

[docs/compatibility.md](docs/compatibility.md) covers what a version number
promises. Worth noting here: diagnostic wording is explicitly not covered, so a
security fix that changes an error message is not a breaking change.
