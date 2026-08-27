package resolver

import (
	"strings"
	"testing"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/parser"
	"github.com/fadion/aria/internal/source"
)

func resolve(t *testing.T, src string) (*ast.Program, *Info, *diag.Bag) {
	t.Helper()
	file := source.NewFile("test.ari", []byte(src))
	bag := diag.New(file)

	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		t.Fatalf("%q: parse failed before resolution could run:\n%s", src, bag.Render())
	}

	info := New(file, bag).Resolve(prog)
	return prog, info, bag
}

// resolveOK resolves and fails on any diagnostic.
func resolveOK(t *testing.T, src string) (*ast.Program, *Info) {
	t.Helper()
	prog, info, bag := resolve(t, src)
	if bag.HasErrors() {
		t.Fatalf("%q: unexpected diagnostics:\n%s", src, bag.Render())
	}
	return prog, info
}

// resolveErr resolves and requires a diagnostic containing want.
func resolveErr(t *testing.T, src, want string) {
	t.Helper()
	_, _, bag := resolve(t, src)
	if !bag.HasErrors() {
		t.Errorf("%q: expected a diagnostic mentioning %q, got none", src, want)
		return
	}
	if got := bag.Render(); !strings.Contains(got, want) {
		t.Errorf("%q: diagnostic did not mention %q:\n%s", src, want, got)
	}
}

// A name may be shadowed by an inner scope. The original walked parent
// scopes when checking for redeclaration, so this was an error.
func TestShadowingIsAllowed(t *testing.T) {
	sources := []string{
		"let x = 1\nlet g = func () do\n  let x = 2\n  x\nend",
		"let x = 1\nif true then\n  let x = 2\n  x\nend",
		"let v = 1\nfor v in [1, 2]\n  v\nend",
		"let x = 1\nlet f = func (x) do\n  x\nend",
	}
	for _, src := range sources {
		resolveOK(t, src)
	}
}

// An inner declaration must actually bind the inner one, not just avoid an error.
func TestShadowResolvesToInnerBinding(t *testing.T) {
	const src = "let x = 1\nlet g = func () do\n  let x = 2\n  x\nend"
	prog, info := resolveOK(t, src)

	var outer, inner *Binding
	ast.Walk(prog, func(n ast.Node) bool {
		if let, ok := n.(*ast.Let); ok && let.Name != nil && let.Name.Value == "x" {
			b, _ := info.Declaration(let.Name)
			if outer == nil {
				outer = b
			} else {
				inner = b
			}
		}
		return true
	})

	if outer == nil || inner == nil {
		t.Fatal("did not find both declarations of x")
	}
	if outer == inner {
		t.Fatal("both declarations produced the same binding")
	}
	if inner.Depth <= outer.Depth {
		t.Errorf("inner x is at depth %d, outer at %d; inner should be deeper", inner.Depth, outer.Depth)
	}

	// The use of x inside the function must reach the inner binding, zero hops up.
	var use *ast.Identifier
	ast.Walk(prog, func(n ast.Node) bool {
		if id, ok := n.(*ast.Identifier); ok && id.Value == "x" {
			if _, isRef := info.Lookup(id); isRef {
				use = id
			}
		}
		return true
	})
	if use == nil {
		t.Fatal("did not find a use of x")
	}
	ref, _ := info.Lookup(use)
	if ref.Binding != inner {
		t.Errorf("use of x resolved to the outer binding, want the inner one")
	}
	if ref.Hops != 0 {
		t.Errorf("Hops = %d, want 0 for a name in the same scope", ref.Hops)
	}
}

// Immutability belongs to the binding, not the name. A `let` inside one
// function must not affect an unrelated `var` of the same name elsewhere.
func TestImmutabilityIsPerBindingNotPerName(t *testing.T) {
	const src = `let f = func () do
  let counter = 1
  counter
end
var counter = 99
counter = 5
counter`
	resolveOK(t, src)
}

func TestCannotReassignLet(t *testing.T) {
	resolveErr(t, "let a = 1\na = 2", "cannot assign to 'a'")
	resolveErr(t, "let a = 1\na += 1", "cannot assign to 'a'")
}

func TestCanReassignVar(t *testing.T) {
	resolveOK(t, "var a = 1\na = 2")
	resolveOK(t, "var a = 1\na += 1")
}

// A subscript write is a rebinding under immutable collections, so it needs
// a `var` even though it looks like an in-place edit.
func TestSubscriptWriteNeedsMutableBinding(t *testing.T) {
	resolveErr(t, "let a = [1, 2]\na[] = 3", "cannot modify 'a'")
	resolveErr(t, "let a = [1, 2]\na[0] = 3", "cannot modify 'a'")
	resolveOK(t, "var a = [1, 2]\na[] = 3")
	resolveOK(t, "var a = [1, 2]\na[0] = 3")
}

// A function cannot rebind its parameter, so it cannot mutate a
// collection its caller owns. This is the hole `Enum.insert` used.
func TestParametersAreImmutable(t *testing.T) {
	resolveErr(t, "let f = func (array) do\n  array[] = 1\nend", "cannot modify 'array'")
	resolveErr(t, "let f = func (x) do\n  x = 1\nend", "cannot assign to 'x'")
}

func TestLoopVariablesAreImmutable(t *testing.T) {
	resolveErr(t, "for v in [1, 2]\n  v = 3\nend", "cannot assign to 'v'")
}

// a loop variable must not outlive its loop.
func TestLoopVariableDoesNotLeak(t *testing.T) {
	resolveErr(t, "for v in [1, 2]\n  v\nend\nv", "'v' is not defined")
}

// Names in an inner scope must not leak outward either.
func TestBlockScopesDoNotLeak(t *testing.T) {
	resolveErr(t, "if true then\n  let inner = 1\n  inner\nend\ninner", "'inner' is not defined")
	resolveErr(t, "let f = func () do\n  let hidden = 1\n  hidden\nend\nhidden", "'hidden' is not defined")
}

func TestUndefinedNamesAreReported(t *testing.T) {
	resolveErr(t, "nope", "'nope' is not defined")
	resolveErr(t, "let a = missing", "'missing' is not defined")
	resolveErr(t, "let a = 1\nlet b = a + gone", "'gone' is not defined")
}

func TestRedeclarationInSameScopeIsReported(t *testing.T) {
	resolveErr(t, "let a = 1\nlet a = 2", "already declared in this scope")
	resolveErr(t, "var a = 1\nlet a = 2", "already declared in this scope")
	resolveErr(t, "let f = func (a, a) do a end", "already declared in this scope")
}

// A name may not be read inside its own initializer, but a function assigned to
// a name may still call that name — the check is limited to the innermost scope
// so recursion keeps working.
func TestSelfReferenceInInitializer(t *testing.T) {
	resolveErr(t, "let x = x", "used in its own definition")
	resolveErr(t, "let x = x + 1", "used in its own definition")

	resolveOK(t, "let fact = func (n) do\n  if n <= 1 then\n    return 1\n  end\n  n * fact(n - 1)\nend")
	resolveOK(t, "let even = func (n) do\n  n\nend\nlet odd = func (n) do\n  even(n)\nend")
}

func TestBuiltinsAreAvailable(t *testing.T) {
	for _, name := range Builtins {
		resolveOK(t, name+"(1)")
	}
}

func TestModules(t *testing.T) {
	resolveOK(t, "module M\n  let a = 1\nend\nM.a")

	// Members may refer to each other in any order, since modules are hoisted.
	resolveOK(t, "module M\n  let a = b\n  let b = 1\nend\nM.a")

	// A module name is an ordinary binding now, so an undefined one is an
	// undefined name — `.` knows nothing about modules.
	resolveErr(t, "Nope.thing", "'Nope' is not defined")
}

func TestPredeclaredModules(t *testing.T) {
	file := source.NewFile("t.ari", []byte("Enum.size([1])"))
	bag := diag.New(file)
	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		t.Fatalf("parse failed:\n%s", bag.Render())
	}

	r := New(file, bag)
	r.PredeclareModule("Enum")
	r.Resolve(prog)

	if bag.HasErrors() {
		t.Errorf("predeclared module was not recognised:\n%s", bag.Render())
	}
}

// An import can bring in names this pass cannot see, so undefined-name checking
// has to stand down rather than guess.
// An import no longer turns undefined-name checking off. Every imported file is
// a unit of the same compilation, resolved into the same scope, so a name that
// is not there is reported like any other — the resolver's headline benefit used
// to be off for exactly the programs large enough to need it.
func TestImportDoesNotSuppressUndefinedErrors(t *testing.T) {
	_, _, bag := resolve(t, "import \"other\"\nsomethingFromOther")
	if !bag.HasErrors() {
		t.Error("an undefined name went unreported in a file that imports")
	}
}

func TestTypeNamesAreNotResolvedAsValues(t *testing.T) {
	resolveOK(t, "let a = 1\na is Int")
	resolveOK(t, "let a = 1\na as String")
	resolveOK(t, "let f = func (n: Int) -> Int\n  n\nend")
}

// Every use of a name must resolve to something, or be reported. A use that is
// silently left unresolved would surface as a nil dereference in the evaluator.
func TestEveryNameUseIsResolvedOrReported(t *testing.T) {
	const src = `let a = 1
var b = 2
let f = func (x, y: Int) -> Int
  let inner = x + y + a
  inner
end
for i, v in [1, 2]
  println(i + v + b)
end
module M
  let m = 1
end
println(M.m)
b = f(1, 2)`

	prog, info := resolveOK(t, src)

	// Collect names that are genuinely value references, skipping the positions
	// where an Identifier is a type name, a member name or a declaration.
	skip := map[*ast.Identifier]bool{}
	ast.Walk(prog, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Is:
			skip[n.Right] = true
		case *ast.As:
			skip[n.Right] = true
		case *ast.Access:
			skip[n.Name] = true
		case *ast.FunctionParameter:
			skip[n.Name], skip[n.Type] = true, true
		case *ast.Function:
			skip[n.ReturnType] = true
		case *ast.Let:
			skip[n.Name] = true
		case *ast.Var:
			skip[n.Name] = true
		case *ast.Module:
			skip[n.Name] = true
		case *ast.For:
			if n.Arguments != nil {
				for _, id := range n.Arguments.Elements {
					skip[id] = true
				}
			}
		}
		return true
	})

	ast.Walk(prog, func(n ast.Node) bool {
		id, ok := n.(*ast.Identifier)
		if !ok || skip[id] {
			return true
		}
		if _, found := info.Lookup(id); !found {
			t.Errorf("use of '%s' at %v was neither resolved nor reported",
				id.Value, id.Span())
		}
		return true
	})
}

// Slots must be unique within a scope, so an evaluator can index a slice.
func TestSlotsAreUniquePerScope(t *testing.T) {
	const src = `let a = 1
let b = 2
let f = func (x, y) do
  let c = 3
  let d = 4
  x + y + c + d
end`
	prog, info := resolveOK(t, src)

	perDepth := map[int]map[int]string{}
	ast.Walk(prog, func(n ast.Node) bool {
		var id *ast.Identifier
		switch n := n.(type) {
		case *ast.Let:
			id = n.Name
		case *ast.Var:
			id = n.Name
		case *ast.FunctionParameter:
			id = n.Name
		}
		if id == nil {
			return true
		}
		b, ok := info.Declaration(id)
		if !ok {
			return true
		}
		if perDepth[b.Depth] == nil {
			perDepth[b.Depth] = map[int]string{}
		}
		if prev, clash := perDepth[b.Depth][b.Slot]; clash {
			t.Errorf("depth %d slot %d used by both '%s' and '%s'", b.Depth, b.Slot, prev, b.Name)
		}
		perDepth[b.Depth][b.Slot] = b.Name
		return true
	})
}

// A Bad node must not stop resolution or provoke a second diagnostic.
func TestBadNodesAreSkipped(t *testing.T) {
	file := source.NewFile("t.ari", []byte("let x = ]\nlet y = 1\ny"))
	bag := diag.New(file)
	prog := parser.New(file, bag).Parse()

	before := bag.Len()
	New(file, bag).Resolve(prog)

	// The parser reported the syntax error; the resolver should add nothing.
	if bag.Len() != before {
		t.Errorf("resolver added %d diagnostics to a tree that already failed to parse:\n%s",
			bag.Len()-before, bag.Render())
	}
}

// tryResolve parses and resolves src, reporting whether it parsed at all.
func tryResolve(src string) (*diag.Bag, bool) {
	file := source.NewFile("test.ari", []byte(src))
	bag := diag.New(file)

	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		return bag, false
	}
	New(file, bag).Resolve(prog)
	return bag, true
}

// break, continue and return outside the construct they control are resolver
// errors. Interp.Run stops its node loop on any signal, so a stray one at top
// level used to discard the rest of the file with no diagnostic at all.
func TestStrayControlKeywords(t *testing.T) {
	for _, src := range []string{
		"break",
		"continue",
		"return 1",
		"for v in [1]\n  let f = func () do break end\nend",
	} {
		bag, ok := tryResolve(src)
		if !ok {
			t.Fatalf("%q did not parse", src)
		}
		if !bag.HasErrors() {
			t.Errorf("%q resolved cleanly, expected a diagnostic", src)
		}
	}

	// The legal placements stay legal.
	for _, src := range []string{
		"for v in [1]\n  break\nend",
		"for v in [1]\n  continue\nend",
		"let f = func () do return 1 end",
	} {
		bag, ok := tryResolve(src)
		if !ok {
			t.Fatalf("%q did not parse", src)
		}
		if bag.HasErrors() {
			t.Errorf("%q was rejected:\n%s", src, bag.Render())
		}
	}
}

// `_` means something as a switch case value and as an append target, and
// nothing anywhere else — where it used to evaluate to nil.
func TestPlaceholderOutOfPosition(t *testing.T) {
	for _, src := range []string{"let x = _", "println([1][_])", "let a = [_]"} {
		bag, ok := tryResolve(src)
		if !ok {
			t.Fatalf("%q did not parse", src)
		}
		if !bag.HasErrors() {
			t.Errorf("%q resolved cleanly, expected a diagnostic", src)
		}
	}

	for _, src := range []string{
		"var a = [1]\na[] = 2",
		"var a = [1]\na[_] = 2",
		"switch 1\ncase _ then 2\nend",
	} {
		bag, ok := tryResolve(src)
		if !ok {
			t.Fatalf("%q did not parse", src)
		}
		if bag.HasErrors() {
			t.Errorf("%q was rejected:\n%s", src, bag.Render())
		}
	}
}

// The evaluator checked loop-variable arity once per iteration, so an empty
// enumerable never reached the check.
func TestForLoopVariableArity(t *testing.T) {
	bag, ok := tryResolve("for a, b, c in []\n  println(a)\nend")
	if !ok {
		t.Fatal("did not parse")
	}
	if !bag.HasErrors() {
		t.Error("three loop variables resolved cleanly, expected a diagnostic")
	}
}
