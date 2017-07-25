package ast

import (
	"bytes"
	"github.com/fadion/aria/token"
	"strings"
)

// A Node on the AST.
type Node interface {
	TokenLexeme() string
	TokenLocation() token.Location
	Inspect() string
}

// A Statement of code.
type Statement interface {
	Node
	statement()
}

// An Expression of code.
type Expression interface {
	Node
	expression()
}

// Program as the root node.
type Program struct {
	Statements []Statement
}

func (e *Program) TokenLexeme() string {
	if len(e.Statements) > 0 {
		return e.Statements[0].TokenLexeme()
	} else {
		return ""
	}
}

func (e *Program) TokenLocation() token.Location {
	return token.Location{}
}

func (e *Program) Inspect() string {
	var out bytes.Buffer

	for _, s := range e.Statements {
		out.WriteString(s.Inspect())
	}

	return out.String()
}

// Let statement.
type Let struct {
	Token token.Token
	Name  *Identifier
	Value Expression
}

func (e *Let) statement()                    {}
func (e *Let) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Let) TokenLocation() token.Location { return e.Token.Location }
func (e *Let) Inspect() string {
	var out bytes.Buffer

	out.WriteString("let ")
	out.WriteString(e.Name.Inspect())
	out.WriteString(" = ")

	if e.Value != nil {
		out.WriteString(e.Value.Inspect())
	}

	return out.String()
}

// Var statement.
type Var struct {
	Token token.Token
	Name  *Identifier
	Value Expression
}

func (e *Var) statement()                    {}
func (e *Var) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Var) TokenLocation() token.Location { return e.Token.Location }
func (e *Var) Inspect() string {
	var out bytes.Buffer

	out.WriteString("var ")
	out.WriteString(e.Name.Inspect())
	out.WriteString(" = ")

	if e.Value != nil {
		out.WriteString(e.Value.Inspect())
	}

	return out.String()
}

// Identifier (variable).
type Identifier struct {
	Token token.Token
	Value string
}

func (e *Identifier) expression()                   {}
func (e *Identifier) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Identifier) TokenLocation() token.Location { return e.Token.Location }
func (e *Identifier) Inspect() string               { return e.Value }

// String literal.
type String struct {
	Token token.Token
	Value string
}

func (e *String) expression()                   {}
func (e *String) TokenLexeme() string           { return e.Token.Lexeme }
func (e *String) TokenLocation() token.Location { return e.Token.Location }
func (e *String) Inspect() string               { return "\"" + e.Token.Lexeme + "\"" }

// Atom literal.
type Atom struct {
	Token token.Token
	Value string
}

func (e *Atom) expression()                   {}
func (e *Atom) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Atom) TokenLocation() token.Location { return e.Token.Location }
func (e *Atom) Inspect() string               { return ":" + e.Token.Lexeme }

// Integer numeric literal.
type Integer struct {
	Token token.Token
	Value int64
}

func (e *Integer) expression()                   {}
func (e *Integer) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Integer) TokenLocation() token.Location { return e.Token.Location }
func (e *Integer) Inspect() string               { return e.Token.Lexeme }

// Float as a floating point literal.
type Float struct {
	Token token.Token
	Value float64
}

func (e *Float) expression()                   {}
func (e *Float) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Float) TokenLocation() token.Location { return e.Token.Location }
func (e *Float) Inspect() string               { return e.Token.Lexeme }

// Boolean literal.
type Boolean struct {
	Token token.Token
	Value bool
}

func (e *Boolean) expression()                   {}
func (e *Boolean) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Boolean) TokenLocation() token.Location { return e.Token.Location }
func (e *Boolean) Inspect() string               { return e.Token.Lexeme }

// Array literal.
type Array struct {
	Token token.Token
	List  *ExpressionList
}

func (e *Array) expression()                   {}
func (e *Array) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Array) TokenLocation() token.Location { return e.Token.Location }
func (e *Array) Inspect() string {
	var out bytes.Buffer

	out.WriteString("Array(")
	out.WriteString(e.List.Inspect())
	out.WriteString(")")

	return out.String()
}

// Subscript for arrays and dictionaries.
type Subscript struct {
	Token token.Token
	Left  Expression
	Index Expression
}

func (e *Subscript) expression()                   {}
func (e *Subscript) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Subscript) TokenLocation() token.Location { return e.Token.Location }
func (e *Subscript) Inspect() string {
	var out bytes.Buffer

	out.WriteString(e.Left.Inspect())
	out.WriteString("[")
	out.WriteString(e.Index.Inspect())
	out.WriteString("]")

	return out.String()
}

// Subscript for arrays and dictionaries.
type Assign struct {
	Token token.Token
	Name  Expression
	Right Expression
}

func (e *Assign) expression()                   {}
func (e *Assign) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Assign) TokenLocation() token.Location { return e.Token.Location }
func (e *Assign) Inspect() string {
	var out bytes.Buffer

	out.WriteString(e.Name.Inspect())
	out.WriteString(" = ")
	out.WriteString(e.Right.Inspect())

	return out.String()
}

// Pipe operator.
type Pipe struct {
	Token token.Token
	Left  Expression
	Right Expression
}

func (e *Pipe) expression()                   {}
func (e *Pipe) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Pipe) TokenLocation() token.Location { return e.Token.Location }
func (e *Pipe) Inspect() string {
	var out bytes.Buffer

	out.WriteString(e.Left.Inspect())
	out.WriteString(" |> ")
	out.WriteString(e.Right.Inspect())

	return out.String()
}

// Dictionary literal.
type Dictionary struct {
	Token token.Token
	Pairs map[Expression]Expression
}

func (e *Dictionary) expression()                   {}
func (e *Dictionary) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Dictionary) TokenLocation() token.Location { return e.Token.Location }
func (e *Dictionary) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}
	for key, value := range e.Pairs {
		pairs = append(pairs, key.Inspect()+":"+value.Inspect())
	}

	out.WriteString("[")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("]")

	return out.String()
}

// Nil type.
type Nil struct {
	Token token.Token
}

func (e *Nil) expression()                   {}
func (e *Nil) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Nil) TokenLocation() token.Location { return e.Token.Location }
func (e *Nil) Inspect() string               { return e.Token.Lexeme }

// Return statement.
type Return struct {
	Token token.Token
	Value Expression
}

func (e *Return) statement()                    {}
func (e *Return) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Return) TokenLocation() token.Location { return e.Token.Location }
func (e *Return) Inspect() string {
	var out bytes.Buffer

	out.WriteString(e.Token.Lexeme + " ")

	if e.Value != nil {
		out.WriteString(e.Value.Inspect())
	}

	return out.String()
}

// If conditional.
type If struct {
	Token     token.Token
	Condition Expression
	Then      *BlockStatement
	Else      *BlockStatement
}

func (e *If) expression()                   {}
func (e *If) TokenLexeme() string           { return e.Token.Lexeme }
func (e *If) TokenLocation() token.Location { return e.Token.Location }
func (e *If) Inspect() string {
	var out bytes.Buffer

	out.WriteString("if ")
	out.WriteString(e.Condition.Inspect())
	out.WriteString(" then ")
	out.WriteString(e.Then.Inspect())

	if e.Else != nil {
		out.WriteString(" else ")
		out.WriteString(e.Else.Inspect())
	}

	return out.String()
}

// Switch conditional.
type Switch struct {
	Token   token.Token
	Control Expression
	Cases   []*SwitchCase
	Default *BlockStatement
}

func (e *Switch) expression()                   {}
func (e *Switch) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Switch) TokenLocation() token.Location { return e.Token.Location }
func (e *Switch) Inspect() string {
	var out bytes.Buffer

	cases := []string{}
	for _, k := range e.Cases {
		cases = append(cases, k.Inspect())
	}

	out.WriteString("switch ")

	if e.Control != nil {
		out.WriteString(e.Control.Inspect())
	}

	out.WriteString(" -> ")
	out.WriteString(strings.Join(cases, "; "))

	if e.Default != nil {
		out.WriteString("; default ")
		out.WriteString(e.Default.Inspect())
	}

	return out.String()
}

// A SwitchCase on a switch.
type SwitchCase struct {
	Token  token.Token
	Values *ExpressionList
	Body   *BlockStatement
}

func (e *SwitchCase) expression()                   {}
func (e *SwitchCase) TokenLexeme() string           { return e.Token.Lexeme }
func (e *SwitchCase) TokenLocation() token.Location { return e.Token.Location }
func (e *SwitchCase) Inspect() string {
	var out bytes.Buffer

	out.WriteString("case ")
	out.WriteString(e.Values.Inspect())
	out.WriteString(" then ")
	out.WriteString(e.Body.Inspect())

	return out.String()
}

// For iterator.
type For struct {
	Token      token.Token
	Arguments  *IdentifierList
	Enumerable Expression
	Body       *BlockStatement
}

func (e *For) expression()                   {}
func (e *For) TokenLexeme() string           { return e.Token.Lexeme }
func (e *For) TokenLocation() token.Location { return e.Token.Location }
func (e *For) Inspect() string {
	var out bytes.Buffer

	out.WriteString(e.Token.Lexeme)

	if e.Enumerable != nil {
		out.WriteString(" (")
		if e.Arguments != nil {
			out.WriteString(e.Arguments.Inspect())
			out.WriteString(" in ")
		}
		out.WriteString(e.Enumerable.Inspect())
		out.WriteString(")")
	}

	out.WriteString(" -> ")
	out.WriteString(e.Body.Inspect())

	return out.String()
}

// Module block.
type Module struct {
	Token token.Token
	Name  *Identifier
	Body  *BlockStatement
}

func (e *Module) expression()                   {}
func (e *Module) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Module) TokenLocation() token.Location { return e.Token.Location }
func (e *Module) Inspect() string {
	var out bytes.Buffer

	out.WriteString("Module ")
	out.WriteString(e.Name.Inspect())
	out.WriteString(" { ")
	out.WriteString(e.Body.Inspect())
	out.WriteString(" } ")

	return out.String()
}

// ModuleAccess to access module properties
// and methods.
type ModuleAccess struct {
	Token     token.Token
	Object    *Identifier
	Parameter *Identifier
}

func (e *ModuleAccess) expression()                   {}
func (e *ModuleAccess) TokenLexeme() string           { return e.Token.Lexeme }
func (e *ModuleAccess) TokenLocation() token.Location { return e.Token.Location }
func (e *ModuleAccess) Inspect() string {
	var out bytes.Buffer

	out.WriteString(e.Object.Inspect())
	out.WriteString("->")
	out.WriteString(e.Parameter.Inspect())

	return out.String()
}

// Function expression.
type Function struct {
	Token      token.Token
	Parameters *IdentifierList
	Body       *BlockStatement
}

func (e *Function) expression()                   {}
func (e *Function) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Function) TokenLocation() token.Location { return e.Token.Location }
func (e *Function) Inspect() string {
	var out bytes.Buffer

	out.WriteString(e.Token.Lexeme)
	out.WriteString(" (")
	out.WriteString(e.Parameters.Inspect())
	out.WriteString(") -> ")
	out.WriteString(e.Body.Inspect())

	return out.String()
}

// FunctionCall calls a function.
type FunctionCall struct {
	Token     token.Token
	Function  Expression
	Arguments *ExpressionList
}

func (e *FunctionCall) expression()                   {}
func (e *FunctionCall) TokenLexeme() string           { return e.Token.Lexeme }
func (e *FunctionCall) TokenLocation() token.Location { return e.Token.Location }
func (e *FunctionCall) Inspect() string {
	var out bytes.Buffer

	out.WriteString(e.Function.Inspect())
	out.WriteString("(")
	out.WriteString(e.Arguments.Inspect())
	out.WriteString(")")

	return out.String()
}

// Break statement.
type Break struct {
	Token token.Token
}

func (e *Break) statement()                    {}
func (e *Break) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Break) TokenLocation() token.Location { return e.Token.Location }
func (e *Break) Inspect() string               { return e.Token.Lexeme }

// Continue statement.
type Continue struct {
	Token token.Token
}

func (e *Continue) statement()                    {}
func (e *Continue) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Continue) TokenLocation() token.Location { return e.Token.Location }
func (e *Continue) Inspect() string               { return e.Token.Lexeme }

// Placeholder.
type Placeholder struct {
	Token token.Token
}

func (e *Placeholder) expression()                   {}
func (e *Placeholder) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Placeholder) TokenLocation() token.Location { return e.Token.Location }
func (e *Placeholder) Inspect() string               {
	return ""
}

// Import a file.
type Import struct {
	Token token.Token
	File *String
}

func (e *Import) expression()                   {}
func (e *Import) TokenLexeme() string           { return e.Token.Lexeme }
func (e *Import) TokenLocation() token.Location { return e.Token.Location }
func (e *Import) Inspect() string               {
	var out *bytes.Buffer

	out.WriteString("Import ")
	out.WriteString(e.File.Value)

	return out.String()
}

// ExpressionStatement as a statement that
// holds expressions.
type ExpressionStatement struct {
	Token      token.Token
	Expression Expression
}

func (e *ExpressionStatement) statement()                    {}
func (e *ExpressionStatement) TokenLexeme() string           { return e.Token.Lexeme }
func (e *ExpressionStatement) TokenLocation() token.Location { return e.Token.Location }
func (e *ExpressionStatement) Inspect() string {
	if e.Expression != nil {
		return e.Expression.Inspect()
	}

	return ""
}

// BlockStatement that holds several statements.
type BlockStatement struct {
	Token      token.Token
	Statements []Statement
}

func (e *BlockStatement) statement()                    {}
func (e *BlockStatement) TokenLexeme() string           { return e.Token.Lexeme }
func (e *BlockStatement) TokenLocation() token.Location { return e.Token.Location }
func (e *BlockStatement) Inspect() string {
	var out bytes.Buffer

	for _, s := range e.Statements {
		out.WriteString(s.Inspect())
	}

	return out.String()
}

// ExpressionList holds a list of expressions.
type ExpressionList struct {
	Token    token.Token
	Elements []Expression
}

func (e *ExpressionList) expression()                   {}
func (e *ExpressionList) TokenLexeme() string           { return e.Token.Lexeme }
func (e *ExpressionList) TokenLocation() token.Location { return e.Token.Location }
func (e *ExpressionList) Inspect() string {
	var out bytes.Buffer

	elements := []string{}
	for _, el := range e.Elements {
		elements = append(elements, el.Inspect())
	}

	out.WriteString(strings.Join(elements, ", "))

	return out.String()
}

// IdentifierList holds a list of identifiers.
type IdentifierList struct {
	Token    token.Token
	Elements []*Identifier
}

func (e *IdentifierList) statement()                    {}
func (e *IdentifierList) TokenLexeme() string           { return e.Token.Lexeme }
func (e *IdentifierList) TokenLocation() token.Location { return e.Token.Location }
func (e *IdentifierList) Inspect() string {
	var out bytes.Buffer

	elements := []string{}
	for _, el := range e.Elements {
		elements = append(elements, el.Inspect())
	}

	out.WriteString(strings.Join(elements, ", "))

	return out.String()
}

// PrefixExpression as an expression with a prefix
// operator.
type PrefixExpression struct {
	Token    token.Token
	Operator string
	Right    Expression
}

func (e *PrefixExpression) expression()                   {}
func (e *PrefixExpression) TokenLexeme() string           { return e.Token.Lexeme }
func (e *PrefixExpression) TokenLocation() token.Location { return e.Token.Location }
func (e *PrefixExpression) Inspect() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(e.Operator)
	out.WriteString(e.Right.Inspect())
	out.WriteString(")")

	return out.String()
}

// InfixExpression with two expressions on the left
// and right, combined by an operator.
type InfixExpression struct {
	Token    token.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (e *InfixExpression) expression()                   {}
func (e *InfixExpression) TokenLexeme() string           { return e.Token.Lexeme }
func (e *InfixExpression) TokenLocation() token.Location { return e.Token.Location }
func (e *InfixExpression) Inspect() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(e.Left.Inspect())
	out.WriteString(" " + e.Operator + " ")
	out.WriteString(e.Right.Inspect())
	out.WriteString(")")

	return out.String()
}
