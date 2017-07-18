package lexer

import "github.com/fadion/aria/token"

// Symbol represents the symbol table.
type Symbol struct{}

// Store of symbols.
var table = make(map[string]token.TokenType)

// Insert adds a new symbol to the store.
func (s *Symbol) Insert(name string, t token.TokenType) {
	table[name] = t
}

// Lookup returns a symbol by name.
func (s *Symbol) Lookup(name string) (token.TokenType, bool) {
	if tok, ok := table[name]; ok {
		return tok, true
	}

	return "", false
}
