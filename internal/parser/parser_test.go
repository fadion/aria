package parser

import (
	"strings"
	"testing"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/source"
)

func parse(t *testing.T, src string) (*ast.Program, *diag.Bag) {
	t.Helper()
	file := source.NewFile("test.ari", []byte(src))
	bag := diag.New(file)
	prog := New(file, bag).Parse()
	return prog, bag
}

// parseOK parses and fails the test on any diagnostic.
func parseOK(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, bag := parse(t, src)
	if bag.HasErrors() {
		t.Fatalf("%q: unexpected diagnostics:\n%s", src, bag.Render())
	}
	if ast.HasBad(prog) {
		t.Fatalf("%q: tree contains a Bad node: %s", src, prog.Inspect())
	}
	return prog
}

// TestPrecedence pins down the binding-power table. docs/architecture.md explains
// why the levels sit where they do; the cases marked "fixed" are the ones that
// changed from the original, and the rest must not move.
func TestPrecedence(t *testing.T) {
	tests := []struct{ src, want string }{
		// Arithmetic.
		{"1 + 2 * 3", "(1 + (2 * 3))"},
		{"(1 + 2) * 3", "((1 + 2) * 3)"},
		{"1 + 2 - 3", "((1 + 2) - 3)"},
		{"10 / 5 / 2", "((10 / 5) / 2)"},
		{"1 + 2 == 3", "((1 + 2) == 3)"},
		// Comparison is left-associative.
		{"a < b == c > d", "(((a < b) == c) > d)"},

		// Fixed: && binds tighter than ||, so this groups the way every other
		// language groups it. It used to yield (a && (b || c)).
		{"a && b || c", "((a && b) || c)"},
		{"a || b && c", "(a || (b && c))"},
		{"b > a && c <= d || e > f", "(((b > a) && (c <= d)) || (e > f))"},
		{"a && b && c", "((a && b) && c)"},
		{"a || b || c", "((a || b) || c)"},

		// Fixed: ** is right-associative, so this is 2 ** (3 ** 2) = 512.
		{"2 ** 3 ** 2", "(2 ** (3 ** 2))"},
		{"2 * 3 ** 2", "(2 * (3 ** 2))"},

		// Fixed: ** outranks a prefix operator on its left, but a prefix operator
		// may still open the exponent.
		{"-2 ** 2", "(-(2 ** 2))"},
		{"2 ** -1", "(2 ** (-1))"},
		{"-a * b", "((-a) * b)"},
		{"!a && b", "((!a) && b)"},

		// Fixed: bitwise binds tighter than comparison, so this compares the
		// masked value instead of masking a boolean.
		{"6 & 3 == 3", "((6 & 3) == 3)"},
		{"1 | 2 == 3", "((1 | 2) == 3)"},
		{"a & b | c", "((a & b) | c)"},

		// Unchanged: shift is looser than additive, as in C.
		{"1 + 2 << 3", "((1 + 2) << 3)"},
		{"a >> b + c", "(a >> (b + c))"},

		// Ranges, calls and indexing.
		{"1..n - 1", "(1 .. (n - 1))"},
		{"a[0] + 1", "(a[0] + 1)"},
		{"f(1) * 2", "(f(1) * 2)"},
		{"M.f(1)", "M.f(1)"},
		{"a[0][1]", "a[0][1]"},

		// Type operators bind tightest.
		{"x as String + \"a\"", "((x as String) + a)"},
		{"n is Int && b", "((n is Int) && b)"},

		// Assignment is right-associative and loosest.
		{"a = b + 1", "a = (b + 1)"},
		{"a += 2", "a = (a + 2)"},
		{"a[0] = 5", "a[0] = 5"},
	}

	for _, test := range tests {
		prog, bag := parse(t, test.src)
		if bag.HasErrors() {
			t.Errorf("%q: unexpected diagnostics:\n%s", test.src, bag.Render())
			continue
		}
		if got := prog.Inspect(); got != test.want {
			t.Errorf("%q\n  got  %s\n  want %s", test.src, got, test.want)
		}
	}
}

func TestLiterals(t *testing.T) {
	tests := []struct{ src, want string }{
		{"42", "42"},
		{"0xff", "0xff"},
		{"1_000", "1_000"},
		{"1.5", "1.5"},
		{"1e-5", "1e-5"},
		{`"hello"`, "hello"},
		{":ok", ":ok"},
		{"true", "true"},
		{"nil", "nil"},
		{"_", "_"},
		{"[1, 2, 3]", "Array(1, 2, 3)"},
		{"[]", "Array()"},
		{`[:a => 1, :b => 2]`, "[:a => 1, :b => 2]"},
	}
	for _, test := range tests {
		prog := parseOK(t, test.src)
		if got := prog.Inspect(); got != test.want {
			t.Errorf("%q: got %s, want %s", test.src, got, test.want)
		}
	}
}

// Literal values must be decoded, not just spanned.
func TestLiteralValues(t *testing.T) {
	prog := parseOK(t, "0xff")
	if n, ok := prog.Nodes[0].(*ast.Integer); !ok || n.Value != 255 {
		t.Errorf("0xff: got %#v, want Integer 255", prog.Nodes[0])
	}

	prog = parseOK(t, "0b1010")
	if n, ok := prog.Nodes[0].(*ast.Integer); !ok || n.Value != 10 {
		t.Errorf("0b1010: got %#v, want Integer 10", prog.Nodes[0])
	}

	prog = parseOK(t, "0o27")
	if n, ok := prog.Nodes[0].(*ast.Integer); !ok || n.Value != 23 {
		t.Errorf("0o27: got %#v, want Integer 23", prog.Nodes[0])
	}

	prog = parseOK(t, "1_000_000")
	if n, ok := prog.Nodes[0].(*ast.Integer); !ok || n.Value != 1000000 {
		t.Errorf("1_000_000: got %#v, want Integer 1000000", prog.Nodes[0])
	}

	prog = parseOK(t, `"a\tb\nc"`)
	if n, ok := prog.Nodes[0].(*ast.String); !ok || n.Value != "a\tb\nc" {
		t.Errorf(`"a\tb\nc": got %#v, want the decoded escapes`, prog.Nodes[0])
	}
}

// Dictionary literals keep source order, so printing one is reproducible. The
// old tree used a Go map, whose iteration order varies between runs.
func TestDictionaryKeepsOrder(t *testing.T) {
	const src = `[:a => 1, :b => 2, :c => 3, :d => 4, :e => 5]`
	want := "[:a => 1, :b => 2, :c => 3, :d => 4, :e => 5]"
	for i := 0; i < 20; i++ {
		if got := parseOK(t, src).Inspect(); got != want {
			t.Fatalf("run %d: got %s, want %s", i, got, want)
		}
	}
}

func TestConstructs(t *testing.T) {
	tests := []struct{ name, src string }{
		{"let", `let a = 1`},
		{"var", `var a = 1`},
		{"if", "if a then 1 end"},
		{"if else", "if a then 1 else 2 end"},
		{"if newline body", "if a\n  1\nend"},
		{"ternary", `a ? 1 : 2`},
		{"for in", "for v in list\n  v\nend"},
		{"for two names", "for i, v in list\n  v\nend"},
		{"for infinite", "for\n  1\nend"},
		{"func", "func (a, b) do\n  a + b\nend"},
		{"func no parens", "func a do\n  a\nend"},
		{"func typed", "func (a: Int) -> Int\n  a\nend"},
		{"func default", `func (a, b = 2) do a end`},
		{"func variadic", "func (...xs) do xs end"},
		{"arrow", `x -> x * 2`},
		{"arrow multi", `(a, b) -> a + b`},
		{"call", `f(1, 2)`},
		{"call empty", `f()`},
		{"pipe", `5 |> double()`},
		{"module", "module M\n  let a = 1\nend"},
		{"module access", `M.a`},
		{"import", `import "file"`},
		{"return", "func () do\n  return 1\nend"},
		{"bare return", "func () do\n  return\nend"},
		{"break", "for\n  break\nend"},
		{"switch", "switch a\ncase 1\n  10\nend"},
		{"switch inline", `switch a do case 1 then 10 end`},
		{"switch default", "switch a\ncase 1\n  10\ndefault\n  20\nend"},
		{"switch multi values", "switch a\ncase 1, 2, 3\n  10\nend"},
		{"subscript", `a[0]`},
		{"subscript append", `a[]`},
		{"multiline array", "[\n  1,\n  2\n]"},
		{"multiline call", "f(\n  1,\n  2\n)"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { parseOK(t, test.src) })
	}
}

// Forms the original parser could not express at all, each documented in the
// README but rejected by the implementation.
func TestPreviouslyBrokenForms(t *testing.T) {
	tests := []struct{ name, src, note string }{
		{"module with do", "module M do\n  let a = 1\nend",
			"the optional 'do' after a module name"},
		{"control-less switch", "switch\ncase a == 1\n  10\nend",
			"a switch with no control expression"},
		{"control-less switch with do", "switch do case 1 == 1 then 10 end",
			"a control-less switch written with 'do'"},
		{"trailing comment", "switch a // note\ncase 1\n  10\nend",
			"a trailing comment before a needed separator"},
		{"leading underscore name", "let _foo = 1",
			"an identifier beginning with an underscore"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prog, bag := parse(t, test.src)
			if bag.HasErrors() {
				t.Errorf("%s still fails to parse:\n%s", test.note, bag.Render())
			}
			if ast.HasBad(prog) {
				t.Errorf("%s produced a Bad node: %s", test.note, prog.Inspect())
			}
		})
	}
}

// A control-less switch must record a nil Control, which is what makes it
// behave as `switch true`.
func TestControlLessSwitchHasNoControl(t *testing.T) {
	prog := parseOK(t, "switch\ncase a == 1\n  10\nend")
	sw, ok := prog.Nodes[0].(*ast.Switch)
	if !ok {
		t.Fatalf("got %T, want *ast.Switch", prog.Nodes[0])
	}
	if sw.Control != nil {
		t.Errorf("Control = %s, want nil", sw.Control.Inspect())
	}
	if len(sw.Cases) != 1 {
		t.Errorf("got %d cases, want 1", len(sw.Cases))
	}
}

// Compound assignment desugars in the parser, so the evaluator only ever sees
// plain assignment.
func TestCompoundAssignDesugars(t *testing.T) {
	for _, test := range []struct{ src, want string }{
		{"a += 1", "a = (a + 1)"},
		{"a -= 1", "a = (a - 1)"},
		{"a *= 2", "a = (a * 2)"},
		{"a /= 2", "a = (a / 2)"},
	} {
		if got := parseOK(t, test.src).Inspect(); got != test.want {
			t.Errorf("%q: got %s, want %s", test.src, got, test.want)
		}
	}
}

// Parsing never returns nil. Every failure is a Bad node with a diagnostic, so
// nothing downstream has to nil-check a child.
func TestErrorsProduceBadNodesNotNil(t *testing.T) {
	inputs := []string{
		"let",
		"let =",
		"let x",
		"let x =",
		"]",
		".foo",
		"a.",
		"f(",
		"[1,",
		"if",
		"if a then",
		"switch",
		"for",
		"func",
		"module",
		"import",
		"1 ? 2",
		"x as",
		"x is",
		":",
		"a[",
		"(1,",
		"0x",
		`"unterminated`,
		"/* unterminated",
		"\x00",
		"0?\x00.0",
	}

	for _, src := range inputs {
		prog, bag := parse(t, src)

		if prog == nil {
			t.Errorf("%q: Parse returned nil", src)
			continue
		}
		for i, n := range prog.Nodes {
			if n == nil {
				t.Errorf("%q: node %d is nil", src, i)
			}
		}
		// Every one of these is malformed, so each must say so.
		if !bag.HasErrors() {
			t.Errorf("%q: parsed without a diagnostic; tree was %s", src, prog.Inspect())
		}
		// Inspect must not panic on a tree containing Bad nodes.
		_ = prog.Inspect()
	}
}

// One mistake should produce one message, not a cascade describing the
// confusion it caused.
func TestOneErrorPerMistake(t *testing.T) {
	for _, src := range []string{"let x =", "0x", `"unterminated`, "f(1,"} {
		_, bag := parse(t, src)
		if n := bag.Len(); n > 2 {
			t.Errorf("%q: %d diagnostics, want at most 2:\n%s", src, n, bag.Render())
		}
	}
}

// A parse error must not swallow the rest of the file. The old recovery set was
// missing VAR, so everything after an error was discarded to end of file.
func TestRecoveryContinuesAfterError(t *testing.T) {
	const src = "let x = ]\nvar y = 1\nlet z = 2"
	prog, bag := parse(t, src)

	if !bag.HasErrors() {
		t.Fatal("expected a diagnostic for the bad let")
	}

	var sawVar, sawLetZ bool
	ast.Walk(prog, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Var:
			if n.Name.Value == "y" {
				sawVar = true
			}
		case *ast.Let:
			if n.Name.Value == "z" {
				sawLetZ = true
			}
		}
		return true
	})

	if !sawVar {
		t.Error("`var y = 1` after the error was not parsed")
	}
	if !sawLetZ {
		t.Error("`let z = 2` after the error was not parsed")
	}
}

// Deep nesting must fail with a diagnostic rather than exhausting the stack.
func TestDepthLimit(t *testing.T) {
	src := strings.Repeat("(", 5000) + "1" + strings.Repeat(")", 5000)
	prog, bag := parse(t, src)

	if !bag.HasErrors() {
		t.Error("expected a depth-limit diagnostic")
	}
	if prog == nil {
		t.Fatal("Parse returned nil")
	}
	if !strings.Contains(bag.Render(), "nested too deeply") {
		t.Errorf("diagnostic did not mention nesting:\n%s", bag.Render())
	}
}

// Every node must carry a span that points into the file, so any node can
// anchor a diagnostic.
func TestNodesHaveValidSpans(t *testing.T) {
	const src = "let a = 1 + 2\nfunc (x: Int) -> Int\n  x * 2\nend\nfor v in [1,2]\n  v\nend"
	file := source.NewFile("t.ari", []byte(src))
	prog := New(file, diag.New(file)).Parse()

	ast.Walk(prog, func(n ast.Node) bool {
		sp := n.Span()
		if !sp.IsValid() {
			t.Errorf("%T has an invalid span %v", n, sp)
		}
		if sp.End > file.Size() {
			t.Errorf("%T span %v runs past the file (size %d)", n, sp, file.Size())
		}
		return true
	})
}

// An empty code block is an unfinished edit, not an intention. The original
// rejected one in `if`, `else`, `for` and `func`; the rewrite accepted it
// everywhere because blocks became ordinary node lists. This pins the rule down
// across every construct, including the ones the original never had.
//
// `for` is the reason the rule is worth having at all: the loop collects a value
// per iteration, so `for i in 1..1000000 end` quietly builds a million-element
// array of nils.
func TestEmptyCodeBodiesAreRejected(t *testing.T) {
	for _, tt := range []struct{ name, src string }{
		{"if", "if true\nend"},
		{"else", "if true\n  nil\nelse\nend"},
		{"for", "for i in 1..3\nend"},
		{"while", "while false\nend"},
		{"until", "until true\nend"},
		{"func", "let f = func ()\nend"},
		{"do", "let v = do\nend"},
		{"try", "try\nrescue\n  nil\nend"},
		{"rescue", "try\n  nil\nrescue\nend"},
		{"case", "switch 1\ncase 1\ndefault\n  nil\nend"},
		{"default", "switch 1\ncase 1\n  nil\ndefault\nend"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, bag := parse(t, tt.src)
			if !bag.HasErrors() {
				t.Fatalf("%q: expected a diagnostic for the empty %s body", tt.src, tt.name)
			}
			if got := bag.Render(); !strings.Contains(got, "empty "+tt.name+" body") {
				t.Errorf("%q: diagnostic does not name the construct:\n%s", tt.src, got)
			}
		})
	}
}

// A module or a record is a container, not a body. An empty one still names
// something that exists, so it stays legal.
func TestEmptyContainersAreAllowed(t *testing.T) {
	for _, src := range []string{"module M\nend", "record R\nend"} {
		parseOK(t, src)
	}
}

// The empty-body diagnostic must not set the parser's panicking flag. That flag
// suppresses the cascade from a parser that has lost its place, and this parser
// has not: it knows where it is and carries on correctly, so a second empty body
// further down is a separate mistake and has to be reported too.
func TestEmptyBodiesDoNotSuppressLaterDiagnostics(t *testing.T) {
	const src = "if true\nend\nfor i in 1..3\nend"
	_, bag := parse(t, src)
	if bag.Len() != 2 {
		t.Fatalf("got %d diagnostics, want 2 (one per empty body):\n%s", bag.Len(), bag.Render())
	}
}

// A construct still being typed has an empty block for the same reason it has no
// `end` yet. Reporting an empty body there would turn the REPL's "keep typing"
// into an error, so nothing is reported at end of input and the missing-'end'
// diagnostic is the one that stands.
func TestUnterminatedConstructIsIncompleteNotEmpty(t *testing.T) {
	for _, src := range []string{"if true", "for i in 1..3", "let f = func ()"} {
		_, bag := parse(t, src)
		if !bag.Incomplete() {
			t.Errorf("%q: want a single incomplete-input diagnostic, got:\n%s", src, bag.Render())
		}
		if strings.Contains(bag.Render(), "empty ") {
			t.Errorf("%q: reported an empty body while still incomplete:\n%s", src, bag.Render())
		}
	}
}
