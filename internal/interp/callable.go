package interp

import (
	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/value"
)

// Function is a user-defined function closed over its defining scope.
//
// File is where the body was written, which the interpreter running the call
// need not know otherwise: a standard library function is called by the
// interpreter for the user's file, and a fault in its body belongs to the
// library's text, not the user's.
type Function struct {
	Decl *ast.Function
	Env  *env
	File *source.File
}

func (*Function) Type() value.Type { return value.TFunc }
func (f *Function) String() string {
	return "func(" + paramNames(f.Decl) + ")"
}
func (f *Function) Inspect() string { return f.String() }

func paramNames(d *ast.Function) string {
	out := ""
	for i, p := range d.Parameters {
		if i > 0 {
			out += ", "
		}
		if d.Variadic && i == len(d.Parameters)-1 {
			out += "..."
		}
		out += p.Name.Value
	}
	return out
}

// Builtin is a function implemented in Go.
type Builtin struct {
	Name string
	// Fn receives evaluated arguments and the interpreter, so a builtin can
	// reach the configured input and output streams.
	Fn func(i *Interp, args []value.Value, span source.Span) value.Value
}

func (*Builtin) Type() value.Type  { return value.TFunc }
func (b *Builtin) String() string  { return "builtin " + b.Name }
func (b *Builtin) Inspect() string { return b.String() }

// Module is a named container of values.
//
// A module is an ordinary value: it lives in the environment under its name, so
// it can be bound, passed and returned like anything else.
//
// That exposed a collision the two-namespace design used to hide. `String` is
// both a conversion builtin and a standard library module, and as a value the
// name has to mean one thing. It means both: a module that shadows a builtin of
// its own name carries it, so `String("x")` converts and `String.join(...)`
// reads a member, and `let S = String` gets a value that still does both.
type Module struct {
	Name    string
	members map[string]value.Value
	order   []string
	call    *Builtin
}

func (*Module) Type() value.Type  { return value.TModule }
func (m *Module) String() string  { return "module " + m.Name }
func (m *Module) Inspect() string { return m.String() }

// Member looks a module member up.
func (m *Module) Member(name string) (value.Value, bool) {
	v, ok := m.members[name]
	return v, ok
}

// Names lists the module's members in declaration order.
func (m *Module) Names() []string { return m.order }

// evalCall evaluates a call expression.
func (i *Interp) evalCall(n *ast.FunctionCall, e *env) value.Value {
	callee := i.eval(n.Function, e)

	args := make([]value.Value, 0, len(n.Arguments.Elements))
	for _, a := range n.Arguments.Elements {
		args = append(args, i.eval(a, e))
	}
	return i.apply(callee, args, n.Span(), e)
}

// apply invokes a callable with already-evaluated arguments.
func (i *Interp) apply(callee value.Value, args []value.Value, span source.Span, e *env) value.Value {
	switch fn := callee.(type) {
	case *Builtin:
		return fn.Fn(i, args, span)
	case *Function:
		return i.callFunction(fn, args, span)
	case *Module:
		if fn.call != nil {
			return fn.call.Fn(i, args, span)
		}
	case *RecordDef:
		// A record's fields are a parameter list, so constructing one is an
		// ordinary call. That is why a record needs no construction syntax.
		return i.construct(fn, args, span)
	}
	i.fail(span, "cannot call %s", callee.Type())
	return value.NilValue
}

// callFunction binds arguments to parameters and runs the body.
//
// Everything reported against span — arity, parameter and return types — is a
// fault at the *call site*, so it is located in the caller's file. Everything
// reported from inside the body, defaults included, belongs to the file the
// body was written in, which is what the pushed frame supplies.
func (i *Interp) callFunction(fn *Function, args []value.Value, span source.Span) value.Value {
	callerFile := i.curFile()
	if len(i.frames) >= maxCallDepth {
		i.failIn(callerFile, span, "call depth of %d exceeded, probably infinite recursion", maxCallDepth)
	}
	i.frames = append(i.frames, frame{file: fn.File, span: span})
	defer func() { i.frames = i.frames[:len(i.frames)-1] }()

	decl := fn.Decl
	scope := newEnv(fn.Env)

	fixed := len(decl.Parameters)
	if decl.Variadic {
		fixed--
	}

	// Defaults first, so an omitted argument has something to fall back to.
	required := 0
	for idx, p := range decl.Parameters {
		if p.Default == nil {
			if idx < fixed {
				required++
			}
			continue
		}
		// The default is checked against the annotation too. checkParamType ran
		// only on arguments actually passed, so a default was bound unchecked
		// and the hint was a lie for every caller that omitted the argument.
		def := i.eval(p.Default, fn.Env)
		i.checkParamType(p, def, i.curFile(), p.Default.Span())
		scope.define(p.Name.Value, def)
	}

	if len(args) < required {
		i.failIn(callerFile, span, "%s expects at least %d argument(s), got %d", fnName(decl), required, len(args))
	}
	if !decl.Variadic && len(args) > fixed {
		i.failIn(callerFile, span, "%s expects at most %d argument(s), got %d", fnName(decl), fixed, len(args))
	}

	for idx := 0; idx < fixed && idx < len(args); idx++ {
		p := decl.Parameters[idx]
		i.checkParamType(p, args[idx], callerFile, span)
		scope.define(p.Name.Value, args[idx])
	}

	if decl.Variadic {
		rest := []value.Value{}
		if len(args) > fixed {
			rest = append(rest, args[fixed:]...)
		}
		last := decl.Parameters[len(decl.Parameters)-1]
		scope.define(last.Name.Value, value.NewArray(rest))
	}

	result := i.evalBlock(decl.Body, scope)

	// A return unwinds only as far as its own function.
	if i.signal == sigReturn {
		result = i.retval
		i.signal, i.retval = sigNone, nil
	}

	if decl.ReturnType != nil && !value.Satisfies(result, decl.ReturnType.Value) {
		i.failIn(callerFile, span, "%s declares it returns %s but returned %s",
			fnName(decl), decl.ReturnType.Value, value.TypeName(result))
	}
	return result
}

func (i *Interp) checkParamType(p *ast.FunctionParameter, arg value.Value, file *source.File, span source.Span) {
	if p.Type == nil {
		return
	}
	if !value.Satisfies(arg, p.Type.Value) {
		i.failIn(file, span, "parameter '%s' expects %s, got %s",
			p.Name.Value, p.Type.Value, value.TypeName(arg))
	}
}

func fnName(d *ast.Function) string { return "function (" + paramNames(d) + ")" }

// ---------------------------------------------------------------------------
// Modules
// ---------------------------------------------------------------------------

// evalModule evaluates a module body once, at its declaration.
//
// Members share one scope, so they can refer to each other in any order — the
// README's promise that module contents "have access to each other".
func (i *Interp) evalModule(n *ast.Module, e *env) {
	if _, exists := i.modules[n.Name.Value]; exists {
		i.fail(n.Name.Span(), "module '%s' is already declared", n.Name.Value)
	}

	scope := newEnv(e)
	m := &Module{Name: n.Name.Value, members: map[string]value.Value{}}
	// Register before evaluating, so a member closing over the module can name
	// it, and so a self-referential member resolves.
	//
	// The name is bound in the enclosing scope as well as in the module
	// registry, because a module IS a value: it can be bound, passed and
	// returned. The registry survives for the redeclaration check and for
	// carrying a separately-evaluated standard library into a later run.
	i.modules[n.Name.Value] = m
	// A builtin of the same name is carried rather than shadowed: `String` is
	// both a conversion and a module, and as a value it has to be one thing.
	if prev, ok := e.lookup(n.Name.Value); ok {
		if b, isBuiltin := prev.(*Builtin); isBuiltin {
			m.call = b
		}
	}
	e.define(n.Name.Value, m)

	for _, node := range n.Body.Nodes {
		let, ok := node.(*ast.Let)
		if !ok {
			i.fail(node.Span(), "a module body accepts only 'let' declarations")
		}
		v := i.eval(let.Value, scope)
		scope.define(let.Name.Value, v)
		if _, seen := m.members[let.Name.Value]; !seen {
			m.order = append(m.order, let.Name.Value)
		}
		m.members[let.Name.Value] = v
	}
}

// evalAccess reads a named member of whatever is on the left.
//
// It used to be two branches keyed off a bare identifier — look the name up in
// i.modules, else look for a dictionary bound to it in scope — which is why `.`
// only worked one level deep and only over a name. Now that a module is an
// ordinary value in the environment, there is one rule: evaluate the left side
// and dispatch on what it is. That is what lets `cfg.db.host`, `f().a` and
// `rows[0].name` work, and it removes a special case rather than adding a
// feature.
func (i *Interp) evalAccess(n *ast.Access, e *env) value.Value {
	left := i.eval(n.Left, e)

	// `?.` stops at the first nil link rather than failing, which is what makes
	// a chain of them worth writing.
	if n.Safe && left == value.NilValue {
		return value.NilValue
	}

	switch v := left.(type) {
	case *value.Record:
		if member, found := v.Get(n.Name.Value); found {
			return member
		}
		i.fail(n.Name.Span(), "%s has no field '%s'", v.Def.Name, n.Name.Value)

	case *Module:
		member, found := v.Member(n.Name.Value)
		if !found {
			i.fail(n.Name.Span(), "module '%s' has no member '%s'", v.Name, n.Name.Value)
		}
		return member

	case *value.Dict:
		// One lookup covers both spellings: an Atom keys as the String of its
		// text, so `config.host` finds :host and "host" alike.
		if member, found := v.Get(value.String(n.Name.Value)); found {
			return member
		}
		if n.Safe {
			// A missing key is a nil link, and `?.` is how a caller says it
			// expects one. Subscripting already answers nil for the same read.
			return value.NilValue
		}
		i.fail(n.Name.Span(), "no key '%s' in the dictionary", n.Name.Value)
	}

	i.fail(n.Span(), "cannot read '.%s' from %s", n.Name.Value, left.Type())
	return value.NilValue
}
