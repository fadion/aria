package interp

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/parser"
	"github.com/fadion/aria/internal/source"
)

// Benchmarks are the arbiter of cost the way the characterization suite is the
// arbiter of meaning. Aria was never trying to be fast, and docs/compatibility.md
// puts performance outside what a version promises, so nothing here is a target
// to hit. What they are for is noticing when a change costs something nobody
// meant to spend.
//
// Read allocs/op, not ns/op. Every collection operation returns a new value, so
// allocation is the design's dominant cost, and the count is deterministic for a
// deterministic program: the same number on a laptop and on a shared CI runner.
// ns/op on a GitHub runner is noise wearing a number's clothes, which is why CI
// runs these with -benchtime=1x to prove they still work and compares nothing.
//
// To actually compare two revisions:
//
//	go test ./internal/interp/ -run=XXX -bench=. -benchmem -count=10 > old.txt
//	# ... make the change ...
//	go test ./internal/interp/ -run=XXX -bench=. -benchmem -count=10 > new.txt
//	benchstat old.txt new.txt
//
// benchstat is what says whether a difference is real. A single run of each
// says nothing at all.

// The macro workloads are examples, not fixtures written for the benchmark.
// They are already deterministic, already realistic, and check-examples.sh
// already keeps them running, so they cannot quietly rot into something that
// measures nothing.
var workloads = []string{
	"mandelbrot",     // float arithmetic in a tight nested loop
	"brainfuck",      // an interpreter loop: dictionary lookups and string building
	"levenshtein",    // nested folds over arrays
	"quicksort",      // recursion, and building arrays from other arrays
	"word-frequency", // dictionary insertion and sorting
}

// The micro workloads isolate what the examples exercise but do not separate.
const (
	// Name lookup through a chain of scopes, which is what the resolver's unused
	// slot indices would replace with an array index. See the Known Gaps entry
	// in docs/architecture.md: this benchmark is what turns that from an opinion
	// into a number.
	srcNameLookup = `
let outer = 1
let level1 = func ()
  let middle = 2
  let level2 = func ()
    let inner = 3
    let level3 = func ()
      var total = 0
      for i in 1..2000
        total += outer + middle + inner
      end
      total
    end
    level3()
  end
  level2()
end
level1()
`

	// The same loop and the same arithmetic with every name declared beside it,
	// so the difference between this and the one above is the cost of walking
	// the scope chain and nothing else. Without a shallow case to subtract, the
	// deep one measures a loop and three additions as well.
	srcNameLookupShallow = `
let level3 = func ()
  let outer = 1
  let middle = 2
  let inner = 3
  var total = 0
  for i in 1..2000
    total += outer + middle + inner
  end
  total
end
level3()
`

	// Function call overhead on its own: a call per iteration and almost nothing
	// else.
	srcCalls = `
let add = func (x, y) do x + y end
var total = 0
for i in 1..5000
  total += add(i, 1)
end
total
`

	// Building a collection an element at a time, which is quadratic because
	// every operation returns a new array. That is the immutability rule's bill,
	// and the number worth watching if anything ever tries to optimise it.
	srcCollections = `
var xs = []
for i in 1..2000
  xs = Enum.insert(xs, i)
end
Enum.size(xs)
`
)

// prepared is a program with everything done to it except running: parsed,
// resolved, with the standard library loaded and imports evaluated.
type prepared struct {
	interp *Interp
	prog   *ast.Program
}

// prepare mirrors what Eval does up to the point of running, so a benchmark can
// build it with the timer stopped and measure only the part it cares about.
func prepare(tb testing.TB, name, src string) prepared {
	tb.Helper()

	file := source.NewFile(name, []byte(src))
	bag := diag.New(file)
	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		tb.Fatalf("%s: %s", name, bag.Render())
	}

	i := New(file, nil)
	i.Out = io.Discard
	i.Err = io.Discard

	var sink discard
	if !i.loadStdlib(&sink) {
		tb.Fatalf("%s: standard library failed to load: %s", name, sink.String())
	}
	units, info, ok := i.compile(file, prog, bag, &sink)
	if !ok {
		tb.Fatalf("%s: %s", name, sink.String())
	}
	i.info = info
	if err := i.evalUnits(units); err != nil {
		tb.Fatalf("%s: %v", name, err)
	}
	return prepared{interp: i, prog: prog}
}

// readExample returns an example's source and the path it should be compiled
// under, so that a workload with an import resolves it the way `aria run` would.
func readExample(tb testing.TB, name string) (path, src string) {
	tb.Helper()
	path = filepath.Join("..", "..", "examples", name+".ari")
	b, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("reading %s: %v", path, err)
	}
	return path, string(b)
}

// BenchmarkStdlibLoad is the fixed cost of starting up: parsing, resolving and
// evaluating the whole standard library before a line of anybody's program runs.
// It is separate because it dominates everything else -- around fifty times the
// allocations of an empty program -- so folding it into the workloads would mean
// measuring it five times over and the workloads not at all.
func BenchmarkStdlibLoad(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		i := New(source.NewFile("bench.ari", nil), nil)
		i.Out = io.Discard
		i.Err = io.Discard
		var sink discard
		if !i.loadStdlib(&sink) {
			b.Fatalf("standard library failed to load: %s", sink.String())
		}
	}
}

// BenchmarkParse is the scanner and the parser, with nothing else attached.
func BenchmarkParse(b *testing.B) {
	for _, name := range workloads {
		path, src := readExample(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				file := source.NewFile(path, []byte(src))
				bag := diag.New(file)
				prog := parser.New(file, bag).Parse()
				if bag.HasErrors() {
					b.Fatalf("%s", bag.Render())
				}
				_ = prog
			}
		})
	}
}

// BenchmarkResolve is name binding and the checks that go with it, over an
// already-parsed program with the standard library already in scope.
func BenchmarkResolve(b *testing.B) {
	for _, name := range workloads {
		path, src := readExample(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				file := source.NewFile(path, []byte(src))
				bag := diag.New(file)
				prog := parser.New(file, bag).Parse()
				i := New(file, nil)
				i.Out, i.Err = io.Discard, io.Discard
				var sink discard
				if !i.loadStdlib(&sink) {
					b.Fatalf("standard library failed to load: %s", sink.String())
				}
				b.StartTimer()

				if _, _, ok := i.compile(file, prog, bag, &sink); !ok {
					b.Fatalf("%s", sink.String())
				}
			}
		})
	}
}

// BenchmarkEval is the evaluator alone. Everything else is built with the timer
// stopped, which excludes it from the allocation counts as well as the time.
func BenchmarkEval(b *testing.B) {
	for _, name := range workloads {
		path, src := readExample(b, name)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				p := prepare(b, path, src)
				b.StartTimer()

				if _, err := p.interp.Run(p.prog); err != nil {
					b.Fatalf("%s: %v", name, err)
				}
			}
		})
	}
}

// BenchmarkMicro isolates one cost each, where the workloads above mix them.
func BenchmarkMicro(b *testing.B) {
	for _, tt := range []struct{ name, src string }{
		{"name-lookup-deep", srcNameLookup},
		{"name-lookup-shallow", srcNameLookupShallow},
		{"calls", srcCalls},
		{"collections", srcCollections},
	} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				p := prepare(b, tt.name+".ari", tt.src)
				b.StartTimer()

				if _, err := p.interp.Run(p.prog); err != nil {
					b.Fatalf("%s: %v", tt.name, err)
				}
			}
		})
	}
}
