// Package value defines Aria's runtime values.
//
// Two things are kept apart here that the original conflated.
//
// Display and equality. The old runtime used one Inspect method as both the
// pretty-printer and the equality primitive: dictionary lookup, switch matching
// and array comparison all compared formatted strings. That made `1` and `"1"`
// the same dictionary key, made lookup O(n), and made which of two colliding
// keys won depend on Go's map iteration order (docs/architecture.md). Equal
// and Key handle meaning; String and Inspect handle presentation.
//
// Top-level and nested display. `println("hi")` prints hi, but `println(["hi"])`
// prints ["hi"] — a string inside a collection is quoted, so an array of strings
// is distinguishable from an array of atoms. String is the former,
// Inspect the latter.
//
// Collections are immutable. Every operation returns a new value, which makes
// an operator that corrupts its own operand unrepresentable rather than merely
// fixed — the old `+` did exactly that, on both arrays and dictionaries.
package value

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// Type identifies a value's kind. The names are what `typeof` reports and what
// type hints are written against, so they are part of the language.
type Type uint8

const (
	TNil Type = iota
	TBool
	TInt
	TFloat
	TString
	TAtom
	TArray
	TDict
	TFunc
	TModule
)

func (t Type) String() string {
	switch t {
	case TBool:
		return "Bool"
	case TInt:
		return "Int"
	case TFloat:
		return "Float"
	case TString:
		return "String"
	case TAtom:
		return "Atom"
	case TArray:
		return "Array"
	case TDict:
		return "Dictionary"
	case TFunc:
		return "Function"
	case TModule:
		return "Module"
	}
	return "Nil"
}

// A Value is any Aria runtime value.
type Value interface {
	Type() Type
	// String renders the value for direct display.
	String() string
	// Inspect renders the value as it appears inside a collection, where a
	// string needs its quotes to stay distinguishable from an atom or a name.
	Inspect() string
}

// ---------------------------------------------------------------------------
// Scalars
// ---------------------------------------------------------------------------

// Nil is the absence of a value. There is one of it.
type Nil struct{}

var NilValue = Nil{}

func (Nil) Type() Type      { return TNil }
func (Nil) String() string  { return "nil" }
func (Nil) Inspect() string { return "nil" }

// Bool is a boolean. True and False are the only two instances.
type Bool bool

const (
	True  = Bool(true)
	False = Bool(false)
)

func (Bool) Type() Type { return TBool }
func (b Bool) String() string {
	if b {
		return "true"
	}
	return "false"
}
func (b Bool) Inspect() string { return b.String() }

// Of returns the Bool for a Go bool.
func Of(b bool) Bool {
	if b {
		return True
	}
	return False
}

// Int is a 64-bit integer.
type Int int64

func (Int) Type() Type        { return TInt }
func (i Int) String() string  { return strconv.FormatInt(int64(i), 10) }
func (i Int) Inspect() string { return i.String() }

// Float is a 64-bit float.
type Float float64

func (Float) Type() Type        { return TFloat }
func (f Float) String() string  { return formatFloat(float64(f)) }
func (f Float) Inspect() string { return f.String() }

// formatFloat prints a float without the six fixed decimals the old runtime
// used, which turned 1.5 into 1.500000 and lost 1e-5 entirely.
//
// An integral value keeps a ".0" so it stays visibly a Float; Go's shortest
// representation would render 3.0 as "3" and hide the distinction the language
// draws between Int and Float.
func formatFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case math.IsNaN(f):
		return "NaN"
	}

	s := strconv.FormatFloat(f, 'g', -1, 64)
	// 'g' switches to exponent form for large and small magnitudes; leave those
	// alone, and give plain decimals an explicit fractional part.
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// String is a text value.
type String string

func (String) Type() Type        { return TString }
func (s String) String() string  { return string(s) }
func (s String) Inspect() string { return strconv.Quote(string(s)) }

// Runes returns the string's characters. Indexing is by rune, not byte, so a
// non-ASCII string indexes the way a reader expects.
func (s String) Runes() []rune { return []rune(string(s)) }

// Atom is a `:name` symbol.
type Atom string

func (Atom) Type() Type        { return TAtom }
func (a Atom) String() string  { return ":" + string(a) }
func (a Atom) Inspect() string { return a.String() }

// ---------------------------------------------------------------------------
// Collections
// ---------------------------------------------------------------------------

// Array is an immutable sequence.
//
// Nothing mutates elems after construction, so an array can be shared freely and
// no operation can corrupt an operand — which is what 4.1 was.
type Array struct{ elems []Value }

// NewArray takes ownership of elems. Callers must not retain the slice.
func NewArray(elems []Value) *Array { return &Array{elems: elems} }

func (*Array) Type() Type       { return TArray }
func (a *Array) Len() int       { return len(a.elems) }
func (a *Array) At(i int) Value { return a.elems[i] }

// Elems returns the contents for reading. The result must not be modified.
func (a *Array) Elems() []Value { return a.elems }

func (a *Array) String() string {
	parts := make([]string, 0, len(a.elems))
	for _, e := range a.elems {
		parts = append(parts, e.Inspect())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
func (a *Array) Inspect() string { return a.String() }

// Append returns a new array with v added. The receiver is untouched: copying
// rather than growing in place is what stops a later append from writing into
// an earlier result's spare capacity.
func (a *Array) Append(v Value) *Array {
	out := make([]Value, len(a.elems)+1)
	copy(out, a.elems)
	out[len(a.elems)] = v
	return &Array{elems: out}
}

// Concat returns a new array holding a's elements followed by b's.
func (a *Array) Concat(b *Array) *Array {
	out := make([]Value, 0, len(a.elems)+len(b.elems))
	out = append(out, a.elems...)
	out = append(out, b.elems...)
	return &Array{elems: out}
}

// Set returns a new array with index i replaced.
func (a *Array) Set(i int, v Value) *Array {
	out := make([]Value, len(a.elems))
	copy(out, a.elems)
	out[i] = v
	return &Array{elems: out}
}

// Pair is one dictionary entry.
type Pair struct {
	Key   Value
	Value Value
}

// Dict is an immutable mapping.
//
// Entries keep insertion order so printing a dictionary is reproducible, and an
// index gives O(1) lookup by value rather than the old O(n) scan comparing
// formatted strings.
type Dict struct {
	pairs []Pair
	index map[Key]int
}

// NewDict builds a dictionary from pairs in order. A repeated key keeps its
// original position and takes the later value.
func NewDict(pairs []Pair) *Dict {
	d := &Dict{index: make(map[Key]int, len(pairs))}
	for _, p := range pairs {
		k, ok := KeyOf(p.Key)
		if !ok {
			continue // the caller reports unhashable keys
		}
		if at, exists := d.index[k]; exists {
			d.pairs[at].Value = p.Value
			continue
		}
		d.index[k] = len(d.pairs)
		d.pairs = append(d.pairs, p)
	}
	return d
}

func (*Dict) Type() Type      { return TDict }
func (d *Dict) Len() int      { return len(d.pairs) }
func (d *Dict) Pairs() []Pair { return d.pairs }

func (d *Dict) String() string {
	if len(d.pairs) == 0 {
		return "[=>]"
	}
	parts := make([]string, 0, len(d.pairs))
	for _, p := range d.pairs {
		parts = append(parts, p.Key.Inspect()+" => "+p.Value.Inspect())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
func (d *Dict) Inspect() string { return d.String() }

// Get looks a key up.
func (d *Dict) Get(key Value) (Value, bool) {
	k, ok := KeyOf(key)
	if !ok {
		return nil, false
	}
	at, found := d.index[k]
	if !found {
		return nil, false
	}
	return d.pairs[at].Value, true
}

// With returns a new dictionary with key set to v.
func (d *Dict) With(key, v Value) *Dict {
	pairs := make([]Pair, len(d.pairs), len(d.pairs)+1)
	copy(pairs, d.pairs)
	return NewDict(append(pairs, Pair{Key: key, Value: v}))
}

// Without returns a new dictionary with key removed.
func (d *Dict) Without(key Value) *Dict {
	k, ok := KeyOf(key)
	if !ok {
		return d
	}
	pairs := make([]Pair, 0, len(d.pairs))
	for i, p := range d.pairs {
		if pk, valid := KeyOf(p.Key); valid && pk == k && d.index[k] == i {
			continue
		}
		pairs = append(pairs, p)
	}
	return NewDict(pairs)
}

// Merge returns a new dictionary with other's entries laid over d's.
func (d *Dict) Merge(other *Dict) *Dict {
	pairs := make([]Pair, 0, len(d.pairs)+len(other.pairs))
	pairs = append(pairs, d.pairs...)
	pairs = append(pairs, other.pairs...)
	return NewDict(pairs)
}

// ---------------------------------------------------------------------------
// Keys, equality
// ---------------------------------------------------------------------------

// Key is the comparable form of a value, for use as a dictionary key.
//
// It carries the type, so `1` and `"1"` are different keys — the old runtime
// compared display strings and merged them.
type Key struct {
	T   Type
	Num int64
	Str string
}

// KeyOf returns the key for v, or ok=false if v cannot be one. Only scalars can:
// a collection is not usable as a key, since two equal collections would have to
// hash alike and that means walking them on every lookup.
func KeyOf(v Value) (Key, bool) {
	switch v := v.(type) {
	case Nil:
		return Key{T: TNil}, true
	case Bool:
		var n int64
		if v {
			n = 1
		}
		return Key{T: TBool, Num: n}, true
	case Int:
		return Key{T: TInt, Num: int64(v)}, true
	case Float:
		// A float with an exact integer value keys as that integer, so `d[1]`
		// and `d[1.0]` reach the same entry. Anything else would contradict
		// `1 == 1.0`, which the language says is true.
		if f := float64(v); f == math.Trunc(f) && !math.IsInf(f, 0) &&
			f >= math.MinInt64 && f <= math.MaxInt64 {
			return Key{T: TInt, Num: int64(f)}, true
		}
		return Key{T: TFloat, Str: strconv.FormatFloat(float64(v), 'b', -1, 64)}, true
	case String:
		return Key{T: TString, Str: string(v)}, true
	case Atom:
		// An Atom keys as the String of its text, because `:a == "a"` is true.
		// Equality and keying are separate operations but they cannot
		// contradict each other: two values that report equal have to be
		// interchangeable as keys, or `d[:a]` and `d["a"]` reach different
		// entries in a dictionary whose keys are "equal". Only the spelling
		// differs, and the dictionary keeps the key value it was given, so a
		// dictionary written with atoms still prints with atoms.
		return Key{T: TString, Str: string(v)}, true
	}
	return Key{}, false
}

// Equal reports whether two values are the same.
//
// Numbers compare across Int and Float, and an Atom compares equal to the String
// of the same text, both of which the language already did. Collections compare
// element by element rather than by identity.
func Equal(a, b Value) bool {
	switch a := a.(type) {
	case Nil:
		_, ok := b.(Nil)
		return ok
	case Bool:
		bb, ok := b.(Bool)
		return ok && a == bb
	case Int:
		switch b := b.(type) {
		case Int:
			return a == b
		case Float:
			return float64(a) == float64(b)
		}
	case Float:
		switch b := b.(type) {
		case Int:
			return float64(a) == float64(b)
		case Float:
			return a == b
		}
	case String:
		switch b := b.(type) {
		case String:
			return a == b
		case Atom:
			return string(a) == string(b)
		}
	case Atom:
		switch b := b.(type) {
		case Atom:
			return a == b
		case String:
			return string(a) == string(b)
		}
	case *Array:
		bb, ok := b.(*Array)
		if !ok || a.Len() != bb.Len() {
			return false
		}
		for i := range a.elems {
			if !Equal(a.elems[i], bb.elems[i]) {
				return false
			}
		}
		return true
	case *Dict:
		bb, ok := b.(*Dict)
		if !ok || a.Len() != bb.Len() {
			return false
		}
		for _, p := range a.pairs {
			other, found := bb.Get(p.Key)
			if !found || !Equal(p.Value, other) {
				return false
			}
		}
		return true
	}
	// Functions and modules compare by identity.
	return a == b
}

// Truthy reports whether v counts as true in a condition. Zero, empty and nil
// are false; an atom is always true.
func Truthy(v Value) bool {
	switch v := v.(type) {
	case Nil:
		return false
	case Bool:
		return bool(v)
	case Int:
		return v != 0
	case Float:
		return v != 0
	case String:
		return v != ""
	case *Array:
		return v.Len() > 0
	case *Dict:
		return v.Len() > 0
	}
	return true
}

// SortedKeys returns a dictionary's keys in a stable order, for callers that
// need determinism regardless of insertion order.
func SortedKeys(d *Dict) []Value {
	keys := make([]Value, 0, len(d.pairs))
	for _, p := range d.pairs {
		keys = append(keys, p.Key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i].Inspect() < keys[j].Inspect()
	})
	return keys
}
