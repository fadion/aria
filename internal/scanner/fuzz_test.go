package scanner

import (
	"testing"

	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/token"
)

// FuzzScan asserts the scanner's two structural invariants on arbitrary input:
//
//   - every Scan consumes at least one byte, so the token stream terminates;
//   - every span stays inside the file and never runs backwards, so the
//     diagnostic renderer can always be handed one safely.
//
// Neither is checked by an assertion in the scanner itself. They fall out of
// the cursor design, and this is what keeps them true.
func FuzzScan(f *testing.F) {
	seeds := []string{
		`let x = 1`,
		`var y = [1, 2, 3]`,
		`func (a, b) do a + b end`,
		`if x then 1 else 2 end`,
		`switch x do case 1 then 2 default then 3 end`,
		`module M let a = 1 end`,
		`[1 => 2, :k => "v"]`,
		`a |> b() |> c()`,
		`x -> x * 2`,
		`1 ? 2 : 3`,
		`0xff 0o27 0b1010 1_000 1e-5 1.5`,
		`"a string with \"escapes\" and \n"`,
		`1..5`,
		`// line comment`,
		"/* block\ncomment */",
		`empty? save! _ _foo`,
		// Shapes that broke the old lexer.
		"0.\xd7\x92\x92",
		"0?\x00.0",
		`0x`,
		`"unterminated`,
		"/* unterminated",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		file := source.NewFile("fuzz.ari", []byte(src))
		bag := diag.New(file)
		bag.SetMax(0)
		s := New(file, bag, ScanComments)

		// One token per byte is the theoretical ceiling; the +2 covers EOF and
		// an empty file. Exceeding it means some path failed to advance.
		limit := len(src) + 2
		for i := 0; ; i++ {
			if i > limit {
				t.Fatalf("scanner produced more than %d tokens, so some path did not advance", limit)
			}

			tok := s.Scan()

			if tok.Span.Start < 0 || tok.Span.End < tok.Span.Start {
				t.Fatalf("token %v has a backwards span %v", tok.Kind, tok.Span)
			}
			if tok.Span.End > file.Size() {
				t.Fatalf("token %v span %v runs past the file (size %d)", tok.Kind, tok.Span, file.Size())
			}

			if tok.Kind == token.EOF {
				break
			}
		}

		// Rendering must not panic on any diagnostic the scanner produced,
		// since that is the path a user hits when their source is broken.
		_ = bag.Render()
	})
}
