package parser

import (
	"testing"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/source"
)

// FuzzParse asserts the parser's structural invariants on arbitrary input:
//
//   - it terminates and never panics;
//   - it never yields a nil node, so no consumer has to nil-check a child;
//   - every span points into the file, so any node can anchor a diagnostic;
//   - Inspect works on any tree it produced, Bad nodes included.
//
// The seeds include both inputs that crashed the original parser. That one
// found a nil dereference in under a second; nothing here should reproduce it.
func FuzzParse(f *testing.F) {
	seeds := []string{
		`let x = 1`,
		`var y = [1, 2, 3]`,
		`func (a, b) do a + b end`,
		`func (a: Int, b = 2) -> Int` + "\n a \nend",
		`if x then 1 else 2 end`,
		"switch x\ncase 1\n 2\ndefault\n 3\nend",
		"switch\ncase a == 1\n 10\nend",
		"for v in [1, 2]\n v\nend",
		"for\n break\nend",
		"module M\n let a = 1\nend",
		`[1 => 2, :k => "v"]`,
		`a |> b() |> c()`,
		`x -> x * 2`,
		`(a, b) -> a + b`,
		`1 ? 2 : 3`,
		`a && b || c`,
		`2 ** 3 ** 2`,
		`-2 ** 2`,
		`6 & 3 == 3`,
		`1..n - 1`,
		`M.member`,
		`a[0][1] = 5`,
		`a[] = 1`,
		`import "file"`,
		`x is Int`,
		`x as String`,
		// Inputs that broke the original.
		"0?\x00.0",
		"0.\xd7\x92\x92",
		`let x = ]`,
		`f(`,
		`0x`,
		`"unterminated`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		file := source.NewFile("fuzz.ari", []byte(src))
		bag := diag.New(file)
		bag.SetMax(0)

		prog := New(file, bag).Parse()
		if prog == nil {
			t.Fatal("Parse returned nil")
		}

		ast.Walk(prog, func(n ast.Node) bool {
			if n == nil {
				t.Fatal("tree contains a nil node")
			}
			sp := n.Span()
			if !sp.IsValid() {
				t.Fatalf("%T has an invalid span %v", n, sp)
			}
			if sp.End > file.Size() {
				t.Fatalf("%T span %v runs past the file (size %d)", n, sp, file.Size())
			}
			return true
		})

		// Both of these are what a user hits when their source is broken, so
		// neither may panic on any tree the parser can produce.
		_ = prog.Inspect()
		_ = bag.Render()
	})
}
