// Package resolver binds every name in a program to its declaration, before
// anything runs.
//
// The original interpreter resolved names at runtime by walking a chain of maps,
// and tracked immutability in one process-wide map keyed by bare name. That had
// two consequences, both recorded in docs/architecture.md:
//
//   - 3.1 A name could not be shadowed. `let x` inside a function failed if any
//     enclosing scope had an `x`, because the check walked parents.
//   - 3.2 `let` marked the NAME immutable everywhere. A `let counter` inside one
//     function stopped an unrelated top-level `var counter` from being
//     reassigned, for the rest of the process.
//
// Neither is patched here. Declaring into the current scope makes shadowing work
// by construction, and hanging mutability off the binding rather than the name
// makes 3.2 impossible to express.
//
// Resolving early also moves "identifier not found" from a runtime surprise to a
// diagnostic reported before the program starts.
package resolver

import (
	"strings"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/value"
)

// Builtins are the runtime functions available without declaration.
var Builtins = []string{
	"println", "print", "prompt", "panic", "typeof",
	"String", "Int", "Float", "Array",
	"runtime_rand", "runtime_tolower", "runtime_toupper", "runtime_regex_match",
	"runtime_floor", "runtime_ceil", "runtime_round", "runtime_trunc",
	"runtime_sqrt", "runtime_cbrt", "runtime_exp",
	"runtime_log", "runtime_log2", "runtime_log10",
	"runtime_sin", "runtime_cos", "runtime_tan",
	"runtime_asin", "runtime_acos", "runtime_atan",
	"runtime_inf", "runtime_nan", "runtime_is_nan", "runtime_is_inf",
	"runtime_len", "runtime_slice", "runtime_index_of", "runtime_last_index_of",
	"runtime_split", "runtime_replace",
	"runtime_join", "runtime_repeat", "runtime_reverse",
	"runtime_sort", "runtime_has_key",
}

// Kind is what introduced a binding.
type Kind uint8

const (
	KindLet Kind = iota
	KindVar
	KindParam
	KindLoop
	KindBuiltin
)

func (k Kind) String() string {
	switch k {
	case KindVar:
		return "var"
	case KindParam:
		return "parameter"
	case KindLoop:
		return "loop variable"
	case KindBuiltin:
		return "builtin"
	}
	return "let"
}

// A Binding is one declared name.
type Binding struct {
	Name string
	Kind Kind
	// Mutable reports whether the name may be rebound. Only `var` may.
	// Parameters and loop variables bind like `let`, which immutability needs:
	// a function must not be able to rebind a collection its caller passed in.
	Mutable bool
	Decl    source.Span

	// Depth is the scope nesting level, 0 for globals. Slot is the index of
	// this binding within its scope, so an evaluator can use a slice rather
	// than a map once it wants to.
	Depth int
	Slot  int
}

// A Ref is a resolved use of a name.
type Ref struct {
	Binding *Binding
	// Hops is how many scopes to walk up from the use site to reach Binding.
	Hops int
}

// Info is what resolution produced.
type Info struct {
	refs  map[*ast.Identifier]Ref
	decls map[*ast.Identifier]*Binding
	sizes map[ast.Node]int
	// unresolved is set when the program imports another file, whose bindings
	// this pass cannot see.
	unresolved bool
}

// Lookup returns the binding a name reference resolved to.
func (i *Info) Lookup(id *ast.Identifier) (Ref, bool) {
	r, ok := i.refs[id]
	return r, ok
}

// Declaration returns the binding a declaring name introduced.
func (i *Info) Declaration(id *ast.Identifier) (*Binding, bool) {
	b, ok := i.decls[id]
	return b, ok
}

// ScopeSize returns how many slots the scope owned by n needs.
func (i *Info) ScopeSize(n ast.Node) int { return i.sizes[n] }

// Incomplete reports whether resolution had to give up on undefined-name
// checking, because the program imports a file this pass did not read.
func (i *Info) Incomplete() bool { return i.unresolved }

// scope is one lexical level.
type scope struct {
	parent *scope
	depth  int
	names  map[string]*Binding
	// pending holds names declared but whose initializer is still being
	// resolved, so `let x = x` can be caught while recursion still works.
	pending map[string]bool
	slots   int
}

func newScope(parent *scope) *scope {
	depth := 0
	if parent != nil {
		depth = parent.depth + 1
	}
	return &scope{
		parent:  parent,
		depth:   depth,
		names:   map[string]*Binding{},
		pending: map[string]bool{},
	}
}

// A Resolver resolves one program.
type Resolver struct {
	file  *source.File
	diags *diag.Bag

	current *scope
	info    *Info

	// modules holds module names declared anywhere in the program, plus any
	// predeclared by the caller. Modules are hoisted: the README states their
	// members can reference each other regardless of order.
	modules map[string]bool
	// hasImport suppresses undefined-name errors, since an import can bring in
	// names this pass cannot see.
	hasImport bool

	// hoisted holds top-level function bindings declared before their `let` was
	// reached, so binding knows to fill one in rather than report it as a
	// redeclaration.
	hoisted map[*Binding]bool

	// moduleMembers maps a module's binding to the names it declares, for the
	// modules this pass can see: the standard library and anything declared in
	// this file.
	moduleMembers map[*Binding][]string

	// loops and funcs count the enclosing loop and function bodies, so that
	// break, continue and return can be checked where they mean something. A
	// function body resets loops: a `break` inside a function nested in a loop
	// belongs to neither, and used to unwind out of the call.
	loops int
	funcs int
}

// New returns a Resolver with a global scope containing the builtins.
func New(file *source.File, diags *diag.Bag) *Resolver {
	r := &Resolver{
		file:  file,
		diags: diags,
		info: &Info{
			refs:  map[*ast.Identifier]Ref{},
			decls: map[*ast.Identifier]*Binding{},
			sizes: map[ast.Node]int{},
		},
		modules:       map[string]bool{},
		hoisted:       map[*Binding]bool{},
		moduleMembers: map[*Binding][]string{},
	}
	r.current = newScope(nil)
	for _, name := range Builtins {
		r.predeclare(name, KindBuiltin)
	}
	return r
}

// Predeclare adds a global name, for values supplied by the host rather than by
// the program.
func (r *Resolver) Predeclare(name string) { r.predeclare(name, KindLet) }

// PredeclareModule adds a module name, for modules supplied by the standard
// library rather than declared in this file.
//
// A module is a value, so the name is an ordinary global binding as well as an
// entry in the module set. It used to be only the latter, which is why
// resolveName had to let bare module names through unresolved and `let E = Enum`
// then failed in the evaluator with "'Enum' is not defined".
func (r *Resolver) PredeclareModule(name string, members ...string) {
	r.modules[name] = true
	r.predeclare(name, KindLet)
	if len(members) > 0 {
		r.moduleMembers[r.current.names[name]] = members
	}
}

func (r *Resolver) predeclare(name string, kind Kind) {
	b := &Binding{Name: name, Kind: kind, Depth: 0, Slot: r.current.slots}
	r.current.slots++
	r.current.names[name] = b
}

// Resolve walks prog and reports any name problems into the Bag.
func (r *Resolver) Resolve(prog *ast.Program) *Info {
	// Modules are hoisted, so collect their names before resolving anything.
	r.collectModules(prog.Nodes)
	r.hoistFunctions(prog.Nodes)

	for _, n := range prog.Nodes {
		r.node(n)
	}

	r.info.sizes[prog] = r.current.slots
	r.info.unresolved = r.hasImport
	return r.info
}

// collectModules records module names declared at this level, and notes whether
// the program imports anything.
func (r *Resolver) collectModules(nodes []ast.Node) {
	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.Module:
			// A second `module M` is left undeclared on purpose: the evaluator
			// reports the redeclaration, with the module's own message.
			if n.Name != nil && !r.modules[n.Name.Value] {
				r.modules[n.Name.Value] = true
				b := r.declare(n.Name, KindLet, false)
				// The members are right here, so an access can be checked
				// against them rather than left to the evaluator.
				if b != nil && n.Body != nil {
					members, wellFormed := []string{}, true
					for _, c := range n.Body.Nodes {
						let, isLet := c.(*ast.Let)
						if !isLet {
							wellFormed = false
							continue
						}
						if let.Name != nil {
							members = append(members, let.Name.Value)
						}
					}
					// A malformed body is reported on its own; checking members
					// against it too would pile a second message onto one
					// mistake.
					if wellFormed {
						r.moduleMembers[b] = members
					}
				}
			}
		case *ast.Import:
			r.hasImport = true
		}
	}
}

// hoistFunctions declares every top-level function-valued `let` before anything
// is resolved, so two functions that call each other can both live at the top
// level. Wrapping them in a module was the only way to write that, which is a
// strange thing to have to do for two free functions.
//
// Only the top level, and only `let` whose value is a function literal. A
// hoisted name is in scope during its own initializer by construction, so
// hoisting anything else would defeat the `pending` check that catches
// `let x = x`; and a function value is the one case the evaluator can hoist to
// match, since a function literal needs nothing evaluated first.
func (r *Resolver) hoistFunctions(nodes []ast.Node) {
	for _, n := range nodes {
		let, ok := n.(*ast.Let)
		if !ok || let.Name == nil {
			continue
		}
		if _, isFunc := let.Value.(*ast.Function); !isFunc {
			continue
		}
		// Already taken — by a module, a builtin, or an earlier `let` of the
		// same name. Leave it to binding, which reports the collision where the
		// second declaration is written.
		if _, taken := r.current.names[let.Name.Value]; taken {
			continue
		}
		if b := r.declare(let.Name, KindLet, false); b != nil {
			r.hoisted[b] = true
		}
	}
}

// ---------------------------------------------------------------------------
// Scopes
// ---------------------------------------------------------------------------

func (r *Resolver) push() { r.current = newScope(r.current) }

// pop leaves the current scope, recording its size against the node that owns
// it so an evaluator knows how many slots to allocate.
func (r *Resolver) pop(owner ast.Node) {
	if owner != nil {
		r.info.sizes[owner] = r.current.slots
	}
	r.current = r.current.parent
}

// declare introduces name into the CURRENT scope only.
//
// Looking only at the current scope is the whole of the shadowing fix: an outer
// binding of the same name is simply hidden, rather than making this a redeclaration.
func (r *Resolver) declare(id *ast.Identifier, kind Kind, mutable bool) *Binding {
	if id == nil {
		return nil
	}

	if prev, ok := r.current.names[id.Value]; ok {
		r.diags.Errorf(id.Span(), "'%s' is already declared in this scope", id.Value)
		if prev.Decl.IsValid() {
			r.diags.Notef(prev.Decl, "'%s' was first declared here", id.Value)
		}
		return prev
	}

	b := &Binding{
		Name:    id.Value,
		Kind:    kind,
		Mutable: mutable,
		Decl:    id.Span(),
		Depth:   r.current.depth,
		Slot:    r.current.slots,
	}
	r.current.slots++
	r.current.names[id.Value] = b
	r.info.decls[id] = b
	return b
}

// resolveName binds a use of id to its declaration.
func (r *Resolver) resolveName(id *ast.Identifier) {
	if id == nil {
		return
	}

	// A name read inside its own initializer, in the same scope, is a mistake:
	// `let x = x` cannot mean anything. The check is deliberately limited to the
	// innermost scope, so a function body referring to the name it is being
	// assigned to — that is, recursion — still resolves.
	if r.current.pending[id.Value] {
		r.diags.Errorf(id.Span(), "'%s' is used in its own definition", id.Value)
		return
	}

	hops := 0
	for s := r.current; s != nil; s = s.parent {
		if b, ok := s.names[id.Value]; ok {
			r.info.refs[id] = Ref{Binding: b, Hops: hops}
			return
		}
		hops++
	}

	// An import can introduce names this pass never saw, so reporting here
	// would be a guess. Import resolution would remove this exception.
	if r.hasImport {
		return
	}

	r.diags.Errorf(id.Span(), "'%s' is not defined", id.Value)
}

// ---------------------------------------------------------------------------
// Nodes
// ---------------------------------------------------------------------------

func (r *Resolver) node(n ast.Node) {
	switch n := n.(type) {
	case nil, *ast.Bad:
		// Already reported by the parser.

	case *ast.Program:
		for _, c := range n.Nodes {
			r.node(c)
		}

	case *ast.Block:
		r.push()
		for _, c := range n.Nodes {
			r.node(c)
		}
		r.pop(n)

	case *ast.Identifier:
		r.resolveName(n)

	case *ast.Let:
		if n.Pattern != nil {
			r.patternBinding(n.Pattern, n.Value, KindLet, false)
			return
		}
		r.binding(n.Name, n.Value, KindLet, false)
	case *ast.Var:
		if n.Pattern != nil {
			r.patternBinding(n.Pattern, n.Value, KindVar, true)
			return
		}
		r.binding(n.Name, n.Value, KindVar, true)
	case *ast.Assign:
		r.assign(n)

	case *ast.Prefix:
		r.node(n.Right)
	case *ast.Infix:
		r.node(n.Left)
		r.node(n.Right)
	case *ast.Subscript:
		r.node(n.Left)
		r.node(n.Index)
	case *ast.Pipe:
		r.node(n.Left)
		r.pipeTarget(n.Right)

	case *ast.Is:
		// Only the left side is a value; the right names a type, which is a
		// fixed set this pass can check.
		r.node(n.Left)
		r.typeName(n.Right)
	case *ast.As:
		r.node(n.Left)
		r.typeName(n.Right)

	case *ast.ExpressionList:
		for _, c := range n.Elements {
			r.node(c)
		}
	case *ast.Interpolation:
		for _, part := range n.Parts {
			r.node(part)
		}
	case *ast.Array:
		r.node(n.List)
	case *ast.Dictionary:
		for _, p := range n.Pairs {
			r.node(p.Key)
			r.node(p.Value)
		}

	case *ast.If:
		r.node(n.Condition)
		r.node(n.Then)
		if n.Else != nil {
			r.node(n.Else)
		}

	case *ast.Switch:
		r.node(n.Control)
		for _, c := range n.Cases {
			// An arm's captures are in scope for its guard and its body and
			// nowhere else, so the arm gets a scope of its own. The body's own
			// Block would push a second one; resolve its contents directly so
			// captures and body names share one level.
			r.push()
			// A case value may be `_`, the wildcard, and an array pattern may
			// hold them too. Walk the elements rather than the list so a
			// placeholder is not reported as one out of position.
			if c.Values != nil {
				for _, el := range c.Values.Elements {
					r.caseValue(el)
				}
			}
			r.node(c.Guard)
			if c.Body != nil {
				for _, node := range c.Body.Nodes {
					r.node(node)
				}
			}
			r.pop(c)
		}
		if n.Default != nil {
			r.node(n.Default)
		}

	case *ast.For:
		r.forLoop(n)
	case *ast.While:
		r.node(n.Condition)
		r.loops++
		r.node(n.Body)
		r.loops--

	case *ast.Function:
		r.function(n)
	case *ast.FunctionCall:
		r.node(n.Function)
		r.node(n.Arguments)

	case *ast.Module:
		r.module(n)
	case *ast.Access:
		// The left side is an ordinary expression. resolveName already lets a
		// bare module name through, so `Enum.size` resolves without `.` having
		// to know anything about modules — and `cfg.db.host`, `f().a` and
		// `a[0].k` resolve for the same reason.
		r.node(n.Left)
		r.moduleMember(n)

	// A control keyword outside the construct it controls is knowable here,
	// and silent otherwise: Interp.Run stops its node loop on any signal, so a
	// stray `break` at top level discarded the rest of the file with no
	// diagnostic and exit code 0.
	case *ast.Break:
		switch {
		case r.loops == 0:
			r.diags.Errorf(n.Span(), "'break' outside a loop")
		case n.Levels > r.loops:
			// `break 3` inside two loops has nothing to leave, and would
			// otherwise unwind out of the enclosing function at runtime.
			r.diags.Errorf(n.Span(), "'break %d' inside %d loop(s)", n.Levels, r.loops)
		}
	case *ast.Continue:
		if r.loops == 0 {
			r.diags.Errorf(n.Span(), "'continue' outside a loop")
		}
	case *ast.Return:
		if r.funcs == 0 {
			r.diags.Errorf(n.Span(), "'return' outside a function")
		}
		r.node(n.Value)

	// `_` is meaningful in exactly two positions, a switch case value and an
	// append target, and neither reaches here: both are skipped by the code
	// that walks them. Anywhere else it evaluated to nil, so `let x = _` was
	// accepted and quietly meant nothing.
	case *ast.Placeholder:
		r.diags.Errorf(n.Span(), "'_' is a placeholder: it means something only as a switch case value or an append target")

	case *ast.Import,
		*ast.Integer, *ast.Float, *ast.String, *ast.Atom,
		*ast.Boolean, *ast.Nil:
		// Nothing to resolve.
	}
}

// caseValue resolves one value of a case arm, where `_` is a wildcard rather
// than a name and `is Type` names a type rather than a value.
func (r *Resolver) caseValue(el ast.Node) {
	switch el := el.(type) {
	case *ast.Placeholder:
	case *ast.TypeCase:
		r.typeName(el.Name)
	case *ast.Binder:
		r.declare(el.Name, KindLet, false)
	case *ast.Array:
		if el.List != nil {
			for _, item := range el.List.Elements {
				r.caseValue(item)
			}
		}
	default:
		r.node(el)
	}
}

// patternBinding resolves a destructuring `let` or `var`: the initializer
// first, then every name the pattern binds.
//
// The pending check that catches `let x = x` does not apply: a pattern's names
// are declared after its value is resolved, so none of them is in scope inside
// it.
func (r *Resolver) patternBinding(pattern *ast.ArrayPattern, value ast.Node, kind Kind, mutable bool) {
	r.node(value)
	r.declarePattern(pattern, kind, mutable)
}

func (r *Resolver) declarePattern(pattern *ast.ArrayPattern, kind Kind, mutable bool) {
	for _, el := range pattern.Elements {
		switch el := el.(type) {
		case *ast.Identifier:
			r.declare(el, kind, mutable)
		case *ast.Rest:
			r.declare(el.Name, kind, mutable)
		case *ast.ArrayPattern:
			r.declarePattern(el, kind, mutable)
		case *ast.Placeholder:
			// A hole, binding nothing.
		}
	}
}

// pipeTarget resolves the right side of a pipe.
//
// It is the third position where `_` means something: among a piped call's
// arguments it marks the slot the piped value lands in, which is the only way
// to pipe into a function whose subject is not its first parameter. At most one
// may appear — two would leave the second with nothing to receive.
func (r *Resolver) pipeTarget(n ast.Node) {
	call, ok := n.(*ast.FunctionCall)
	if !ok || call.Arguments == nil {
		r.node(n)
		return
	}

	r.node(call.Function)

	slots := 0
	for _, a := range call.Arguments.Elements {
		if ph, slot := a.(*ast.Placeholder); slot {
			slots++
			if slots == 2 {
				r.diags.Errorf(ph.Span(), "a piped call takes at most one '_', which marks where the piped value goes")
			}
			continue
		}
		r.node(a)
	}
}

// binding resolves a let or var: the initializer first, then the declaration.
func (r *Resolver) binding(name *ast.Identifier, value ast.Node, kind Kind, mutable bool) {
	if name == nil {
		r.node(value)
		return
	}

	// Declare before resolving the initializer, so a function can call the name
	// it is being assigned to. That is what makes recursion work:
	//
	//     let fact = func (n) do ... fact(n - 1) ... end
	//
	// Marking it pending meanwhile catches the case that cannot mean anything,
	// `let x = x`. The pending check looks only at the innermost scope, and a
	// function body opens a new one, so the two do not conflict.
	// A hoisted name is already declared. Claim it rather than declaring it
	// again, which would report the hoist as a redeclaration of itself; a
	// second `let` of the same name finds it unclaimed and is reported.
	if b, ok := r.current.names[name.Value]; ok && r.hoisted[b] {
		delete(r.hoisted, b)
		r.info.decls[name] = b
	} else {
		r.declare(name, kind, mutable)
	}

	r.current.pending[name.Value] = true
	r.node(value)
	delete(r.current.pending, name.Value)
}

// assign checks that the target may be rebound.
func (r *Resolver) assign(n *ast.Assign) {
	r.node(n.Right)

	// Find the name being written to. For a subscript the target is the
	// collection itself, because `a[0] = v` rebinds `a` to a new collection
	// under the frozen-collections rule.
	target := n.Name
	subscript := false
	for {
		sub, ok := target.(*ast.Subscript)
		if !ok {
			break
		}
		subscript = true
		// `a[] = v` and `a[_] = v` both parse to a placeholder index. On the
		// left of an assignment that is the append target, not a stray `_`.
		if _, appendTarget := sub.Index.(*ast.Placeholder); !appendTarget {
			r.node(sub.Index)
		}
		target = sub.Left
	}

	id, ok := target.(*ast.Identifier)
	if !ok {
		r.node(n.Name)
		return
	}

	r.resolveName(id)

	ref, found := r.info.refs[id]
	if !found {
		return // already reported as undefined, or hidden behind an import
	}

	if !ref.Binding.Mutable {
		what := "assign to"
		if subscript {
			// Worth naming: under frozen collections this is a rebinding, not
			// an in-place edit, so it needs a `var` even though it looks like
			// a mutation of the contents.
			what = "modify"
		}
		r.diags.Errorf(id.Span(), "cannot %s '%s': it is bound with %s, which cannot be rebound",
			what, id.Value, ref.Binding.Kind)
		if ref.Binding.Decl.IsValid() {
			r.diags.Notef(ref.Binding.Decl, "'%s' is declared here", id.Value)
		}
	}
}

// forLoop gives the loop its own scope, so its variables do not outlive it.
//
// The original wrote loop variables into the ENCLOSING scope while creating an
// unused child, so `v` was still bound after the loop ended.
func (r *Resolver) forLoop(n *ast.For) {
	r.node(n.Enumerable)

	r.push()
	if n.Arguments != nil {
		// The evaluator checked this per iteration, so an empty enumerable
		// never reached it and `for a, b, c in []` was accepted. The count is
		// right here, before anything runs.
		if len(n.Arguments.Elements) > 2 {
			r.diags.Errorf(n.Arguments.Span(), "a for loop takes at most 2 variables, got %d",
				len(n.Arguments.Elements))
		}
		for _, id := range n.Arguments.Elements {
			r.declare(id, KindLoop, false)
		}
	}
	// The body's own Block would push a second scope; resolve its contents
	// directly so loop variables and body names share one level.
	r.loops++
	if n.Body != nil {
		for _, c := range n.Body.Nodes {
			r.node(c)
		}
	}
	r.loops--
	r.pop(n)
}

// function scopes its parameters with its body.
func (r *Resolver) function(n *ast.Function) {
	// Default values are evaluated in the ENCLOSING scope, so resolve them
	// before opening the function's own.
	for _, p := range n.Parameters {
		if p == nil {
			continue
		}
		r.typeName(p.Type)
		if p.Default == nil {
			continue
		}
		r.node(p.Default)
		// checkParamType runs only on arguments actually passed, so a default
		// was bound unchecked and the annotation was a lie for every caller
		// that omitted it. A literal default is knowable here.
		if p.Type != nil && p.Type.Value != value.Any {
			if got := literalType(p.Default); got != "" && got != p.Type.Value {
				r.diags.Errorf(p.Default.Span(), "parameter '%s' is declared %s but defaults to %s",
					p.Name.Value, p.Type.Value, got)
			}
		}
	}
	r.typeName(n.ReturnType)

	r.push()
	for _, p := range n.Parameters {
		if p == nil {
			continue
		}
		// Parameters bind like `let`. A function that could rebind its
		// parameter could mutate a collection its caller owns, which is the
		// hole that immutable collections close.
		r.declare(p.Name, KindParam, false)
	}
	// A `return` in here has something to return from, and a `break` does not
	// reach a loop outside the call.
	outerLoops := r.loops
	r.loops = 0
	r.funcs++
	if n.Body != nil {
		for _, c := range n.Body.Nodes {
			r.node(c)
		}
	}
	r.funcs--
	r.loops = outerLoops
	r.pop(n)
}

// module resolves a module body. Members are hoisted so they can refer to each
// other in any order, which is what the README promises.
func (r *Resolver) module(n *ast.Module) {
	if n.Body == nil {
		return
	}

	r.push()

	// First pass: declare every member. A module body accepts only `let`, which
	// the evaluator reported at runtime for a mistake plainly visible here.
	for _, c := range n.Body.Nodes {
		if let, ok := c.(*ast.Let); ok {
			if let.Name != nil {
				r.declare(let.Name, KindLet, false)
			}
			continue
		}
		if _, bad := c.(*ast.Bad); !bad && c != nil {
			r.diags.Errorf(c.Span(), "a module body accepts only 'let' declarations")
		}
	}

	// Second pass: resolve the values, now that every name is visible.
	for _, c := range n.Body.Nodes {
		if let, ok := c.(*ast.Let); ok {
			r.node(let.Value)
			continue
		}
		r.node(c)
	}

	r.pop(n)
}

// typeName checks that id names a type an annotation may use.
//
// `is` was a string comparison against Type().String() and the resolver skipped
// the right-hand side entirely, so `5 is Banana` was a permanently-false test
// rather than a typo. The set is fixed and known here.
func (r *Resolver) typeName(id *ast.Identifier) {
	if id == nil || value.IsTypeName(id.Value) {
		return
	}
	// One diagnostic rather than an error and a note: a note with the same span
	// would draw the same line and caret underneath itself.
	r.diags.Errorf(id.Span(), "'%s' is not a type: expected one of %s",
		id.Value, strings.Join(value.TypeNames(), ", "))
}

// moduleMember checks an access against a module whose members are known.
//
// resolver.moduleAccess used to skip this on the grounds that matching members
// across files is the evaluator's job. True for a module declared in an imported
// file, which this pass never sees — but not for the standard library, whose
// members are known before the user's program is parsed, and not for a module
// declared in the same file. Moving "not found" from runtime to a diagnostic is
// the resolver's stated reason for existing.
func (r *Resolver) moduleMember(n *ast.Access) {
	id, ok := n.Left.(*ast.Identifier)
	if !ok || n.Name == nil {
		return
	}
	ref, found := r.info.refs[id]
	if !found {
		return // undefined, or hidden behind an import; already handled
	}
	members, known := r.moduleMembers[ref.Binding]
	if !known {
		return // not a module, or one whose members this pass cannot see
	}
	for _, m := range members {
		if m == n.Name.Value {
			return
		}
	}
	r.diags.Errorf(n.Name.Span(), "module '%s' has no member '%s'", id.Value, n.Name.Value)
}

// literalType is the type name of a node whose type is knowable without running
// it, or "" for anything else.
func literalType(n ast.Node) string {
	switch n.(type) {
	case *ast.Integer:
		return "Int"
	case *ast.Float:
		return "Float"
	case *ast.String, *ast.Interpolation:
		return "String"
	case *ast.Atom:
		return "Atom"
	case *ast.Boolean:
		return "Bool"
	case *ast.Nil:
		return "Nil"
	case *ast.Array:
		return "Array"
	case *ast.Dictionary:
		return "Dictionary"
	case *ast.Function:
		return "Function"
	}
	return ""
}
