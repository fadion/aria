package parser

import "github.com/fadion/aria/token"

const (
	_           int = iota
	LOWEST
	BOOLEAN
	BITWISE
	ASSIGNEMENT
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
	token.PLUS:       SUM,
	token.MINUS:      SUM,
	token.ASTERISK:   PRODUCT,
	token.SLASH:      PRODUCT,
	token.MODULO:     PRODUCT,
	token.POWER:      POWER,
	token.EQ:         ASSIGNEMENT,
	token.UNEQ:       ASSIGNEMENT,
	token.LT:         COMPARISON,
	token.LTE:        COMPARISON,
	token.GTE:        COMPARISON,
	token.GT:         COMPARISON,
	token.OR:         BOOLEAN,
	token.AND:        BOOLEAN,
	token.DOT:        CALL,
	token.LPAREN:     CALL,
	token.LBRACK:     INDEX,
	token.RANGE:      RANGE,
	token.BITOR:      BITWISE,
	token.BITAND:     BITWISE,
	token.BITNOT:     BITWISE,
	token.BITSHLEFT:  BITSHIFT,
	token.BITSHRIGHT: BITSHIFT,
}
