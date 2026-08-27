package resolver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/parser"
	"github.com/fadion/aria/internal/resolver"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/stdlib"
)

// stdlibModules are the module names the standard library defines.
var stdlibModules = stdlib.Names()

// resolveSource parses and resolves one unit, with the standard library's
// module names predeclared.
func resolveSource(name, src string) (*diag.Bag, bool) {
	file := source.NewFile(name, []byte(src))
	bag := diag.New(file)

	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() || ast.HasBad(prog) {
		return bag, false // a parse problem, not a resolution one
	}

	r := resolver.New(file, bag)
	for _, m := range stdlibModules {
		r.PredeclareModule(m)
	}
	r.Resolve(prog)
	return bag, true
}

// TestResolvesStandardLibrary is the resolver's hardest real input: the standard
// library is the largest Aria program that exists, and it exercises closures,
// recursion, module members referring to each other, and every collection form.
//
// Anything it reports is either a resolver bug or a genuine bug in the library.
// It must resolve with no diagnostics at all. It did not when the resolver
// first ran over it: three functions mutated a parameter, which is forbidden,
// and Enum.random called a `rand` that has never existed. All four are
// fixed in internal/stdlib, and this test is what keeps them fixed.
func TestResolvesStandardLibrary(t *testing.T) {
	mods := stdlib.Modules()
	if len(mods) == 0 {
		t.Fatal("the standard library is empty")
	}

	for _, m := range mods {
		bag, parsed := resolveSource(m.Path, m.Src)
		if !parsed {
			t.Errorf("stdlib module %s does not parse:\n%s", m.Name, bag.Render())
			continue
		}
		if bag.HasErrors() {
			t.Errorf("stdlib module %s does not resolve:\n%s", m.Name, bag.Render())
		}
	}
}

// Enum.random must call runtime_rand. It used to call a bare `rand`, which does
// not exist, so the function was never callable — and the runtime error named a
// line in the embedded library rather than in the caller's file.
func TestEnumRandomCallsRuntimeRand(t *testing.T) {
	var enum string
	for _, m := range stdlib.Modules() {
		if m.Name == "Enum" {
			enum = m.Src
		}
	}
	if enum == "" {
		t.Fatal("could not find the Enum module")
	}

	if strings.Contains(enum, "rand(0, size(array) - 1)") &&
		!strings.Contains(enum, "runtime_rand(0, size(array) - 1)") {
		t.Error("Enum.random still calls the undefined `rand`")
	}
	if !strings.Contains(enum, "runtime_rand") {
		t.Error("Enum.random does not call runtime_rand")
	}
}

// expectedResolveErrors lists corpus cases that SHOULD fail resolution, with
// what they are demonstrating. Every one is a case whose golden already records
// the same problem being reported at runtime; the resolver now catches it before
// the program starts.
var expectedResolveErrors = map[string]string{
	"undefined-identifier.ari": "reads a name that was never declared",
	"redeclare-same-scope.ari": "declares the same name twice in one scope",
	"let-immutable.ari":        "assigns to a let binding",
	"mutate-through-let.ari":   "3.4 writes through a let-bound collection",
	"for-scope-leak.ari":       "3.3 reads a loop variable after the loop",
	"undefined-module.ari":     "accesses a module that was never declared",

	"toplevel-break.ari":             "breaks where no loop is running",
	"toplevel-return.ari":            "returns from outside a function",
	"break-inside-nested-func.ari":   "breaks from a function nested in a loop",
	"placeholder-as-value.ari":       "reads `_` where it means nothing",
	"for-too-many-vars.ari":          "gives a for loop three variables",
	"pipe-placeholder-duplicate.ari": "marks two slots for one piped value",
}

// Three cases that USED to fail now resolve cleanly, which is the point:
//
//	shadowing.ari                     3.1 an inner `let x` no longer collides
//	                                  with an outer one
//	immutable-leaks-across-scopes.ari 3.2 a `let` inside a function no longer
//	                                  freezes an unrelated top-level `var`
//	leading-underscore-name.ari       8.4 `_foo` is a name
//
// They are deliberately absent from the map above. Their goldens still record
// the old failures, and will change when the evaluator lands.

// TestResolvesCorpus runs the resolver over every characterization case that
// parses cleanly, and requires it to accept them unless the case is listed above
// as demonstrating a name problem.
func TestResolvesCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "semantics")

	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".ari") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking corpus: %v", err)
	}

	resolved, rejected, skipped := 0, 0, 0

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		base := filepath.Base(path)

		bag, parsed := resolveSource(base, string(src))
		if !parsed {
			skipped++ // the case is about a syntax error, not a name
			continue
		}

		reason, expectFail := expectedResolveErrors[base]

		switch {
		case bag.HasErrors() && expectFail:
			rejected++
		case bag.HasErrors():
			t.Errorf("%s: resolver rejects a case that should resolve:\n%s", base, bag.Render())
		case expectFail:
			t.Errorf("%s: expected a diagnostic (%s) but it resolved cleanly", base, reason)
		default:
			resolved++
		}
	}

	t.Logf("%d corpus files resolved cleanly, %d rejected as expected, %d skipped (syntax cases)",
		resolved, rejected, skipped)

	if resolved < 70 {
		t.Errorf("only %d files resolved; expected most of the corpus to pass", resolved)
	}
}
