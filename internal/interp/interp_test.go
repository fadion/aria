package interp_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fadion/aria/internal/interp"
	"github.com/fadion/aria/internal/value"
)

// run evaluates src without the standard library and returns its value.
func run(t *testing.T, src string) value.Value {
	t.Helper()
	var out, errOut strings.Builder
	v, err := interp.Eval("test.ari", src, interp.Options{
		Out: &out, Err: &errOut, In: strings.NewReader(""), NoStdlib: true,
	})
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return v
}

// str evaluates src and returns its value rendered for display.
func str(t *testing.T, src string) string {
	t.Helper()
	return run(t, src).String()
}

// output evaluates src and returns what it printed.
func output(t *testing.T, src string) string {
	t.Helper()
	var out, errOut strings.Builder
	if _, err := interp.Eval("test.ari", src, interp.Options{
		Out: &out, Err: &errOut, In: strings.NewReader(""), NoStdlib: true,
	}); err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return out.String()
}

// fails requires src to fail with a message containing want.
func fails(t *testing.T, src, want string) {
	t.Helper()
	var out, errOut strings.Builder
	_, err := interp.Eval("test.ari", src, interp.Options{
		Out: &out, Err: &errOut, In: strings.NewReader(""), NoStdlib: true,
	})
	if err == nil {
		t.Errorf("%q: expected a failure mentioning %q, but it succeeded", src, want)
		return
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("%q: failure did not mention %q:\n%v", src, want, err)
	}
}

// withStdlib evaluates src with the standard library loaded.
func withStdlib(t *testing.T, src string) string {
	t.Helper()
	var out, errOut strings.Builder
	if _, err := interp.Eval("test.ari", src, interp.Options{
		Out: &out, Err: &errOut, In: strings.NewReader(""),
	}); err != nil {
		t.Fatalf("%q: %v\n%s", src, err, errOut.String())
	}
	return strings.TrimRight(out.String(), "\n")
}

// ---------------------------------------------------------------------------
// Literals and display
// ---------------------------------------------------------------------------

func TestLiterals(t *testing.T) {
	tests := []struct{ src, want string }{
		{"42", "42"},
		{"0xff", "255"},
		{"0o27", "23"},
		{"0b1010", "10"},
		{"1_000_000", "1000000"},
		{"1.5", "1.5"},
		{"1e3", "1000.0"},
		{`"hi"`, "hi"},
		{":ok", ":ok"},
		{"true", "true"},
		{"false", "false"},
		{"nil", "nil"},
		{"[1, 2]", "[1, 2]"},
		{"[]", "[]"},
		{"[=>]", "[=>]"},
		{`[:a => 1]`, "[:a => 1]"},
		// The most-negative int64 is only writable with its sign attached.
		{"-9223372036854775808", "-9223372036854775808"},
		{"9223372036854775807", "9223372036854775807"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// Escape sequences are decoded. The old implementation validated them and then
// wrote the backslash back, so nothing ever decoded.
func TestStringEscapes(t *testing.T) {
	tests := []struct{ src, want string }{
		{`"a\tb"`, "a\tb"},
		{`"a\nb"`, "a\nb"},
		{`"say \"hi\""`, `say "hi"`},
		{`"back\\slash"`, `back\slash`},
		{`"a\rb"`, "a\rb"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Arithmetic
// ---------------------------------------------------------------------------

// Int arithmetic stays Int, including division, so a declared return type is
// checkable by reading the code rather than by running it.
func TestIntArithmetic(t *testing.T) {
	tests := []struct{ src, want string }{
		{"7 + 3", "10"},
		{"7 - 3", "4"},
		{"7 * 3", "21"},
		{"10 / 5", "2"},
		{"10 / 4", "2"},
		{"1 / 3", "0"},
		{"-7 / 2", "-3"},
		{"7 % 3", "1"},
		{"2 ** 8", "256"},
		{"2 ** -1", "0"},
		{"typeof(10 / 4)", "Int"},
		{"typeof(10 / 5)", "Int"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

func TestFloatArithmetic(t *testing.T) {
	tests := []struct{ src, want string }{
		{"1.5 + 2.5", "4.0"},
		{"5.0 / 2.0", "2.5"},
		{"10 / 4.0", "2.5"},
		{"1 + 2.5", "3.5"},
		{"2.0 ** -1", "0.5"},
		{"9 ** 0.5", "3.0"},
		{"typeof(1 + 2.0)", "Float"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

func TestDivisionByZero(t *testing.T) {
	fails(t, "1 / 0", "division by zero")
	fails(t, "1 % 0", "division by zero")
}

// The binding-power table, at runtime (2.1 through 2.4).
func TestOperatorPrecedence(t *testing.T) {
	tests := []struct{ src, want string }{
		{"false && false || true", "true"}, // 2.1
		{"true || false && false", "true"}, // 2.1
		{"2 ** 3 ** 2", "512"},             // 2.2
		{"-2 ** 2", "-4"},                  // 2.3
		{"2 ** -1", "0"},                   // 2.3
		{"6 & 3 == 3", "false"},            // 2.4
		{"1 + 2 * 3", "7"},
		{"1 + 2 << 3", "24"},
		{"1..3", "[1, 2, 3]"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// && and || short-circuit, so a failing right side is never reached.
func TestShortCircuit(t *testing.T) {
	if got := str(t, "false && (1 / 0)"); got != "false" {
		t.Errorf("&& did not short-circuit: got %s", got)
	}
	if got := str(t, "true || (1 / 0)"); got != "true" {
		t.Errorf("|| did not short-circuit: got %s", got)
	}
}

// Strings compare lexicographically. The old runtime compared LENGTHS.
func TestStringComparison(t *testing.T) {
	tests := []struct{ src, want string }{
		{`"abc" < "zz"`, "true"},
		{`"a" < "bbb"`, "true"},
		{`"zzz" > "a"`, "true"},
		{`"abc" == "abc"`, "true"},
		{`"abc" + "def"`, "abcdef"},
		{`"a" < "a"`, "false"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Collections
// ---------------------------------------------------------------------------

// Two concatenations off one base must not affect each other.
func TestArrayConcatDoesNotAlias(t *testing.T) {
	const src = `let a = [1, 2, 3, 4, 5]
let b = a + [98]
let c = a + [99]
b`
	if got := str(t, src); got != "[1, 2, 3, 4, 5, 98]" {
		t.Errorf("b was corrupted by the later concat: %s", got)
	}
}

// Merging dictionaries must not modify either operand.
func TestDictMergeDoesNotMutate(t *testing.T) {
	const src = `let a = [:k => 1]
let b = [:j => 2]
let c = a + b
b[:k]`
	if got := str(t, src); got != "nil" {
		t.Errorf("the right operand was modified: b[:k] = %s, want nil", got)
	}
}

func TestIndexing(t *testing.T) {
	tests := []struct{ src, want string }{
		{"[10, 20, 30][0]", "10"},
		{"[10, 20, 30][2]", "30"},
		{"[10, 20, 30][-1]", "30"},
		{"[10, 20, 30][-3]", "10"},
		// Out of bounds reads yield nil rather than failing.
		{"[1, 2][99]", "nil"},
		{"[1, 2][-99]", "nil"},
		{`[:a => 1][:a]`, "1"},
		{`[:a => 1][:missing]`, "nil"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// Strings index by rune, so a multi-byte character is one element.
func TestStringIndexingIsByRune(t *testing.T) {
	tests := []struct{ src, want string }{
		{`"héllo"[0]`, "h"},
		{`"héllo"[1]`, "é"},
		{`"héllo"[2]`, "l"},
		{`"héllo"[-1]`, "o"},
		{`"héllo"[99]`, "nil"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// Writing through a subscript rebinds the name, so a `var` is required and the
// old value is untouched.
func TestSubscriptWriteRebinds(t *testing.T) {
	if got := str(t, "var a = [1, 2]\na[] = 3\na"); got != "[1, 2, 3]" {
		t.Errorf("append: got %s", got)
	}
	if got := str(t, "var a = [1, 2]\na[0] = 9\na"); got != "[9, 2]" {
		t.Errorf("index write: got %s", got)
	}
	if got := str(t, "var d = [:a => 1]\nd[:b] = 2\nd"); got != "[:a => 1, :b => 2]" {
		t.Errorf("dictionary write: got %s", got)
	}
	// Nested writes reach into the inner collection.
	if got := str(t, "var a = [[1, 2], [3, 4]]\na[0][1] = 9\na"); got != "[[1, 9], [3, 4]]" {
		t.Errorf("nested write: got %s", got)
	}
}

func TestSubscriptWriteNeedsVar(t *testing.T) {
	fails(t, "let a = [1, 2]\na[] = 3", "cannot modify 'a'")
	fails(t, "let a = [1, 2]\na[0] = 3", "cannot modify 'a'")
}

func TestRanges(t *testing.T) {
	tests := []struct{ src, want string }{
		{"1..5", "[1, 2, 3, 4, 5]"},
		{"5..1", "[5, 4, 3, 2, 1]"},
		{"3..3", "[3]"},
		{`"a".."e"`, `["a", "b", "c", "d", "e"]`},
		{`"c".."a"`, `["c", "b", "a"]`},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Scoping
// ---------------------------------------------------------------------------

// A name may be shadowed by an inner scope, and the inner binding is what the
// inner code sees.
func TestShadowing(t *testing.T) {
	const src = `let x = 1
let g = func () do
  let x = 2
  x
end
g()`
	if got := str(t, src); got != "2" {
		t.Errorf("got %s, want 2 — the inner binding should win", got)
	}

	// The outer binding survives.
	const src2 = `let x = 1
let g = func () do
  let x = 2
  x
end
g()
x`
	if got := str(t, src2); got != "1" {
		t.Errorf("the outer x is now %s, want 1", got)
	}
}

// Immutability belongs to the binding, not the name.
func TestImmutabilityIsPerBinding(t *testing.T) {
	const src = `let f = func () do
  let counter = 1
  counter
end
f()
var counter = 99
counter = 5
counter`
	if got := str(t, src); got != "5" {
		t.Errorf("got %s, want 5 — a let inside f must not freeze the outer var", got)
	}
}

func TestLetCannotBeReassigned(t *testing.T) {
	fails(t, "let a = 1\na = 2", "cannot assign to 'a'")
	fails(t, "let a = 1\na += 1", "cannot assign to 'a'")
}

// Reassignment preserves the original type.
func TestTypeLock(t *testing.T) {
	if got := str(t, "var n = 1\nn = 2\nn"); got != "2" {
		t.Errorf("same-type reassignment failed: %s", got)
	}
	fails(t, `var n = 1
n = "text"`, "cannot be assigned")
}

// A loop variable does not outlive its loop.
func TestLoopVariableDoesNotLeak(t *testing.T) {
	fails(t, "for v in [1, 2]\n  v\nend\nv", "'v' is not defined")
}

// ---------------------------------------------------------------------------
// Control flow
// ---------------------------------------------------------------------------

func TestIf(t *testing.T) {
	tests := []struct{ src, want string }{
		{"if true then 1 else 2 end", "1"},
		{"if false then 1 else 2 end", "2"},
		{"if false then 1 end", "nil"},
		{"if 0 then 1 else 2 end", "2"},
		{`if "" then 1 else 2 end`, "2"},
		{"if [] then 1 else 2 end", "2"},
		{"if [1] then 1 else 2 end", "1"},
		{"1 > 0 ? 10 : 20", "10"},
		{"1 < 0 ? 10 : 20", "20"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

func TestSwitch(t *testing.T) {
	tests := []struct{ src, want string }{
		{"switch 1 do case 1 then 10 case 2 then 20 end", "10"},
		{"switch 2 do case 1 then 10 case 2 then 20 end", "20"},
		{"switch 9 do case 1 then 10 default then 20 end", "20"},
		// No match and no default yields nil, and must not abort the block.
		{"switch 9 do case 1 then 10 end", "nil"},
		{"switch 1 do case 1 then 10 case 1 then 20 end", "10"},
		// An atom case matches a string control.
		{`switch "ok" do case :ok then 1 default then 2 end`, "1"},
		// A control-less switch chains on true.
		{"switch\ncase 1 == 2\n  10\ncase 1 == 1\n  20\nend", "20"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

func TestSwitchPatternMatching(t *testing.T) {
	tests := []struct{ src, want string }{
		{"switch [1, 2] do case 1, 2 then 10 default then 20 end", "10"},
		{"switch [1, 9] do case 1, _ then 10 default then 20 end", "10"},
		{"switch [1, 9] do case 2, _ then 10 default then 20 end", "20"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// A statement that produces nothing must not discard the rest of its block.
// The old evaluator stopped the moment a statement evaluated to nil.
func TestNilStatementDoesNotAbortBlock(t *testing.T) {
	const src = `let f = func () do
  switch 99
    case 1 then "one"
  end
  println("reached")
  42
end
f()`
	var out strings.Builder
	v, err := interp.Eval("test.ari", src, interp.Options{
		Out: &out, Err: &strings.Builder{}, In: strings.NewReader(""), NoStdlib: true,
	})
	if err != nil {
		t.Fatalf("unexpected failure: %v", err)
	}
	if !strings.Contains(out.String(), "reached") {
		t.Error("the statement after the non-matching switch did not run")
	}
	if v.String() != "42" {
		t.Errorf("got %s, want 42", v.String())
	}
}

func TestForLoops(t *testing.T) {
	tests := []struct{ src, want string }{
		{"for v in [1, 2, 3]\n  v * 2\nend", "[2, 4, 6]"},
		{"for i, v in [10, 20]\n  i\nend", "[0, 1]"},
		{`for c in "abc"
  c
end`, `["a", "b", "c"]`},
		{"for k, v in [:a => 1]\n  k\nend", "[:a]"},
		{"for i in 1..3\n  i\nend", "[1, 2, 3]"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

func TestBreakAndContinue(t *testing.T) {
	if got := str(t, "for i in 1..10\n  if i > 3 then\n    break\n  end\n  i\nend"); got != "[1, 2, 3]" {
		t.Errorf("break: got %s", got)
	}
	if got := str(t, "for i in 1..4\n  if i == 2 then\n    continue\n  end\n  i\nend"); got != "[1, 3, 4]" {
		t.Errorf("continue: got %s", got)
	}
	// An infinite loop terminates on break.
	if got := str(t, "var n = 0\nfor\n  n += 1\n  if n > 2 then\n    break\n  end\n  n\nend"); got != "[1, 2]" {
		t.Errorf("infinite loop: got %s", got)
	}
}

// ---------------------------------------------------------------------------
// Functions
// ---------------------------------------------------------------------------

func TestFunctions(t *testing.T) {
	tests := []struct{ src, want string }{
		{"let f = func (a, b) do a + b end\nf(1, 2)", "3"},
		{"let f = func x do x * 2 end\nf(21)", "42"},
		{"let f = x -> x * 2\nf(21)", "42"},
		{"let f = (a, b) -> a + b\nf(3, 4)", "7"},
		// The last expression is the value.
		{"let f = func () do\n  1\n  2\n  3\nend\nf()", "3"},
		// return exits early.
		{"let f = func (x) do\n  if x > 0 then\n    return 1\n  end\n  2\nend\nf(5)", "1"},
		{"let f = func (x) do\n  if x > 0 then\n    return 1\n  end\n  2\nend\nf(-5)", "2"},
		{"let f = func () do\n  return\nend\nf()", "nil"},
		// Defaults.
		{`let g = func (n, s = "hi") do s + n end
g("!")`, "hi!"},
		{`let g = func (n, s = "hi") do s + n end
g("!", "yo")`, "yo!"},
		// Variadic.
		{"let s = func (...xs) do xs end\ns(1, 2, 3)", "[1, 2, 3]"},
		{"let s = func (...xs) do xs end\ns()", "[]"},
		{"let s = func (a, ...xs) do xs end\ns(1, 2, 3)", "[2, 3]"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

func TestClosuresAndRecursion(t *testing.T) {
	const counter = `let make = func () do
  var n = 0
  func () do
    n += 1
    n
  end
end
let c = make()
c()
c()`
	if got := str(t, counter); got != "2" {
		t.Errorf("closure over mutable state: got %s, want 2", got)
	}

	const fact = `let fact = func (n) do
  if n <= 1 then
    return 1
  end
  n * fact(n - 1)
end
fact(10)`
	if got := str(t, fact); got != "3628800" {
		t.Errorf("recursion: got %s", got)
	}
}

// A closure captures its defining scope, so two closures made in the same loop
// do not share one variable.
func TestClosuresCaptureIndependently(t *testing.T) {
	const src = `let make = func (n) do
  func () do n end
end
let a = make(1)
let b = make(2)
a() + b()`
	if got := str(t, src); got != "3" {
		t.Errorf("got %s, want 3", got)
	}
}

func TestArity(t *testing.T) {
	fails(t, "let f = func (a) do a end\nf(1, 2)", "at most 1")
	fails(t, "let f = func (a, b) do a end\nf(1)", "at least 2")
}

func TestTypeHints(t *testing.T) {
	if got := str(t, "let f = func (n: Int) -> Int\n  n * 2\nend\nf(21)"); got != "42" {
		t.Errorf("got %s, want 42", got)
	}
	fails(t, "let f = func (n: Int) -> Int\n  n\nend\nf(\"x\")", "expects Int")
	fails(t, "let f = func (n: Int) -> String\n  n\nend\nf(1)", "declares it returns String")

	// Int division stays Int, so this signature now holds for every input.
	if got := str(t, "let f = func (n: Int) -> Int\n  n / 4\nend\nf(10)"); got != "2" {
		t.Errorf("got %s, want 2", got)
	}
}

func TestPipe(t *testing.T) {
	const src = `let double = func (x) do x * 2 end
let inc = func (x) do x + 1 end
5 |> double() |> inc()`
	if got := str(t, src); got != "11" {
		t.Errorf("got %s, want 11", got)
	}

	// A piped call inside a loop must not accumulate arguments across
	// iterations, which it would if the pipe rewrote the AST in place.
	const loop = `let double = func (x) do x * 2 end
for v in [1, 2, 3]
  v |> double()
end`
	if got := str(t, loop); got != "[2, 4, 6]" {
		t.Errorf("piped call in a loop: got %s, want [2, 4, 6]", got)
	}
}

func TestCallingANonFunction(t *testing.T) {
	fails(t, "let x = 5\nx(1)", "cannot call")
}

// ---------------------------------------------------------------------------
// Modules
// ---------------------------------------------------------------------------

func TestModules(t *testing.T) {
	const src = `module M
  let a = 1
  let f = func (x) do x + a end
end
M.f(41)`
	if got := str(t, src); got != "42" {
		t.Errorf("got %s, want 42", got)
	}

	// The optional `do` is accepted.
	if got := str(t, "module M do\n  let a = 7\nend\nM.a"); got != "7" {
		t.Errorf("module with do: got %s", got)
	}

	fails(t, "module M\n  let a = 1\nend\nM.nope", "no member 'nope'")
	fails(t, "module M\n  let a = 1\nend\nmodule M\n  let b = 2\nend\nM.a", "already declared")
}

// Module members see each other regardless of declaration order.
func TestModuleMembersAreMutuallyVisible(t *testing.T) {
	const src = `module M
  let f = func () do g() + 1 end
  let g = func () do 41 end
end
M.f()`
	if got := str(t, src); got != "42" {
		t.Errorf("got %s, want 42", got)
	}
}

// ---------------------------------------------------------------------------
// Types, conversion, builtins
// ---------------------------------------------------------------------------

func TestTypeofAndIs(t *testing.T) {
	tests := []struct{ src, want string }{
		{"typeof(1)", "Int"},
		{"typeof(1.0)", "Float"},
		{`typeof("s")`, "String"},
		{"typeof(:a)", "Atom"},
		{"typeof(true)", "Bool"},
		{"typeof([1])", "Array"},
		{"typeof([:a => 1])", "Dictionary"},
		{"typeof(nil)", "Nil"},
		{"1 is Int", "true"},
		{"1 is Float", "false"},
		{"[1] is Array", "true"},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

func TestConversions(t *testing.T) {
	tests := []struct{ src, want string }{
		{"1 as String", "1"},
		{"1.5 as String", "1.5"},
		{`"42" as Int`, "42"},
		{"2.9 as Int", "2"},
		{"true as Int", "1"},
		{`"1.5" as Float`, "1.5"},
		{"3 as Float", "3.0"},
		{"1 as Array", "[1]"},
		{`"ab" as Array`, `["a", "b"]`},
	}
	for _, test := range tests {
		if got := str(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
	fails(t, `"nope" as Int`, "cannot convert")
	fails(t, "[1] as Int", "cannot convert")
}

func TestPrintBuiltins(t *testing.T) {
	if got := output(t, `println("a")`); got != "a\n" {
		t.Errorf("println: got %q", got)
	}
	if got := output(t, `print("a")
print("b")`); got != "ab" {
		t.Errorf("print: got %q", got)
	}
	// Strings nested in a collection are quoted.
	if got := output(t, `println(["a", :a])`); got != `["a", :a]`+"\n" {
		t.Errorf("nested display: got %q", got)
	}
}

// A panic message is data, not a format string.
func TestPanicMessageIsNotAFormatString(t *testing.T) {
	fails(t, `panic("100% done for %s")`, "100% done for %s")
}

// runtime_rand's range is inclusive and min may equal max. The old one called
// rand.Intn(max-min), which panicked the process.
func TestRuntimeRand(t *testing.T) {
	if got := str(t, "runtime_rand(5, 5)"); got != "5" {
		t.Errorf("runtime_rand(5, 5) = %s, want 5", got)
	}
	for i := 0; i < 50; i++ {
		v := run(t, "runtime_rand(1, 3)")
		n, ok := v.(value.Int)
		if !ok || n < 1 || n > 3 {
			t.Fatalf("runtime_rand(1, 3) = %v, outside the range", v)
		}
	}
	fails(t, "runtime_rand(10, 1)", "at least min")
}

// ---------------------------------------------------------------------------
// Failure behavior
// ---------------------------------------------------------------------------

// A runtime error stops the program. The old evaluator carried on with a nil in
// hand, which produced a second misleading message about the call that received
// it.
func TestRuntimeErrorHaltsAndReportsOnce(t *testing.T) {
	var out, errOut strings.Builder
	_, err := interp.Eval("test.ari", `println("before")
println(1 / 0)
println("after")`, interp.Options{
		Out: &out, Err: &errOut, In: strings.NewReader(""), NoStdlib: true,
	})

	if err == nil {
		t.Fatal("expected a failure")
	}
	if !strings.Contains(out.String(), "before") {
		t.Error("output before the error was lost")
	}
	if strings.Contains(out.String(), "after") {
		t.Error("evaluation continued past the error")
	}
	if n := strings.Count(err.Error(), "error"); n > 1 {
		t.Errorf("got %d messages for one mistake:\n%v", n, err)
	}
}

// Names are checked before anything runs, so an error on a path that never
// executes is still reported.
func TestUndefinedNamesAreCaughtBeforeRunning(t *testing.T) {
	var out strings.Builder
	_, err := interp.Eval("test.ari", `println("first")
if false then
  neverDefined
end`, interp.Options{
		Out: &out, Err: &strings.Builder{}, In: strings.NewReader(""), NoStdlib: true,
	})

	if err == nil {
		t.Fatal("expected the undefined name to be reported")
	}
	if strings.Contains(out.String(), "first") {
		t.Error("the program ran before name checking finished")
	}
}

// ---------------------------------------------------------------------------
// Standard library
// ---------------------------------------------------------------------------

func TestStdlibLoads(t *testing.T) {
	if got := withStdlib(t, `println(Enum.size([1, 2, 3]))`); got != "3" {
		t.Errorf("got %s, want 3", got)
	}
}

func TestStdlibEnum(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(Enum.size([1, 2, 3]))`, "3"},
		{`println(Enum.empty?([]))`, "true"},
		{`println(Enum.first([1, 2]))`, "1"},
		{`println(Enum.last([1, 2]))`, "2"},
		{`println(Enum.reverse([1, 2, 3]))`, "[3, 2, 1]"},
		{`println(Enum.map([1, 2], x -> x * 2))`, "[2, 4]"},
		{`println(Enum.filter([1, 2, 3], x -> x > 1))`, "[2, 3]"},
		{`println(Enum.contains?([1, 2], 2))`, "true"},
		{`println(Enum.unique([1, 1, 2]))`, "[1, 2]"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// Enum.insert returns a new array rather than mutating its argument, which is
// what immutable collections require.
func TestStdlibEnumInsertDoesNotMutate(t *testing.T) {
	const src = `let a = [1, 2, 3]
let b = Enum.insert(a, 4)
println(a)
println(b)`
	got := withStdlib(t, src)
	want := "[1, 2, 3]\n[1, 2, 3, 4]"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Enum.random used to call `rand`, which does not exist.
func TestStdlibEnumRandom(t *testing.T) {
	for i := 0; i < 20; i++ {
		got := withStdlib(t, `println(Enum.random([7, 7, 7]))`)
		if got != "7" {
			t.Fatalf("got %s, want 7", got)
		}
	}
}

func TestStdlibDict(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(Dict.size([:a => 1]))`, "1"},
		{`println(Dict.contains?([:a => 1], :a))`, "true"},
		{`println(Dict.empty?([=>]))`, "true"},
		{`println(Dict.insert([:a => 1], :b, 2))`, "[:a => 1, :b => 2]"},
		{`println(Dict.update([:a => 1], :a, 9))`, "[:a => 9]"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// Dict.insert must not modify its argument either.
func TestStdlibDictInsertDoesNotMutate(t *testing.T) {
	const src = `let d = [:a => 1]
let e = Dict.insert(d, :b, 2)
println(d)
println(e)`
	got := withStdlib(t, src)
	want := "[:a => 1]\n[:a => 1, :b => 2]"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStdlibString(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(String.count("hello"))`, "5"},
		{`println(String.upper("abc"))`, "ABC"},
		{`println(String.lower("ABC"))`, "abc"},
		{`println(String.reverse("abc"))`, "cba"},
		{`println(String.starts?("hello", "he"))`, "true"},
		{`println(String.ends?("hello", "lo"))`, "true"},
		{`println(String.contains?("hello", "ell"))`, "true"},
		{`println(String.join(["a", "b"], "-"))`, "a-b"},
		{`println(String.match?("abc123", "[0-9]+"))`, "true"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// String functions built on subscripting now handle non-ASCII correctly, since
// indexing is by rune.
func TestStdlibStringHandlesUnicode(t *testing.T) {
	if got := withStdlib(t, `println(String.reverse("héllo"))`); got != "olléh" {
		t.Errorf("got %q, want %q", got, "olléh")
	}
	if got := withStdlib(t, `println(String.count("héllo"))`); got != "5" {
		t.Errorf("got %s, want 5", got)
	}
}

func TestStdlibMath(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(Math.floor(1.7))`, "1"},
		{`println(Math.ceil(1.2))`, "2"},
		// floor and ceil were `nr % 1` arithmetic, which follows Go's math.Mod
		// in taking the sign of the dividend, so negative input got the other
		// function's answer. They also declared Float and rejected an Int.
		{`println(Math.floor(-2.5))`, "-3"},
		{`println(Math.ceil(-2.5))`, "-2"},
		{`println(Math.floor(3))`, "3"},
		{`println(Math.ceil(-3))`, "-3"},
		{`println(Math.max(3, 7))`, "7"},
		{`println(Math.min(3, 7))`, "3"},
		{`println(Math.abs(-5))`, "5"},
		{`println(Math.pow(2, 8))`, "256"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %s, want %s", test.src, got, test.want)
		}
	}
}

// Recursion past maxCallDepth is an ordinary runtime error. Without the limit
// it grew the goroutine stack to Go's 1 GB ceiling and killed the process.
func TestRecursionIsBounded(t *testing.T) {
	fails(t, `let f = func (n) do f(n + 1) end
f(0)`, "call depth")
}

// A runtime error raised inside a standard library module is located in that
// module's file, not in the file being run. The span is a byte offset into
// <stdlib>, so rendering it against the user's file pointed at nothing.
func TestRuntimeErrorCarriesItsOwnFile(t *testing.T) {
	var out, errOut strings.Builder
	_, err := interp.Eval("test.ari", `println(Dict.delete([:a => 1], :zz))`, interp.Options{
		Out: &out, Err: &errOut, In: strings.NewReader(""),
	})
	if err == nil {
		t.Fatal("expected a failure")
	}
	var re *interp.Error
	if !errors.As(err, &re) {
		t.Fatalf("expected an *interp.Error, got %T", err)
	}
	if re.File == nil {
		t.Fatal("the error carries no file")
	}
	if !strings.HasPrefix(re.File.Name, "<stdlib>/") {
		t.Errorf("error located in %q, want a <stdlib> module", re.File.Name)
	}
}

// join built its result with `+`, which does not coerce, so it could join
// nothing but strings.
func TestStdlibStringJoinRendersNonStrings(t *testing.T) {
	if got := withStdlib(t, `println(String.join([1, 2, 3], ","))`); got != "1,2,3" {
		t.Errorf("got %q, want %q", got, "1,2,3")
	}
}

// `<` and `>` on collections compared lengths while `<=` and `>=` were not
// defined at all — four operators meaning two things on one type, and the one
// they meant is not what they read as.
func TestCollectionsHaveNoOrder(t *testing.T) {
	for _, op := range []string{"<", "<=", ">", ">="} {
		fails(t, `println([1, 2, 3] `+op+` [9])`, "Arrays have no order")
		fails(t, `println([:a => 1] `+op+` [:b => 2])`, "Dictionaries have no order")
	}
}

// An Atom keys as the String of its text, because the two compare equal.
func TestAtomAndStringAreTheSameKey(t *testing.T) {
	if got := output(t, `let d = [:a => 1]
println(d["a"])
println(d[:a])
println(d)`); got != "1\n1\n[:a => 1]\n" {
		t.Errorf("got %q", got)
	}
}

// Integer arithmetic fails rather than wrapping: Aria's Int is one fixed-width
// signed integer with no unsigned counterpart, so a program has no way to
// observe or intend a wrap.
func TestIntArithmeticFailsOnOverflow(t *testing.T) {
	const intMax = "9223372036854775807"
	const intMin = "-9223372036854775807 - 1"

	for _, src := range []string{
		`println(` + intMax + ` + 1)`,
		`println(` + intMax + ` * 2)`,
		`println((` + intMin + `) - 1)`,
		`println(-(` + intMin + `))`,
		`println((` + intMin + `) / -1)`,
		`println(3 ** 40)`,
		`println(2 ** 63)`,
		`println(1 << 63)`,
	} {
		fails(t, src, "Int overflow")
	}

	// A shift count of 64 or more has no answer; Go's is 0.
	fails(t, `println(1 << 100)`, "shift count must be less than 64")

	// Everything that does fit is exact, including the powers that used to
	// round through float64.
	tests := []struct{ src, want string }{
		{`println(2 ** 62)`, "4611686018427387904"},
		{`println(10 ** 18)`, "1000000000000000000"},
		{`println((-2) ** 3)`, "-8"},
		{`println(0 ** 0)`, "1"},
		{`println(2 ** -1)`, "0"},
		{`println(1 ** -3)`, "1"},
		{`println((-1) ** -3)`, "-1"},
		{`println((` + intMin + `) % -1)`, "0"},
		{`println(1 << 62)`, "4611686018427387904"},
	}
	for _, test := range tests {
		if got := output(t, test.src); got != test.want+"\n" {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

// Float % 0.0 fell through to math.Mod and returned NaN while every sibling
// divide-by-zero raised.
func TestFloatModuloByZeroFails(t *testing.T) {
	fails(t, `println(5.0 % 0.0)`, "division by zero")
	fails(t, `println(0 ** -1)`, "division by zero")
}

// A module is an ordinary value: bindable, passable, returnable. It used to
// live only in the interpreter's registry, so the resolver let a bare module
// name through as "not a binding" and the evaluator then failed on it.
func TestModuleIsAValue(t *testing.T) {
	tests := []struct{ src, want string }{
		{`let E = Enum
println(E.size([1, 2, 3]))`, "3"},
		{`println(typeof(Enum))`, "Module"},
		{`let f = func (m) do m.size([1, 2]) end
println(f(Enum))`, "2"},
		// String is both a conversion builtin and a module; the module carries
		// the builtin it shadows, so the one value does both.
		{`println(String(42))`, "42"},
		{`println(String.join([1, 2], "-"))`, "1-2"},
		{`let S = String
println(S(7))`, "7"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

// `.` is an operator over expressions, so it chains over calls and subscripts.
// Each of these used to be a parse error: "cannot be used as a module name".
func TestAccessChains(t *testing.T) {
	tests := []struct{ src, want string }{
		{`let cfg = [:db => [:host => "x"]]
println(cfg.db.host)`, "x"},
		{`let f = func () do [:a => 1] end
println(f().a)`, "1"},
		{`let rows = [[:name => "first"]]
println(rows[0].name)`, "first"},
	}
	for _, test := range tests {
		if got := output(t, test.src); got != test.want+"\n" {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}

	fails(t, `println(1 .foo)`, "cannot read '.foo' from Int")
}

// trim required an explicit character set and stripped exactly one character
// per side, so trim("  hi  ", " ") was " hi ".
func TestStdlibStringTrim(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(String.trim("  hi  "))`, "hi"},
		{`println(String.trim("  hi  ", " "))`, "hi"},
		{`println(String.trimLeft("xxxhi", "x"))`, "hi"},
		{`println(String.trimRight("hixxx", "x"))`, "hi"},
		{`println(String.trim("\t\n hi \r\n"))`, "hi"},
		{`println(String.trim(""))`, ""},
		{`println(String.trim("     "))`, ""},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

func TestStdlibStringCoverage(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(String.isEmpty?(""))`, "true"},
		{`println(String.repeat("ab", 3))`, "ababab"},
		{`println(String.repeat("ab", 0))`, ""},
		{`println(String.padLeft("7", 4, "0"))`, "0007"},
		{`println(String.padRight("7", 4))`, "7   "},
		{`println(String.padLeft("abcd", 2))`, "abcd"},
		{`println(String.indexOf("hello", "l"))`, "2"},
		{`println(String.lastIndexOf("hello", "l"))`, "3"},
		{`println(String.indexOf("hello", "z"))`, "-1"},
		{`println(String.lines("a\nb"))`, `["a", "b"]`},
		{`println(String.words("  a  b\tc "))`, `["a", "b", "c"]`},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

func TestStdlibMathCoverage(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(Math.round(2.5))`, "3"},
		{`println(Math.round(-2.5))`, "-3"},
		{`println(Math.trunc(-2.9))`, "-2"},
		{`println(Math.sign(-3.2))`, "-1"},
		{`println(Math.sign(0))`, "0"},
		{`println(Math.max(3, 7, 1))`, "7"},
		{`println(Math.min(3, 7, 1))`, "1"},
		{`println(Math.clamp(15, 0, 10))`, "10"},
		{`println(Math.sqrt(9))`, "3.0"},
		{`println(Math.log2(8))`, "3.0"},
		{`println(Math.isNaN?(Math.nan))`, "true"},
		{`println(Math.isInfinite?(Math.infinity))`, "true"},
		{`println(Math.isNaN?(1))`, "false"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}

	// An Int answer that does not exist raises, rather than converting out of
	// range and landing on MinInt64.
	var out, errOut strings.Builder
	if _, err := interp.Eval("test.ari", `println(Math.floor(1.0e300))`, interp.Options{
		Out: &out, Err: &errOut, In: strings.NewReader(""),
	}); err == nil {
		t.Error("Math.floor(1e300) succeeded, expected an overflow")
	} else if !strings.Contains(err.Error(), "Int overflow") {
		t.Errorf("failure did not mention an Int overflow: %v", err)
	}
}

// The right side of a pipe had to be a call, and the piped value could only
// land first.
func TestPipeTargets(t *testing.T) {
	tests := []struct{ src, want string }{
		{`let double = func (x) do x * 2 end
println(4 |> double)`, "8"},
		{`let double = func (x) do x * 2 end
println(4 |> double())`, "8"},
		{`let sub = func (a, b) do a - b end
println(3 |> sub(10, _))`, "7"},
		{`let sub = func (a, b) do a - b end
println(3 |> sub(_, 10))`, "-7"},
	}
	for _, test := range tests {
		if got := output(t, test.src); got != test.want+"\n" {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}

	fails(t, `let f = func (a, b) do a end
println(1 |> f(_, _))`, "at most one '_'")

	// The argument list is built per evaluation, so a piped call in a loop does
	// not grow its own arguments.
	if got := output(t, `let three = func (a, b, c) do [a, b, c] end
for i in 1..2
  println(i |> three(_, "b", "c"))
end`); got != "[1, \"b\", \"c\"]\n[2, \"b\", \"c\"]\n" {
		t.Errorf("got %q", got)
	}
}

// Top-level `let` did not hoist, so two functions that call each other could not
// both live at the top level. Wrapping both in a module was the workaround.
func TestTopLevelFunctionsHoist(t *testing.T) {
	src := `let isEven = func (n) do
  if n == 0 then return true end
  isOdd(n - 1)
end

let isOdd = func (n) do
  if n == 0 then return false end
  isEven(n - 1)
end

println(isEven(10))
println(isOdd(10))`
	if got := output(t, src); got != "true\nfalse\n" {
		t.Errorf("got %q", got)
	}

	// The evaluator hoists to match, so the name is bound before its own `let`
	// has run rather than resolving to something that is not there.
	if got := output(t, `println(later())
let later = func () do "hoisted" end`); got != "hoisted\n" {
		t.Errorf("got %q", got)
	}
}

// The line is deliberate on both sides: only function literals, only the top
// level.
func TestHoistingIsNarrow(t *testing.T) {
	fails(t, `let x = x`, "used in its own definition")
	fails(t, `let f = func () do config end
let config = 42`, "'config' is not defined")
	fails(t, `let outer = func () do
  let a = func () do b() end
  let b = func () do 5 end
end`, "'b' is not defined")
	fails(t, `let f = func () do 1 end
let f = func () do 2 end`, "already declared")
}

// Moving String.count, slice, split, replace, join, repeat and reverse onto
// runtime primitives had to keep every answer, including the empty-string edges
// that fell out of walking the subject's characters.
func TestStdlibStringPrimitiveEdges(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(String.split("a,,b", ","))`, `["a", "b"]`},
		{`println(String.split("a,b,", ","))`, `["a", "b", ""]`},
		{`println(String.split("", ","))`, `[""]`},
		{`println(String.split("abc", ""))`, `["a", "b", "c"]`},
		{`println(String.contains?("", ""))`, "false"},
		{`println(String.contains?("abc", ""))`, "true"},
		{`println(String.replace("abc", "", "-"))`, "-a-b-c"},
		{`println(String.indexOf("", ""))`, "-1"},
		{`println(String.join(["a", 1, :b, nil], "-"))`, "a-1-b-nil"},
		// Overlapping separators used to compute a negative slice length and
		// panic. Matches are non-overlapping now.
		{`println(String.replace("aaa", "aa", "b"))`, "ba"},
		// Everything stays rune-indexed.
		{`println(String.slice("héllo", 1, 3))`, "éll"},
		{`println(String.indexOf("héllo", "llo"))`, "2"},
		{`println(String.reverse("héllo"))`, "olléh"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

// There was no sort anywhere in the language. Ordering is the language's own
// `<`, so a pair it cannot compare is an error rather than an invented
// cross-type ranking.
func TestStdlibEnumSort(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(Enum.sort([3, 1, 2]))`, "[1, 2, 3]"},
		{`println(Enum.sort([3, 1.5, 2]))`, "[1.5, 2, 3]"},
		{`println(Enum.sort(["pear", "apple"]))`, `["apple", "pear"]`},
		{`println(Enum.sort([]))`, "[]"},
		{`println(Enum.sortBy(["bbb", "a", "cc"], (s) -> String.count(s)))`, `["a", "cc", "bbb"]`},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}

	var out, errOut strings.Builder
	_, err := interp.Eval("test.ari", `println(Enum.sort([1, "a"]))`, interp.Options{
		Out: &out, Err: &errOut, In: strings.NewReader(""),
	})
	if err == nil || !strings.Contains(err.Error(), "cannot order") {
		t.Errorf("sorting a mixed array: %v", err)
	}
}

func TestStdlibEnumCoverage(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(Enum.sum([1, 2, 3]))`, "6"},
		{`println(Enum.min([3, 1, 2]))`, "1"},
		{`println(Enum.max(["a", "b"]))`, "b"},
		{`println(Enum.count([1, 2, 3, 4], (v) -> v % 2 == 0))`, "2"},
		{`println(Enum.any?([1, 2], (v) -> v > 1))`, "true"},
		{`println(Enum.all?([1, 2], (v) -> v > 1))`, "false"},
		{`println(Enum.take([1, 2, 3], 9))`, "[1, 2, 3]"},
		{`println(Enum.drop([1, 2, 3], 9))`, "[]"},
		{`println(Enum.takeWhile([1, 2, 3, 1], (v) -> v < 3))`, "[1, 2]"},
		{`println(Enum.dropWhile([1, 2, 3, 1], (v) -> v < 3))`, "[3, 1]"},
		{`println(Enum.zip([1, 2, 3], ["a", "b"]))`, `[[1, "a"], [2, "b"]]`},
		{`println(Enum.flatten([1, [2, [3, 4]], 5]))`, "[1, 2, 3, 4, 5]"},
		{`println(Enum.chunk([1, 2, 3, 4, 5], 2))`, "[[1, 2], [3, 4], [5]]"},
		{`println(Enum.indexOf([1, 2, 3], 9))`, "-1"},
		{`println(Enum.each([1], (v) -> v))`, "nil"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

// Dict.contains? walked the entries with Equal, so it found an atom key given
// the equal string while dict[key] did not, and insert/update/delete asked
// `dict[key] != nil`, which cannot tell a missing key from a nil-valued one.
func TestStdlibDictCoverage(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(Dict.keys([:a => 1, :b => 2]))`, "[:a, :b]"},
		{`println(Dict.values([:a => 1, :b => 2]))`, "[1, 2]"},
		{`println(Dict.get([:a => 1], :zz, "none"))`, "none"},
		{`println(Dict.get([:a => 1], :a))`, "1"},
		{`println(Dict.has?([:a => 1], "a"))`, "true"},
		{`println(Dict.has?([:a => 1], :zz))`, "false"},
		{`println(Dict.has?([:a => nil], :a))`, "true"},
		{`println(Dict.delete([:a => nil], :a))`, "[=>]"},
		{`println(Dict.map([:a => 1], (k, v) -> v * 10))`, "[:a => 10]"},
		{`println(Dict.filter([:a => 1, :b => 2], (k, v) -> v > 1))`, "[:b => 2]"},
		{`println(Dict.toPairs([:a => 1]))`, "[[:a, 1]]"},
		{`println(Dict.fromPairs([[:x, 1]]))`, "[:x => 1]"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

// The small surface gaps: xor, the two missing compound assignments, codepoint
// escapes, raw strings, and the two conversions that had a definition and no
// way to reach it.
func TestSmallSyntaxGaps(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println(6 ^ 3)`, "5"},
		// `^` binds between `|` and `&`, which is Python's ordering.
		{`println(1 | 6 ^ 3 & 2)`, "5"},
		{"var n = 10\nn %= 3\nprintln(n)", "1"},
		{"var n = 2\nn **= 8\nprintln(n)", "256"},
		{`println("\x41")`, "A"},
		{`println("\u00e9")`, "é"},
		{`println(String.count("\u{1F600}"))`, "1"},
		{"println(`no \\n escape`)", `no \n escape`},
		{"println(`one\ntwo`)", "one\ntwo"},
		{`println(1 as Bool)`, "true"},
		{`println("" as Bool)`, "false"},
		{`println([[:a, 1]] as Dictionary)`, "[:a => 1]"},
	}
	for _, test := range tests {
		if got := withStdlib(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}

	fails(t, `println(1 as Dictionary)`, "cannot convert Int to Dictionary")
	fails(t, `println([1] as Dictionary)`, "[key, value] pairs")
}

// Printing a message with a value in it used to be a chain of conversions.
func TestStringInterpolation(t *testing.T) {
	tests := []struct{ src, want string }{
		{`let n = 3
println("has #{n} items")`, "has 3 items"},
		{`println("#{1 + 2}")`, "3"},
		// A hole renders the way println renders it: String, not Inspect.
		{`println("#{[1, 2]}")`, "[1, 2]"},
		{`let s = "x"
println("#{s}")`, "x"},
		{`println("no holes")`, "no holes"},
		// A string inside a hole may interpolate in turn.
		{`let n = 3
println("outer #{ "inner #{n}" } done")`, "outer inner 3 done"},
		// `#` is only special before a `{`, and `\#` opts out.
		{`println("hash # alone")`, "hash # alone"},
		{`println("escaped \#{x}")`, "escaped #{x}"},
		// A raw literal processes nothing, interpolation included.
		{"println(`raw #{x}`)", "raw #{x}"},
	}
	for _, test := range tests {
		if got := output(t, test.src); got != test.want+"\n" {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}

	// A name in a hole resolves like any other, and is reported where it is
	// written rather than against a synthetic file.
	fails(t, `println("#{nope}")`, "'nope' is not defined")
}

// println and print take any number of arguments, joined with a space.
func TestPrintIsVariadic(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println("count:", 3)`, "count: 3\n"},
		{`println("a", "b", "c")`, "a b c\n"},
		{`println()`, "\n"},
		{`print("x", "y")`, "x y"},
	}
	for _, test := range tests {
		if got := output(t, test.src); got != test.want {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

// `a[1..3]` was an error, because a subscript index had to be an Int. The range
// is inclusive, negative endpoints count from the end, a descending range keeps
// its order, and anything outside the collection clamps.
func TestSlicing(t *testing.T) {
	tests := []struct{ src, want string }{
		{`println([1, 2, 3, 4, 5][1..3])`, "[2, 3, 4]"},
		{`println([1, 2, 3, 4, 5][0..0])`, "[1]"},
		{`println([1, 2, 3, 4, 5][-2..-1])`, "[4, 5]"},
		{`println([1, 2, 3, 4, 5][3..1])`, "[4, 3, 2]"},
		{`println([1, 2, 3, 4, 5][0..99])`, "[1, 2, 3, 4, 5]"},
		{`println([1, 2, 3, 4, 5][-10..1])`, "[1, 2]"},
		{`println([1, 2, 3, 4, 5][10..20])`, "[]"},
		{`println([][0..2])`, "[]"},
		// Rune-indexed, like the scalar subscript.
		{`println("héllo"[1..3])`, "éll"},
		{`println("héllo"[-2..-1])`, "lo"},
		{`println("héllo"[3..1])`, "llé"},
		{`println("héllo"[9..12])`, ""},
		{`println(""[0..2])`, ""},
	}
	for _, test := range tests {
		if got := output(t, test.src); got != test.want+"\n" {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}

	fails(t, `println([1, 2][1.."a"])`, "cannot apply '..'")
}

// A range in the enumerable position counts rather than materialising, and the
// endpoints are evaluated exactly once either way.
func TestForRangeCountsLazily(t *testing.T) {
	if got := output(t, `var total = 0
for i in 1..1000000
  total += i
end
println(total)`); got != "500000500000\n" {
		t.Errorf("got %q", got)
	}

	if got := output(t, `var down = []
for i in 3..1
  down[] = i
end
println(down)`); got != "[3, 2, 1]\n" {
		t.Errorf("got %q", got)
	}

	// A range that is not two Ints is still the ordinary value.
	if got := output(t, `for c in "a".."c"
  print(c)
end
println()`); got != "abc\n" {
		t.Errorf("got %q", got)
	}

	// The endpoints run once, not once per use.
	if got := output(t, `var calls = 0
let bump = func () do
  calls += 1
  2
end
for i in 1..bump()
  print(i)
end
println()
println(calls)`); got != "12\n1\n" {
		t.Errorf("got %q", got)
	}
}

// while and until evaluate to nil: there is no per-iteration value worth
// collecting, which is why they exist alongside `for`.
func TestWhileAndUntil(t *testing.T) {
	tests := []struct{ src, want string }{
		{`var i = 0
while i < 5
  i += 1
end
println(i)`, "5"},
		{`var i = 0
until i == 3 do
  i += 1
end
println(i)`, "3"},
		{`println(while false do 1 end)`, "nil"},
		{`println(until true do 1 end)`, "nil"},
		{`var k = 0
while true
  k += 1
  if k == 2 then break end
end
println(k)`, "2"},
		{`var odds = []
var n = 0
while n < 5
  n += 1
  if n % 2 == 0 then continue end
  odds[] = n
end
println(odds)`, "[1, 3, 5]"},
		// A return unwinds out of a while, as it does out of a for.
		{`let f = func () do
  while true
    return 7
  end
end
println(f())`, "7"},
	}
	for _, test := range tests {
		if got := output(t, test.src); got != test.want+"\n" {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}
}

// Breaking out of two loops needed a flag variable, which needs a `var`.
func TestNestedBreak(t *testing.T) {
	tests := []struct{ src, want string }{
		{`var found = nil
for row in [[1, 2], [3, 4]]
  for cell in row
    if cell == 3
      found = cell
      break 2
    end
  end
end
println(found)`, "3"},
		// One level still stops only the inner loop.
		{`var seen = []
for a in [1, 2]
  for b in [1, 2]
    seen[] = b
    break
  end
end
println(seen)`, "[1, 1]"},
		// It crosses loop kinds, since a loop is a loop.
		{`var n = 0
while true
  for i in 1..10
    n = i
    break 2
  end
end
println(n)`, "1"},
	}
	for _, test := range tests {
		if got := output(t, test.src); got != test.want+"\n" {
			t.Errorf("%s: got %q, want %q", test.src, got, test.want)
		}
	}

	fails(t, `for a in [1]
  break 2
end`, "'break 2' inside 1 loop(s)")
}

// A `for` whose value nobody reads collects nothing. Invisible to a program;
// the difference is that the result array is never built.
func TestForValueDiscarded(t *testing.T) {
	// Not the last node of its block, so discarded.
	if got := output(t, `var count = 0
for i in 1..5
  count += i
end
println(count)`); got != "15\n" {
		t.Errorf("got %q", got)
	}

	// The last node of a block is left alone, because the block evaluates to it.
	if got := output(t, `let f = func () do
  for i in 1..3
    i * 2
  end
end
println(f())`); got != "[2, 4, 6]\n" {
		t.Errorf("got %q", got)
	}

	// And a `for` in expression position still collects.
	if got := output(t, `let squares = for i in 1..4
  i * i
end
println(squares)`); got != "[1, 4, 9, 16]\n" {
		t.Errorf("got %q", got)
	}
}
