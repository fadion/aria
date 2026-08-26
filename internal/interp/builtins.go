package interp

import (
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"

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

	// floor and ceil are primitives, not arithmetic. Written in Aria in terms
	// of `nr % 1` they inherited Go's math.Mod, which takes the sign of the
	// dividend, so for negative input the two returned each other's answers.
	def("runtime_floor", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		return value.Int(int64(math.Floor(ip.wantNumber("runtime_floor", args, span))))
	})
	def("runtime_ceil", func(ip *Interp, args []value.Value, span source.Span) value.Value {
		return value.Int(int64(math.Ceil(ip.wantNumber("runtime_ceil", args, span))))
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
