// Package token defines Aria's lexical tokens.
//
// A Kind is a small integer rather than a string, so comparisons are integer
// compares and precedence tables can be plain arrays indexed by kind. A Token
// carries only its kind and the span it covers; the text is recovered from the
// source file when needed, which keeps scanning allocation-free.
package token

import "github.com/fadion/aria/internal/source"

// Kind identifies the lexical class of a token.
type Kind uint8

const (
	// Invalid is produced where the scanner could not form a real token. It
	// always comes with a diagnostic, so consumers should stay quiet about it
	// rather than reporting a second error for the same mistake.
	Invalid Kind = iota
	EOF
	Newline
	Comment

	// Literals and names.
	Ident
	Int
	Float
	String
	Bool
	Nil

	// Operators.
	Assign      // =
	AssignPlus  // +=
	AssignMinus // -=
	AssignMul   // *=
	AssignDiv   // /=
	Eq          // ==
	NotEq       // !=
	Lt          // <
	LtEq        // <=
	Gt          // >
	GtEq        // >=
	Plus        // +
	Minus       // -
	Star        // *
	Power       // **
	Slash       // /
	Percent     // %
	BitOr       // |
	BitAnd      // &
	BitNot      // ~
	ShiftLeft   // <<
	ShiftRight  // >>
	Or          // ||
	And         // &&
	Bang        // !
	Pipe        // |>
	Arrow       // ->
	FatArrow    // =>
	Question    // ?

	// Delimiters.
	Comma
	LParen
	RParen
	LBracket
	RBracket
	Colon
	Range      // ..
	Ellipsis   // ...
	Dot        // .
	Underscore // _

	// Keywords. keywordStart and keywordEnd bracket this run so IsKeyword is
	// a range check; keep every keyword between them.
	keywordStart
	Let
	Var
	Func
	Do
	End
	If
	Else
	For
	In
	Is
	As
	Return
	Then
	Switch
	Case
	Default
	Break
	Continue
	Module
	Import
	keywordEnd
)

// names is indexed by Kind. Entries for operators and delimiters are the
// literal text; the rest are descriptive names shown in diagnostics.
var names = [...]string{
	Invalid: "invalid token",
	EOF:     "end of file",
	Newline: "newline",
	Comment: "comment",

	Ident:  "identifier",
	Int:    "integer",
	Float:  "float",
	String: "string",
	Bool:   "boolean",
	Nil:    "nil",

	Assign:      "=",
	AssignPlus:  "+=",
	AssignMinus: "-=",
	AssignMul:   "*=",
	AssignDiv:   "/=",
	Eq:          "==",
	NotEq:       "!=",
	Lt:          "<",
	LtEq:        "<=",
	Gt:          ">",
	GtEq:        ">=",
	Plus:        "+",
	Minus:       "-",
	Star:        "*",
	Power:       "**",
	Slash:       "/",
	Percent:     "%",
	BitOr:       "|",
	BitAnd:      "&",
	BitNot:      "~",
	ShiftLeft:   "<<",
	ShiftRight:  ">>",
	Or:          "||",
	And:         "&&",
	Bang:        "!",
	Pipe:        "|>",
	Arrow:       "->",
	FatArrow:    "=>",
	Question:    "?",

	Comma:      ",",
	LParen:     "(",
	RParen:     ")",
	LBracket:   "[",
	RBracket:   "]",
	Colon:      ":",
	Range:      "..",
	Ellipsis:   "...",
	Dot:        ".",
	Underscore: "_",

	Let:      "let",
	Var:      "var",
	Func:     "func",
	Do:       "do",
	End:      "end",
	If:       "if",
	Else:     "else",
	For:      "for",
	In:       "in",
	Is:       "is",
	As:       "as",
	Return:   "return",
	Then:     "then",
	Switch:   "switch",
	Case:     "case",
	Default:  "default",
	Break:    "break",
	Continue: "continue",
	Module:   "module",
	Import:   "import",
}

// String returns the token's literal text for operators and keywords, or a
// descriptive name for everything else. Suitable for use in diagnostics.
func (k Kind) String() string {
	if int(k) < len(names) && names[k] != "" {
		return names[k]
	}
	return "unknown token"
}

// IsKeyword reports whether k is a reserved word.
func (k Kind) IsKeyword() bool { return k > keywordStart && k < keywordEnd }

// keywords maps reserved words to their kind. It is written once here and only
// ever read afterwards, so it is safe to share across concurrent scanners —
// unlike a table that each scanner populates on construction.
var keywords = map[string]Kind{
	"let":      Let,
	"var":      Var,
	"func":     Func,
	"do":       Do,
	"end":      End,
	"if":       If,
	"else":     Else,
	"for":      For,
	"in":       In,
	"is":       Is,
	"as":       As,
	"return":   Return,
	"then":     Then,
	"switch":   Switch,
	"case":     Case,
	"default":  Default,
	"break":    Break,
	"continue": Continue,
	"module":   Module,
	"import":   Import,
	// true, false and nil are keywords lexically, but carry a literal's kind
	// so the parser treats them as the values they are.
	"true":  Bool,
	"false": Bool,
	"nil":   Nil,
}

// Lookup returns the keyword kind for name, or Ident if it is not reserved.
func Lookup(name string) Kind {
	if k, ok := keywords[name]; ok {
		return k
	}
	return Ident
}

// A Token is one lexical unit: its kind and the source it covers.
type Token struct {
	Kind Kind
	Span source.Span
}

// String describes the token for debugging. It does not include the source
// text, which the token does not carry.
func (t Token) String() string { return t.Kind.String() }
