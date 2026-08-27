// Package ast defines Aria's syntax tree.
//
// Aria is expression-oriented: `if`, `switch` and `for` all produce values, and
// `let` can appear anywhere an expression can. So there is one Node interface
// rather than a Statement/Expression split. The old tree carried `statement()`
// and `expression()` marker methods that had drifted far enough apart for vet
// to catch an impossible type assertion between them.
//
// Every node carries the span it was parsed from, so any node can anchor a
// diagnostic without the parser having to thread positions separately.
package ast

import (
	"strconv"
	"strings"

	"github.com/fadion/aria/internal/source"
)

// A Node is any piece of the syntax tree.
type Node interface {
	Span() source.Span
	// Inspect renders the node back to a readable form. It is a debugging and
	// testing aid, not a formatter: precedence is made explicit with parens so
	// the shape of the tree is visible.
	Inspect() string
}

// Base supplies Span to every node. Embed it rather than repeating the method.
type Base struct{ Sp source.Span }

func (b Base) Span() source.Span { return b.Sp }

// ---------------------------------------------------------------------------
// Root
// ---------------------------------------------------------------------------

// Program is the root of a parsed file.
type Program struct {
	Base
	Nodes []Node
}

func (n *Program) Inspect() string { return inspectAll(n.Nodes, "") }

// Block is a sequence of nodes, as in a function or loop body.
type Block struct {
	Base
	Nodes []Node
}

func (n *Block) Inspect() string { return inspectAll(n.Nodes, "") }

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// Bad marks source the parser could not make sense of.
//
// Parsing never returns a nil Node. A failed parse yields a Bad covering the
// offending span, so the tree is always well-formed and no consumer has to
// nil-check. A Bad always comes with a diagnostic already reported, so anything
// walking the tree should stay silent about it rather than adding a second
// message for the same mistake.
type Bad struct {
	Base
	// Text is the source the parser gave up on, for Inspect only.
	Text string
}

func (n *Bad) Inspect() string { return "<bad:" + n.Text + ">" }

// HasBad reports whether the tree rooted at n contains a Bad node.
func HasBad(n Node) bool {
	found := false
	Walk(n, func(child Node) bool {
		if _, ok := child.(*Bad); ok {
			found = true
		}
		return !found
	})
	return found
}

// ---------------------------------------------------------------------------
// Literals and names
// ---------------------------------------------------------------------------

// Identifier is a variable, function or module name.
type Identifier struct {
	Base
	Value string
}

func (n *Identifier) Inspect() string { return n.Value }

// Integer is an integer literal. Text preserves the original spelling, so
// `0xff` and `1_000` render as written.
type Integer struct {
	Base
	Value int64
	Text  string
}

func (n *Integer) Inspect() string { return n.Text }

// Float is a floating point literal.
type Float struct {
	Base
	Value float64
	Text  string
}

func (n *Float) Inspect() string { return n.Text }

// String is a string literal. Value is the decoded text; Text is the source
// spelling without its quotes.
type String struct {
	Base
	Value string
	Text  string
}

func (n *String) Inspect() string { return n.Text }

// Interpolation is a string literal with #{...} holes, as an alternating list
// of literal pieces and expressions.
//
// It is a node rather than a desugaring into `+` and String(...) calls for two
// reasons. `+` does not coerce, and the String conversion refuses a collection,
// so both would make `"#{[1, 2]}"` an error where println prints it; and a
// desugaring that names String would mean anything shadowing that name silently
// changes what every interpolated string in scope does.
type Interpolation struct {
	Base
	Parts []Node
}

func (n *Interpolation) Inspect() string {
	out := "\""
	for _, p := range n.Parts {
		out += p.Inspect()
	}
	return out + "\""
}

// Atom is a `:name` symbol.
type Atom struct {
	Base
	Value string
}

func (n *Atom) Inspect() string { return ":" + n.Value }

// Boolean is `true` or `false`.
type Boolean struct {
	Base
	Value bool
}

func (n *Boolean) Inspect() string {
	if n.Value {
		return "true"
	}
	return "false"
}

// Nil is the `nil` literal.
type Nil struct{ Base }

func (n *Nil) Inspect() string { return "nil" }

// Placeholder is `_`, used as a switch wildcard and an append target.
type Placeholder struct{ Base }

func (n *Placeholder) Inspect() string { return "_" }

// ---------------------------------------------------------------------------
// Collections
// ---------------------------------------------------------------------------

// ExpressionList is a comma-separated list, as in call arguments.
type ExpressionList struct {
	Base
	Elements []Node
}

func (n *ExpressionList) Inspect() string { return inspectAll(n.Elements, ", ") }

// Array is an array literal.
type Array struct {
	Base
	List *ExpressionList
}

func (n *Array) Inspect() string { return "Array(" + n.List.Inspect() + ")" }

// Pair is one key/value entry of a dictionary literal.
type Pair struct {
	Key   Node
	Value Node
}

// Dictionary is a dictionary literal.
//
// Pairs are an ordered slice, not a map. The old tree used
// map[Expression]Expression, which lost source order and made Inspect vary
// between runs — printing a dictionary was not reproducible.
type Dictionary struct {
	Base
	Pairs []Pair
}

func (n *Dictionary) Inspect() string {
	parts := make([]string, 0, len(n.Pairs))
	for _, p := range n.Pairs {
		parts = append(parts, p.Key.Inspect()+" => "+p.Value.Inspect())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ---------------------------------------------------------------------------
// Bindings
// ---------------------------------------------------------------------------

// ArrayPattern is `[a, b, ...rest]` on the left of a binding, taking an array
// apart by shape.
//
// Distinct from an Array literal, which is what a case arm uses: in a binding
// every name is a name being bound, and in a case every value is a value being
// compared. `let` is what marks binding in both — at the front of the statement
// here, and on the individual name in a case.
type ArrayPattern struct {
	Base
	Elements []Node
}

func (n *ArrayPattern) Inspect() string { return "[" + inspectAll(n.Elements, ", ") + "]" }

// Rest is `...name` in a pattern, taking whatever elements are left over.
type Rest struct {
	Base
	Name *Identifier
}

func (n *Rest) Inspect() string { return "..." + inspectOr(n.Name, "") }

// Binder is `let name` inside a case pattern, capturing what matched.
//
// A bare identifier in a case is still a reference compared with the control,
// which is what it has always been. Making an undeclared one bind instead would
// mean the same arm binds in one file and compares in another, and a typo would
// become a binding that always matches — `let` says which is meant, using the
// keyword that already means exactly that.
type Binder struct {
	Base
	Name *Identifier
}

func (n *Binder) Inspect() string { return "let " + inspectOr(n.Name, "") }

// Let binds an immutable name, or takes a value apart by shape when Pattern is
// set instead of Name.
type Let struct {
	Base
	Name    *Identifier
	Pattern *ArrayPattern
	Value   Node
}

func (n *Let) Inspect() string {
	return "let " + bindTarget(n.Name, n.Pattern) + " = " + inspectOr(n.Value, "")
}

// Var binds a reassignable name.
type Var struct {
	Base
	Name    *Identifier
	Pattern *ArrayPattern
	Value   Node
}

func (n *Var) Inspect() string {
	return "var " + bindTarget(n.Name, n.Pattern) + " = " + inspectOr(n.Value, "")
}

// bindTarget renders whichever of a binding's two shapes is in use.
func bindTarget(name *Identifier, pattern *ArrayPattern) string {
	if pattern != nil {
		return pattern.Inspect()
	}
	return inspectOr(name, "")
}

// Assign reassigns an existing binding. Name is an Identifier or a Subscript.
type Assign struct {
	Base
	Name     Node
	Operator string
	Right    Node
}

func (n *Assign) Inspect() string {
	return n.Name.Inspect() + " " + n.Operator + " " + n.Right.Inspect()
}

// ---------------------------------------------------------------------------
// Operators
// ---------------------------------------------------------------------------

// Prefix is a unary operator applied to its operand.
type Prefix struct {
	Base
	Operator string
	Right    Node
}

func (n *Prefix) Inspect() string { return "(" + n.Operator + n.Right.Inspect() + ")" }

// Infix is a binary operator. Inspect always parenthesises, which is what makes
// precedence and associativity visible in tests.
type Infix struct {
	Base
	Left     Node
	Operator string
	Right    Node
}

func (n *Infix) Inspect() string {
	return "(" + n.Left.Inspect() + " " + n.Operator + " " + n.Right.Inspect() + ")"
}

// Subscript indexes into an array, dictionary or string.
type Subscript struct {
	Base
	Left  Node
	Index Node
}

func (n *Subscript) Inspect() string { return n.Left.Inspect() + "[" + n.Index.Inspect() + "]" }

// Pipe feeds its left operand in as the first argument on the right.
type Pipe struct {
	Base
	Left  Node
	Right Node
}

func (n *Pipe) Inspect() string { return n.Left.Inspect() + " |> " + n.Right.Inspect() }

// Is is the type-test operator.
type Is struct {
	Base
	Left  Node
	Right *Identifier
}

func (n *Is) Inspect() string { return "(" + n.Left.Inspect() + " is " + n.Right.Inspect() + ")" }

// As is the type-conversion operator.
type As struct {
	Base
	Left  Node
	Right *Identifier
}

func (n *As) Inspect() string { return "(" + n.Left.Inspect() + " as " + n.Right.Inspect() + ")" }

// ---------------------------------------------------------------------------
// Control flow
// ---------------------------------------------------------------------------

// If is a conditional. Else is nil when absent.
type If struct {
	Base
	Condition Node
	Then      *Block
	Else      *Block
}

func (n *If) Inspect() string {
	out := "if " + n.Condition.Inspect() + " then " + n.Then.Inspect()
	if n.Else != nil {
		out += " else " + n.Else.Inspect()
	}
	return out
}

// SwitchCase is one `case` arm.
type SwitchCase struct {
	Base
	Values *ExpressionList
	// Guard is a `when` clause, tested only once a value has already matched.
	// It is what lets the control-less form replace an else-if chain without
	// repeating the subject.
	Guard Node
	Body  *Block
}

func (n *SwitchCase) Inspect() string {
	out := "case " + n.Values.Inspect()
	if n.Guard != nil {
		out += " when " + n.Guard.Inspect()
	}
	return out + " then " + n.Body.Inspect()
}

// TypeCase is `case is Type`, which matches on the control's type rather than
// its value. Aria already has `is` as an operator; this is it in case position,
// where there is no left-hand side to write.
type TypeCase struct {
	Base
	Name *Identifier
}

func (n *TypeCase) Inspect() string { return "is " + n.Name.Inspect() }

// Switch is a multi-way branch. Control is nil for the control-less form, which
// behaves as `switch true`.
type Switch struct {
	Base
	Control Node
	Cases   []*SwitchCase
	Default *Block
}

func (n *Switch) Inspect() string {
	parts := make([]string, 0, len(n.Cases))
	for _, c := range n.Cases {
		parts = append(parts, c.Inspect())
	}
	out := "switch " + inspectOr(n.Control, "") + " -> " + strings.Join(parts, "; ")
	if n.Default != nil {
		out += "; default " + n.Default.Inspect()
	}
	return out
}

// IdentifierList is the loop-variable list of a `for`.
type IdentifierList struct {
	Base
	Elements []*Identifier
}

func (n *IdentifierList) Inspect() string {
	parts := make([]string, 0, len(n.Elements))
	for _, e := range n.Elements {
		parts = append(parts, e.Inspect())
	}
	return strings.Join(parts, ", ")
}

// For is a loop. Enumerable is nil for the infinite form.
//
// Discard says the loop's value is thrown away — it is not the last node of the
// block it sits in — so the evaluator can skip collecting one. `for` is an
// expression that evaluates to an array of every iteration's value, and building
// that array unconditionally made a two-million-iteration side-effect loop peak
// at 137 MB of results nobody read.
type For struct {
	Base
	Arguments  *IdentifierList
	Enumerable Node
	Body       *Block
	Discard    bool
}

func (n *For) Inspect() string {
	out := "for"
	if n.Enumerable != nil {
		out += " ("
		if n.Arguments != nil && len(n.Arguments.Elements) > 0 {
			out += n.Arguments.Inspect() + " in "
		}
		out += n.Enumerable.Inspect() + ")"
	}
	return out + " -> " + n.Body.Inspect()
}

// Return exits a function early.
type Return struct {
	Base
	Value Node
}

func (n *Return) Inspect() string { return "return " + inspectOr(n.Value, "") }

// Break exits a loop.
// Break leaves Levels enclosing loops. `break` is `break 1`; a count is what
// lets a nested loop break outward without a flag variable, which would need a
// `var` and fight the immutability the README spends a section on.
type Break struct {
	Base
	Levels int
}

func (n *Break) Inspect() string {
	if n.Levels > 1 {
		return "break " + strconv.Itoa(n.Levels)
	}
	return "break"
}

// Continue skips to the next iteration.
type Continue struct{ Base }

func (n *Continue) Inspect() string { return "continue" }

// While repeats a body while its condition holds. Until is the same node with
// the condition negated, so the two share every rule.
//
// It evaluates to nil rather than to a collected array: there is no
// per-iteration value worth keeping, which is the whole reason it exists
// alongside `for`.
type While struct {
	Base
	Condition Node
	Until     bool
	Body      *Block
}

func (n *While) Inspect() string {
	keyword := "while "
	if n.Until {
		keyword = "until "
	}
	return keyword + inspectOr(n.Condition, "") + " do " + inspectOr(n.Body, "") + " end"
}

// ---------------------------------------------------------------------------
// Functions and modules
// ---------------------------------------------------------------------------

// FunctionParameter is one declared parameter, with an optional type and
// default value.
type FunctionParameter struct {
	Base
	Name    *Identifier
	Type    *Identifier
	Default Node
}

func (n *FunctionParameter) Inspect() string {
	out := n.Name.Inspect()
	if n.Type != nil {
		out += ":" + n.Type.Inspect()
	}
	if n.Default != nil {
		out += " = " + n.Default.Inspect()
	}
	return out
}

// Function is a function literal.
type Function struct {
	Base
	Parameters []*FunctionParameter
	ReturnType *Identifier
	Variadic   bool
	Body       *Block
}

func (n *Function) Inspect() string {
	parts := make([]string, 0, len(n.Parameters))
	for i, p := range n.Parameters {
		s := p.Inspect()
		if n.Variadic && i == len(n.Parameters)-1 {
			s = "..." + s
		}
		parts = append(parts, s)
	}
	out := "func (" + strings.Join(parts, ", ") + ") "
	if n.ReturnType != nil {
		out += " -> " + n.ReturnType.Value + "\n"
	}
	return out + n.Body.Inspect()
}

// FunctionCall invokes Function with Arguments.
type FunctionCall struct {
	Base
	Function  Node
	Arguments *ExpressionList
}

func (n *FunctionCall) Inspect() string {
	return n.Function.Inspect() + "(" + n.Arguments.Inspect() + ")"
}

// Module is a named container of `let` bindings.
type Module struct {
	Base
	Name *Identifier
	Body *Block
}

func (n *Module) Inspect() string {
	return "module " + n.Name.Inspect() + " { " + n.Body.Inspect() + " }"
}

// Access reads a named member of whatever is on its left, as in `Enum.size`,
// `config.host` or `rows[0].name`.
//
// Left is an arbitrary expression, not an identifier. As a special form over
// two identifiers, `.` could not chain: `cfg.db.host` had `cfg.db` on its left
// and was rejected as a module name, and so were `f().a` and `a[0].k`.
type Access struct {
	Base
	Left Node
	Name *Identifier
	// Safe is `?.`, which yields nil as soon as a link is nil instead of
	// failing. Reading a missing key already yields nil, so threading nil
	// through a chain is an ordinary thing to be doing in Aria.
	Safe bool
}

func (n *Access) Inspect() string {
	dot := "."
	if n.Safe {
		dot = "?."
	}
	return n.Left.Inspect() + dot + n.Name.Inspect()
}

// Import splices another source file into this scope.
type Import struct {
	Base
	File string
}

func (n *Import) Inspect() string { return "import " + n.File }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func inspectAll(nodes []Node, sep string) string {
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		parts = append(parts, n.Inspect())
	}
	return strings.Join(parts, sep)
}

// inspectOr renders n, or fallback when n is absent. Optional children are the
// one place a nil Node is legitimate — an `if` with no `else`, a bare `return`.
func inspectOr(n Node, fallback string) string {
	if n == nil {
		return fallback
	}
	return n.Inspect()
}
