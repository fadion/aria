package scanner

import (
	"strings"
	"testing"

	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/token"
)

// scan runs the scanner to EOF and returns the tokens plus any diagnostics.
func scan(t *testing.T, src string, mode Mode) ([]token.Token, *diag.Bag, *source.File) {
	t.Helper()
	file := source.NewFile("test.ari", []byte(src))
	bag := diag.New(file)
	s := New(file, bag, mode)

	var toks []token.Token
	for i := 0; ; i++ {
		if i > 100000 {
			t.Fatalf("scanner did not reach EOF for %q", src)
		}
		tok := s.Scan()
		toks = append(toks, tok)
		if tok.Kind == token.EOF {
			return toks, bag, file
		}
	}
}

// kindsOf drops the trailing EOF, which every case would otherwise repeat.
func kindsOf(toks []token.Token) []token.Kind {
	kinds := make([]token.Kind, 0, len(toks))
	for _, tok := range toks {
		if tok.Kind == token.EOF {
			break
		}
		kinds = append(kinds, tok.Kind)
	}
	return kinds
}

func TestOperators(t *testing.T) {
	tests := []struct {
		src  string
		want []token.Kind
	}{
		{"=", []token.Kind{token.Assign}},
		{"==", []token.Kind{token.Eq}},
		{"=>", []token.Kind{token.FatArrow}},
		{"+ += - -= * *= / /= %", []token.Kind{
			token.Plus, token.AssignPlus, token.Minus, token.AssignMinus,
			token.Star, token.AssignMul, token.Slash, token.AssignDiv, token.Percent,
		}},
		{"**", []token.Kind{token.Power}},
		{"< <= > >= != !", []token.Kind{
			token.Lt, token.LtEq, token.Gt, token.GtEq, token.NotEq, token.Bang,
		}},
		{"<< >>", []token.Kind{token.ShiftLeft, token.ShiftRight}},
		{"| || |> & && ~", []token.Kind{
			token.BitOr, token.Or, token.Pipe, token.BitAnd, token.And, token.BitNot,
		}},
		{"-> ?", []token.Kind{token.Arrow, token.Question}},
		{", ( ) [ ] :", []token.Kind{
			token.Comma, token.LParen, token.RParen,
			token.LBracket, token.RBracket, token.Colon,
		}},
		{". .. ...", []token.Kind{token.Dot, token.Range, token.Ellipsis}},
		// The maximal-munch cases: each must not be split into the shorter one.
		{"....", []token.Kind{token.Ellipsis, token.Dot}},
		{"===", []token.Kind{token.Eq, token.Assign}},
		{"<<=", []token.Kind{token.ShiftLeft, token.Assign}},
	}

	for _, test := range tests {
		toks, bag, _ := scan(t, test.src, 0)
		if bag.HasErrors() {
			t.Errorf("%q: unexpected diagnostics:\n%s", test.src, bag.Render())
		}
		got := kindsOf(toks)
		if !equalKinds(got, test.want) {
			t.Errorf("%q: got %v, want %v", test.src, got, test.want)
		}
	}
}

func TestKeywordsAndIdentifiers(t *testing.T) {
	src := `let var func do end if else for in is as return then switch case default break continue module import true false nil`
	want := []token.Kind{
		token.Let, token.Var, token.Func, token.Do, token.End, token.If, token.Else,
		token.For, token.In, token.Is, token.As, token.Return, token.Then,
		token.Switch, token.Case, token.Default, token.Break, token.Continue,
		token.Module, token.Import, token.Bool, token.Bool, token.Nil,
	}

	toks, bag, _ := scan(t, src, 0)
	if bag.HasErrors() {
		t.Fatalf("unexpected diagnostics:\n%s", bag.Render())
	}
	if got := kindsOf(toks); !equalKinds(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestIdentifierShapes(t *testing.T) {
	tests := []struct {
		src  string
		want []token.Kind
	}{
		{"foo", []token.Kind{token.Ident}},
		{"foo_bar", []token.Kind{token.Ident}},
		{"foo123", []token.Kind{token.Ident}},
		{"empty?", []token.Kind{token.Ident}},
		{"save!", []token.Kind{token.Ident}},
		// A lone underscore is the placeholder.
		{"_", []token.Kind{token.Underscore}},
		// A leading underscore starts a name. The old lexer split this into
		// UNDERSCORE + IDENTIFIER, which made `let _foo = 1` a parse error.
		{"_foo", []token.Kind{token.Ident}},
		{"__", []token.Kind{token.Ident}},
		// `letter` must not be mistaken for the `let` keyword.
		{"letter", []token.Kind{token.Ident}},
		{"ends", []token.Kind{token.Ident}},
		// A name may not begin with a digit.
		{"1abc", []token.Kind{token.Int, token.Ident}},
		// `?` both ends a name and opens a ternary, so an unspaced `a?b:c`
		// cannot be a ternary: the scanner takes `a?` as one identifier. The
		// old lexer took `a?b` as one instead. Both need spaces here; neither
		// reading is more correct, and the parser sees a name either way.
		{"a?b:c", []token.Kind{token.Ident, token.Ident, token.Colon, token.Ident}},
		{"a ? b : c", []token.Kind{
			token.Ident, token.Question, token.Ident, token.Colon, token.Ident,
		}},
	}

	for _, test := range tests {
		toks, _, _ := scan(t, test.src, 0)
		if got := kindsOf(toks); !equalKinds(got, test.want) {
			t.Errorf("%q: got %v, want %v", test.src, got, test.want)
		}
	}
}

func TestNumbers(t *testing.T) {
	tests := []struct {
		src  string
		want []token.Kind
		text []string
	}{
		{"42", []token.Kind{token.Int}, []string{"42"}},
		{"0", []token.Kind{token.Int}, []string{"0"}},
		{"1_000_000", []token.Kind{token.Int}, []string{"1_000_000"}},
		{"0xff", []token.Kind{token.Int}, []string{"0xff"}},
		{"0XFF", []token.Kind{token.Int}, []string{"0XFF"}},
		{"0o27", []token.Kind{token.Int}, []string{"0o27"}},
		{"0b1010", []token.Kind{token.Int}, []string{"0b1010"}},
		{"1.5", []token.Kind{token.Float}, []string{"1.5"}},
		{"1e3", []token.Kind{token.Float}, []string{"1e3"}},
		{"1e-5", []token.Kind{token.Float}, []string{"1e-5"}},
		{"1e+5", []token.Kind{token.Float}, []string{"1e+5"}},
		{"2.5e2", []token.Kind{token.Float}, []string{"2.5e2"}},
		// A range must not be swallowed by the number before it. This needed
		// a rewind in the old lexer; here a single lookahead settles it.
		{"1..5", []token.Kind{token.Int, token.Range, token.Int}, []string{"1", "..", "5"}},
		{"1...5", []token.Kind{token.Int, token.Ellipsis, token.Int}, []string{"1", "...", "5"}},
		// A trailing dot with no digit after it is not part of the number.
		{"1.", []token.Kind{token.Int, token.Dot}, []string{"1", "."}},
		// `e` not followed by digits is an identifier, not an exponent.
		{"1end", []token.Kind{token.Int, token.End}, []string{"1", "end"}},
	}

	for _, test := range tests {
		toks, bag, file := scan(t, test.src, 0)
		if bag.HasErrors() {
			t.Errorf("%q: unexpected diagnostics:\n%s", test.src, bag.Render())
		}
		got := kindsOf(toks)
		if !equalKinds(got, test.want) {
			t.Errorf("%q: got %v, want %v", test.src, got, test.want)
			continue
		}
		for i, want := range test.text {
			if got := file.Text(toks[i].Span); got != want {
				t.Errorf("%q: token %d text = %q, want %q", test.src, i, got, want)
			}
		}
	}
}

func TestBadNumbers(t *testing.T) {
	// One mistake must produce exactly one diagnostic. The old lexer reported
	// `0x` twice, once from the lexer and once from the parser.
	for _, src := range []string{"0x", "0o", "0b", "0xg"} {
		toks, bag, _ := scan(t, src, 0)
		if !bag.HasErrors() {
			t.Errorf("%q: expected a diagnostic", src)
		}
		if n := bag.Len(); n != 1 {
			t.Errorf("%q: got %d diagnostics, want 1:\n%s", src, n, bag.Render())
		}
		if toks[0].Kind != token.Invalid {
			t.Errorf("%q: got %v, want Invalid", src, toks[0].Kind)
		}
	}
}

func TestStrings(t *testing.T) {
	tests := []struct {
		src     string
		want    token.Kind
		wantErr bool
	}{
		{`"hello"`, token.String, false},
		{`""`, token.String, false},
		{`"with \"quotes\""`, token.String, false},
		{`"tab\there"`, token.String, false},
		{`"back\\slash"`, token.String, false},
		{`"unicode héllo"`, token.String, false},
		{`"unterminated`, token.Invalid, true},
		{"\"spans\nlines\"", token.Invalid, true},
		{`"bad \q escape"`, token.String, true},
	}

	for _, test := range tests {
		toks, bag, _ := scan(t, test.src, 0)
		if toks[0].Kind != test.want {
			t.Errorf("%q: got %v, want %v", test.src, toks[0].Kind, test.want)
		}
		if bag.HasErrors() != test.wantErr {
			t.Errorf("%q: hasErrors = %v, want %v:\n%s", test.src, bag.HasErrors(), test.wantErr, bag.Render())
		}
	}
}

func TestComments(t *testing.T) {
	src := "1 // line comment\n2 /* block\ncomment */ 3"

	// Skipped by default.
	toks, bag, _ := scan(t, src, 0)
	if bag.HasErrors() {
		t.Fatalf("unexpected diagnostics:\n%s", bag.Render())
	}
	want := []token.Kind{token.Int, token.Newline, token.Int, token.Int}
	if got := kindsOf(toks); !equalKinds(got, want) {
		t.Errorf("default mode: got %v, want %v", got, want)
	}

	// Returned under ScanComments.
	toks, _, _ = scan(t, src, ScanComments)
	want = []token.Kind{
		token.Int, token.Comment, token.Newline,
		token.Int, token.Comment, token.Int,
	}
	if got := kindsOf(toks); !equalKinds(got, want) {
		t.Errorf("ScanComments: got %v, want %v", got, want)
	}
}

func TestUnterminatedBlockComment(t *testing.T) {
	toks, bag, _ := scan(t, "/* never closed", 0)
	if !bag.HasErrors() {
		t.Error("expected a diagnostic")
	}
	if toks[0].Kind != token.Invalid {
		t.Errorf("got %v, want Invalid", toks[0].Kind)
	}
}

func TestNewlinesAreSignificant(t *testing.T) {
	toks, _, _ := scan(t, "1\n\n2\r\n3", 0)
	want := []token.Kind{
		token.Int, token.Newline, token.Newline,
		token.Int, token.Newline, token.Int,
	}
	if got := kindsOf(toks); !equalKinds(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSpansAndPositions(t *testing.T) {
	src := "let x = 42\nlet yy = 1"
	toks, _, file := scan(t, src, 0)

	// Every token's span must recover exactly the text it came from.
	for _, tok := range toks {
		if tok.Kind == token.EOF || tok.Kind == token.Newline {
			continue
		}
		if text := file.Text(tok.Span); text == "" {
			t.Errorf("%v: empty text for span %v", tok.Kind, tok.Span)
		}
	}

	// Spot-check that positions land where a reader would point.
	cases := []struct {
		index     int
		text      string
		line, col int
	}{
		{0, "let", 1, 1},
		{1, "x", 1, 5},
		{3, "42", 1, 9},
		{6, "yy", 2, 5},
	}
	for _, c := range cases {
		tok := toks[c.index]
		if got := file.Text(tok.Span); got != c.text {
			t.Errorf("token %d: text %q, want %q", c.index, got, c.text)
		}
		pos := file.Position(tok.Span.Start)
		if pos.Line != c.line || pos.Col != c.col {
			t.Errorf("token %d (%s): at %d:%d, want %d:%d",
				c.index, c.text, pos.Line, pos.Col, c.line, c.col)
		}
	}
}

// Columns count runes, so a caret lines up under the character a reader sees
// rather than under the middle of a multi-byte one.
func TestColumnsCountRunes(t *testing.T) {
	toks, _, file := scan(t, `let s = "héllo" + x`, 0)

	var last token.Token
	for _, tok := range toks {
		if tok.Kind == token.Ident && file.Text(tok.Span) == "x" {
			last = tok
		}
	}
	if last.Kind != token.Ident {
		t.Fatal("did not find the trailing identifier")
	}

	// "héllo" is 5 runes plus quotes; x sits at column 19 counting runes and
	// would be 20 counting bytes.
	if pos := file.Position(last.Span.Start); pos.Col != 19 {
		t.Errorf("column %d, want 19 (rune-counted)", pos.Col)
	}
}

// Scanning must terminate on any input at all. The old lexer looped forever on
// invalid UTF-8; here progress is structural, and this pins it down.
func TestAlwaysTerminates(t *testing.T) {
	inputs := []string{
		"",
		"\x00",
		"0.\xd7\x92\x92",           // the input that hung the old lexer
		"\xff\xfe\xfd",             // invalid UTF-8 throughout
		"\"unterminated",           // unterminated string
		"/* unterminated",          // unterminated block comment
		"0x",                       // incomplete radix literal
		strings.Repeat("(", 10000), // deep but flat
		strings.Repeat("\x80", 500),
		strings.Repeat("1..", 1000),
	}

	for _, src := range inputs {
		file := source.NewFile("t.ari", []byte(src))
		bag := diag.New(file)
		bag.SetMax(0)
		s := New(file, bag, 0)

		// Every token must consume at least one byte, so the number of tokens
		// can never exceed the number of bytes plus the final EOF.
		limit := len(src) + 2
		for i := 0; ; i++ {
			if i > limit {
				t.Fatalf("scanning %q produced more than %d tokens", truncate(src), limit)
			}
			if s.Scan().Kind == token.EOF {
				break
			}
		}
	}
}

// Spans must always be within the file and never run backwards, whatever the
// input. A bad span would crash the diagnostic renderer rather than the parser.
func TestSpansStayInBounds(t *testing.T) {
	for _, src := range []string{
		"let x = 1", "0.\xd7\x92\x92", "\"abc", "/*x", "0x", "\xff",
	} {
		file := source.NewFile("t.ari", []byte(src))
		bag := diag.New(file)
		s := New(file, bag, ScanComments)

		for {
			tok := s.Scan()
			if tok.Span.Start < 0 || tok.Span.End < tok.Span.Start || tok.Span.End > file.Size() {
				t.Errorf("%q: bad span %v for %v (file size %d)",
					truncate(src), tok.Span, tok.Kind, file.Size())
			}
			if tok.Kind == token.EOF {
				break
			}
		}
	}
}

func TestEOFRepeats(t *testing.T) {
	file := source.NewFile("t.ari", []byte("1"))
	s := New(file, diag.New(file), 0)

	s.Scan() // the 1
	for i := 0; i < 5; i++ {
		if tok := s.Scan(); tok.Kind != token.EOF {
			t.Fatalf("scan %d after end: got %v, want EOF", i, tok.Kind)
		}
	}
}

func TestDiagnosticRendering(t *testing.T) {
	file := source.NewFile("main.ari", []byte("let a = 1\nlet s = \"oops\nlet b = 2"))
	bag := diag.New(file)
	s := New(file, bag, 0)
	for s.Scan().Kind != token.EOF {
	}

	out := bag.Render()
	for _, want := range []string{"main.ari:2:9", "error:", "unterminated string", "^"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered diagnostic missing %q:\n%s", want, out)
		}
	}
}

func TestDiagnosticCap(t *testing.T) {
	bag := diag.New(source.NewFile("t.ari", []byte("")))
	bag.SetMax(3)
	for i := 0; i < 10; i++ {
		bag.Errorf(source.SpanAt(0), "problem %d", i)
	}
	if bag.Len() != 3 {
		t.Errorf("kept %d diagnostics, want 3", bag.Len())
	}
	if !bag.Capped() {
		t.Error("Capped() = false, want true")
	}
}

func equalKinds(a, b []token.Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func truncate(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}

// A rune by codepoint, which a language that indexes strings by rune had no way
// to write. Both halves are validated where they are written.
func TestScanHexEscapes(t *testing.T) {
	for _, src := range []string{`"\x41"`, `"\u00e9"`, `"\u{1F600}"`, `"\u{e9}"`} {
		toks, bag, _ := scan(t, src, 0)
		if bag.HasErrors() {
			t.Errorf("%s was rejected:\n%s", src, bag.Render())
			continue
		}
		if kinds := kindsOf(toks); len(kinds) == 0 || kinds[0] != token.String {
			t.Errorf("%s did not scan as a String: %v", src, kinds)
		}
	}

	for _, src := range []string{`"\xZZ"`, `"\u00"`, `"\u{110000}"`, `"\u{D800}"`, `"\u{}"`} {
		if _, bag, _ := scan(t, src, 0); !bag.HasErrors() {
			t.Errorf("%s was accepted", src)
		}
	}
}

// A backtick literal spans lines and processes no escapes.
func TestScanRawString(t *testing.T) {
	toks, bag, _ := scan(t, "`line one\nline two`", 0)
	if bag.HasErrors() {
		t.Fatalf("a raw string was rejected:\n%s", bag.Render())
	}
	if kinds := kindsOf(toks); len(kinds) == 0 || kinds[0] != token.String {
		t.Errorf("did not scan as a String: %v", kinds)
	}

	if _, bag, _ := scan(t, "`never closed", 0); !bag.HasErrors() {
		t.Error("an unterminated raw string was accepted")
	}
}
