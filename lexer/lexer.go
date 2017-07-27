package lexer

import (
	"bytes"
	"fmt"
	"github.com/fadion/aria/reader"
	"github.com/fadion/aria/reporter"
	"github.com/fadion/aria/token"
)

// Lexer represents the lexer.
type Lexer struct {
	reader   *reader.Reader
	char     rune
	row      int
	col      int
	token    token.Token
	rewinded bool
	symbol   *Symbol
}

// New initializes a Lexer.
func New(reader *reader.Reader) *Lexer {
	l := &Lexer{
		reader:   reader,
		row:      1,
		col:      1,
		rewinded: false,
		symbol:   &Symbol{},
	}

	// List of valid keywords.
	l.symbol.Insert("true", token.BOOLEAN)
	l.symbol.Insert("false", token.BOOLEAN)
	l.symbol.Insert("nil", token.NIL)
	l.symbol.Insert("let", token.LET)
	l.symbol.Insert("var", token.VAR)
	l.symbol.Insert("fn", token.FUNCTION)
	l.symbol.Insert("do", token.DO)
	l.symbol.Insert("end", token.END)
	l.symbol.Insert("if", token.IF)
	l.symbol.Insert("else", token.ELSE)
	l.symbol.Insert("for", token.FOR)
	l.symbol.Insert("in", token.IN)
	l.symbol.Insert("return", token.RETURN)
	l.symbol.Insert("then", token.THEN)
	l.symbol.Insert("switch", token.SWITCH)
	l.symbol.Insert("case", token.CASE)
	l.symbol.Insert("default", token.DEFAULT)
	l.symbol.Insert("break", token.BREAK)
	l.symbol.Insert("continue", token.CONTINUE)
	l.symbol.Insert("module", token.MODULE)
	l.symbol.Insert("import", token.IMPORT)

	// Move to the first token.
	l.advance()

	return l
}

// NextToken returns the next token.
func (l *Lexer) NextToken() token.Token {
	// Ignore any number of sequential whitespace.
	l.consumeWhitespace()

	switch {
	case l.char == 0:
		l.assignToken(token.EOF, "")
	case l.char == '=':
		switch l.peek() {
		case '=': // ==
			l.advance()
			l.assignToken(token.EQ, "==")
		case '>': // =>
			l.advance()
			l.assignToken(token.FATARROW, "=>")
		default: // =
			l.assignToken(token.ASSIGN, string(l.char))
		}
	case l.char == '>':
		switch l.peek() {
		case '=': // >=
			l.advance()
			l.assignToken(token.GTE, ">=")
		case '>': // >>
			l.advance()
			l.assignToken(token.BITSHRIGHT, ">>")
		default: // >
			l.assignToken(token.GT, string(l.char))
		}
	case l.char == '<':
		switch l.peek() {
		case '=': // <=
			l.advance()
			l.assignToken(token.LTE, "<=")
		case '<': // <<
			l.advance()
			l.assignToken(token.BITSHLEFT, "<<")
		default: // <
			l.assignToken(token.LT, string(l.char))
		}
	case l.char == '+':
		l.assignToken(token.PLUS, string(l.char))
	case l.char == '-':
		switch l.peek() {
		case '>': // ->
			l.advance()
			l.assignToken(token.ARROW, string("->"))
		default:
			l.assignToken(token.MINUS, string(l.char))
		}
	case l.char == '*':
		switch l.peek() {
		case '*': // **
			l.advance()
			l.assignToken(token.POWER, "**")
		default: // *
			l.assignToken(token.ASTERISK, string(l.char))
		}
	case l.char == '/':
		switch l.peek() {
		case '/': // single line comment
			l.advance()
			l.consumeComment()
		case '*': // multiline comment
			l.advance()
			l.consumeMultilineComment()
		default:
			l.assignToken(token.SLASH, string(l.char))
		}
	case l.char == '%':
		l.assignToken(token.MODULO, string(l.char))
	case l.char == ',':
		l.assignToken(token.COMMA, string(l.char))
	case l.char == '.':
		switch l.peek() {
		case '.':
			l.advance()
			switch l.peek() {
			case '.': // ...
				l.advance()
				l.assignToken(token.ELLIPSIS, "...")
			default: // ..
				l.assignToken(token.RANGE, "..")
			}
		default: // .
			l.assignToken(token.DOT, string(l.char))
		}
	case l.char == '|':
		switch l.peek() {
		case '|': // ||
			l.advance()
			l.assignToken(token.OR, "||")
		case '>': // |>
			l.advance()
			l.assignToken(token.PIPE, "|>")
		default: // |
			l.assignToken(token.BITOR, string(l.char))
		}
	case l.char == '&':
		switch l.peek() {
		case '&': // &&
			l.advance()
			l.assignToken(token.AND, "&&")
		default: // &
			l.assignToken(token.BITAND, string(l.char))
		}
	case l.char == '~':
		l.assignToken(token.BITNOT, string(l.char))
	case l.char == '!':
		switch l.peek() {
		case '=': // !=
			l.advance()
			l.assignToken(token.UNEQ, "!=")
		default: // !
			l.assignToken(token.BANG, string(l.char))
		}
	case l.char == '(':
		l.assignToken(token.LPAREN, "(")
	case l.char == ')':
		l.assignToken(token.RPAREN, ")")
	case l.char == '[':
		l.assignToken(token.LBRACK, "[")
	case l.char == ']':
		l.assignToken(token.RBRACK, "]")
	case l.char == '?':
		l.assignToken(token.QUESTION, "?")
	case l.char == ':':
		l.assignToken(token.COLON, ":")
	case l.char == '_':
		l.assignToken(token.UNDERSCORE, "_")
	case l.char == '\n':
		l.assignToken(token.NEWLINE, "\\n")
	case l.char == '"': // Anything inside double quotes is a string.
		l.consumeString()
	case l.char == '0' && l.peek() == 'x': // Hex.
		l.consumeSpecialInteger(l.isHex)
	case l.char == '0' && l.peek() == 'o': // Octal.
		l.consumeSpecialInteger(l.isOctal)
	case l.char == '0' && l.peek() == 'b': // Binary.
		l.consumeSpecialInteger(l.isBinary)
	case l.isNumber(l.char): // Numeric literal.
		l.consumeNumeric()
	default:
		// Identifier or keyword.
		if l.isName(l.char) {
			l.consumeIdent()
		} else {
			l.reportError(fmt.Sprintf("Unidentified character '%s'", string(l.char)))
		}
	}

	l.advance()

	return l.token
}

// Move the cursor ahead.
func (l *Lexer) advance() {
	rn, err := l.reader.Advance()
	if err != nil {
		l.reportError(fmt.Sprintf("Invalid '%s' character in source file", string(rn)))
	}

	// Don't move the location if it was a
	// rewind, or it will report an incorrect
	// line and column.
	if !l.rewinded {
		l.moveLocation()
	}
	l.rewinded = false
	l.char = rn
}

// Check characters ahead but don't move the cursor.
func (l *Lexer) peek() rune {
	rn, err := l.reader.Peek()
	if err != nil {
		l.reportError(fmt.Sprintf("Invalid '%s' character in source file", string(rn)))
	}

	return rn
}

// Move the cursor to the previous character.
func (l *Lexer) rewind() {
	if err := l.reader.Unread(); err != nil {
		l.reportError("Unable to move to the previous character. This is an internal Lexer fail.")
	}

	l.rewinded = true
}

// Move row and column cursor.
func (l *Lexer) moveLocation() {
	switch l.char {
	case '\n':
		l.row += 1
		l.col = 2
	default:
		l.col += 1
	}
}

// Pass a token to the active token cursor.
func (l *Lexer) assignToken(toktype token.TokenType, value string) {
	l.token = token.Token{
		Type:     toktype,
		Lexeme:   value,
		Location: token.Location{Row: l.row, Col: l.col},
	}
}

// Check if the character makes for a valid identifier
// or keyword.
func (l *Lexer) isName(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') || char == '_' || char == '!' || char == '?'
}

// Check if the character is a number.
func (l *Lexer) isNumber(char rune) bool {
	return char >= '0' && char <= '9'
}

// Check if the character is an hexadecimal.
func (l *Lexer) isHex(char rune) bool {
	return l.isNumber(char) || (char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F')
}

// Check if the character is an octal.
func (l *Lexer) isOctal(char rune) bool {
	return char >= '0' && char <= '7'
}

// Check if the character is a binary.
func (l *Lexer) isBinary(char rune) bool {
	return char == '0' || char == '1'
}

// Read all valid characters from an identifier or keyword.
func (l *Lexer) readName() string {
	var out bytes.Buffer
	out.WriteRune(l.char)

	// Read until a non-name character is found.
	for l.isName(l.peek()) {
		l.advance()
		out.WriteRune(l.char)
	}

	return out.String()
}

// Move the cursor until it exhausts whitespace.
func (l *Lexer) consumeWhitespace() {
	for l.char == ' ' || l.char == '\t' || l.char == '\r' {
		l.advance()
	}
}

// Read a string literal.
func (l *Lexer) consumeString() {
	var out bytes.Buffer

	// Move past the opening double quote.
	l.advance()

loop:
	for {
		switch l.char {
		case '\\': // escape characters
			l.advance()
			switch l.char {
			case '"': // \"
				out.WriteRune('\\')
				out.WriteRune('"')
			case '\\': // \\
				out.WriteRune('\\')
			case 'n', 't', 'r', 'a', 'b', 'f', 'v':
				out.WriteRune('\\')
				out.WriteRune(l.char)
			default:
				l.reportError(fmt.Sprintf("Invalid escape character '%s", string(l.char)))
			}
		case 0:
			// String should be closed before the end of file.
			l.reportError("Unterminated string")
			break loop
		case '"': // Closing quote.
			break loop
		default:
			out.WriteRune(l.char)
		}

		l.advance()
	}

	l.assignToken(token.STRING, out.String())
}

// Read a numeric literal.
func (l *Lexer) consumeNumeric() {
	var out bytes.Buffer
	// Write the first character, as we're sure
	// it's numeric.
	out.WriteRune(l.char)
	floatFound := false
	scientificFound := false

loop:
	for {
		l.advance()

		switch {
		case l.isNumber(l.char):
			out.WriteRune(l.char)
		case l.char == '_': // Thousands separator is ignored.
		case l.char == '.' && l.isNumber(l.peek()): // Float.
			floatFound = true
			out.WriteRune('.')
		case l.char == 'e' && (l.isNumber(l.peek()) || l.peek() == '-'): // Scientific notation.
			// Numbers in scientific notation are
			// treated as floats for easy of use.
			floatFound = true
			scientificFound = true
			out.WriteRune('e')
		case l.char == '-' && scientificFound: // Negative scientific notation.
			out.WriteRune('-')
		case l.char == '.' && l.peek() == '.': // Range operator.
			l.rewind()
			break loop
		case l.char == 0: // Don't rewind on EOF.
			break loop
		default:
			l.rewind()
			break loop
		}
	}

	if floatFound {
		l.assignToken(token.FLOAT, out.String())
	} else {
		l.assignToken(token.INTEGER, out.String())
	}
}

// Read a binary, octal or hexadecimal literal.
func (l *Lexer) consumeSpecialInteger(fn func(rune) bool) {
	var out bytes.Buffer

	out.WriteRune(l.char)
	out.WriteRune(l.peek())
	// Move past the 'x', 'b' or 'o'.
	l.advance()

	for fn(l.peek()) {
		out.WriteRune(l.peek())

		l.advance()
	}

	ret := out.String()
	// A starter like '0x' without other characters
	// is not enough to make up an Integer.
	if len(ret) == 2 {
		l.reportError(fmt.Sprintf("Literal sequence '%s' started but not continued", ret))
	}

	l.assignToken(token.INTEGER, ret)
}

// Read a single line comment.
func (l *Lexer) consumeComment() {
	var out bytes.Buffer

	l.advance()

loop:
	for {
		switch l.char {
		case '\n', 0: // Comment ends on a line break or EOF
			break loop
		case '\r': // Or possibly on a \r\n
			l.advance()
			switch l.char {
			case '\n', 0:
				break loop
			default:
				l.reportError("Unexpected comment line ending")
				break loop
			}
		default:
			out.WriteRune(l.char)
		}

		l.advance()
	}

	l.assignToken(token.COMMENT, out.String())
}

// Read multiline comment.
func (l *Lexer) consumeMultilineComment() {
	var out bytes.Buffer

loop:
	for {
		l.advance()
		switch l.char {
		case '*':
			switch l.peek() {
			case '/': // Multiline comments end with */
				l.advance()
				break loop
			}
		case 0: // EOF and yet not comment terminator.
			l.reportError("Unterminated multiline comment")
			break loop
		default:
			out.WriteRune(l.char)
		}
	}

	l.assignToken(token.COMMENT, out.String())
}

// Read an identifier or keyword.
func (l *Lexer) consumeIdent() {
	ident := l.readName()

	// Check the symbol table for a known keyword.
	// Otherwise call it an Identifier.
	if toktype, found := l.symbol.Lookup(ident); found {
		l.assignToken(toktype, ident)
	} else {
		l.assignToken(token.IDENTIFIER, ident)
	}
}

// Report an error in the current location.
func (l *Lexer) reportError(message string) {
	reporter.Error(reporter.PARSE, token.Location{l.row, l.col}, message)
}
