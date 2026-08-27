package interp

import (
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/value"
)

// installBuiltins seeds the global scope with the runtime functions.
func (i *Interp) installBuiltins() {
	def := func(name string, fn func(*Interp, []value.Value, source.Span) value.Value) {
		i.globals.define(name, &Builtin{Name: name, Fn: fn})
	}

	// println and print take exactly one argument, as they always have. The
	// "expects exactly 1 argument" message that used to show up all over the
	// goldens was the cascade from a failed expression yielding nil, not this
	// check, so the arity is left alone.
	def("println", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("println", args, 1, span)
		fmt.Fprintln(ip.Out, args[0].String())
		return value.NilValue
	})

	def("print", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("print", args, 1, span)
		fmt.Fprint(ip.Out, args[0].String())
		return value.NilValue
	})

	def("prompt", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		if len(args) > 0 {
			fmt.Fprint(ip.Out, args[0].String())
		}
		line, _ := ip.stdin().ReadString('\n')
		return value.String(strings.TrimRight(line, "\r\n"))
	})

	def("panic", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		msg := ""
		if len(args) > 0 {
			msg = args[0].String()
		}
		// The message is data, not a format string: panic("100% done") must not
		// come out mangled.
		ip.fail(span, "%s", msg)
		return value.NilValue
	})

	def("typeof", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("typeof", args, 1, span)
		return value.String(args[0].Type().String())
	})

	def("String", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("String", args, 1, span)
		return ip.convert(args[0], "String", span)
	})
	def("Int", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("Int", args, 1, span)
		return ip.convert(args[0], "Int", span)
	})
	def("Float", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("Float", args, 1, span)
		return ip.convert(args[0], "Float", span)
	})
	def("Array", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("Array", args, 1, span)
		return ip.convert(args[0], "Array", span)
	})

	def("runtime_rand", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_rand", args, 2, span)
		lo, ok1 := args[0].(value.Int)
		hi, ok2 := args[1].(value.Int)
		if !ok1 || !ok2 {
			ip.fail(span, "runtime_rand() expects two Ints")
		}
		if hi < lo {
			ip.fail(span, "runtime_rand() expects max to be at least min")
		}
		// The range is inclusive. The original called rand.Intn(max-min), which
		// panicked the whole process when min equalled max.
		return lo + value.Int(rand.N(int64(hi-lo)+1))
	})

	// The rounding functions are primitives, not arithmetic. Written in Aria in
	// terms of `nr % 1` they inherited Go's math.Mod, which takes the sign of
	// the dividend, so for negative input floor and ceil returned each other's
	// answers.
	//
	// They return an Int, and raise when the exact result does not fit in one
	// rather than converting out of range — the same ruling as integer
	// overflow, and for the same reason.
	for _, r := range []struct {
		name string
		fn   func(float64) float64
	}{
		{"runtime_floor", math.Floor},
		{"runtime_ceil", math.Ceil},
		{"runtime_round", math.Round},
		{"runtime_trunc", math.Trunc},
	} {
		def(r.name, func(ip *Interp, args []value.Value, span source.Span) value.Value {
			return ip.wholeNumber(r.fn(ip.wantNumber(r.name, args, span)), span)
		})
	}

	// The transcendental functions are primitives too: writing them in Aria
	// would be numeric analysis in an interpreted language. All the same shape,
	// so they are registered from a table. Every one returns a Float, because
	// none of them has an integer answer in general.
	for _, r := range []struct {
		name string
		fn   func(float64) float64
	}{
		{"runtime_sqrt", math.Sqrt},
		{"runtime_cbrt", math.Cbrt},
		{"runtime_exp", math.Exp},
		{"runtime_log", math.Log},
		{"runtime_log2", math.Log2},
		{"runtime_log10", math.Log10},
		{"runtime_sin", math.Sin},
		{"runtime_cos", math.Cos},
		{"runtime_tan", math.Tan},
		{"runtime_asin", math.Asin},
		{"runtime_acos", math.Acos},
		{"runtime_atan", math.Atan},
	} {
		def(r.name, func(ip *Interp, args []value.Value, span source.Span) value.Value {
			return value.Float(r.fn(ip.wantNumber(r.name, args, span)))
		})
	}

	// Infinity and NaN are values value.formatFloat already renders, and were
	// reachable only by accident. These make them nameable, so Math.isNaN? has
	// something to be asked about.
	def("runtime_inf", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_inf", args, 0, span)
		return value.Float(math.Inf(1))
	})
	def("runtime_nan", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_nan", args, 0, span)
		return value.Float(math.NaN())
	})
	def("runtime_is_nan", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		return value.Of(math.IsNaN(ip.wantNumber("runtime_is_nan", args, span)))
	})
	def("runtime_is_inf", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		return value.Of(math.IsInf(ip.wantNumber("runtime_is_inf", args, span), 0))
	})

	// ---------------------------------------------------------------------
	// Primitives the standard library was reimplementing in Aria
	// ---------------------------------------------------------------------
	//
	// String.count and Enum.size were interpreted loops over data whose length
	// the runtime already knows, and String.slice walked the whole string to
	// take a window out of it. Both were then called from inside per-character
	// loops in split, replace, contains?, starts?, ends? and trim, so each of
	// those was quadratic or worse in its input.
	//
	// Everything here is rune-indexed, matching the subscript rule: "héllo"[1]
	// is é, not a byte of it.
	def("runtime_len", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_len", args, 1, span)
		switch v := args[0].(type) {
		case value.String:
			return value.Int(utf8.RuneCountInString(string(v)))
		case *value.Array:
			return value.Int(v.Len())
		case *value.Dict:
			return value.Int(v.Len())
		}
		ip.fail(span, "runtime_len() expects a String, Array or Dictionary, got %s", args[0].Type())
		return value.NilValue
	})

	// runtime_slice clamps rather than failing, which is what the interpreted
	// version did by construction: it walked the string and took what it found.
	def("runtime_slice", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_slice", args, 3, span)
		start, ok1 := args[1].(value.Int)
		length, ok2 := args[2].(value.Int)
		if !ok1 || !ok2 {
			ip.fail(span, "runtime_slice() expects two Ints for start and length")
		}
		switch v := args[0].(type) {
		case value.String:
			if isASCII(string(v)) {
				lo, hi := clampRange(len(v), int(start), int(length))
				return value.String(string(v)[lo:hi])
			}
			runes := v.Runes()
			lo, hi := clampRange(len(runes), int(start), int(length))
			return value.String(string(runes[lo:hi]))
		case *value.Array:
			lo, hi := clampRange(v.Len(), int(start), int(length))
			out := make([]value.Value, hi-lo)
			copy(out, v.Elems()[lo:hi])
			return value.NewArray(out)
		}
		ip.fail(span, "runtime_slice() expects a String or Array, got %s", args[0].Type())
		return value.NilValue
	})

	// runtime_index_of and runtime_last_index_of search by rune index, so a
	// scan for every match jumps between them instead of testing every
	// position with a slice and a comparison.
	def("runtime_index_of", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		str, search, from := ip.wantSearch("runtime_index_of", args, span)
		return value.Int(int64(runeIndex(str, search, from)))
	})

	def("runtime_last_index_of", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_last_index_of", args, 2, span)
		str, ok1 := args[0].(value.String)
		search, ok2 := args[1].(value.String)
		if !ok1 || !ok2 {
			ip.fail(span, "runtime_last_index_of() expects two Strings")
		}
		runes, needle := []rune(string(str)), []rune(string(search))
		for i := len(runes) - len(needle); i >= 0; i-- {
			if string(runes[i:i+len(needle)]) == string(needle) {
				return value.Int(int64(i))
			}
		}
		return value.Int(-1)
	})

	// split and replace are the two functions the whole quadratic problem came
	// to a head in: each walked the string character by character and called
	// String.slice — itself a walk of the whole string — from inside that loop,
	// so replace was O(n^2) at best. Done once over the runes here, both are
	// linear, and neither can hit the negative-length slice that overlapping
	// separators used to panic on.
	def("runtime_split", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_split", args, 2, span)
		str, ok1 := args[0].(value.String)
		sep, ok2 := args[1].(value.String)
		if !ok1 || !ok2 {
			ip.fail(span, "runtime_split() expects two Strings")
		}

		runes, needle := []rune(string(str)), []rune(string(sep))
		out := []value.Value{}
		last := 0
		for i := runeIndexIn(runes, needle, 0); i >= 0 && i < len(runes); i = runeIndexIn(runes, needle, i+step(len(needle))) {
			// An empty piece between two separators is dropped, which is what
			// the interpreted version did; the final piece is kept either way.
			if i > last {
				out = append(out, value.String(string(runes[last:i])))
			}
			last = i + len(needle)
		}
		out = append(out, value.String(string(runes[last:])))
		return value.NewArray(out)
	})

	def("runtime_replace", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_replace", args, 3, span)
		str, ok1 := args[0].(value.String)
		search, ok2 := args[1].(value.String)
		with, ok3 := args[2].(value.String)
		if !ok1 || !ok2 || !ok3 {
			ip.fail(span, "runtime_replace() expects three Strings")
		}

		runes, needle := []rune(string(str)), []rune(string(search))
		var b strings.Builder
		last := 0
		for i := runeIndexIn(runes, needle, 0); i >= 0 && i < len(runes); i = runeIndexIn(runes, needle, i+step(len(needle))) {
			b.WriteString(string(runes[last:i]))
			b.WriteString(string(with))
			last = i + len(needle)
		}
		b.WriteString(string(runes[last:]))
		return value.String(b.String())
	})

	// Accumulating a string a character at a time allocates a whole new string
	// per character, so the accumulation is itself quadratic. join, repeat and
	// reverse are where the library did that.
	def("runtime_join", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_join", args, 2, span)
		arr, ok1 := args[0].(*value.Array)
		glue, ok2 := args[1].(value.String)
		if !ok1 || !ok2 {
			ip.fail(span, "runtime_join() expects an Array and a String")
		}
		// Elements are rendered with the String conversion, not with their
		// display form: the interpreted version built its result with String(v),
		// and those differ for an Atom — :b converts to "b" and displays as ":b".
		var b strings.Builder
		for i, el := range arr.Elems() {
			if i > 0 {
				b.WriteString(string(glue))
			}
			b.WriteString(string(ip.convert(el, "String", span).(value.String)))
		}
		return value.String(b.String())
	})

	def("runtime_repeat", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_repeat", args, 2, span)
		str, ok1 := args[0].(value.String)
		times, ok2 := args[1].(value.Int)
		if !ok1 || !ok2 {
			ip.fail(span, "runtime_repeat() expects a String and an Int")
		}
		if times < 0 {
			ip.fail(span, "runtime_repeat() expects a non-negative count")
		}
		return value.String(strings.Repeat(string(str), int(times)))
	})

	def("runtime_reverse", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		runes := []rune(ip.wantString("runtime_reverse", args, span))
		for l, r := 0, len(runes)-1; l < r; l, r = l+1, r-1 {
			runes[l], runes[r] = runes[r], runes[l]
		}
		return value.String(string(runes))
	})

	// Sorting had no expression in the language at all: value.SortedKeys exists
	// in Go and was unexported to Aria, so a program could not order a
	// collection by any means.
	//
	// One builtin covers both sort and sortBy: it reorders values by a parallel
	// array of keys, which for a plain sort is the array itself. The ordering is
	// the language's own `<` — numbers among numbers, text among text — so a
	// pair the language cannot compare is an error here too, rather than an
	// invented cross-type order nobody would guess.
	def("runtime_sort", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_sort", args, 2, span)
		keys, ok1 := args[0].(*value.Array)
		vals, ok2 := args[1].(*value.Array)
		if !ok1 || !ok2 {
			ip.fail(span, "runtime_sort() expects two Arrays")
		}
		if keys.Len() != vals.Len() {
			ip.fail(span, "runtime_sort() expects the keys and the values to be the same length")
		}

		idx := make([]int, keys.Len())
		for i := range idx {
			idx[i] = i
		}
		var bad [2]value.Value
		sort.SliceStable(idx, func(a, b int) bool {
			ka, kb := keys.At(idx[a]), keys.At(idx[b])
			c, ok := orderOf(ka, kb)
			if !ok && bad[0] == nil {
				bad[0], bad[1] = ka, kb
			}
			return c < 0
		})
		if bad[0] != nil {
			ip.fail(span, "cannot order %s and %s", bad[0].Type(), bad[1].Type())
		}

		out := make([]value.Value, len(idx))
		for i, at := range idx {
			out[i] = vals.At(at)
		}
		return value.NewArray(out)
	})

	// runtime_has_key answers through the dictionary's index, so it agrees with
	// subscript lookup. Dict.contains? walked the entries comparing with Equal,
	// which found an atom key given the equal string while dict[key] did not,
	// and was O(n) over a structure with an O(1) index.
	def("runtime_has_key", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_has_key", args, 2, span)
		d, ok := args[0].(*value.Dict)
		if !ok {
			ip.fail(span, "runtime_has_key() expects a Dictionary, got %s", args[0].Type())
		}
		if _, keyable := value.KeyOf(args[1]); !keyable {
			return value.False
		}
		_, found := d.Get(args[1])
		return value.Of(found)
	})

	def("runtime_tolower", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		return value.String(strings.ToLower(ip.wantString("runtime_tolower", args, span)))
	})
	def("runtime_toupper", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		return value.String(strings.ToUpper(ip.wantString("runtime_toupper", args, span)))
	})

	def("runtime_regex_match", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		ip.wantArgs("runtime_regex_match", args, 2, span)
		subject, ok1 := args[0].(value.String)
		pattern, ok2 := args[1].(value.String)
		if !ok1 || !ok2 {
			ip.fail(span, "runtime_regex_match() expects two Strings")
		}
		re, err := regexp.Compile(string(pattern))
		if err != nil {
			ip.fail(span, "runtime_regex_match() could not compile %q: %v", string(pattern), err)
		}
		return value.Of(re.MatchString(string(subject)))
	})
}

// orderOf compares two values the way `<` does — numbers among numbers, text
// among text — reporting whether the pair is comparable at all. Nothing else is:
// a total order across every type would have to invent a ranking of types, which
// is exactly the kind of meaning-nobody-would-guess that `<` on collections was
// removed for.
func orderOf(a, b value.Value) (int, bool) {
	if x, ok := numberOf(a); ok {
		if y, ok := numberOf(b); ok {
			switch {
			case x < y:
				return -1, true
			case x > y:
				return 1, true
			}
			return 0, true
		}
		return 0, false
	}
	if x, ok := textOf(a); ok {
		if y, ok := textOf(b); ok {
			return strings.Compare(x, y), true
		}
	}
	return 0, false
}

func numberOf(v value.Value) (float64, bool) {
	switch n := v.(type) {
	case value.Int:
		return float64(n), true
	case value.Float:
		return float64(n), true
	}
	return 0, false
}

func textOf(v value.Value) (string, bool) {
	switch t := v.(type) {
	case value.String:
		return string(t), true
	case value.Atom:
		return string(t), true
	}
	return "", false
}

// step is how far a scan advances past a match. An empty needle matches at every
// position, so it advances by one rather than standing still.
func step(needleLen int) int {
	if needleLen == 0 {
		return 1
	}
	return needleLen
}

// runeIndexIn finds needle in runes at or after from, by rune index, or -1.
func runeIndexIn(runes, needle []rune, from int) int {
	if from < 0 {
		from = 0
	}
	for i := from; i+len(needle) <= len(runes); i++ {
		if string(runes[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}

// runeIndex is runeIndexIn over strings, with a fast path for input that has no
// multi-byte sequence in it: there, a byte offset is a rune index, so the search
// costs no conversion at all.
func runeIndex(str, search string, from int) int {
	if isASCII(str) && isASCII(search) {
		if from < 0 {
			from = 0
		}
		if from > len(str) {
			return -1
		}
		at := strings.Index(str[from:], search)
		if at < 0 {
			return -1
		}
		return from + at
	}
	return runeIndexIn([]rune(str), []rune(search), from)
}

// isASCII reports whether byte offsets and rune indices coincide, which they do
// for any string with no multi-byte sequence in it.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func (i *Interp) wantSearch(name string, args []value.Value, span source.Span) (string, string, int) {
	i.wantArgs(name, args, 3, span)
	str, ok1 := args[0].(value.String)
	search, ok2 := args[1].(value.String)
	from, ok3 := args[2].(value.Int)
	if !ok1 || !ok2 || !ok3 {
		i.fail(span, "%s() expects two Strings and an Int", name)
	}
	return string(str), string(search), int(from)
}

// clampRange turns a start and a length into bounds inside [0, size], the way
// walking the collection and taking what was there did.
func clampRange(size, start, length int) (int, int) {
	if start < 0 {
		start = 0
	}
	if start > size {
		start = size
	}
	if length < 0 {
		length = 0
	}
	end := start + length
	if end > size || end < start {
		end = size
	}
	return start, end
}

func (i *Interp) wantArgs(name string, args []value.Value, n int, span source.Span) {
	if len(args) != n {
		i.fail(span, "%s() expects %d argument(s), got %d", name, n, len(args))
	}
}

// wantNumber takes one Int or Float argument as a float64. Int is accepted
// because floor and ceil of an integer are meaningful, and rejecting it made
// Math.floor(3) an error.
func (i *Interp) wantNumber(name string, args []value.Value, span source.Span) float64 {
	i.wantArgs(name, args, 1, span)
	switch n := args[0].(type) {
	case value.Int:
		return float64(n)
	case value.Float:
		return float64(n)
	}
	i.fail(span, "%s() expects an Int or a Float, got %s", name, args[0].Type())
	return 0
}

// wholeNumber converts an already-rounded float to an Int, raising when it does
// not fit. Converting out of int64's range is undefined in Go and lands on
// MinInt64 on amd64, which is the same silent wrong answer integer overflow
// used to produce.
func (i *Interp) wholeNumber(f float64, span source.Span) value.Value {
	if math.IsNaN(f) || f < math.MinInt64 || f >= -float64(math.MinInt64) {
		i.fail(span, "Int overflow: %s does not fit in an Int", value.Float(f).String())
	}
	return value.Int(int64(f))
}

func (i *Interp) wantString(name string, args []value.Value, span source.Span) string {
	i.wantArgs(name, args, 1, span)
	s, ok := args[0].(value.String)
	if !ok {
		i.fail(span, "%s() expects a String, got %s", name, args[0].Type())
	}
	return string(s)
}

// Convert implements the `as` operator and the conversion builtins. It returns
// a message rather than reporting, so both call sites can attach their own span.
func Convert(v value.Value, to string) (value.Value, string) {
	switch to {
	case "String":
		switch v := v.(type) {
		case value.String:
			return v, ""
		case value.Atom:
			return value.String(string(v)), ""
		case value.Int, value.Float, value.Bool, value.Nil:
			return value.String(v.String()), ""
		}
		return nil, fmt.Sprintf("cannot convert %s to String", v.Type())

	case "Int":
		switch v := v.(type) {
		case value.Int:
			return v, ""
		case value.Float:
			return value.Int(int64(v)), ""
		case value.Bool:
			if v {
				return value.Int(1), ""
			}
			return value.Int(0), ""
		case value.String:
			n, err := strconv.ParseInt(strings.TrimSpace(string(v)), 0, 64)
			if err != nil {
				return nil, fmt.Sprintf("cannot convert %q to Int", string(v))
			}
			return value.Int(n), ""
		}
		return nil, fmt.Sprintf("cannot convert %s to Int", v.Type())

	case "Float":
		switch v := v.(type) {
		case value.Float:
			return v, ""
		case value.Int:
			return value.Float(v), ""
		case value.Bool:
			if v {
				return value.Float(1), ""
			}
			return value.Float(0), ""
		case value.String:
			f, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64)
			if err != nil {
				return nil, fmt.Sprintf("cannot convert %q to Float", string(v))
			}
			return value.Float(f), ""
		}
		return nil, fmt.Sprintf("cannot convert %s to Float", v.Type())

	case "Array":
		switch v := v.(type) {
		case *value.Array:
			return v, ""
		case value.String:
			runes := v.Runes()
			out := make([]value.Value, 0, len(runes))
			for _, r := range runes {
				out = append(out, value.String(string(r)))
			}
			return value.NewArray(out), ""
		case *value.Dict:
			out := make([]value.Value, 0, v.Len())
			for _, p := range v.Pairs() {
				out = append(out, value.NewArray([]value.Value{p.Key, p.Value}))
			}
			return value.NewArray(out), ""
		}
		return value.NewArray([]value.Value{v}), ""
	}

	return nil, fmt.Sprintf("unknown type '%s'", to)
}
