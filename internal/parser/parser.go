// Package parser turns a token stream into an AST.
//
// It is a Pratt parser, like the one it replaces. What changed is the
// discipline around it:
//
//   - One cursor invariant, stated once and never broken (see below).
//   - Parsing never returns nil. A failure yields an ast.Bad covering the
//     offending span, so the tree is always well-formed and no consumer has to
//     nil-check a child. `nil` used to mean both "no value" and "an error
//     happened", which is what let a nil dereference reach production.
//   - Reporting and recovery are separate. reportError only reports; a caller
//     that wants to resynchronise asks for it. They used to be fused, so
//     reporting an error silently advanced the token stream at every call site.
//   - Depth is bounded, so deeply nested input fails with a diagnostic instead
//     of exhausting the stack.
//
// # Cursor invariant
//
// Every parse method is entered with p.tok on the FIRST token of the construct
// it parses, and returns with p.tok on the LAST token of that construct — never
// past it. Advancing to the next construct is the caller's job. Helpers
// (at, accept, expect, advance) are the only places the cursor moves, so the
// invariant is checkable by reading them rather than by tracing ~90 bare
// advance() calls.
package parser

import (
	"math"
	"strconv"
	"strings"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/scanner"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/token"
)

// maxDepth bounds nesting. Go grows goroutine stacks on demand, so deep input
// does not crash outright, but it does slow to a crawl and can exhaust memory.
// A limit turns that into a diagnostic.
const maxDepth = 250

// A Parser builds an AST from one source file.
type Parser struct {
	file  *source.File
	diags *diag.Bag

	sc   *scanner.Scanner
	tok  token.Token // current
	peek token.Token // one ahead

	depth int
	// panicking suppresses cascading diagnostics: after an error, messages are
	// held until the parser reaches a token it can resynchronise on.
	panicking bool
}

// New returns a Parser over file, reporting into diags.
func New(file *source.File, diags *diag.Bag) *Parser {
	p := &Parser{
		file:  file,
		diags: diags,
		sc:    scanner.New(file, diags, 0),
	}
	// Prime both tokens.
	p.tok = p.sc.Scan()
	p.peek = p.sc.Scan()
	return p
}

// Parse reads the whole file.
func (p *Parser) Parse() *ast.Program {
	start := p.tok.Span.Start
	prog := &ast.Program{}

	for !p.at(token.EOF) {
		if p.acceptSeparator() {
			continue
		}

		node := p.parseNode()
		prog.Nodes = append(prog.Nodes, node)
		markDiscarded(prog.Nodes)

		// A construct ends on its last token, so step off it. If the parse
		// failed we may already be on a separator, which the loop head handles.
		if !p.at(token.EOF) {
			p.advance()
		}
	}

	prog.Sp = source.Span{Start: start, End: p.tok.Span.End}
	return prog
}

// ---------------------------------------------------------------------------
// Cursor
// ---------------------------------------------------------------------------

func (p *Parser) advance() {
	p.tok = p.peek
	p.peek = p.sc.Scan()
}

// at reports whether the current token is one of kinds.
func (p *Parser) at(kinds ...token.Kind) bool {
	for _, k := range kinds {
		if p.tok.Kind == k {
			return true
		}
	}
	return false
}

// atPeek reports whether the next token is one of kinds.
func (p *Parser) atPeek(kinds ...token.Kind) bool {
	for _, k := range kinds {
		if p.peek.Kind == k {
			return true
		}
	}
	return false
}

// accept consumes the current token if it matches, leaving the cursor on it.
func (p *Parser) accept(kind token.Kind) bool {
	if p.tok.Kind != kind {
		return false
	}
	p.advance()
	return true
}

// expect requires the NEXT token to be kind and steps onto it, which is the
// shape most call sites want: having parsed a construct's last token, move onto
// the delimiter that must follow.
func (p *Parser) expectPeek(kind token.Kind) bool {
	if p.peek.Kind != kind {
		p.errorAt(p.peek.Span, "expected %s but found %s", kind, p.describe(p.peek))
		return false
	}
	p.advance()
	return true
}

// acceptSeparator consumes a newline, the statement separator.
func (p *Parser) acceptSeparator() bool {
	return p.accept(token.Newline)
}

// skipSeparators consumes any run of newlines.
func (p *Parser) skipSeparators() {
	for p.at(token.Newline) {
		p.advance()
	}
}

// describe names a token for a diagnostic, quoting the source text for tokens
// whose kind alone would not identify them.
func (p *Parser) describe(tok token.Token) string {
	switch tok.Kind {
	case token.Ident, token.Int, token.Float, token.String:
		return "'" + p.text(tok) + "'"
	case token.EOF:
		return "end of file"
	case token.Newline:
		return "end of line"
	}
	return "'" + tok.Kind.String() + "'"
}

func (p *Parser) text(tok token.Token) string { return p.file.Text(tok.Span) }

// ---------------------------------------------------------------------------
// Errors and recovery
// ---------------------------------------------------------------------------

// errorAt records a diagnostic. It does not move the cursor: recovery is a
// separate decision, made by whoever knows where a safe resume point is.
func (p *Parser) errorAt(span source.Span, format string, args ...any) {
	if p.panicking {
		// Everything between an error and the next resume point describes the
		// confusion caused by the first error, not a new problem.
		return
	}
	p.panicking = true
	p.diags.Errorf(span, format, args...)
}

// bad reports an error and returns a Bad node covering span.
func (p *Parser) bad(span source.Span, format string, args ...any) *ast.Bad {
	p.errorAt(span, format, args...)
	return &ast.Bad{Base: ast.Base{Sp: span}, Text: p.file.Text(span)}
}

// badHere is bad() for the current token.
func (p *Parser) badHere(format string, args ...any) *ast.Bad {
	return p.bad(p.tok.Span, format, args...)
}

// recover advances to a token that can begin a new construct, so parsing can
// continue and find further independent errors.
//
// The original recovery set was missing VAR, added to the language later, so
// any parse error swallowed every following `var` statement to end of file.
// Deriving the set from what actually starts a construct avoids that drift.
func (p *Parser) recover() {
	p.panicking = false
	for !p.at(token.EOF) {
		if p.at(token.Newline) {
			p.advance()
			return
		}
		if startsConstruct(p.tok.Kind) {
			return
		}
		p.advance()
	}
}

// startsConstruct reports whether kind can begin a top-level construct. These
// are the resume points for error recovery.
func startsConstruct(kind token.Kind) bool {
	switch kind {
	case token.Let, token.Var, token.Func, token.If, token.Switch, token.For,
		token.While, token.Until,
		token.Module, token.Import, token.Return, token.Break, token.Continue,
		token.Case, token.Default, token.End:
		return true
	}
	return false
}

// enter bounds recursion depth. It returns false when the limit is hit, and the
// caller must return a Bad node.
func (p *Parser) enter() bool {
	p.depth++
	if p.depth > maxDepth {
		p.errorAt(p.tok.Span, "expression nested too deeply (limit %d)", maxDepth)
		return false
	}
	return true
}

func (p *Parser) leave() { p.depth-- }

// ---------------------------------------------------------------------------
// Nodes
// ---------------------------------------------------------------------------

// parseNode parses one construct: a statement-like keyword or an expression.
func (p *Parser) parseNode() ast.Node {
	switch p.tok.Kind {
	case token.Return:
		return p.parseReturn()
	case token.Break:
		return p.parseBreak()
	case token.Continue:
		n := &ast.Continue{Base: ast.Base{Sp: p.tok.Span}}
		return n
	}

	node := p.parseExpr(lowest)
	if _, isBad := node.(*ast.Bad); isBad {
		p.recover()
	}
	return node
}

// parseExpr is the Pratt loop: parse a prefix, then absorb infix operators
// that bind tighter than minPower.
func (p *Parser) parseExpr(minPower int) ast.Node {
	if !p.enter() {
		return &ast.Bad{Base: ast.Base{Sp: p.tok.Span}, Text: p.text(p.tok)}
	}
	defer p.leave()

	left := p.parsePrefix()

	for {
		power := lbp(p.peek.Kind)
		if power <= minPower {
			return left
		}
		// A Bad on the left cannot be operated on; stop rather than build a
		// tree around it and report a second error describing the first.
		if _, isBad := left.(*ast.Bad); isBad {
			return left
		}

		p.advance()
		left = p.parseInfix(left)
	}
}

// parsePrefix parses whatever can begin an expression.
func (p *Parser) parsePrefix() ast.Node {
	switch p.tok.Kind {
	case token.Ident:
		return &ast.Identifier{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok)}
	case token.Int:
		return p.parseInteger()
	case token.Float:
		return p.parseFloat()
	case token.String:
		return p.parseString()
	case token.StringStart:
		return p.parseInterpolation()
	case token.Bool:
		return &ast.Boolean{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok) == "true"}
	case token.Nil:
		return &ast.Nil{Base: ast.Base{Sp: p.tok.Span}}
	case token.Underscore:
		return &ast.Placeholder{Base: ast.Base{Sp: p.tok.Span}}
	case token.Colon:
		return p.parseAtom()

	case token.Bang, token.BitNot, token.Minus:
		return p.parsePrefixOp()

	case token.LParen:
		return p.parseGroup()
	case token.LBracket:
		return p.parseArrayOrDictionary()

	case token.Let:
		return p.parseBinding(token.Let)
	case token.Var:
		return p.parseBinding(token.Var)
	case token.Func:
		return p.parseFunction()
	case token.If:
		return p.parseIf()
	case token.Switch:
		return p.parseSwitch()
	case token.While:
		return p.parseWhile(false)
	case token.Until:
		return p.parseWhile(true)
	case token.Do:
		return p.parseBlockExpression()
	case token.For:
		return p.parseFor()
	case token.Module:
		return p.parseModule()
	case token.Import:
		return p.parseImport()

	case token.Invalid:
		// The scanner already reported why. Yield a Bad without a second
		// message describing the same mistake.
		return &ast.Bad{Base: ast.Base{Sp: p.tok.Span}, Text: p.text(p.tok)}
	}

	return p.badHere("unexpected %s", p.describe(p.tok))
}

// parseInfix parses an operator with left already parsed. The cursor is on the
// operator.
func (p *Parser) parseInfix(left ast.Node) ast.Node {
	switch p.tok.Kind {
	case token.Assign, token.AssignPlus, token.AssignMinus, token.AssignMul,
		token.AssignDiv, token.AssignMod, token.AssignPow:
		return p.parseAssign(left)
	case token.LParen:
		return p.parseCall(left)
	case token.LBracket:
		return p.parseSubscript(left)
	case token.Dot:
		return p.parseAccess(left, false)
	case token.SafeDot:
		return p.parseAccess(left, true)
	case token.Pipe:
		return p.parsePipe(left)
	case token.Arrow:
		return p.parseArrowFunction(left)
	case token.Question:
		return p.parseTernary(left)
	case token.Is:
		return p.parseTypeOp(left, true)
	case token.As:
		return p.parseTypeOp(left, false)
	}
	return p.parseInfixOp(left)
}

// parseInfixOp handles the ordinary binary operators.
func (p *Parser) parseInfixOp(left ast.Node) ast.Node {
	op := p.tok
	operator := p.text(op)

	p.advance()
	right := p.parseExpr(rightBindingPower(op.Kind))

	return &ast.Infix{
		Base:     ast.Base{Sp: span(left.Span(), right.Span())},
		Left:     left,
		Operator: operator,
		Right:    right,
	}
}

func (p *Parser) parsePrefixOp() ast.Node {
	op := p.tok
	operator := p.text(op)

	// The most-negative int64 has no positive counterpart, so it can only be
	// written with its minus sign attached. Folding the two together here is the
	// only way to express it.
	//
	// This does not disturb 2.3: `-2 ** 2` still groups as `-(2 ** 2)`, because
	// only this one magnitude folds and no ordinary literal is affected.
	if op.Kind == token.Minus && p.peek.Kind == token.Int {
		if text := p.text(p.peek); isMinInt64Magnitude(text) {
			p.advance()
			return &ast.Integer{
				Base:  ast.Base{Sp: span(op.Span, p.tok.Span)},
				Value: math.MinInt64,
				Text:  "-" + text,
			}
		}
	}

	p.advance()
	right := p.parseExpr(prefixBindingPower)

	return &ast.Prefix{
		Base:     ast.Base{Sp: span(op.Span, right.Span())},
		Operator: operator,
		Right:    right,
	}
}

// ---------------------------------------------------------------------------
// Literals
// ---------------------------------------------------------------------------

func (p *Parser) parseInteger() ast.Node {
	text := p.text(p.tok)
	clean := strings.ReplaceAll(text, "_", "")

	// ParseInt understands 0x and 0b directly, but wants 0o27 spelled as 0o27
	// or 027; base 0 handles both. Passing base 0 lets one call cover every
	// prefix the scanner accepts.
	value, err := strconv.ParseInt(clean, 0, 64)
	if err != nil {
		return p.badHere("integer literal %s is out of range", text)
	}
	return &ast.Integer{Base: ast.Base{Sp: p.tok.Span}, Value: value, Text: text}
}

func (p *Parser) parseFloat() ast.Node {
	text := p.text(p.tok)
	value, err := strconv.ParseFloat(strings.ReplaceAll(text, "_", ""), 64)
	if err != nil {
		return p.badHere("float literal %s is out of range", text)
	}
	return &ast.Float{Base: ast.Base{Sp: p.tok.Span}, Value: value, Text: text}
}

func (p *Parser) parseString() ast.Node {
	raw := p.text(p.tok)
	// The scanner validated the delimiters and escapes; strip the delimiters
	// here.
	inner := raw
	if len(inner) >= 2 {
		inner = inner[1 : len(inner)-1]
	}

	// A backtick literal is raw: it spans lines and processes no escapes, so a
	// regex can be written the way the regex engine reads it. Carriage returns
	// are dropped, as Go does, so the same source means the same string on a
	// checkout with CRLF line endings.
	if strings.HasPrefix(raw, "`") {
		text := strings.ReplaceAll(inner, "\r", "")
		return &ast.String{Base: ast.Base{Sp: p.tok.Span}, Value: text, Text: text}
	}

	return &ast.String{
		Base:  ast.Base{Sp: p.tok.Span},
		Value: unescape(inner),
		Text:  inner,
	}
}

// unescape decodes the escape sequences the scanner accepted.
func unescape(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		case 'x', 'u':
			// A codepoint, not a byte, even for \xNN: Aria strings are UTF-8
			// and index by rune, so a raw high byte would build a string the
			// rest of the language cannot read. The scanner has already proved
			// the digits are there and spell something legal.
			r, width := decodeHexEscape(s[i:])
			b.WriteRune(r)
			i += width - 1
		default:
			// Covers \\ and \" ; anything else was rejected by the scanner.
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// parseInterpolation reads a string literal with `#{...}` holes. The cursor is
// on the StringStart piece.
//
// The scanner hands the pieces over as ordinary tokens with the hole's own
// tokens between them, so the expressions parse with the grammar the parser
// already has and their spans point at where they are written — which is what a
// diagnostic inside a hole needs.
func (p *Parser) parseInterpolation() ast.Node {
	start := p.tok.Span
	parts := []ast.Node{p.stringPiece(1, 2)} // "...#{

	for {
		p.advance()
		if p.at(token.EOF) {
			return p.bad(span(start, p.tok.Span), "unterminated string interpolation")
		}
		parts = append(parts, p.parseExpr(lowest))
		p.advance()

		switch p.tok.Kind {
		case token.StringPart:
			parts = append(parts, p.stringPiece(1, 2)) // }...#{
		case token.StringEnd:
			parts = append(parts, p.stringPiece(1, 1)) // }..."
			return &ast.Interpolation{Base: ast.Base{Sp: span(start, p.tok.Span)}, Parts: parts}
		default:
			return p.bad(span(start, p.tok.Span),
				"expected the end of an interpolation, found %s", p.describe(p.tok))
		}
	}
}

// stringPiece turns the current string-piece token into a literal, trimming
// head bytes from the front and tail from the back — the delimiters, which
// differ per piece.
func (p *Parser) stringPiece(head, tail int) ast.Node {
	raw := p.text(p.tok)
	inner := ""
	if len(raw) >= head+tail {
		inner = raw[head : len(raw)-tail]
	}
	return &ast.String{Base: ast.Base{Sp: p.tok.Span}, Value: unescape(inner), Text: inner}
}

// decodeHexEscape reads \xNN, \uNNNN or \u{N...} from the start of s, which
// begins at the x or the u. It returns the rune and how much of s it consumed.
func decodeHexEscape(s string) (rune, int) {
	i := 1
	braced := s[0] == 'u' && i < len(s) && s[i] == '{'
	if braced {
		i++
	}

	var v rune
	for i < len(s) && isHexDigit(s[i]) {
		v = v*16 + rune(hexValue(s[i]))
		i++
	}
	if braced && i < len(s) && s[i] == '}' {
		i++
	}
	return v, i
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return int(c-'A') + 10
	}
}

// parseAtom reads `:name`. The cursor is on the colon.
func (p *Parser) parseAtom() ast.Node {
	start := p.tok.Span
	if !p.expectPeek(token.Ident) {
		return &ast.Bad{Base: ast.Base{Sp: start}, Text: p.file.Text(start)}
	}
	return &ast.Atom{
		Base:  ast.Base{Sp: span(start, p.tok.Span)},
		Value: p.text(p.tok),
	}
}

// ---------------------------------------------------------------------------
// Bindings
// ---------------------------------------------------------------------------

// parseBinding reads `let name = value` or `var name = value`.
func (p *Parser) parseBinding(kind token.Kind) ast.Node {
	start := p.tok.Span
	word := kind.String()

	if !p.expectPeek(token.Ident) {
		return &ast.Bad{Base: ast.Base{Sp: start}, Text: word}
	}
	name := &ast.Identifier{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok)}

	if !p.expectPeek(token.Assign) {
		return &ast.Bad{Base: ast.Base{Sp: span(start, p.tok.Span)}, Text: word}
	}

	p.advance()
	value := p.parseExpr(lowest)
	sp := span(start, value.Span())

	if kind == token.Let {
		return &ast.Let{Base: ast.Base{Sp: sp}, Name: name, Value: value}
	}
	return &ast.Var{Base: ast.Base{Sp: sp}, Name: name, Value: value}
}

// parseAssign reads `name = value` and the compound forms. The cursor is on the
// operator.
func (p *Parser) parseAssign(left ast.Node) ast.Node {
	op := p.tok
	operator := p.text(op)

	switch target := left.(type) {
	case *ast.Identifier:
	case *ast.Subscript:
		// Indexing can nest, as in a[0][1], so walk down to the name being
		// written through rather than only checking one level.
		base := ast.Node(target)
		for {
			sub, ok := base.(*ast.Subscript)
			if !ok {
				break
			}
			base = sub.Left
		}
		if _, ok := base.(*ast.Identifier); !ok {
			return p.bad(left.Span(), "assignment expects a name on the left")
		}
	default:
		return p.bad(left.Span(), "assignment expects a name on the left")
	}

	p.advance()
	right := p.parseExpr(precAssign - 1) // right-associative
	sp := span(left.Span(), right.Span())

	// Compound assignment desugars here: `a += b` becomes `a = a + b`, so the
	// evaluator only ever sees plain assignment.
	if operator != "=" {
		right = &ast.Infix{
			Base:     ast.Base{Sp: sp},
			Left:     left,
			Operator: strings.TrimSuffix(operator, "="),
			Right:    right,
		}
	}

	return &ast.Assign{Base: ast.Base{Sp: sp}, Name: left, Operator: "=", Right: right}
}

// ---------------------------------------------------------------------------
// Grouping and collections
// ---------------------------------------------------------------------------

// parseGroup reads a parenthesised expression, or a parenthesised list when it
// finds a comma — the argument list of an arrow function.
func (p *Parser) parseGroup() ast.Node {
	start := p.tok.Span

	// An empty pair is the parameter list of a zero-argument arrow function.
	if p.atPeek(token.RParen) {
		p.advance()
		return &ast.ExpressionList{Base: ast.Base{Sp: span(start, p.tok.Span)}}
	}

	p.advance()
	first := p.parseExpr(lowest)

	if p.atPeek(token.Comma) {
		list := &ast.ExpressionList{Elements: []ast.Node{first}}
		for p.atPeek(token.Comma) {
			p.advance() // onto the comma
			p.advance() // onto the element
			list.Elements = append(list.Elements, p.parseExpr(lowest))
		}
		if !p.expectPeek(token.RParen) {
			return &ast.Bad{Base: ast.Base{Sp: span(start, p.tok.Span)}, Text: "("}
		}
		list.Sp = span(start, p.tok.Span)
		return list
	}

	if !p.expectPeek(token.RParen) {
		return &ast.Bad{Base: ast.Base{Sp: span(start, p.tok.Span)}, Text: "("}
	}
	return first
}

// parseArrayOrDictionary reads `[...]`, deciding which by looking for `=>`.
func (p *Parser) parseArrayOrDictionary() ast.Node {
	start := p.tok.Span

	if p.atPeek(token.RBracket) {
		p.advance()
		return &ast.Array{
			Base: ast.Base{Sp: span(start, p.tok.Span)},
			List: &ast.ExpressionList{Base: ast.Base{Sp: span(start, p.tok.Span)}},
		}
	}

	// `[=>]` is the empty dictionary, distinguishing it from the empty array
	// `[]`. The standard library uses it to seed an accumulator.
	if p.atPeek(token.FatArrow) {
		p.advance()
		if !p.expectPeek(token.RBracket) {
			return &ast.Bad{Base: ast.Base{Sp: span(start, p.tok.Span)}, Text: "["}
		}
		return &ast.Dictionary{Base: ast.Base{Sp: span(start, p.tok.Span)}}
	}

	var elements []ast.Node
	var pairs []ast.Pair
	isDict := false

	for {
		p.advance()
		p.skipSeparators()
		if p.at(token.RBracket) {
			break
		}

		key := p.parseExpr(lowest)

		if p.atPeek(token.FatArrow) {
			isDict = true
			p.advance() // onto =>
			p.advance() // onto the value
			pairs = append(pairs, ast.Pair{Key: key, Value: p.parseExpr(lowest)})
		} else {
			elements = append(elements, key)
		}

		p.skipPeekSeparators()
		if p.atPeek(token.Comma) {
			// Step onto the comma; the loop head then steps onto the element
			// after it. The cursor is still on the previous element's last
			// token here, per the invariant.
			p.advance()
			continue
		}
		if p.atPeek(token.RBracket) {
			p.advance()
			break
		}
		if p.atPeek(token.EOF) {
			return p.bad(span(start, p.tok.Span), "missing ']' to close this literal")
		}
		// Commas are optional between elements, so `[5 7 9]` is three elements.
		// The README documents this, and dropping it would silently remove a
		// language feature.
	}

	sp := span(start, p.tok.Span)

	if isDict {
		if len(elements) > 0 {
			return p.bad(sp, "dictionary literal mixes plain elements with key => value pairs")
		}
		return &ast.Dictionary{Base: ast.Base{Sp: sp}, Pairs: pairs}
	}
	return &ast.Array{
		Base: ast.Base{Sp: sp},
		List: &ast.ExpressionList{Base: ast.Base{Sp: sp}, Elements: elements},
	}
}

// skipPeekSeparators consumes newlines sitting between the cursor and the next
// meaningful token, so a collection may span lines.
func (p *Parser) skipPeekSeparators() {
	for p.atPeek(token.Newline) {
		p.advance()
	}
}

// ---------------------------------------------------------------------------
// Postfix operators
// ---------------------------------------------------------------------------

// parseCall reads an argument list. The cursor is on `(`.
func (p *Parser) parseCall(fn ast.Node) ast.Node {
	start := p.tok.Span
	list := &ast.ExpressionList{Base: ast.Base{Sp: start}}

	if p.atPeek(token.RParen) {
		p.advance()
		list.Sp = span(start, p.tok.Span)
		return &ast.FunctionCall{
			Base:      ast.Base{Sp: span(fn.Span(), p.tok.Span)},
			Function:  fn,
			Arguments: list,
		}
	}

	for {
		p.advance()
		p.skipSeparators()
		list.Elements = append(list.Elements, p.parseExpr(lowest))

		p.skipPeekSeparators()
		if p.atPeek(token.Comma) {
			p.advance()
			continue
		}
		if !p.expectPeek(token.RParen) {
			return &ast.Bad{Base: ast.Base{Sp: span(fn.Span(), p.tok.Span)}, Text: "("}
		}
		break
	}

	list.Sp = span(start, p.tok.Span)
	return &ast.FunctionCall{
		Base:      ast.Base{Sp: span(fn.Span(), p.tok.Span)},
		Function:  fn,
		Arguments: list,
	}
}

// parseSubscript reads `left[index]`. The cursor is on `[`.
func (p *Parser) parseSubscript(left ast.Node) ast.Node {
	start := p.tok.Span

	// `a[]` and `a[_]` are both the append target.
	if p.atPeek(token.RBracket) {
		p.advance()
		return &ast.Subscript{
			Base:  ast.Base{Sp: span(left.Span(), p.tok.Span)},
			Left:  left,
			Index: &ast.Placeholder{Base: ast.Base{Sp: span(start, p.tok.Span)}},
		}
	}

	p.advance()
	index := p.parseExpr(lowest)

	if !p.expectPeek(token.RBracket) {
		return &ast.Bad{Base: ast.Base{Sp: span(left.Span(), p.tok.Span)}, Text: "["}
	}

	return &ast.Subscript{
		Base:  ast.Base{Sp: span(left.Span(), p.tok.Span)},
		Left:  left,
		Index: index,
	}
}

// parseAccess reads `left.member`. The cursor is on the dot.
//
// Left is whatever expression preceded the dot. It used to have to be a bare
// identifier, which is what stopped `.` from chaining; the old parser also
// called TokenLexeme on it without checking for nil, which is the crash a
// fuzzer found in under a second.
func (p *Parser) parseAccess(left ast.Node, safe bool) ast.Node {
	if !p.expectPeek(token.Ident) {
		return &ast.Bad{Base: ast.Base{Sp: span(left.Span(), p.tok.Span)}, Text: "."}
	}

	return &ast.Access{
		Base: ast.Base{Sp: span(left.Span(), p.tok.Span)},
		Left: left,
		Name: &ast.Identifier{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok)},
		Safe: safe,
	}
}

// parsePipe reads `left |> right`.
func (p *Parser) parsePipe(left ast.Node) ast.Node {
	p.advance()
	right := p.parseExpr(precPipe)
	return &ast.Pipe{
		Base:  ast.Base{Sp: span(left.Span(), right.Span())},
		Left:  left,
		Right: right,
	}
}

// parseTypeOp reads `left is Type` or `left as Type`.
func (p *Parser) parseTypeOp(left ast.Node, isTest bool) ast.Node {
	word := "as"
	if isTest {
		word = "is"
	}

	if !p.expectPeek(token.Ident) {
		return &ast.Bad{Base: ast.Base{Sp: span(left.Span(), p.tok.Span)}, Text: word}
	}
	right := &ast.Identifier{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok)}
	sp := span(left.Span(), p.tok.Span)

	if isTest {
		return &ast.Is{Base: ast.Base{Sp: sp}, Left: left, Right: right}
	}
	return &ast.As{Base: ast.Base{Sp: sp}, Left: left, Right: right}
}

// parseTernary reads `cond ? then : else`, desugared to an If.
func (p *Parser) parseTernary(cond ast.Node) ast.Node {
	p.advance()
	then := p.parseExpr(precTernary - 1)

	if !p.expectPeek(token.Colon) {
		return &ast.Bad{Base: ast.Base{Sp: span(cond.Span(), p.tok.Span)}, Text: "?"}
	}

	p.advance()
	alt := p.parseExpr(precTernary - 1)
	sp := span(cond.Span(), alt.Span())

	return &ast.If{
		Base:      ast.Base{Sp: sp},
		Condition: cond,
		Then:      &ast.Block{Base: ast.Base{Sp: then.Span()}, Nodes: []ast.Node{then}},
		Else:      &ast.Block{Base: ast.Base{Sp: alt.Span()}, Nodes: []ast.Node{alt}},
	}
}

// parseArrowFunction reads `params -> body`, where params was already parsed as
// an identifier or a parenthesised list.
func (p *Parser) parseArrowFunction(left ast.Node) ast.Node {
	fn := &ast.Function{}

	switch params := left.(type) {
	case *ast.Identifier:
		fn.Parameters = append(fn.Parameters, &ast.FunctionParameter{
			Base: ast.Base{Sp: params.Span()},
			Name: params,
		})
	case *ast.ExpressionList:
		for _, el := range params.Elements {
			name, ok := el.(*ast.Identifier)
			if !ok {
				return p.bad(el.Span(), "arrow function expects parameter names")
			}
			fn.Parameters = append(fn.Parameters, &ast.FunctionParameter{
				Base: ast.Base{Sp: name.Span()},
				Name: name,
			})
		}
	default:
		return p.bad(left.Span(), "arrow function expects parameter names")
	}

	p.advance()
	body := p.parseExpr(precArrow - 1)

	fn.Sp = span(left.Span(), body.Span())
	fn.Body = &ast.Block{Base: ast.Base{Sp: body.Span()}, Nodes: []ast.Node{body}}
	return fn
}

// ---------------------------------------------------------------------------
// Keyword constructs
// ---------------------------------------------------------------------------

func (p *Parser) parseReturn() ast.Node {
	start := p.tok.Span

	// A bare `return` yields nil.
	if p.atPeek(token.Newline, token.EOF, token.End) {
		return &ast.Return{Base: ast.Base{Sp: start}}
	}

	p.advance()
	value := p.parseExpr(lowest)
	return &ast.Return{Base: ast.Base{Sp: span(start, value.Span())}, Value: value}
}

func (p *Parser) parseImport() ast.Node {
	start := p.tok.Span

	if !p.atPeek(token.String, token.Ident) {
		return p.bad(span(start, p.peek.Span), "import expects a file name")
	}
	p.advance()

	name := p.text(p.tok)
	if p.tok.Kind == token.String && len(name) >= 2 {
		name = name[1 : len(name)-1]
	}
	return &ast.Import{Base: ast.Base{Sp: span(start, p.tok.Span)}, File: name}
}

// parseIf reads `if cond [then|do] body [else body] end`.
func (p *Parser) parseIf() ast.Node {
	start := p.tok.Span

	p.advance()
	cond := p.parseExpr(lowest)

	p.advance()
	// `then` and `do` are both optional noise before the body.
	if p.at(token.Then, token.Do) {
		p.advance()
	}

	then := p.parseBlock(token.Else, token.End)

	node := &ast.If{Base: ast.Base{Sp: start}, Condition: cond, Then: then}

	if p.at(token.Else) {
		p.advance()
		node.Else = p.parseBlock(token.End)
	}

	if !p.at(token.End) {
		return p.bad(span(start, p.tok.Span), "missing 'end' to close 'if'")
	}

	node.Sp = span(start, p.tok.Span)
	return node
}

// parseBlock reads nodes until one of terminators, leaving the cursor ON the
// terminator.
func (p *Parser) parseBlock(terminators ...token.Kind) *ast.Block {
	start := p.tok.Span
	block := &ast.Block{Base: ast.Base{Sp: start}}

	stop := append([]token.Kind{token.EOF}, terminators...)

	for !p.at(stop...) {
		if p.acceptSeparator() {
			continue
		}
		node := p.parseNode()
		block.Nodes = append(block.Nodes, node)
		markDiscarded(block.Nodes)

		// Step off the construct's last token before re-testing for a
		// terminator. Testing first would misread a nested construct's own
		// `end` as this block's, closing every enclosing block at the first
		// inner one.
		if p.at(token.EOF) {
			break
		}
		p.advance()
	}

	block.Sp = span(start, p.tok.Span)
	return block
}

// markDiscarded flags every `for` in nodes but the last as producing a value
// nobody reads, so the evaluator can skip collecting one.
//
// The last node is left alone because a block evaluates to its last node's
// value, which its own caller may well want. Called after each append rather
// than once at the end, so the flag is right whichever node turns out to be
// last.
func markDiscarded(nodes []ast.Node) {
	for i, n := range nodes {
		if loop, ok := n.(*ast.For); ok {
			loop.Discard = i < len(nodes)-1
		}
	}
}

// parseBlockExpression reads `do ... end` in expression position.
//
// Everything in Aria is expression-valued except a block, which is the one place
// the claim was not true. ast.Block and evalBlock already do exactly this; it
// needed a prefix parse and nothing else.
func (p *Parser) parseBlockExpression() ast.Node {
	start := p.tok.Span
	p.advance()

	block := p.parseBlock(token.End)
	if !p.at(token.End) {
		return p.bad(span(start, p.tok.Span), "missing 'end' to close 'do'")
	}
	block.Sp = span(start, p.tok.Span)
	return block
}

// parseBreak reads `break` or `break N`, where N is how many enclosing loops to
// leave. The count is on the same line by construction: a newline ends the
// construct, so `break` followed by a line starting with a number is two nodes.
func (p *Parser) parseBreak() ast.Node {
	n := &ast.Break{Base: ast.Base{Sp: p.tok.Span}, Levels: 1}

	if p.atPeek(token.Int) {
		p.advance()
		levels, err := strconv.ParseInt(strings.ReplaceAll(p.text(p.tok), "_", ""), 0, 64)
		if err != nil || levels < 1 {
			return p.bad(span(n.Sp, p.tok.Span), "break takes a positive number of loops to leave")
		}
		n.Levels = int(levels)
		n.Sp = span(n.Sp, p.tok.Span)
	}
	return n
}

// parseWhile reads `while cond [do] body end`, or the same with `until`, which
// is the identical loop with its condition negated.
func (p *Parser) parseWhile(until bool) ast.Node {
	start := p.tok.Span
	node := &ast.While{Base: ast.Base{Sp: start}, Until: until}

	p.advance()
	node.Condition = p.parseExpr(lowest)
	p.advance()

	if p.at(token.Do) {
		p.advance()
	}

	node.Body = p.parseBlock(token.End)
	if !p.at(token.End) {
		return p.bad(span(start, p.tok.Span), "missing 'end' to close '%s'", keywordOf(until))
	}

	node.Sp = span(start, p.tok.Span)
	return node
}

func keywordOf(until bool) string {
	if until {
		return "until"
	}
	return "while"
}

// parseFor reads `for [names in enumerable] [do] body end`.
func (p *Parser) parseFor() ast.Node {
	start := p.tok.Span
	node := &ast.For{Base: ast.Base{Sp: start}}

	p.advance()

	// An immediate body means the infinite form.
	if !p.at(token.Do, token.Newline) {
		args := &ast.IdentifierList{Base: ast.Base{Sp: p.tok.Span}}
		for !p.at(token.In, token.Do, token.Newline, token.EOF) {
			if p.at(token.Comma) {
				p.advance()
				continue
			}
			if !p.at(token.Ident) {
				return p.bad(p.tok.Span, "for expects loop variable names, found %s", p.describe(p.tok))
			}
			args.Elements = append(args.Elements, &ast.Identifier{
				Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok),
			})
			p.advance()
		}
		// Widen the list to cover every name, not just the first: a diagnostic
		// about the list as a whole — too many loop variables — should underline
		// the whole list.
		if len(args.Elements) > 0 {
			args.Sp = span(args.Elements[0].Span(), args.Elements[len(args.Elements)-1].Span())
		}
		node.Arguments = args

		if p.at(token.In) {
			p.advance()
			node.Enumerable = p.parseExpr(lowest)
			p.advance()
		}
	}

	if p.at(token.Do) {
		p.advance()
	}

	node.Body = p.parseBlock(token.End)

	if !p.at(token.End) {
		return p.bad(span(start, p.tok.Span), "missing 'end' to close 'for'")
	}

	node.Sp = span(start, p.tok.Span)
	return node
}

// parseModule reads `module Name [do] body end`.
func (p *Parser) parseModule() ast.Node {
	start := p.tok.Span

	if !p.expectPeek(token.Ident) {
		return &ast.Bad{Base: ast.Base{Sp: start}, Text: "module"}
	}
	name := &ast.Identifier{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok)}

	// The optional `do` is accepted here. The original checked for it while the
	// cursor was still on the module name, so the branch never fired and
	// `module M do` was a parse error despite the code meaning to allow it.
	if p.atPeek(token.Do) {
		p.advance()
	}

	p.advance()
	body := p.parseBlock(token.End)

	if !p.at(token.End) {
		return p.bad(span(start, p.tok.Span), "missing 'end' to close 'module'")
	}

	return &ast.Module{
		Base: ast.Base{Sp: span(start, p.tok.Span)},
		Name: name,
		Body: body,
	}
}

// parseFunction reads `func (params) [-> Type] body end`.
func (p *Parser) parseFunction() ast.Node {
	start := p.tok.Span
	node := &ast.Function{Base: ast.Base{Sp: start}}
	variadicSeen := false

	p.advance()

	for !p.at(token.Do, token.Newline, token.EOF) {
		switch p.tok.Kind {
		case token.LParen, token.RParen, token.Comma:
			// Parentheses are optional and commas are separators.

		case token.Ellipsis:
			if node.Variadic {
				return p.bad(p.tok.Span, "a function may have only one variadic parameter")
			}
			if !p.atPeek(token.Ident) {
				return p.bad(p.tok.Span, "variadic parameter expects a name")
			}
			node.Variadic = true

		case token.Arrow:
			if !p.atPeek(token.Ident) {
				return p.bad(p.tok.Span, "return type expects a type name")
			}
			p.advance()
			node.ReturnType = &ast.Identifier{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok)}

		case token.Ident:
			param, ok := p.parseParameter(node, variadicSeen)
			if !ok {
				return param
			}
			if node.Variadic {
				variadicSeen = true
			}

		default:
			return p.bad(p.tok.Span, "unexpected %s in parameter list", p.describe(p.tok))
		}

		p.advance()
	}

	if p.at(token.Do) {
		p.advance()
	}

	node.Body = p.parseBlock(token.End)

	if !p.at(token.End) {
		return p.bad(span(start, p.tok.Span), "missing 'end' to close 'func'")
	}

	node.Sp = span(start, p.tok.Span)
	return node
}

// parseParameter reads one parameter with its optional type and default. It
// returns ok=false and a Bad node on failure.
//
// variadicSeen reports whether the variadic parameter has already been taken. It
// is what distinguishes `func (a, ...xs)`, which is fine, from `func (...xs, a)`,
// which is not — checking `fn.Variadic` alone would reject both, since the flag
// is set while the variadic parameter's own name is still being read.
func (p *Parser) parseParameter(fn *ast.Function, variadicSeen bool) (ast.Node, bool) {
	if variadicSeen {
		return p.bad(p.tok.Span, "the variadic parameter must be last"), false
	}

	param := &ast.FunctionParameter{
		Base: ast.Base{Sp: p.tok.Span},
		Name: &ast.Identifier{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok)},
	}

	if p.atPeek(token.Colon) {
		p.advance()
		if !p.expectPeek(token.Ident) {
			return &ast.Bad{Base: ast.Base{Sp: param.Sp}, Text: param.Name.Value}, false
		}
		param.Type = &ast.Identifier{Base: ast.Base{Sp: p.tok.Span}, Value: p.text(p.tok)}
	}

	if p.atPeek(token.Assign) {
		p.advance()
		p.advance()
		param.Default = p.parseExpr(precAssign)
	}

	param.Sp = span(param.Sp, p.tok.Span)
	fn.Parameters = append(fn.Parameters, param)
	return nil, true
}

// parseSwitch reads `switch [control] cases... end`.
func (p *Parser) parseSwitch() ast.Node {
	start := p.tok.Span
	node := &ast.Switch{Base: ast.Base{Sp: start}}

	// A newline or `do` right after `switch` means no control expression, which
	// behaves as `switch true`. The original parsed a control expression
	// unconditionally, so this documented form never parsed at all.
	if !p.atPeek(token.Newline, token.Do) {
		p.advance()
		node.Control = p.parseExpr(lowest)
	}

	p.advance()
	if p.at(token.Do) {
		p.advance()
	}
	p.skipSeparators()

	for !p.at(token.End, token.EOF) {
		switch p.tok.Kind {
		case token.Case:
			c := &ast.SwitchCase{Base: ast.Base{Sp: p.tok.Span}}
			values := &ast.ExpressionList{Base: ast.Base{Sp: p.tok.Span}}

			for {
				p.advance()
				values.Elements = append(values.Elements, p.parseExpr(lowest))
				if p.atPeek(token.Comma) {
					p.advance()
					continue
				}
				break
			}
			c.Values = values

			p.advance()
			if p.at(token.Then) {
				p.advance()
			}
			c.Body = p.parseBlock(token.Case, token.Default, token.End)
			c.Sp = span(c.Sp, p.tok.Span)
			node.Cases = append(node.Cases, c)
			continue

		case token.Default:
			p.advance()
			if p.at(token.Then) {
				p.advance()
			}
			node.Default = p.parseBlock(token.Case, token.Default, token.End)
			continue

		case token.Newline:
			p.advance()

		default:
			return p.bad(p.tok.Span, "expected 'case' or 'default' in switch, found %s", p.describe(p.tok))
		}
	}

	if !p.at(token.End) {
		return p.bad(span(start, p.tok.Span), "missing 'end' to close 'switch'")
	}

	node.Sp = span(start, p.tok.Span)
	return node
}

// span returns the range covering both a and b.
func span(a, b source.Span) source.Span {
	s := a.Start
	if b.Start < s {
		s = b.Start
	}
	e := a.End
	if b.End > e {
		e = b.End
	}
	return source.Span{Start: s, End: e}
}

// isMinInt64Magnitude reports whether text spells 2^63, the magnitude of the
// most-negative int64. Separators are ignored, so 9_223_372_036_854_775_808
// counts too.
func isMinInt64Magnitude(text string) bool {
	return strings.ReplaceAll(text, "_", "") == "9223372036854775808"
}
