package interp_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fadion/aria/internal/interp"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/value"
)

func newSession(t *testing.T, out *strings.Builder) *interp.Session {
	t.Helper()
	s, err := interp.NewSession(interp.Options{
		Out: out, Err: out, In: strings.NewReader(""),
	})
	if err != nil {
		t.Fatalf("starting a session: %v", err)
	}
	return s
}

// A REPL reading one line at a time cannot enter a multi-line func, module, if
// or for at all, which is most of what a REPL is for. Eval says ErrIncomplete
// when more input could complete the fragment.
func TestSessionIncompleteInput(t *testing.T) {
	var out strings.Builder
	s := newSession(t, &out)

	incomplete := []string{
		"let f = func (x) do",
		"let f = func (x) do\n  x * 2",
		"if true",
		"for i in 1..3",
		"module M",
		"switch 1\ncase 1 then 2",
		"let a = [",
		"let s = `unterminated",
	}
	for _, src := range incomplete {
		if _, err := s.Eval(src); !errors.Is(err, interp.ErrIncomplete) {
			t.Errorf("%q: got %v, want ErrIncomplete", src, err)
		}
	}

	// Something actually wrong is not incomplete. `println(` is not in this
	// list on purpose: a call's arguments may span lines, so it genuinely is
	// unfinished rather than wrong.
	for _, src := range []string{"let 1 = 2", "let a = )", `let s = "unterminated`} {
		if _, err := s.Eval(src); errors.Is(err, interp.ErrIncomplete) {
			t.Errorf("%q: reported incomplete, expected a real error", src)
		}
	}
}

// Buffering until it parses is what a REPL does with ErrIncomplete, and the
// completed fragment has to run as one unit.
func TestSessionMultiLineConstructs(t *testing.T) {
	var out strings.Builder
	s := newSession(t, &out)

	fragment := "let double = func (x) do\n  x * 2\nend"
	if _, err := s.Eval(fragment); err != nil {
		t.Fatalf("%q: %v", fragment, err)
	}

	v, err := s.Eval("double(21)")
	if err != nil {
		t.Fatalf("calling it back: %v", err)
	}
	if v.String() != "42" {
		t.Errorf("got %s, want 42", v)
	}

	// A name declared across lines is carried forward like any other.
	if _, err := s.Eval("module M\n  let a = 1\nend"); err != nil {
		t.Fatalf("declaring a module: %v", err)
	}
	if v, err := s.Eval("M.a"); err != nil || v.String() != "1" {
		t.Errorf("got %v, %v; want 1", v, err)
	}
}

// A statement that produced nothing has nothing to print: half a session used to
// be `nil` from println calls.
func TestSessionNilResults(t *testing.T) {
	var out strings.Builder
	s := newSession(t, &out)

	v, err := s.Eval(`println("hi")`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if v != value.NilValue {
		t.Errorf("println returned %v, want nil", v)
	}
	if out.String() != "hi\n" {
		t.Errorf("printed %q", out.String())
	}
}

func TestSessionIntrospection(t *testing.T) {
	var out strings.Builder
	s := newSession(t, &out)

	if names := s.Declared(); len(names) != 0 {
		t.Errorf("a fresh session declares %v", names)
	}
	if _, err := s.Eval("let a = 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval("var b = 2"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.Declared(), ","); got != "a,b" {
		t.Errorf("got %q, want a,b", got)
	}

	mods := strings.Join(s.Modules(), ",")
	for _, want := range []string{"Enum", "String", "Math", "Dict", "Type"} {
		if !strings.Contains(mods, want) {
			t.Errorf("%s missing from %q", want, mods)
		}
	}
}

// Check runs the pipeline as far as resolution and no further, which is what an
// editor or a CI step wants.
func TestCheckDoesNotEvaluate(t *testing.T) {
	var out strings.Builder
	file := source.NewFile("test.ari", []byte(`println("SHOULD NOT PRINT")`))
	if !interp.Check(file, interp.Options{Err: &out}) {
		t.Errorf("a valid file failed to check:\n%s", out.String())
	}
	if out.Len() != 0 {
		t.Errorf("check wrote %q", out.String())
	}

	// A name error is caught without anything running.
	out.Reset()
	bad := source.NewFile("test.ari", []byte("println(nope)"))
	if interp.Check(bad, interp.Options{Err: &out}) {
		t.Error("an undefined name passed the check")
	}
	if !strings.Contains(out.String(), "'nope' is not defined") {
		t.Errorf("check reported %q", out.String())
	}

	// So is a runtime-only mistake that the resolver can see.
	out.Reset()
	typo := source.NewFile("test.ari", []byte("println(Enum.sizze([1]))"))
	if interp.Check(typo, interp.Options{Err: &out}) {
		t.Error("a mistyped module member passed the check")
	}
}
