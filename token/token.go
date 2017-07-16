package token

// A language token.
type Token struct {
	Type     TokenType
	Lexeme   string
	Location Location
}

// Token location.
type Location struct {
	Row int
	Col int
}

// String alias.
type TokenType string

const (
	// Literals
	IDENTIFIER = "IDENTIFIER"
	INTEGER    = "INTEGER"
	FLOAT      = "FLOAT"
	STRING     = "STRING"
	BOOLEAN    = "BOOLEAN"

	// Operators
	ASSIGN     = "="
	EQ         = "=="
	UNEQ       = "!="
	GT         = ">"
	GTE        = ">="
	LT         = "<"
	LTE        = "<="
	PLUS       = "+"
	MINUS      = "-"
	ASTERISK   = "*"
	POWER      = "**"
	MODULO     = "%"
	SLASH      = "/"
	BITOR      = "|"
	BITAND     = "&"
	BITNOT     = "~"
	BITSHLEFT  = "<<"
	BITSHRIGHT = ">>"
	OR         = "||"
	AND        = "&&"
	BANG       = "!"

	// Delimiters
	COMMA   = ","
	LPAREN  = "("
	RPAREN  = ")"
	NEWLINE = "\\n"
	LBRACK  = "["
	RBRACK  = "]"
	COLON   = ":"
	RANGE   = ".."
	DOT     = "."

	// Keywords
	LET      = "LET"
	FUNCTION = "FUNCTION"
	DO       = "DO"
	END      = "END"
	IF       = "IF"
	ELSE     = "ELSE"
	FOR      = "FOR"
	IN       = "IN"
	NIL      = "NIL"
	RETURN   = "RETURN"
	THEN     = "THEN"
	SWITCH   = "SWITCH"
	CASE     = "CASE"
	DEFAULT  = "DEFAULT"
	BREAK    = "BREAK"
	CONTINUE = "CONTINUE"
	MODULE   = "MODULE"

	// Misc
	COMMENT = "COMMENT"
	EOF     = "EOF"
)
