# Examples

Twenty programs, each one complete and runnable:

```bash
aria run examples/quicksort.ari
```

Every one has a `.out` golden beside it recording exactly what it prints and the
code it exits with, and `scripts/check-examples.sh` runs the lot against them. An
example that has stopped working teaches the wrong thing, so they are checked the
same way the README's code blocks are.

| | |
|---|---|
| **Algorithms** | [quicksort](quicksort.ari), [binary-search](binary-search.ari), [sieve](sieve.ari), [levenshtein](levenshtein.ari), [matrix](matrix.ari) |
| **Data** | [word-frequency](word-frequency.ari), [csv-records](csv-records.ari), [group-report](group-report.ari), [pretty-print](pretty-print.ari) |
| **Interpreters** | [rpn-calculator](rpn-calculator.ari), [brainfuck](brainfuck.ari) |
| **Programs** | [adventure](adventure.ari), [ledger](ledger.ari), [todo](todo.ari), [mandelbrot](mandelbrot.ari) |
| **Language tour** | [error-handling](error-handling.ari), [destructuring](destructuring.ari), [imports](imports.ari) |
| **Classics** | [fizzbuzz](fizzbuzz.ari), [fibonacci](fibonacci.ari) |

`lib/` holds files meant to be imported rather than run. They have no output of
their own and no golden, and the checker skips them.

## Adding one

1. Write it. Keep it deterministic: no `Math.random`, no `Time.now`, and no
   walking a dictionary without sorting its keys first, since dictionaries have
   no order and the golden would not survive another machine.
2. `scripts/check-examples.sh record` to write its golden.
3. Read the golden. It is the claim the example is making.

The checker runs each example three times and reports any whose output varies
rather than recording one of the answers as the expected one.
