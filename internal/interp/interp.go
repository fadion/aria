// Package interp evaluates a resolved Aria program.
//
// Three things differ structurally from the original evaluator.
//
// A runtime error stops the program. The old one collected errors and carried
// on with a nil in hand, so one real failure produced a second misleading
// message about the call that received the nil, and a statement that
// legitimately produced nothing silently discarded the rest of its block.
// Here a failure unwinds to the top and the process exits non-zero (7.1, 7.5).
//
// Control flow is a signal, not a value. `return`, `break` and `continue` used
// to be ordinary values flowing through the same paths as data, which is what
// made "no value" and "stop evaluating" indistinguishable.
//
// Collections are immutable. `a[] = v` rebinds `a` to a new collection rather
// than growing one in place, so no operator can corrupt its own operand.
package interp

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/parser"
	"github.com/fadion/aria/internal/resolver"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/value"
)

// Error is a runtime failure, located in the source.
//
// File is the file Span indexes into, which is not necessarily the file that
// was run: a fault inside an imported file or a standard library module is
// located in that file's text. Without it the span was rendered against the
// wrong source and pointed at unrelated lines, or past the end of the file.
type Error struct {
	File *source.File
	Span source.Span
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// signal is a pending control transfer.
type signal uint8

const (
	sigNone signal = iota
	sigReturn
	sigBreak
	sigContinue
)

// env is one lexical scope at runtime.
//
// Lookup walks outward from the innermost scope, so an inner declaration
// naturally hides an outer one. The resolver has already proved every name
// resolves and that no immutable binding is assigned to, so nothing here needs
// to re-check either.
type env struct {
	parent *env
	vars   map[string]value.Value
}

func newEnv(parent *env) *env {
	return &env{parent: parent, vars: map[string]value.Value{}}
}

func (e *env) lookup(name string) (value.Value, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

func (e *env) define(name string, v value.Value) { e.vars[name] = v }

// assign writes to the scope that owns name, reporting whether it found one.
func (e *env) assign(name string, v value.Value) bool {
	for s := e; s != nil; s = s.parent {
		if _, ok := s.vars[name]; ok {
			s.vars[name] = v
			return true
		}
	}
	return false
}

// Interp evaluates programs.
type Interp struct {
	file *source.File
	info *resolver.Info

	Out io.Writer
	Err io.Writer
	In  io.Reader

	globals *env
	modules map[string]*Module
	// dir is where relative imports resolve from.
	dir string
	// imported guards against re-importing, matching the original's cache.
	imported map[string]bool

	signal signal
	retval value.Value

	// frames is the call stack. Each frame names the file its function's body
	// lives in, so a fault inside it is reported against that file, and its
	// depth is what bounds recursion.
	frames []frame
}

// maxCallDepth bounds recursion, for the reason the parser bounds nesting at
// 250: exhausting the goroutine stack kills the process with a Go traceback,
// which is not a diagnostic anybody can act on. It is far above any depth a
// sensible program reaches and far below where the 1 GB stack ceiling is in
// sight.
const maxCallDepth = 3000

// A frame is one active call.
type frame struct {
	// file is where the callee's body was written.
	file *source.File
	// span locates the call site, in the caller's file.
	span source.Span
}

// New returns an interpreter writing to stdout and stderr.
func New(file *source.File, info *resolver.Info) *Interp {
	i := &Interp{
		file:     file,
		info:     info,
		Out:      os.Stdout,
		Err:      os.Stderr,
		In:       os.Stdin,
		globals:  newEnv(nil),
		modules:  map[string]*Module{},
		imported: map[string]bool{},
	}
	i.dir = filepath.Dir(file.Name)
	i.installBuiltins()
	return i
}

// Globals exposes the top-level scope, so a host can seed it.
func (i *Interp) Globals() *env { return i.globals }

// Modules exposes declared modules, so a standard library evaluated separately
// can be carried into a later run.
func (i *Interp) Modules() map[string]*Module { return i.modules }

// Run evaluates prog. It returns the program's final value, or an error if a
// runtime failure stopped it.
func (i *Interp) Run(prog *ast.Program) (result value.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			re, ok := r.(*Error)
			if !ok {
				panic(r)
			}
			result, err = nil, re
		}
	}()

	result = value.Value(value.NilValue)
	for _, n := range prog.Nodes {
		result = i.eval(n, i.globals)
		if i.signal != sigNone {
			break
		}
	}
	return result, nil
}

// fail stops evaluation with a runtime error. Unwinding rather than returning a
// sentinel is what keeps one failure from producing a cascade of follow-on
// messages describing the confusion it caused.
func (i *Interp) fail(span source.Span, format string, args ...any) {
	i.failIn(i.curFile(), span, format, args...)
}

// failIn is fail for a span that belongs to a file other than the one being
// evaluated — a call site, when the callee was written elsewhere.
func (i *Interp) failIn(file *source.File, span source.Span, format string, args ...any) {
	panic(&Error{File: file, Span: span, Msg: fmt.Sprintf(format, args...)})
}

// curFile is the file whose text the node being evaluated came from: the body
// of the innermost active call, or the file being run at top level.
func (i *Interp) curFile() *source.File {
	if n := len(i.frames); n > 0 {
		if f := i.frames[n-1].file; f != nil {
			return f
		}
	}
	return i.file
}

// ---------------------------------------------------------------------------
// Evaluation
// ---------------------------------------------------------------------------

func (i *Interp) eval(n ast.Node, e *env) value.Value {
	switch n := n.(type) {
	case *ast.Bad:
		i.fail(n.Span(), "cannot evaluate: this expression failed to parse")

	case *ast.Integer:
		return value.Int(n.Value)
	case *ast.Float:
		return value.Float(n.Value)
	case *ast.String:
		return value.String(n.Value)
	case *ast.Atom:
		return value.Atom(n.Value)
	case *ast.Boolean:
		return value.Of(n.Value)
	case *ast.Nil:
		return value.NilValue
	case *ast.Placeholder:
		return value.NilValue

	case *ast.Identifier:
		v, ok := e.lookup(n.Value)
		if !ok {
			// The resolver proves this cannot happen for a resolved program;
			// reaching it means the two disagree about scoping.
			i.fail(n.Span(), "'%s' is not defined", n.Value)
		}
		return v

	case *ast.Block:
		return i.evalBlock(n, newEnv(e))

	case *ast.Let:
		v := i.eval(n.Value, e)
		e.define(n.Name.Value, v)
		return v
	case *ast.Var:
		v := i.eval(n.Value, e)
		e.define(n.Name.Value, v)
		return v
	case *ast.Assign:
		return i.evalAssign(n, e)

	case *ast.Prefix:
		return i.evalPrefix(n, e)
	case *ast.Infix:
		return i.evalInfix(n, e)
	case *ast.Subscript:
		return i.evalSubscript(n, e)
	case *ast.Pipe:
		return i.evalPipe(n, e)
	case *ast.Is:
		return value.Of(i.eval(n.Left, e).Type().String() == n.Right.Value)
	case *ast.As:
		return i.convert(i.eval(n.Left, e), n.Right.Value, n.Span())

	case *ast.Array:
		elems := make([]value.Value, 0, len(n.List.Elements))
		for _, el := range n.List.Elements {
			elems = append(elems, i.eval(el, e))
		}
		return value.NewArray(elems)

	case *ast.Dictionary:
		pairs := make([]value.Pair, 0, len(n.Pairs))
		for _, p := range n.Pairs {
			k := i.eval(p.Key, e)
			if _, ok := value.KeyOf(k); !ok {
				i.fail(p.Key.Span(), "%s cannot be a dictionary key", k.Type())
			}
			pairs = append(pairs, value.Pair{Key: k, Value: i.eval(p.Value, e)})
		}
		return value.NewDict(pairs)

	case *ast.If:
		if value.Truthy(i.eval(n.Condition, e)) {
			return i.evalBlock(n.Then, newEnv(e))
		}
		if n.Else != nil {
			return i.evalBlock(n.Else, newEnv(e))
		}
		return value.NilValue

	case *ast.Switch:
		return i.evalSwitch(n, e)
	case *ast.For:
		return i.evalFor(n, e)

	case *ast.Function:
		return &Function{Decl: n, Env: e, File: i.curFile()}
	case *ast.FunctionCall:
		return i.evalCall(n, e)

	case *ast.Module:
		i.evalModule(n, e)
		return value.NilValue
	case *ast.Access:
		return i.evalAccess(n, e)

	case *ast.Import:
		return i.evalImport(n, e)

	case *ast.Return:
		v := value.Value(value.NilValue)
		if n.Value != nil {
			v = i.eval(n.Value, e)
		}
		i.signal, i.retval = sigReturn, v
		return v
	case *ast.Break:
		i.signal = sigBreak
		return value.NilValue
	case *ast.Continue:
		i.signal = sigContinue
		return value.NilValue
	}

	i.fail(n.Span(), "cannot evaluate %T", n)
	return value.NilValue
}

// evalBlock runs a block's nodes and yields the last value.
//
// It stops only on a control signal. The original also stopped whenever a
// statement produced nil, which is why a non-matching `switch` in the middle of
// a function silently discarded everything after it.
func (i *Interp) evalBlock(b *ast.Block, e *env) value.Value {
	result := value.Value(value.NilValue)
	for _, n := range b.Nodes {
		result = i.eval(n, e)
		if i.signal != sigNone {
			return result
		}
	}
	return result
}

// evalAssign writes to a name, or rebinds one through a subscript.
func (i *Interp) evalAssign(n *ast.Assign, e *env) value.Value {
	rhs := i.eval(n.Right, e)

	if id, ok := n.Name.(*ast.Identifier); ok {
		i.assignChecked(id, rhs, e)
		return rhs
	}

	sub, ok := n.Name.(*ast.Subscript)
	if !ok {
		i.fail(n.Span(), "assignment expects a name on the left")
	}

	// Under immutable collections a subscript write is a rebinding: build the
	// updated collection, then assign it back to the name it came from.
	base, path := flattenSubscript(sub)
	id, ok := base.(*ast.Identifier)
	if !ok {
		i.fail(n.Span(), "assignment expects a name on the left")
	}

	current, found := e.lookup(id.Value)
	if !found {
		i.fail(id.Span(), "'%s' is not defined", id.Value)
	}

	updated := i.updatePath(current, path, rhs, e, n.Span())
	i.assignChecked(id, updated, e)
	return rhs
}

// assignChecked writes v to name, enforcing the type lock.
func (i *Interp) assignChecked(id *ast.Identifier, v value.Value, e *env) {
	old, found := e.lookup(id.Value)
	if !found {
		i.fail(id.Span(), "'%s' is not defined", id.Value)
	}
	// Reassignment preserves the original type. Nil is exempt, since a binding
	// that started nil has no type to preserve.
	if old.Type() != v.Type() && old.Type() != value.TNil && v.Type() != value.TNil {
		i.fail(id.Span(), "'%s' holds %s, so it cannot be assigned %s",
			id.Value, old.Type(), v.Type())
	}
	if !e.assign(id.Value, v) {
		i.fail(id.Span(), "'%s' is not defined", id.Value)
	}
}

// flattenSubscript peels nested indexing down to the base name, returning the
// index expressions outermost-last so a[0][1] can be rebuilt in order.
func flattenSubscript(s *ast.Subscript) (ast.Node, []ast.Node) {
	var path []ast.Node
	var node ast.Node = s
	for {
		sub, ok := node.(*ast.Subscript)
		if !ok {
			break
		}
		path = append([]ast.Node{sub.Index}, path...)
		node = sub.Left
	}
	return node, path
}

// updatePath returns a copy of container with the value at path replaced.
func (i *Interp) updatePath(container value.Value, path []ast.Node, v value.Value, e *env, span source.Span) value.Value {
	if len(path) == 0 {
		return v
	}

	idxNode := path[0]
	rest := path[1:]

	switch c := container.(type) {
	case *value.Array:
		// An empty index or `_` appends.
		if _, isPlaceholder := idxNode.(*ast.Placeholder); isPlaceholder {
			if len(rest) > 0 {
				i.fail(span, "cannot index past an append")
			}
			return c.Append(v)
		}

		idx := i.eval(idxNode, e)
		n, ok := idx.(value.Int)
		if !ok {
			i.fail(idxNode.Span(), "array index must be Int, got %s", idx.Type())
		}
		at, err := arrayIndex(c.Len(), int(n))
		if err != "" {
			i.fail(idxNode.Span(), "%s", err)
		}
		return c.Set(at, i.updatePath(c.At(at), rest, v, e, span))

	case *value.Dict:
		key := i.eval(idxNode, e)
		if _, ok := value.KeyOf(key); !ok {
			i.fail(idxNode.Span(), "%s cannot be a dictionary key", key.Type())
		}
		if len(rest) == 0 {
			return c.With(key, v)
		}
		inner, found := c.Get(key)
		if !found {
			i.fail(idxNode.Span(), "key %s is not in the dictionary", key.Inspect())
		}
		return c.With(key, i.updatePath(inner, rest, v, e, span))

	case value.String:
		idx := i.eval(idxNode, e)
		n, ok := idx.(value.Int)
		if !ok {
			i.fail(idxNode.Span(), "string index must be Int, got %s", idx.Type())
		}
		runes := c.Runes()
		at, err := arrayIndex(len(runes), int(n))
		if err != "" {
			i.fail(idxNode.Span(), "%s", err)
		}
		repl, ok := v.(value.String)
		if !ok {
			i.fail(span, "a string index can only be assigned a String, got %s", v.Type())
		}
		out := make([]rune, 0, len(runes)+len(repl.Runes()))
		out = append(out, runes[:at]...)
		out = append(out, repl.Runes()...)
		out = append(out, runes[at+1:]...)
		return value.String(string(out))
	}

	i.fail(span, "cannot index into %s", container.Type())
	return value.NilValue
}

// arrayIndex normalises a possibly-negative index, or returns a message.
func arrayIndex(length, idx int) (int, string) {
	original := idx
	if idx < 0 {
		idx += length
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Sprintf("index %d is out of bounds for length %d", original, length)
	}
	return idx, ""
}

func (i *Interp) evalSubscript(n *ast.Subscript, e *env) value.Value {
	left := i.eval(n.Left, e)

	if _, isPlaceholder := n.Index.(*ast.Placeholder); isPlaceholder {
		i.fail(n.Span(), "an empty index can only be used on the left of an assignment")
	}
	idx := i.eval(n.Index, e)

	switch c := left.(type) {
	case *value.Array:
		nIdx, ok := idx.(value.Int)
		if !ok {
			i.fail(n.Index.Span(), "array index must be Int, got %s", idx.Type())
		}
		at, err := arrayIndex(c.Len(), int(nIdx))
		if err != "" {
			return value.NilValue // reading out of bounds yields nil
		}
		return c.At(at)

	case *value.Dict:
		if v, found := c.Get(idx); found {
			return v
		}
		return value.NilValue

	case value.String:
		nIdx, ok := idx.(value.Int)
		if !ok {
			i.fail(n.Index.Span(), "string index must be Int, got %s", idx.Type())
		}
		runes := c.Runes()
		at, err := arrayIndex(len(runes), int(nIdx))
		if err != "" {
			return value.NilValue
		}
		return value.String(string(runes[at]))
	}

	i.fail(n.Span(), "cannot index into %s", left.Type())
	return value.NilValue
}

func (i *Interp) evalPipe(n *ast.Pipe, e *env) value.Value {
	call, ok := n.Right.(*ast.FunctionCall)
	if !ok {
		i.fail(n.Right.Span(), "the right side of |> must be a function call")
	}

	// The piped value becomes the first argument. Building the argument list
	// here rather than rewriting the AST keeps the tree reusable, which the
	// original's in-place prepend did not — a piped call inside a loop grew its
	// own argument list on every iteration.
	args := make([]value.Value, 0, len(call.Arguments.Elements)+1)
	args = append(args, i.eval(n.Left, e))
	for _, a := range call.Arguments.Elements {
		args = append(args, i.eval(a, e))
	}
	return i.apply(i.eval(call.Function, e), args, call.Span(), e)
}

func (i *Interp) evalSwitch(n *ast.Switch, e *env) value.Value {
	// A missing control expression behaves as `switch true`.
	control := value.Value(value.True)
	if n.Control != nil {
		control = i.eval(n.Control, e)
	}

	for _, c := range n.Cases {
		if i.caseMatches(c, control, e) {
			return i.evalBlock(c.Body, newEnv(e))
		}
	}
	if n.Default != nil {
		return i.evalBlock(n.Default, newEnv(e))
	}
	return value.NilValue
}

// caseMatches reports whether a case arm matches the control value. An array
// control pattern-matches element by element, with `_` as a wildcard.
func (i *Interp) caseMatches(c *ast.SwitchCase, control value.Value, e *env) bool {
	// An array control with a matching arity is pattern-matched element by
	// element, and that answer is final. Falling through to scalar matching
	// afterwards would let a `_` anywhere in the pattern match everything.
	if arr, ok := control.(*value.Array); ok && len(c.Values.Elements) == arr.Len() {
		for idx, el := range c.Values.Elements {
			if _, wild := el.(*ast.Placeholder); wild {
				continue
			}
			if !value.Equal(i.eval(el, e), arr.At(idx)) {
				return false
			}
		}
		return true
	}

	for _, el := range c.Values.Elements {
		if _, wild := el.(*ast.Placeholder); wild {
			return true
		}
		if value.Equal(i.eval(el, e), control) {
			return true
		}
	}
	return false
}

func (i *Interp) evalFor(n *ast.For, e *env) value.Value {
	if n.Enumerable == nil {
		return i.loop(n, e, nil)
	}

	enum := i.eval(n.Enumerable, e)
	switch c := enum.(type) {
	case *value.Array:
		return i.loop(n, e, arrayItems(c))
	case value.String:
		return i.loop(n, e, stringItems(c))
	case value.Atom:
		return i.loop(n, e, stringItems(value.String(c)))
	case *value.Dict:
		return i.loop(n, e, dictItems(c))
	}

	i.fail(n.Enumerable.Span(), "cannot loop over %s", enum.Type())
	return value.NilValue
}

// item is one iteration's key and value.
type item struct{ key, val value.Value }

func arrayItems(a *value.Array) []item {
	out := make([]item, 0, a.Len())
	for idx, v := range a.Elems() {
		out = append(out, item{key: value.Int(idx), val: v})
	}
	return out
}

func stringItems(s value.String) []item {
	runes := s.Runes()
	out := make([]item, 0, len(runes))
	for idx, r := range runes {
		out = append(out, item{key: value.Int(idx), val: value.String(string(r))})
	}
	return out
}

func dictItems(d *value.Dict) []item {
	out := make([]item, 0, d.Len())
	for _, p := range d.Pairs() {
		out = append(out, item{key: p.Key, val: p.Value})
	}
	return out
}

// loop runs the body once per item, or forever when items is nil. It collects
// each iteration's value, which is what a `for` evaluates to.
//
// Loop variables are defined in a scope created per iteration, inside the loop's
// own scope. The original wrote them into the ENCLOSING scope, so they outlived
// the loop.
func (i *Interp) loop(n *ast.For, e *env, items []item) value.Value {
	results := []value.Value{}
	names := []string{}
	if n.Arguments != nil {
		for _, id := range n.Arguments.Elements {
			names = append(names, id.Value)
		}
	}

	// The resolver rejects more than two loop variables, so this is a backstop
	// for the one path that never reaches the resolver: an imported file, which
	// is parsed and evaluated but not resolved. It is checked once rather than
	// per iteration, which is why `for a, b, c in []` used to be accepted.
	if len(names) > 2 {
		i.fail(n.Span(), "a for loop takes at most 2 variables, got %d", len(names))
	}

	run := func(it item) bool {
		scope := newEnv(e)
		switch len(names) {
		case 0:
		case 1:
			scope.define(names[0], it.val)
		case 2:
			scope.define(names[0], it.key)
			scope.define(names[1], it.val)
		}

		v := i.evalBlock(n.Body, scope)

		switch i.signal {
		case sigBreak:
			i.signal = sigNone
			return false
		case sigContinue:
			i.signal = sigNone
			return true
		case sigReturn:
			return false // leave the signal set; the caller unwinds
		}

		results = append(results, v)
		return true
	}

	if items == nil {
		for {
			if !run(item{key: value.NilValue, val: value.NilValue}) {
				break
			}
		}
	} else {
		for _, it := range items {
			if !run(it) {
				break
			}
		}
	}

	if i.signal == sigReturn {
		return i.retval
	}
	return value.NewArray(results)
}

// ---------------------------------------------------------------------------
// Imports
// ---------------------------------------------------------------------------

func (i *Interp) evalImport(n *ast.Import, e *env) value.Value {
	name := n.File
	if filepath.Ext(name) == "" {
		name += ".ari"
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(i.dir, name)
	}

	if i.imported[path] {
		return value.NilValue
	}
	i.imported[path] = true

	src, err := os.ReadFile(path)
	if err != nil {
		i.fail(n.Span(), "cannot read imported file '%s'", n.File)
	}

	file := source.NewFile(path, src)
	bag := diag.New(file)
	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		i.fail(n.Span(), "imported file '%s' has errors:\n%s", n.File, strings.TrimRight(bag.Render(), "\n"))
	}

	// The imported file's names join the importing scope, as the language says.
	sub := &Interp{
		file: file, info: i.info,
		Out: i.Out, Err: i.Err, In: i.In,
		globals: e, modules: i.modules,
		dir: filepath.Dir(path), imported: i.imported,
	}
	for _, node := range prog.Nodes {
		sub.eval(node, e)
	}
	return value.NilValue
}

// ---------------------------------------------------------------------------
// Numbers and operators
// ---------------------------------------------------------------------------

func (i *Interp) evalPrefix(n *ast.Prefix, e *env) value.Value {
	v := i.eval(n.Right, e)

	switch n.Operator {
	case "!":
		return value.Of(!value.Truthy(v))
	case "-":
		switch v := v.(type) {
		case value.Int:
			// MinInt64 has no positive counterpart; Go negates it back to
			// itself, which is the same silent wrong answer as any other
			// overflow.
			if int64(v) == math.MinInt64 {
				i.fail(n.Span(), "Int overflow: -(%d) does not fit in an Int", int64(v))
			}
			return -v
		case value.Float:
			return -v
		}
		i.fail(n.Span(), "cannot negate %s", v.Type())
	case "~":
		if n, ok := v.(value.Int); ok {
			return ^n
		}
		i.fail(n.Span(), "bitwise NOT needs an Int, got %s", v.Type())
	}

	i.fail(n.Span(), "unknown prefix operator '%s'", n.Operator)
	return value.NilValue
}

func (i *Interp) evalInfix(n *ast.Infix, e *env) value.Value {
	// Short-circuit before evaluating the right side.
	switch n.Operator {
	case "&&":
		if !value.Truthy(i.eval(n.Left, e)) {
			return value.False
		}
		return value.Of(value.Truthy(i.eval(n.Right, e)))
	case "||":
		if value.Truthy(i.eval(n.Left, e)) {
			return value.True
		}
		return value.Of(value.Truthy(i.eval(n.Right, e)))
	}

	left := i.eval(n.Left, e)
	right := i.eval(n.Right, e)
	span := n.Span()

	switch n.Operator {
	case "==":
		return value.Of(value.Equal(left, right))
	case "!=":
		return value.Of(!value.Equal(left, right))
	}

	switch l := left.(type) {
	case value.Int:
		switch r := right.(type) {
		case value.Int:
			return i.intOp(n.Operator, l, r, span)
		case value.Float:
			return i.floatOp(n.Operator, value.Float(l), r, span)
		}
	case value.Float:
		switch r := right.(type) {
		case value.Int:
			return i.floatOp(n.Operator, l, value.Float(r), span)
		case value.Float:
			return i.floatOp(n.Operator, l, r, span)
		}
	case value.String:
		return i.textOp(n.Operator, string(l), right, span)
	case value.Atom:
		return i.textOp(n.Operator, string(l), right, span)
	case *value.Array:
		if r, ok := right.(*value.Array); ok {
			return i.arrayOp(n.Operator, l, r, span)
		}
	case *value.Dict:
		if r, ok := right.(*value.Dict); ok {
			return i.dictOp(n.Operator, l, r, span)
		}
	}

	i.fail(span, "cannot apply '%s' to %s and %s", n.Operator, left.Type(), right.Type())
	return value.NilValue
}

// intOp implements Int-on-Int arithmetic.
//
// Division truncates toward zero and stays an Int. The original returned
// an Int when the division happened to be exact and a Float otherwise, so the
// result TYPE depended on the runtime values and a declared `-> Int` could fail
// for inputs the author never tried.
func (i *Interp) intOp(op string, l, r value.Int, span source.Span) value.Value {
	a, b := int64(l), int64(r)

	// check turns an overflow-reporting result into a value or a diagnostic.
	check := func(n int64, ok bool) value.Value {
		if !ok {
			i.overflow(op, a, b, span)
		}
		return value.Int(n)
	}

	switch op {
	case "+":
		return check(addInt(a, b))
	case "-":
		return check(subInt(a, b))
	case "*":
		return check(mulInt(a, b))
	case "/":
		if b == 0 {
			i.fail(span, "division by zero")
		}
		// The one division that overflows: MinInt64 / -1 has no positive
		// counterpart to land on, and Go wraps it back to MinInt64.
		if a == math.MinInt64 && b == -1 {
			i.overflow(op, a, b, span)
		}
		return value.Int(a / b)
	case "%":
		if b == 0 {
			i.fail(span, "division by zero")
		}
		if a == math.MinInt64 && b == -1 {
			return value.Int(0)
		}
		return value.Int(a % b)
	case "**":
		// Int ** Int stays an Int, so a negative exponent truncates to 0 the
		// same way 1 / 2 does. It is computed in integer arithmetic: routed
		// through float64 it lost precision above 2^53 and then converted out
		// of range, which on amd64 produced MinInt64 — a plausible-looking
		// negative number with nothing attached to say it was wrong.
		if b < 0 {
			switch a {
			case 0:
				i.fail(span, "division by zero")
			case 1:
				return value.Int(1)
			case -1:
				if b%2 == 0 {
					return value.Int(1)
				}
				return value.Int(-1)
			}
			return value.Int(0)
		}
		return check(powInt(a, b))
	case "<":
		return value.Of(l < r)
	case "<=":
		return value.Of(l <= r)
	case ">":
		return value.Of(l > r)
	case ">=":
		return value.Of(l >= r)
	case "&":
		return l & r
	case "|":
		return l | r
	case "<<", ">>":
		if a < 0 || b < 0 {
			i.fail(span, "bitwise shift needs two non-negative Ints")
		}
		// Go shifts a count of 64 or more all the way out, so `1 << 100` was
		// 0 — an answer that looks computed and is not.
		if b >= 64 {
			i.fail(span, "bitwise shift count must be less than 64, got %d", b)
		}
		if op == ">>" {
			return value.Int(a >> uint(b))
		}
		if shifted := a << uint(b); shifted>>uint(b) == a {
			return value.Int(shifted)
		}
		i.overflow(op, a, b, span)
	case "..":
		return intRange(a, b)
	}
	i.fail(span, "unknown Int operator '%s'", op)
	return value.NilValue
}

func (i *Interp) floatOp(op string, l, r value.Float, span source.Span) value.Value {
	switch op {
	case "+":
		return l + r
	case "-":
		return l - r
	case "*":
		return l * r
	case "/":
		if r == 0 {
			i.fail(span, "division by zero")
		}
		return l / r
	case "%":
		// Every other divide-by-zero in the language is an error; falling
		// through to math.Mod made this one a NaN, which then propagated
		// through arithmetic and compared false against everything including
		// itself, so the mistake surfaced a long way from its cause.
		if r == 0 {
			i.fail(span, "division by zero")
		}
		return value.Float(math.Mod(float64(l), float64(r)))
	case "**":
		return value.Float(math.Pow(float64(l), float64(r)))
	case "<":
		return value.Of(l < r)
	case "<=":
		return value.Of(l <= r)
	case ">":
		return value.Of(l > r)
	case ">=":
		return value.Of(l >= r)
	}
	i.fail(span, "unknown Float operator '%s'", op)
	return value.NilValue
}

// textOp implements String and Atom operators. Comparison is lexicographic; the
// original compared string LENGTHS, so "abc" < "zz" was false.
func (i *Interp) textOp(op string, l string, right value.Value, span source.Span) value.Value {
	var r string
	switch v := right.(type) {
	case value.String:
		r = string(v)
	case value.Atom:
		r = string(v)
	default:
		i.fail(span, "cannot apply '%s' to String and %s", op, right.Type())
	}

	switch op {
	case "+":
		return value.String(l + r)
	case "<":
		return value.Of(l < r)
	case "<=":
		return value.Of(l <= r)
	case ">":
		return value.Of(l > r)
	case ">=":
		return value.Of(l >= r)
	case "..":
		return i.charRange(l, r, span)
	}
	i.fail(span, "unknown String operator '%s'", op)
	return value.NilValue
}

// arrayOp implements the operators defined on two Arrays.
//
// Ordering is not among them. `<` and `>` used to compare lengths while `<=`
// and `>=` were not defined at all, so the four comparison operators meant two
// different things on one type — and the one they did mean is not what they
// read as: `[1, 2, 3] < [9]` was false, because it asked whether 3 < 1. Length
// has a spelling that says so.
func (i *Interp) arrayOp(op string, l, r *value.Array, span source.Span) value.Value {
	switch op {
	case "+":
		return l.Concat(r)
	case "<", "<=", ">", ">=":
		i.fail(span, "Arrays have no order; to compare lengths write Enum.size(a) %s Enum.size(b)", op)
	}
	i.fail(span, "unknown Array operator '%s'", op)
	return value.NilValue
}

func (i *Interp) dictOp(op string, l, r *value.Dict, span source.Span) value.Value {
	switch op {
	case "+":
		// Returns a new dictionary. The original folded the left operand into
		// the right and returned it, silently modifying an operand.
		return l.Merge(r)
	case "<", "<=", ">", ">=":
		// Same ruling as arrayOp: a dictionary has no order to compare.
		i.fail(span, "Dictionaries have no order; to compare sizes write Dict.size(a) %s Dict.size(b)", op)
	}
	i.fail(span, "unknown Dictionary operator '%s'", op)
	return value.NilValue
}

func (i *Interp) overflow(op string, a, b int64, span source.Span) {
	i.fail(span, "Int overflow: %d %s %d does not fit in an Int", a, op, b)
}

// addInt, subInt and mulInt report whether the result is representable.
//
// Aria's Int is one fixed-width signed integer with no unsigned counterpart and
// no way for a program to observe or intend a wrap, so a wrapped result is a
// wrong answer that looks like a computed one. See docs/architecture.md.
func addInt(a, b int64) (int64, bool) {
	s := a + b
	if (a > 0 && b > 0 && s < 0) || (a < 0 && b < 0 && s >= 0) {
		return 0, false
	}
	return s, true
}

func subInt(a, b int64) (int64, bool) {
	d := a - b
	if (b < 0 && d < a) || (b > 0 && d > a) {
		return 0, false
	}
	return d, true
}

func mulInt(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	// MinInt64 has no positive counterpart, so p/b == a cannot distinguish an
	// overflow from a correct product where it is involved.
	if a == math.MinInt64 || b == math.MinInt64 {
		if a == 1 || b == 1 {
			return a * b, true
		}
		return 0, false
	}
	p := a * b
	if p/b != a {
		return 0, false
	}
	return p, true
}

// powInt is exponentiation by squaring, exact within int64. exp must not be
// negative; intOp handles that case before calling.
func powInt(base, exp int64) (int64, bool) {
	result, b := int64(1), base
	for exp > 0 {
		if exp&1 == 1 {
			r, ok := mulInt(result, b)
			if !ok {
				return 0, false
			}
			result = r
		}
		exp >>= 1
		if exp == 0 {
			break // the last squaring is never used, and can overflow
		}
		sq, ok := mulInt(b, b)
		if !ok {
			return 0, false
		}
		b = sq
	}
	return result, true
}

func intRange(from, to int64) value.Value {
	var out []value.Value
	if from <= to {
		for n := from; n <= to; n++ {
			out = append(out, value.Int(n))
		}
	} else {
		for n := from; n >= to; n-- {
			out = append(out, value.Int(n))
		}
	}
	return value.NewArray(out)
}

func (i *Interp) charRange(from, to string, span source.Span) value.Value {
	fr, tr := []rune(from), []rune(to)
	if len(fr) != 1 || len(tr) != 1 {
		i.fail(span, "a String range needs two single-character strings")
	}

	var out []value.Value
	if fr[0] <= tr[0] {
		for c := fr[0]; c <= tr[0]; c++ {
			out = append(out, value.String(string(c)))
		}
	} else {
		for c := fr[0]; c >= tr[0]; c-- {
			out = append(out, value.String(string(c)))
		}
	}
	return value.NewArray(out)
}

// ---------------------------------------------------------------------------
// Conversions
// ---------------------------------------------------------------------------

func (i *Interp) convert(v value.Value, to string, span source.Span) value.Value {
	out, err := Convert(v, to)
	if err != "" {
		i.fail(span, "%s", err)
	}
	return out
}

// ---------------------------------------------------------------------------
// Diagnostics
// ---------------------------------------------------------------------------

// Report writes a runtime error in the same shape as a compile-time diagnostic,
// so both look the same to whoever is reading the terminal.
func (i *Interp) Report(err *Error) {
	file := err.File
	if file == nil {
		file = i.file
	}
	bag := diag.New(file)
	bag.Errorf(err.Span, "%s", err.Msg)
	fmt.Fprint(i.Err, bag.Render())
}

// stdin returns a buffered reader over the configured input.
func (i *Interp) stdin() *bufio.Reader { return bufio.NewReader(i.In) }
