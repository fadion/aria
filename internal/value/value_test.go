package value

import (
	"math"
	"strings"
	"testing"
)

// Float display must not use fixed decimals, and must keep a Float visibly a
// Float (docs/architecture.md).
func TestFloatFormatting(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{1.5, "1.5"},
		{0.1, "0.1"},
		{3.0, "3.0"},
		{-2.5, "-2.5"},
		{1000, "1000.0"},
		{1e-5, "1e-05"},
		{1e21, "1e+21"},
		{0, "0.0"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
		{math.NaN(), "NaN"},
	}
	for _, test := range tests {
		if got := Float(test.in).String(); got != test.want {
			t.Errorf("Float(%v): got %q, want %q", test.in, got, test.want)
		}
	}
}

// A string inside a collection is quoted, so an array of strings is
// distinguishable from an array of atoms.
func TestNestedStringsAreQuoted(t *testing.T) {
	if got := String("hi").String(); got != "hi" {
		t.Errorf("String display: got %q, want %q", got, "hi")
	}
	if got := String("hi").Inspect(); got != `"hi"` {
		t.Errorf("nested display: got %q, want %q", got, `"hi"`)
	}

	arr := NewArray([]Value{String("a"), Atom("a"), Int(1)})
	if got, want := arr.String(), `["a", :a, 1]`; got != want {
		t.Errorf("array display: got %q, want %q", got, want)
	}
}

func TestEmptyCollectionsAreDistinguishable(t *testing.T) {
	if got := NewArray(nil).String(); got != "[]" {
		t.Errorf("empty array: got %q, want []", got)
	}
	if got := NewDict(nil).String(); got != "[=>]" {
		t.Errorf("empty dictionary: got %q, want [=>]", got)
	}
}

// Dictionary keys carry their type, so `1` and `"1"` are distinct entries. The
// old runtime compared display strings and merged them, with which one won
// depending on Go's map iteration order.
func TestDictKeysAreTypedAndDistinct(t *testing.T) {
	d := NewDict([]Pair{
		{Key: Int(1), Value: String("int one")},
		{Key: String("1"), Value: String("string one")},
	})

	if d.Len() != 2 {
		t.Fatalf("got %d entries, want 2 — the keys collided", d.Len())
	}

	v, ok := d.Get(Int(1))
	if !ok || v.String() != "int one" {
		t.Errorf("d[1] = %v (found %v), want int one", v, ok)
	}
	v, ok = d.Get(String("1"))
	if !ok || v.String() != "string one" {
		t.Errorf(`d["1"] = %v (found %v), want string one`, v, ok)
	}
}

// Lookup must be reproducible: the same dictionary must answer the same way
// every time, which the old string-comparison scan did not guarantee.
func TestDictLookupIsDeterministic(t *testing.T) {
	d := NewDict([]Pair{
		{Key: Int(1), Value: String("a")},
		{Key: String("1"), Value: String("b")},
		{Key: Atom("1"), Value: String("c")},
	})
	for i := 0; i < 50; i++ {
		v, _ := d.Get(Int(1))
		if v.String() != "a" {
			t.Fatalf("run %d: d[1] = %v, want a", i, v)
		}
		if got := d.String(); !strings.HasPrefix(got, "[1 => ") {
			t.Fatalf("run %d: display order changed: %s", i, got)
		}
	}
}

// `1 == 1.0` is true, so they must also be the same dictionary key — otherwise
// equality and lookup would disagree.
func TestIntAndIntegralFloatShareAKey(t *testing.T) {
	d := NewDict([]Pair{{Key: Int(2), Value: String("two")}})
	if v, ok := d.Get(Float(2.0)); !ok || v.String() != "two" {
		t.Errorf("d[2.0] did not reach the entry keyed by 2")
	}
	if !Equal(Int(2), Float(2.0)) {
		t.Error("Equal(2, 2.0) = false, want true")
	}
	// A non-integral float is its own key.
	if _, ok := d.Get(Float(2.5)); ok {
		t.Error("d[2.5] matched the entry keyed by 2")
	}
}

func TestUnhashableKeysAreRejected(t *testing.T) {
	for _, v := range []Value{NewArray(nil), NewDict(nil)} {
		if _, ok := KeyOf(v); ok {
			t.Errorf("%s was accepted as a dictionary key", v.Type())
		}
	}
}

func TestEquality(t *testing.T) {
	tests := []struct {
		a, b Value
		want bool
	}{
		{Int(1), Int(1), true},
		{Int(1), Int(2), false},
		{Int(1), Float(1), true},
		{Float(1.5), Float(1.5), true},
		{String("a"), String("a"), true},
		{String("a"), Atom("a"), true},
		{Atom("a"), String("a"), true},
		{String("a"), Int(1), false},
		{NilValue, NilValue, true},
		{NilValue, False, false},
		{True, True, true},
		{NewArray([]Value{Int(1), Int(2)}), NewArray([]Value{Int(1), Int(2)}), true},
		{NewArray([]Value{Int(1)}), NewArray([]Value{Int(2)}), false},
		{NewArray([]Value{Int(1)}), NewArray([]Value{Int(1), Int(2)}), false},
		{
			NewDict([]Pair{{Key: Atom("a"), Value: Int(1)}}),
			NewDict([]Pair{{Key: Atom("a"), Value: Int(1)}}),
			true,
		},
		{
			NewDict([]Pair{{Key: Atom("a"), Value: Int(1)}}),
			NewDict([]Pair{{Key: Atom("a"), Value: Int(2)}}),
			false,
		},
	}
	for _, test := range tests {
		if got := Equal(test.a, test.b); got != test.want {
			t.Errorf("Equal(%s, %s) = %v, want %v", test.a.Inspect(), test.b.Inspect(), got, test.want)
		}
	}
}

// Dictionary equality ignores insertion order: two dictionaries with the same
// entries are the same value.
func TestDictEqualityIgnoresOrder(t *testing.T) {
	a := NewDict([]Pair{{Key: Atom("x"), Value: Int(1)}, {Key: Atom("y"), Value: Int(2)}})
	b := NewDict([]Pair{{Key: Atom("y"), Value: Int(2)}, {Key: Atom("x"), Value: Int(1)}})
	if !Equal(a, b) {
		t.Error("dictionaries with the same entries in a different order compared unequal")
	}
}

func TestTruthiness(t *testing.T) {
	falsy := []Value{NilValue, False, Int(0), Float(0), String(""), NewArray(nil), NewDict(nil)}
	for _, v := range falsy {
		if Truthy(v) {
			t.Errorf("%s is truthy, want falsy", v.Inspect())
		}
	}
	truthy := []Value{True, Int(1), Int(-1), Float(0.1), String("x"), Atom("a"),
		NewArray([]Value{Int(1)}), NewDict([]Pair{{Key: Int(1), Value: Int(1)}})}
	for _, v := range truthy {
		if !Truthy(v) {
			t.Errorf("%s is falsy, want truthy", v.Inspect())
		}
	}
}

// Every collection operation returns a new value, which is what makes operators
// that corrupt their own operands impossible to express.
func TestCollectionsAreImmutable(t *testing.T) {
	base := NewArray([]Value{Int(1), Int(2), Int(3)})

	b := base.Append(Int(98))
	c := base.Append(Int(99))

	if base.Len() != 3 {
		t.Errorf("Append modified the receiver: length is now %d", base.Len())
	}
	if b.At(3).String() != "98" {
		t.Errorf("b was changed by the later append: %s", b.String())
	}
	if c.At(3).String() != "99" {
		t.Errorf("c = %s, want the 99 append", c.String())
	}

	// Set must not touch the original either.
	d := base.Set(0, Int(100))
	if base.At(0).String() != "1" {
		t.Errorf("Set modified the receiver: %s", base.String())
	}
	if d.At(0).String() != "100" {
		t.Errorf("Set result = %s", d.String())
	}

	// Concat leaves both operands alone.
	x := NewArray([]Value{Int(1)})
	y := NewArray([]Value{Int(2)})
	_ = x.Concat(y)
	if x.Len() != 1 || y.Len() != 1 {
		t.Error("Concat modified an operand")
	}
}

func TestDictOperationsAreImmutable(t *testing.T) {
	base := NewDict([]Pair{{Key: Atom("k"), Value: Int(1)}})

	extended := base.With(Atom("j"), Int(2))
	if base.Len() != 1 {
		t.Errorf("With modified the receiver: %s", base.String())
	}
	if extended.Len() != 2 {
		t.Errorf("With result has %d entries, want 2", extended.Len())
	}

	left := NewDict([]Pair{{Key: Atom("k"), Value: Int(1)}})
	right := NewDict([]Pair{{Key: Atom("j"), Value: Int(2)}})
	merged := left.Merge(right)

	if left.Len() != 1 || right.Len() != 1 {
		t.Error("Merge modified an operand")
	}
	if _, ok := right.Get(Atom("k")); ok {
		t.Error("Merge folded the left operand into the right one")
	}
	if merged.Len() != 2 {
		t.Errorf("merged has %d entries, want 2", merged.Len())
	}
}

// A repeated key keeps its original position and takes the later value, so
// display stays stable while the value updates.
func TestDictRepeatedKey(t *testing.T) {
	d := NewDict([]Pair{
		{Key: Atom("a"), Value: Int(1)},
		{Key: Atom("b"), Value: Int(2)},
		{Key: Atom("a"), Value: Int(3)},
	})
	if d.Len() != 2 {
		t.Fatalf("got %d entries, want 2", d.Len())
	}
	if got, want := d.String(), "[:a => 3, :b => 2]"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestDictWithout(t *testing.T) {
	d := NewDict([]Pair{
		{Key: Atom("a"), Value: Int(1)},
		{Key: Atom("b"), Value: Int(2)},
	})
	out := d.Without(Atom("a"))

	if d.Len() != 2 {
		t.Error("Without modified the receiver")
	}
	if out.Len() != 1 {
		t.Fatalf("result has %d entries, want 1", out.Len())
	}
	if _, ok := out.Get(Atom("a")); ok {
		t.Error("the removed key is still present")
	}
}

// Strings index by rune, so a non-ASCII string behaves the way a reader expects
// . The old runtime indexed bytes and split multi-byte characters.
func TestStringRunes(t *testing.T) {
	s := String("héllo")
	runes := s.Runes()
	if len(runes) != 5 {
		t.Fatalf("got %d runes, want 5", len(runes))
	}
	if string(runes[1]) != "é" {
		t.Errorf("rune 1 is %q, want é", string(runes[1]))
	}
}

func TestTypeNames(t *testing.T) {
	tests := []struct {
		v    Value
		want string
	}{
		{NilValue, "Nil"},
		{True, "Bool"},
		{Int(1), "Int"},
		{Float(1), "Float"},
		{String(""), "String"},
		{Atom(""), "Atom"},
		{NewArray(nil), "Array"},
		{NewDict(nil), "Dictionary"},
	}
	for _, test := range tests {
		if got := test.v.Type().String(); got != test.want {
			t.Errorf("%T: got %q, want %q", test.v, got, test.want)
		}
	}
}
