package lexer

import (
	"github.com/fadion/aria/token"
	"testing"
)

func TestSymbolInsert(t *testing.T) {
	table = make(map[string]token.TokenType)
	symbol := &Symbol{}
	symbol.Insert("let", token.LET)
	symbol.Insert("for", token.FOR)
	expected := 2

	if len(table) != 2 {
		t.Errorf("Expected %d but got %d", expected, len(table))
	}
}

func TestSymbolLookup(t *testing.T) {
	table = make(map[string]token.TokenType)
	symbol := &Symbol{}
	symbol.Insert("let", token.LET)
	symbol.Insert("for", token.FOR)

	tok, found := symbol.Lookup("for")

	if !found {
		t.Errorf("Expected to find a symbol but didn't.")
	}

	if tok != token.FOR {
		t.Errorf("Expected %s but got %s.", token.FOR, tok)
	}
}
