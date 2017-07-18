package lexer

import (
	"github.com/fadion/aria/reader"
	"github.com/fadion/aria/token"
	"testing"
)

func TestOperators(t *testing.T) {
	input := `let a = 1 + 2 * 3 % 1 / (5 + 2) ** 2 + 1..5
let b = true && false || 0 >= 1 < 5 && !true
let c = 10 & 5 >> 1 | 0 & ~1`
	tests := []struct {
		Type   token.TokenType
		Lexeme string
	}{
		{token.LET, "let"},
		{token.IDENTIFIER, "a"},
		{token.ASSIGN, "="},
		{token.INTEGER, "1"},
		{token.PLUS, "+"},
		{token.INTEGER, "2"},
		{token.ASTERISK, "*"},
		{token.INTEGER, "3"},
		{token.MODULO, "%"},
		{token.INTEGER, "1"},
		{token.SLASH, "/"},
		{token.LPAREN, "("},
		{token.INTEGER, "5"},
		{token.PLUS, "+"},
		{token.INTEGER, "2"},
		{token.RPAREN, ")"},
		{token.POWER, "**"},
		{token.INTEGER, "2"},
		{token.PLUS, "+"},
		{token.INTEGER, "1"},
		{token.RANGE, ".."},
		{token.INTEGER, "5"},
		{token.NEWLINE, "\\n"},
		{token.LET, "let"},
		{token.IDENTIFIER, "b"},
		{token.ASSIGN, "="},
		{token.BOOLEAN, "true"},
		{token.AND, "&&"},
		{token.BOOLEAN, "false"},
		{token.OR, "||"},
		{token.INTEGER, "0"},
		{token.GTE, ">="},
		{token.INTEGER, "1"},
		{token.LT, "<"},
		{token.INTEGER, "5"},
		{token.AND, "&&"},
		{token.BANG, "!"},
		{token.BOOLEAN, "true"},
		{token.NEWLINE, "\\n"},
		{token.LET, "let"},
		{token.IDENTIFIER, "c"},
		{token.ASSIGN, "="},
		{token.INTEGER, "10"},
		{token.BITAND, "&"},
		{token.INTEGER, "5"},
		{token.BITSHRIGHT, ">>"},
		{token.INTEGER, "1"},
		{token.BITOR, "|"},
		{token.INTEGER, "0"},
		{token.BITAND, "&"},
		{token.BITNOT, "~"},
		{token.INTEGER, "1"},
	}

	lex := New(reader.New([]byte(input)))

	for i, v := range tests {
		tok := lex.NextToken()
		if tok.Type != v.Type || tok.Lexeme != v.Lexeme {
			t.Errorf("Expected [%s %s] but got [%s %s] in line %d", string(v.Type), v.Lexeme, string(tok.Type), tok.Lexeme, i)
		}
	}
}

func TestDataTypes(t *testing.T) {
	input := `1 5 true 5.20 3.4789 false "yes"`
	tests := []struct {
		Type   token.TokenType
		Lexeme string
	}{
		{token.INTEGER, "1"},
		{token.INTEGER, "5"},
		{token.BOOLEAN, "true"},
		{token.FLOAT, "5.20"},
		{token.FLOAT, "3.4789"},
		{token.BOOLEAN, "false"},
		{token.STRING, "yes"},
	}

	lex := New(reader.New([]byte(input)))

	for i, v := range tests {
		tok := lex.NextToken()
		if tok.Type != v.Type || tok.Lexeme != v.Lexeme {
			t.Errorf("Expected [%s %s] but got [%s %s] in line %d", string(v.Type), v.Lexeme, string(tok.Type), tok.Lexeme, i)
		}
	}
}

func TestDelimiters(t *testing.T) {
	input := `(1, 2, a) ["yes", 5.1, b] [a: b, c: d] a.b a..b`
	tests := []struct {
		Type   token.TokenType
		Lexeme string
	}{
		{token.LPAREN, "("},
		{token.INTEGER, "1"},
		{token.COMMA, ","},
		{token.INTEGER, "2"},
		{token.COMMA, ","},
		{token.IDENTIFIER, "a"},
		{token.RPAREN, ")"},
		{token.LBRACK, "["},
		{token.STRING, "yes"},
		{token.COMMA, ","},
		{token.FLOAT, "5.1"},
		{token.COMMA, ","},
		{token.IDENTIFIER, "b"},
		{token.RBRACK, "]"},
		{token.LBRACK, "["},
		{token.IDENTIFIER, "a"},
		{token.COLON, ":"},
		{token.IDENTIFIER, "b"},
		{token.COMMA, ","},
		{token.IDENTIFIER, "c"},
		{token.COLON, ":"},
		{token.IDENTIFIER, "d"},
		{token.RBRACK, "]"},
		{token.IDENTIFIER, "a"},
		{token.DOT, "."},
		{token.IDENTIFIER, "b"},
		{token.IDENTIFIER, "a"},
		{token.RANGE, ".."},
		{token.IDENTIFIER, "b"},
	}

	lex := New(reader.New([]byte(input)))

	for i, v := range tests {
		tok := lex.NextToken()
		if tok.Type != v.Type || tok.Lexeme != v.Lexeme {
			t.Errorf("Expected [%s %s] but got [%s %s] in line %d", string(v.Type), v.Lexeme, string(tok.Type), tok.Lexeme, i)
		}
	}
}

func TestKeywords(t *testing.T) {
	input := `let var fn function do end not if else right for in left then return middle switch not case module yes`
	tests := []struct {
		Type   token.TokenType
		Lexeme string
	}{
		{token.LET, "let"},
		{token.IDENTIFIER, "var"},
		{token.FUNCTION, "fn"},
		{token.IDENTIFIER, "function"},
		{token.DO, "do"},
		{token.END, "end"},
		{token.IDENTIFIER, "not"},
		{token.IF, "if"},
		{token.ELSE, "else"},
		{token.IDENTIFIER, "right"},
		{token.FOR, "for"},
		{token.IN, "in"},
		{token.IDENTIFIER, "left"},
		{token.THEN, "then"},
		{token.RETURN, "return"},
		{token.IDENTIFIER, "middle"},
		{token.SWITCH, "switch"},
		{token.IDENTIFIER, "not"},
		{token.CASE, "case"},
		{token.MODULE, "module"},
		{token.IDENTIFIER, "yes"},
	}

	lex := New(reader.New([]byte(input)))

	for i, v := range tests {
		tok := lex.NextToken()
		if tok.Type != v.Type || tok.Lexeme != v.Lexeme {
			t.Errorf("Expected [%s %s] but got [%s %s] in line %d", string(v.Type), v.Lexeme, string(tok.Type), tok.Lexeme, i)
		}
	}
}

func TestMiniProgram(t *testing.T) {
	input := `let a = 10
let b = 20.2
if b > a then
  for i in 5..10
    i + 2
  end
else
  "exiting..."
end
let c = fn x, y, z
  "hi" + x + y + z
end`
	tests := []struct {
		Type   token.TokenType
		Lexeme string
	}{
		{token.LET, "let"},
		{token.IDENTIFIER, "a"},
		{token.ASSIGN, "="},
		{token.INTEGER, "10"},
		{token.NEWLINE, "\\n"},
		{token.LET, "let"},
		{token.IDENTIFIER, "b"},
		{token.ASSIGN, "="},
		{token.FLOAT, "20.2"},
		{token.NEWLINE, "\\n"},
		{token.IF, "if"},
		{token.IDENTIFIER, "b"},
		{token.GT, ">"},
		{token.IDENTIFIER, "a"},
		{token.THEN, "then"},
		{token.NEWLINE, "\\n"},
		{token.FOR, "for"},
		{token.IDENTIFIER, "i"},
		{token.IN, "in"},
		{token.INTEGER, "5"},
		{token.RANGE, ".."},
		{token.INTEGER, "10"},
		{token.NEWLINE, "\\n"},
		{token.IDENTIFIER, "i"},
		{token.PLUS, "+"},
		{token.INTEGER, "2"},
		{token.NEWLINE, "\\n"},
		{token.END, "end"},
		{token.NEWLINE, "\\n"},
		{token.ELSE, "else"},
		{token.NEWLINE, "\\n"},
		{token.STRING, "exiting..."},
		{token.NEWLINE, "\\n"},
		{token.END, "end"},
		{token.NEWLINE, "\\n"},
		{token.LET, "let"},
		{token.IDENTIFIER, "c"},
		{token.ASSIGN, "="},
		{token.FUNCTION, "fn"},
		{token.IDENTIFIER, "x"},
		{token.COMMA, ","},
		{token.IDENTIFIER, "y"},
		{token.COMMA, ","},
		{token.IDENTIFIER, "z"},
		{token.NEWLINE, "\\n"},
		{token.STRING, "hi"},
		{token.PLUS, "+"},
		{token.IDENTIFIER, "x"},
		{token.PLUS, "+"},
		{token.IDENTIFIER, "y"},
		{token.PLUS, "+"},
		{token.IDENTIFIER, "z"},
		{token.NEWLINE, "\\n"},
		{token.END, "end"},
	}

	lex := New(reader.New([]byte(input)))

	for i, v := range tests {
		tok := lex.NextToken()
		if tok.Type != v.Type || tok.Lexeme != v.Lexeme {
			t.Errorf("Expected [%s %s] but got [%s %s] in line %d", string(v.Type), v.Lexeme, string(tok.Type), tok.Lexeme, i)
		}
	}
}
