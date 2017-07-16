package lexer

import "github.com/fadion/aria/token"

// Symbol.
type Symbol struct {}

// Store of symbols.
var table = make(map[string]token.TokenType)

// Insert a new symbol.
func (s *Symbol) Insert(name string, t token.TokenType) {
	table[name] = t
}

// Get a symbol by name.
func (s *Symbol) Lookup(name string) (token.TokenType, bool) {
	if tok, ok := table[name]; ok {
		return tok, true
	}

	return "", false
}
