package parser

import (
	"fmt"
	"github.com/fadion/aria/ast"
	"github.com/fadion/aria/lexer"
	"github.com/fadion/aria/reporter"
	"github.com/fadion/aria/token"
	"reflect"
	"strconv"
	"strings"
)

// Parser represents the parser.
type Parser struct {
	lex             *lexer.Lexer
	token           token.Token
	peekToken       token.Token
	prefixFunctions map[token.TokenType]prefixParseFn
	infixFunctions  map[token.TokenType]infixParseFn
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn func(ast.Expression) ast.Expression
)

// New initializes a parser.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{lex: l}

	// Register prefix functions.
	p.prefixFunctions = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.MODULE, p.parseModule)
	p.registerPrefix(token.IF, p.parseIf)
	p.registerPrefix(token.SWITCH, p.parseSwitch)
	p.registerPrefix(token.FOR, p.parseFor)
	p.registerPrefix(token.FUNCTION, p.parseFunction)
	p.registerPrefix(token.IMPORT, p.parseImport)
	p.registerPrefix(token.LBRACK, p.parseArrayOrDictionary)
	p.registerPrefix(token.IDENTIFIER, p.parseIdentifier)
	p.registerPrefix(token.INTEGER, p.parseInteger)
	p.registerPrefix(token.FLOAT, p.parseFloat)
	p.registerPrefix(token.STRING, p.parseString)
	p.registerPrefix(token.BOOLEAN, p.parseBoolean)
	p.registerPrefix(token.NIL, p.parseNil)
	p.registerPrefix(token.BANG, p.parsePrefix)
	p.registerPrefix(token.BITNOT, p.parsePrefix)
	p.registerPrefix(token.MINUS, p.parsePrefix)
	p.registerPrefix(token.LPAREN, p.parseGroup)

	// Register infix functions.
	p.infixFunctions = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.DOT, p.parseModuleAccess)
	p.registerInfix(token.LPAREN, p.parseFunctionCall)
	p.registerInfix(token.LBRACK, p.parseSubscript)
	p.registerInfix(token.PIPE, p.parsePipe)
	p.registerInfix(token.PLUS, p.parseInfix)
	p.registerInfix(token.MINUS, p.parseInfix)
	p.registerInfix(token.SLASH, p.parseInfix)
	p.registerInfix(token.ASTERISK, p.parseInfix)
	p.registerInfix(token.MODULO, p.parseInfix)
	p.registerInfix(token.POWER, p.parseInfix)
	p.registerInfix(token.EQ, p.parseInfix)
	p.registerInfix(token.UNEQ, p.parseInfix)
	p.registerInfix(token.LT, p.parseInfix)
	p.registerInfix(token.LTE, p.parseInfix)
	p.registerInfix(token.GT, p.parseInfix)
	p.registerInfix(token.GTE, p.parseInfix)
	p.registerInfix(token.OR, p.parseInfix)
	p.registerInfix(token.AND, p.parseInfix)
	p.registerInfix(token.BITAND, p.parseInfix)
	p.registerInfix(token.BITOR, p.parseInfix)
	p.registerInfix(token.BITSHLEFT, p.parseInfix)
	p.registerInfix(token.BITSHRIGHT, p.parseInfix)
	p.registerInfix(token.RANGE, p.parseInfix)

	// In the first advance, only the peek token
	// is set. The second sets both the current and
	// peek correctly.
	p.advance()
	p.advance()

	return p
}

// Advance to the next token.
func (p *Parser) advance() {
	p.token = p.peekToken
	p.peekToken = p.lex.NextToken()
}

// Check against the current token.
func (p *Parser) match(t ...token.TokenType) bool {
	for _, v := range t {
		if p.token.Type == v {
			return true
		}
	}

	return false
}

// Check against the next token.
func (p *Parser) peekMatch(t ...token.TokenType) bool {
	for _, v := range t {
		if p.peekToken.Type == v {
			return true
		}
	}

	return false
}

// Register prefix function.
func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixFunctions[tokenType] = fn
}

// Register infix function.
func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixFunctions[tokenType] = fn
}

// Get the precedence of the current token.
func (p *Parser) precedence() int {
	if p, ok := precedences[p.token.Type]; ok {
		return p
	}

	return LOWEST
}

// Get the precedence of the next token.
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

// Parse tokens into an AST.
func (p *Parser) Parse() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	// Scan tokens until an EOF.
	for !p.match(token.EOF) {
		statement := p.parseStatement()
		// IsValid() checks the inner value of the interface.
		// Simply checking for nil wouldn't work.
		if reflect.ValueOf(statement).IsValid() {
			program.Statements = append(program.Statements, statement)
		}
		p.advance()
	}

	return program
}

// Parse a statetement.
func (p *Parser) parseStatement() ast.Statement {
	switch p.token.Type {
	case token.LET:
		return p.parseLet()
	case token.RETURN:
		return p.parseReturn()
	case token.BREAK:
		return p.parseBreak()
	case token.CONTINUE:
		return p.parseContinue()
	case token.COMMENT: // Ignore comments.
		return nil
	case token.NEWLINE: // Ignore newlines.
		return nil
	default:
		return p.parseExpressionStatement()
	}
}

// module IDENT BODY
func (p *Parser) parseModule() ast.Expression {
	expression := &ast.Module{Token: p.token}

	// Check for an identifier.
	if !p.peekMatch(token.IDENTIFIER) {
		p.reportError("Expecting an identifier as MODULE name")
		return nil
	}

	p.advance()

	expression.Name = &ast.Identifier{Token: p.token, Value: p.token.Lexeme}

	// Ignore optional DO.
	if p.match(token.DO) {
		p.advance()
	}

	expression.Body = p.parseBlockBody()

	// Missing END token.
	if !p.match(token.END) {
		p.reportError("Missing END closing statement in MODULE")
		return nil
	}

	return expression
}

// IDENT.IDENT
func (p *Parser) parseModuleAccess(left ast.Expression) ast.Expression {
	switch object := left.(type) {
	case *ast.Identifier: // Expect an identifier on the left side.
		expression := &ast.ModuleAccess{Token: p.token, Object: object}
		// Expect an identifier on the right side.
		if p.peekMatch(token.IDENTIFIER) {
			p.advance()
			expression.Parameter = &ast.Identifier{Token: p.token, Value: p.token.Lexeme}
			return expression
		}
	default:
		p.reportError(fmt.Sprintf("Cannot use '%s' as MODULE caller", object.TokenLexeme()))
	}

	return nil
}

// let IDENT = EXPRESSION
func (p *Parser) parseLet() *ast.Let {
	statement := &ast.Let{Token: p.token}

	// Check for identifier.
	if !p.peekMatch(token.IDENTIFIER) {
		p.reportError("LET statement expects an identifier")
		return nil
	}

	p.advance()
	statement.Name = &ast.Identifier{Token: p.token, Value: p.token.Lexeme}

	// Check for assignment operator.
	if !p.peekMatch(token.ASSIGN) {
		p.reportError("Missing assignment in LET statement")
		return nil
	}

	p.advance()
	p.advance()
	statement.Value = p.parseExpression(LOWEST)

	return statement
}

// A variable, function, etc.
func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.token, Value: p.token.Lexeme}
}

// Integer literal.
func (p *Parser) parseInteger() ast.Expression {
	literal := &ast.Integer{Token: p.token}
	lexeme := p.token.Lexeme

	var value int64
	var err error

	if strings.HasPrefix(lexeme, "0b") {
		// Binary: 0b1010. Native Go type doesn't use the "0b" prefix.
		value, err = strconv.ParseInt(lexeme[2:], 2, 64)
	} else if strings.HasPrefix(lexeme, "0x") {
		// Hexadecimal: 0xff.
		value, err = strconv.ParseInt(lexeme, 0, 64)
	} else if strings.HasPrefix(lexeme, "0o") {
		// Octal: 0o27. Native Go type uses just a "0" as prefix.
		value, err = strconv.ParseInt(lexeme[:1]+lexeme[2:], 0, 64)
	} else {
		// Normal integer.
		value, err = strconv.ParseInt(lexeme, 0, 64)
	}

	if err != nil {
		p.reportError(fmt.Sprintf("Couldn't parse %s as Integer", p.token.Lexeme))
		return nil
	}

	literal.Value = value

	return literal
}

// Floating point literal.
func (p *Parser) parseFloat() ast.Expression {
	literal := &ast.Float{Token: p.token}

	// Convert to a 64 bit floating point.
	value, err := strconv.ParseFloat(p.token.Lexeme, 64)
	if err != nil {
		p.reportError(fmt.Sprintf("Couldn't parse %s as Float", p.token.Lexeme))
		return nil
	}

	literal.Value = value

	return literal
}

// String literal.
func (p *Parser) parseString() ast.Expression {
	return &ast.String{Token: p.token, Value: p.token.Lexeme}
}

// Boolean literal.
func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.token, Value: p.token.Lexeme == "true"}
}

// Nil.
func (p *Parser) parseNil() ast.Expression {
	return &ast.Nil{Token: p.token}
}

// if CONDITION then THEN else ELSE end
func (p *Parser) parseIf() ast.Expression {
	expression := &ast.If{Token: p.token}
	p.advance()
	expression.Condition = p.parseExpression(LOWEST)

	// Condition is required.
	if expression.Condition == nil {
		p.reportError("Missing condition expression in IF")
		return nil
	}

	p.advance()

	// Remove the optional THEN or DO.
	if p.match(token.THEN, token.DO) {
		p.advance()
	}

	block := &ast.BlockStatement{Token: p.token}
	block.Statements = []ast.Statement{}

	// Parse the THEN block until it ends, normally
	// by an ELSE or END token. Doesn't use parseBlockBody()
	// as any other block does, as it needs to check for ELSE too.
	for !p.match(token.END, token.ELSE, token.EOF) {
		statement := p.parseStatement()
		if statement != nil {
			block.Statements = append(block.Statements, statement)
		}
		p.advance()
	}

	if len(block.Statements) == 0 {
		p.reportError("Empty body in IF")
		return nil
	}

	expression.Then = block

	// Parse the optional ELSE block.
	if p.match(token.ELSE) {
		elseBody := p.parseBlockBody()

		if len(elseBody.Statements) == 0 {
			p.reportError("Empty ELSE body in IF")
			return nil
		}

		expression.Else = elseBody
	}

	// Missing END token.
	if !p.match(token.END) {
		p.reportError("Missing END closing statement in IF")
		return nil
	}

	return expression
}

// switch EXPRESSION case EXPRESSION LIST BLOCK default BLOCK end
func (p *Parser) parseSwitch() ast.Expression {
	expression := &ast.Switch{Token: p.token}
	p.advance()
	expression.Control = p.parseExpression(LOWEST)

	// Control is required.
	if expression.Control != nil {
		p.advance()
	}

	// Ignore the optional DO token.
	if !p.match(token.DO, token.NEWLINE) {
		p.reportError("Missing DO statement in inline SWITCH")
	}

	p.advance()

	var cases []*ast.SwitchCase
	// Consume the switch until it ends, either with an
	// END or EOF token.
	for !p.match(token.END, token.EOF) {
		switch p.token.Type {
		case token.CASE: // A case,
			switchcase := &ast.SwitchCase{Token: p.token}

			// A case can have more than one parameter to
			// compare to.
			list := &ast.ExpressionList{Token: p.token}
			p.advance()
			list.Elements = p.parseDelimited(token.COMMA, token.NEWLINE, token.THEN)

			// A case should have at least one expression.
			if len(list.Elements) == 0 {
				p.reportError("Missing expression in SWITCH CASE")
				break
			}

			switchcase.Values = list
			switchcase.Body = p.parseSwitchCase()

			cases = append(cases, switchcase)
		case token.DEFAULT: // Default case.
			// Anything except a THEN or a NEWLINE means there are
			// parameters, which the default case can't have.
			if !p.peekMatch(token.THEN, token.NEWLINE) {
				p.reportError("DEFAULT case in SWITCH can't have parameters")
				return nil
			}

			p.advance()

			expression.Default = p.parseSwitchCase()

			if len(expression.Default.Statements) == 0 {
				p.reportError("Missing DEFAULT case body in SWITCH")
				return nil
			}
		}

		p.advance()
	}

	expression.Cases = cases

	// Missing END token.
	if !p.match(token.END) {
		p.reportError("Missing END closing statement in SWITCH")
		return nil
	}

	return expression
}

// Parse the body of a case or default case.
func (p *Parser) parseSwitchCase() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.token}
	block.Statements = []ast.Statement{}

	// Parse the case until a CASE, DEFAULT or END token.
	// AN EOF token usually means a syntax error, caught by
	// parseSwitch().
	for !p.peekMatch(token.CASE, token.DEFAULT, token.END, token.EOF) {
		p.advance()
		statement := p.parseStatement()
		if statement != nil {
			block.Statements = append(block.Statements, statement)
		}
	}

	return block
}

// for IDENT1, IDENT2 in IDENT3 STATEMENTS end
func (p *Parser) parseFor() ast.Expression {
	expression := &ast.For{Token: p.token}
	list := &ast.IdentifierList{}

	p.advance()

	// An immediate DO or NEWLINE token mean there
	// is no expression after FOR.
	if p.match(token.NEWLINE, token.DO) {
		p.reportError("Missing expression in FOR loop")
		return nil
	}

	// Get the arguments until an IN token.
	for !p.match(token.IN, token.EOF) {
		switch p.token.Type {
		case token.COMMA: // Ignore commas.
		case token.DO, token.NEWLINE:
			// An IN is expected to close the arguments.
			// Anything else means it's missing.
			p.reportError("IN statement missing in FOR loop")
			return nil
		default:
			list.Elements = append(list.Elements, &ast.Identifier{Token: p.token, Value: p.token.Lexeme})
		}

		p.advance()
	}

	if len(list.Elements) == 0 {
		p.reportError("Missing arguments in FOR loop")
		return nil
	}

	expression.Arguments = list

	// Ignore the IN token.
	if p.match(token.IN) {
		p.advance()
	}

	expression.Enumerable = p.parseExpression(LOWEST)
	// Missing enumerable.
	if expression.Enumerable == nil {
		p.reportError("Missing enumerable in FOR loop")
		return nil
	}

	// Remove the optional DO token.
	if p.peekMatch(token.DO) {
		p.advance()
	}

	expression.Body = p.parseBlockBody()

	// Empty body.
	if len(expression.Body.Statements) == 0 {
		p.reportError("Empty body in FOR loop")
		return nil
	}

	// Missing END token.
	if !p.match(token.END) {
		p.reportError("Missing END closing statement in FOR loop")
		return nil
	}

	return expression
}

// fn (PARAM1, PARAM2) BODY end
func (p *Parser) parseFunction() ast.Expression {
	literal := &ast.Function{Token: p.token}
	identifiers := &ast.IdentifierList{}
	p.advance()

	// Find parameters until a DO or NEWLINE token.
	for !p.match(token.DO, token.NEWLINE) {
		switch p.token.Type {
		// Ignore commas. Parantheses are optional in
		// a function definition, so they're ignored too.
		case token.COMMA, token.LPAREN, token.RPAREN:
		case token.EOF: // EOF reached. Something's wrong with the syntax.
			p.reportError("Missing body in function")
			return nil
		case token.IDENTIFIER:
			identifiers.Elements = append(identifiers.Elements, &ast.Identifier{Token: p.token, Value: p.token.Lexeme})
		default:
			p.reportError(fmt.Sprintf("Unexpected token '%s' as function parameter", p.token.Type))
			return nil
		}

		p.advance()
	}

	literal.Parameters = identifiers
	literal.Body = p.parseBlockBody()

	// Empty body.
	if len(literal.Body.Statements) == 0 {
		p.reportError("Empty body in function")
		return nil
	}

	// Missing END token.
	if !p.match(token.END) {
		p.reportError("Missing END statement in function")
		return nil
	}

	return literal
}

// IDENT(PARAM1, PARAM2)
func (p *Parser) parseFunctionCall(function ast.Expression) ast.Expression {
	expression := &ast.FunctionCall{Token: p.token, Function: function}
	list := &ast.ExpressionList{Token: p.token}
	p.advance()

	list.Elements = p.parseDelimited(token.COMMA, token.RPAREN)
	expression.Arguments = list

	return expression
}

// IMPORT STRING
func (p *Parser) parseImport() ast.Expression {
	expression := &ast.Import{Token: p.token}
	p.advance()
	file := p.parseExpression(LOWEST)

	// Import needs a string as the filename
	// to be imported.
	switch fileString := file.(type) {
	case *ast.String:
		expression.File = fileString
	default:
		p.reportError("IMPORT expects a string as filename")
		return nil
	}

	return expression
}

// Find out if it's an array or a dictionary.
func (p *Parser) parseArrayOrDictionary() ast.Expression {
	p.advance()

	// After the first key of a dictionary, it's
	// expected a COLON. Otherwise, it's an array.
	if p.peekMatch(token.COLON) {
		return p.parseDictionary()
	}

	return p.parseArray()
}

// [IDENT1, IDENT2]
func (p *Parser) parseArray() ast.Expression {
	expression := &ast.Array{Token: p.token}
	list := &ast.ExpressionList{Token: p.token}
	list.Elements = p.parseDelimited(token.COMMA, token.RBRACK)
	expression.List = list

	return expression
}

// [KEY1: VALUE1, KEY2: VALUE2]
func (p *Parser) parseDictionary() ast.Expression {
	expression := &ast.Dictionary{Token: p.token}
	pairs := map[ast.Expression]ast.Expression{}

	// Parse until a closing right bracket.
	for !p.match(token.RBRACK) {
		switch {
		case p.match(token.NEWLINE) || p.match(token.EOF):
			p.reportError("Missing closing ']' in Dictionary")
			return nil
		case p.match(token.COLON): // Ignore colons.
		case p.peekMatch(token.COLON):
			// As the next token is a colon, the current one should
			// be a key. Check if it's string, the only supported
			// type for a key.
			if !p.match(token.STRING) {
				p.reportError("Dictionary keys can only be String")
				return nil
			}

			key := p.parseString()
			// Advance the current key and colon.
			p.advance()
			p.advance()
			value := p.parseExpression(LOWEST)

			if value == nil {
				p.reportError(fmt.Sprintf("Found key '%s' in Dictionary but no value", key.Inspect()))
				return nil
			}

			pairs[key] = value
		}

		p.advance()
	}

	expression.Pairs = pairs

	return expression
}

// return EXPRESSION
func (p *Parser) parseReturn() *ast.Return {
	statement := &ast.Return{Token: p.token}
	p.advance()
	statement.Value = p.parseExpression(LOWEST)

	return statement
}

// break
func (p *Parser) parseBreak() ast.Statement {
	return &ast.Break{Token: p.token}
}

// continue
func (p *Parser) parseContinue() ast.Statement {
	return &ast.Continue{Token: p.token}
}

// IDENT[INTEGER | STRING]
func (p *Parser) parseSubscript(left ast.Expression) ast.Expression {
	expression := &ast.Subscript{Token: p.token, Left: left}
	p.advance()
	expression.Index = p.parseExpression(LOWEST)

	// Missing closing right bracket.
	if !p.peekMatch(token.RBRACK) {
		p.reportError("Missing closing ] in subscript expression")
		return nil
	}

	p.advance()

	return expression
}

// IDENT() |> IDENT()
func (p *Parser) parsePipe(left ast.Expression) ast.Expression {
	expression := &ast.Pipe{
		Token:    p.token,
		Left:     left,
	}

	precedence := p.precedence()
	p.advance()
	expression.Right = p.parseExpression(precedence)

	return expression
}

// Parse a delimited list of expressions.
func (p *Parser) parseDelimited(delimiter token.TokenType, end ...token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	// Parse until it matches the provided token(s).
	for !p.match(end...) {
		switch p.token.Type {
		case delimiter: // Ignore delimiter.
		case token.NEWLINE, token.EOF:
			p.reportError(fmt.Sprintf("Missing closing '%s' in parameter list", end))
			return list
		default:
			elem := p.parseExpression(LOWEST)
			if elem == nil {
				p.reportError(fmt.Sprintf("Unexpected '%s' in expression list", p.token.Lexeme))
				return list
			}

			list = append(list, elem)
		}

		p.advance()
	}

	return list
}

// Parse a statement of expressions.
func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	statement := &ast.ExpressionStatement{Token: p.token}
	statement.Expression = p.parseExpression(LOWEST)

	return statement
}

// Check if a token is ignored in expression parsing.
func (p *Parser) isIgnoredAsExpression(tok token.TokenType) bool {
	ignored := []token.TokenType{token.NEWLINE, token.EOF, token.RBRACK, token.COLON, token.DO}
	for _, v := range ignored {
		if v == tok {
			return true
		}
	}

	return false
}

// Parse an expression, ie: 1 + 2 | 3 * 5 | fn (x)...
func (p *Parser) parseExpression(precedence int) ast.Expression {
	// Return if token shouldn't be considered in
	// expression parsing.
	if p.isIgnoredAsExpression(p.token.Type) {
		return nil
	}

	// Check if it's a prefix function.
	prefix := p.prefixFunctions[p.token.Type]
	if prefix == nil {
		p.reportError(fmt.Sprintf("Unexpected expression '%s'", p.token.Lexeme))
		return nil
	}
	left := prefix()

	// Run the infix function until the next token has
	// a higher precedence.
	for precedence < p.peekPrecedence() {
		infix := p.infixFunctions[p.peekToken.Type]
		if infix == nil {
			return left
		}

		p.advance()
		left = infix(left)
	}

	return left
}

// Parse a group expression, ie: (IDENT1 + IDENT2)
func (p *Parser) parseGroup() ast.Expression {
	p.advance()
	expression := p.parseExpression(LOWEST)

	// Missing closing right parantheses.
	if !p.peekMatch(token.RPAREN) {
		p.reportError("Missing closing ')' for grouped expression")
		return nil
	}

	p.advance()

	return expression
}

// Parse a prefix expression.
func (p *Parser) parsePrefix() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.token,
		Operator: p.token.Lexeme,
	}

	p.advance()
	expression.Right = p.parseExpression(PREFIX)

	return expression
}

// Parse an infix expression.
func (p *Parser) parseInfix(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.token,
		Operator: p.token.Lexeme,
		Left:     left,
	}

	precedence := p.precedence()
	p.advance()
	expression.Right = p.parseExpression(precedence)

	return expression
}

// Parse expressions in a block statement.
func (p *Parser) parseBlockBody() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.token}
	block.Statements = []ast.Statement{}

	p.advance()

	// A block typically ends with an END or EOF token.
	for !p.match(token.END, token.EOF) {
		if statement := p.parseStatement(); statement != nil {
			block.Statements = append(block.Statements, statement)
		}

		p.advance()
	}

	return block
}

// Report an error in the current location and
// synchronize tokens.
func (p *Parser) reportError(message string) {
	reporter.Error(reporter.PARSE, p.token.Location, message)
	p.synchronize()
}

// Move the cursor until a known token is found, to prevent
// error reporting from showing unneeded consequences.
func (p *Parser) synchronize() {
	for !p.match(token.EOF) {
		switch p.token.Type {
		case token.LET, token.IF, token.SWITCH, token.FOR, token.FUNCTION,
			token.CASE, token.DEFAULT, token.RETURN, token.MODULE:
			return
		}

		p.advance()
	}
}
