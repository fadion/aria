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
type Module struct {
	Name    string
	members map[string]value.Value
	order   []string
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
		scope.define(p.Name.Value, i.eval(p.Default, fn.Env))
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

	if decl.ReturnType != nil && result.Type().String() != decl.ReturnType.Value {
		i.failIn(callerFile, span, "%s declares it returns %s but returned %s",
			fnName(decl), decl.ReturnType.Value, result.Type())
	}
	return result
}

func (i *Interp) checkParamType(p *ast.FunctionParameter, arg value.Value, file *source.File, span source.Span) {
	if p.Type == nil {
		return
	}
	if arg.Type().String() != p.Type.Value {
		i.failIn(file, span, "parameter '%s' expects %s, got %s", p.Name.Value, p.Type.Value, arg.Type())
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
	i.modules[n.Name.Value] = m

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

func (i *Interp) evalModuleAccess(n *ast.ModuleAccess, e *env) value.Value {
	if m, ok := i.modules[n.Object.Value]; ok {
		v, found := m.Member(n.Parameter.Value)
		if !found {
			i.fail(n.Parameter.Span(), "module '%s' has no member '%s'",
				n.Object.Value, n.Parameter.Value)
		}
		return v
	}

	// A dictionary bound to a name supports dotted access to its keys, which
	// keeps `config.host` working for plain data.
	if v, ok := e.lookup(n.Object.Value); ok {
		if d, isDict := v.(*value.Dict); isDict {
			// One lookup covers both spellings: an Atom keys as the String of
			// its text, so `config.host` finds :host and "host" alike.
			if member, found := d.Get(value.String(n.Parameter.Value)); found {
				return member
			}
			i.fail(n.Parameter.Span(), "no key '%s' in the dictionary", n.Parameter.Value)
		}
	}

	i.fail(n.Object.Span(), "module '%s' is not defined", n.Object.Value)
	return value.NilValue
}
