package parser

import "github.com/fadion/aria/token"

// Precedence.
const (
	_          int = iota
	LOWEST
	ASSIGN
	PIPE
	ARROW
	TERNARY
	BOOLEAN
	BITWISE
	COMPARISON
	RANGE
	BITSHIFT
	SUM
	PRODUCT
	POWER
	PREFIX
	CALL
	INDEX
)

// List of tokens and their respective precedence.
var precedences = map[token.TokenType]int{
	token.ASSIGN:     ASSIGN,
	token.ASSIGNPLUS: ASSIGN,
	token.ASSIGNMIN:  ASSIGN,
	token.ASSIGNMULT: ASSIGN,
	token.ASSIGNDIV:  ASSIGN,

	token.PLUS:  SUM,
	token.MINUS: SUM,

	token.ASTERISK: PRODUCT,
	token.SLASH:    PRODUCT,
	token.MODULO:   PRODUCT,

	token.POWER: POWER,

	token.EQ:   COMPARISON,
	token.UNEQ: COMPARISON,
	token.LT:   COMPARISON,
	token.LTE:  COMPARISON,
	token.GTE:  COMPARISON,
	token.GT:   COMPARISON,

	token.OR:  BOOLEAN,
	token.AND: BOOLEAN,

	token.DOT:    CALL,
	token.LPAREN: CALL,

	token.LBRACK: INDEX,

	token.BITOR:      BITWISE,
	token.BITAND:     BITWISE,
	token.BITNOT:     BITWISE,
	token.BITSHLEFT:  BITSHIFT,
	token.BITSHRIGHT: BITSHIFT,

	token.RANGE:    RANGE,
	token.PIPE:     PIPE,
	token.ARROW:    ARROW,
	token.QUESTION: TERNARY,
	token.IS:       ASSIGN,
}
