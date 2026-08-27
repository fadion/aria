// Package scanner turns Aria source text into a stream of tokens.
//
// The scanner reads an in-memory byte slice through a single integer cursor.
// There is no buffer, no unread, and no way to move backwards: every path
// through Scan advances the cursor by at least one byte before returning, so
// the token stream is guaranteed to terminate on any input. That property is
// structural rather than tested-in — the previous lexer could rewind, and a
// peek that corrupted the cursor made it re-scan one token forever on
// malformed UTF-8.
package scanner

import (
	"unicode"
	"unicode/utf8"

	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/token"
)

// eof is the sentinel returned past the end of input. It is not a valid rune,
// so it can never collide with real source text.
const eof rune = -1

// Mode controls optional scanner behavior.
type Mode uint

// ScanComments makes Scan return comment tokens instead of skipping them.
// Off by default: the parser does not want them, but a formatter would.
const ScanComments Mode = 1 << iota

// A Scanner produces tokens from a source file.
type Scanner struct {
	file  *source.File
	src   []byte
	diags *diag.Bag
	mode  Mode

	ch   rune       // rune at off, or eof
	off  source.Pos // offset of ch
	next source.Pos // offset just past ch

	// interp counts the string interpolations currently open. Aria has no other
	// use for braces, so a `}` while one is open always closes it and resumes
	// the string — no depth counting inside the hole is needed. It nests: an
	// interpolation may hold a string that interpolates in turn.
	interp int
}

// New returns a Scanner over file, reporting problems into diags.
func New(file *source.File, diags *diag.Bag, mode Mode) *Scanner {
	s := &Scanner{file: file, src: file.Bytes(), diags: diags, mode: mode}
	s.advance()
	return s
}

// advance moves to the next rune. It always makes progress: an invalid UTF-8
// byte is consumed as one byte rather than being retried.
func (s *Scanner) advance() {
	if s.next >= s.file.Size() {
		s.off = s.file.Size()
		s.ch = eof
		return
	}

	s.off = s.next
	if c := s.src[s.next]; c < utf8.RuneSelf {
		s.ch, s.next = rune(c), s.next+1
		return
	}

	r, width := utf8.DecodeRune(s.src[s.next:])
	if r == utf8.RuneError && width == 1 {
		s.diags.Errorf(source.Span{Start: s.next, End: s.next + 1},
			"invalid UTF-8 byte 0x%02x in source", s.src[s.next])
	}
	s.ch, s.next = r, s.next+source.Pos(width)
}

// peek returns the rune after the current one without touching any state.
// Being pure is the whole point: a peek that recorded a read is what let the
// old lexer rewind past input it had already consumed.
func (s *Scanner) peek() rune {
	if s.next >= s.file.Size() {
		return eof
	}
	if c := s.src[s.next]; c < utf8.RuneSelf {
		return rune(c)
	}
	r, _ := utf8.DecodeRune(s.src[s.next:])
	return r
}

// peek2 returns the rune two positions ahead, for the few places that need to
// tell `..` from `...`.
func (s *Scanner) peek2() rune {
	if s.next >= s.file.Size() {
		return eof
	}
	_, width := utf8.DecodeRune(s.src[s.next:])
	after := s.next + source.Pos(width)
	if after >= s.file.Size() {
		return eof
	}
	r, _ := utf8.DecodeRune(s.src[after:])
	return r
}

// Scan returns the next token. At end of input it returns token.EOF
// indefinitely, so callers may loop until they see it without risk of
// running off the end.
func (s *Scanner) Scan() token.Token {
	for {
		s.skipSpace()
		start := s.off

		tok, ok := s.scanOne(start)
		if ok {
			return tok
		}
		// A comment was scanned in a mode that discards it. Loop rather than
		// recurse so a run of comments cannot grow the stack.
	}
}

// scanOne scans a single token. It reports ok=false only for a comment being
// discarded, which is the one case with nothing to return.
func (s *Scanner) scanOne(start source.Pos) (token.Token, bool) {
	switch ch := s.ch; {
	case ch == eof:
		return s.token(token.EOF, start), true

	case ch == '\n':
		s.advance()
		return s.token(token.Newline, start), true

	case isNameStart(ch):
		return s.scanIdent(start), true

	case isDigit(ch):
		return s.scanNumber(start), true

	case ch == '"':
		return s.scanString(start), true

	case ch == '`':
		return s.scanRawString(start), true

	case ch == '}' && s.interp > 0:
		// The end of an interpolation hole. Scanning resumes inside the string
		// that opened it.
		s.interp--
		s.advance()
		return s.scanStringFrom(start, true), true
	}

	return s.scanOperator(start)
}

// scanOperator handles punctuation and comments.
func (s *Scanner) scanOperator(start source.Pos) (token.Token, bool) {
	ch := s.ch
	s.advance()

	switch ch {
	case '+':
		return s.token(s.choose('=', token.AssignPlus, token.Plus), start), true
	case '-':
		switch s.ch {
		case '>':
			s.advance()
			return s.token(token.Arrow, start), true
		case '=':
			s.advance()
			return s.token(token.AssignMinus, start), true
		}
		return s.token(token.Minus, start), true
	case '*':
		switch s.ch {
		case '*':
			s.advance()
			return s.token(s.choose('=', token.AssignPow, token.Power), start), true
		case '=':
			s.advance()
			return s.token(token.AssignMul, start), true
		}
		return s.token(token.Star, start), true
	case '/':
		switch s.ch {
		case '/':
			return s.scanLineComment(start)
		case '*':
			return s.scanBlockComment(start)
		case '=':
			s.advance()
			return s.token(token.AssignDiv, start), true
		}
		return s.token(token.Slash, start), true
	case '%':
		return s.token(s.choose('=', token.AssignMod, token.Percent), start), true
	case '=':
		switch s.ch {
		case '=':
			s.advance()
			return s.token(token.Eq, start), true
		case '>':
			s.advance()
			return s.token(token.FatArrow, start), true
		}
		return s.token(token.Assign, start), true
	case '!':
		return s.token(s.choose('=', token.NotEq, token.Bang), start), true
	case '<':
		switch s.ch {
		case '=':
			s.advance()
			return s.token(token.LtEq, start), true
		case '<':
			s.advance()
			return s.token(token.ShiftLeft, start), true
		}
		return s.token(token.Lt, start), true
	case '>':
		switch s.ch {
		case '=':
			s.advance()
			return s.token(token.GtEq, start), true
		case '>':
			s.advance()
			return s.token(token.ShiftRight, start), true
		}
		return s.token(token.Gt, start), true
	case '|':
		switch s.ch {
		case '|':
			s.advance()
			return s.token(token.Or, start), true
		case '>':
			s.advance()
			return s.token(token.Pipe, start), true
		}
		return s.token(token.BitOr, start), true
	case '&':
		return s.token(s.choose('&', token.And, token.BitAnd), start), true
	case '^':
		return s.token(token.BitXor, start), true
	case '~':
		return s.token(token.BitNot, start), true
	case '?':
		switch s.ch {
		case '?':
			s.advance()
			return s.token(token.Coalesce, start), true
		case '.':
			s.advance()
			return s.token(token.SafeDot, start), true
		}
		return s.token(token.Question, start), true
	case ',':
		return s.token(token.Comma, start), true
	case '(':
		return s.token(token.LParen, start), true
	case ')':
		return s.token(token.RParen, start), true
	case '[':
		return s.token(token.LBracket, start), true
	case ']':
		return s.token(token.RBracket, start), true
	case ':':
		return s.token(token.Colon, start), true
	case '.':
		if s.ch == '.' {
			s.advance()
			if s.ch == '.' {
				s.advance()
				return s.token(token.Ellipsis, start), true
			}
			return s.token(token.Range, start), true
		}
		return s.token(token.Dot, start), true
	}

	// utf8.RuneError from an invalid byte was already reported by advance;
	// reporting again here would give two messages for one mistake.
	if ch != utf8.RuneError {
		s.diags.Errorf(source.Span{Start: start, End: s.off},
			"unexpected character %q", ch)
	}
	return s.token(token.Invalid, start), true
}

// choose consumes want if it is next, returning yes, and otherwise returns no.
func (s *Scanner) choose(want rune, yes, no token.Kind) token.Kind {
	if s.ch == want {
		s.advance()
		return yes
	}
	return no
}

func (s *Scanner) token(kind token.Kind, start source.Pos) token.Token {
	return token.Token{Kind: kind, Span: source.Span{Start: start, End: s.off}}
}

// skipSpace consumes spaces, tabs and carriage returns. Newlines are
// significant in Aria and are returned as tokens.
func (s *Scanner) skipSpace() {
	for s.ch == ' ' || s.ch == '\t' || s.ch == '\r' {
		s.advance()
	}
}

// scanIdent reads an identifier or keyword. Aria allows `?` and `!` to end a
// name, as in `empty?`.
func (s *Scanner) scanIdent(start source.Pos) token.Token {
	for isNamePart(s.ch) {
		s.advance()
	}
	// A trailing ? or ! belongs to the name, but only one and only at the end,
	// so `a?b` is still one identifier while `a ? b : c` stays a ternary.
	//
	// `?.` is the exception: a `?` followed by a dot is safe navigation, not the
	// end of a name, so `cfg?.db` reads as cfg and `?.db` rather than as a name
	// called `cfg?`. That makes `empty?.x` unwritable as a member access on the
	// result of `empty?` — parenthesise it — which is the side of the ambiguity
	// worth losing, since the other side is every safe navigation there is.
	if (s.ch == '?' && s.peek() != '.') || s.ch == '!' {
		s.advance()
	}

	text := string(s.src[start:s.off])

	// A lone underscore is the placeholder, used as a switch wildcard and as
	// an append target. Anything longer is an ordinary name: scanning the
	// whole word first is what lets `_foo` be one identifier, where the old
	// lexer matched `_` before trying names and split it into two tokens.
	if text == "_" {
		return s.token(token.Underscore, start)
	}

	return s.token(token.Lookup(text), start)
}

// scanNumber reads an integer or float literal.
func (s *Scanner) scanNumber(start source.Pos) token.Token {
	// Base-prefixed integers: 0x1f, 0o27, 0b1010.
	if s.ch == '0' {
		switch lower(s.peek()) {
		case 'x':
			return s.scanRadix(start, isHexDigit, "hexadecimal")
		case 'o':
			return s.scanRadix(start, isOctalDigit, "octal")
		case 'b':
			return s.scanRadix(start, isBinaryDigit, "binary")
		}
	}

	s.scanDigits(isDigit)
	kind := token.Int

	// A '.' begins a fraction only when a digit follows. That keeps `1..5`
	// a range rather than a malformed float, without needing to back up.
	if s.ch == '.' && isDigit(s.peek()) {
		kind = token.Float
		s.advance()
		s.scanDigits(isDigit)
	}

	// Scientific notation is always a float, as in the old lexer.
	if lower(s.ch) == 'e' {
		if next := s.peek(); isDigit(next) || ((next == '-' || next == '+') && isDigit(s.peek2())) {
			kind = token.Float
			s.advance()
			if s.ch == '-' || s.ch == '+' {
				s.advance()
			}
			s.scanDigits(isDigit)
		}
	}

	return s.token(kind, start)
}

// scanRadix reads a 0x / 0o / 0b literal, whose prefix has not been consumed.
func (s *Scanner) scanRadix(start source.Pos, valid func(rune) bool, name string) token.Token {
	s.advance() // '0'
	s.advance() // the base letter

	digits := s.scanDigits(valid)
	if digits == 0 {
		s.diags.Errorf(source.Span{Start: start, End: s.off},
			"%s literal %q has no digits", name, string(s.src[start:s.off]))
		return s.token(token.Invalid, start)
	}
	return s.token(token.Int, start)
}

// scanDigits consumes digits accepted by valid, allowing `_` as a separator,
// and returns how many digits it saw.
func (s *Scanner) scanDigits(valid func(rune) bool) int {
	n := 0
	for valid(s.ch) || s.ch == '_' {
		if s.ch != '_' {
			n++
		}
		s.advance()
	}
	return n
}

// scanString reads a double-quoted string. Escape sequences are validated here
// but left in the span; decoding to a value is the parser's job, since only it
// knows whether it needs the text.
func (s *Scanner) scanString(start source.Pos) token.Token {
	s.advance() // opening quote
	return s.scanStringFrom(start, false)
}

// scanStringFrom reads a string literal, or one piece of an interpolated one.
//
// It stops at the closing quote or at the next `#{`, whichever comes first, so
// a literal with holes arrives as StringStart, the hole's own tokens, then
// StringPart... and StringEnd. `resuming` says the cursor started just past a
// `}` rather than just past the opening quote, which is the only thing that
// distinguishes the two pairs of kinds.
func (s *Scanner) scanStringFrom(start source.Pos, resuming bool) token.Token {
	closing, opening := token.String, token.StringStart
	if resuming {
		closing, opening = token.StringEnd, token.StringPart
	}

	for {
		switch s.ch {
		case eof, '\n':
			// A string may not span lines. Reporting at the opening quote
			// points at where the reader has to go to fix it.
			s.diags.Errorf(source.Span{Start: start, End: s.off}, "unterminated string")
			return s.token(token.Invalid, start)

		case '"':
			s.advance()
			return s.token(closing, start)

		case '#':
			if s.peek() != '{' {
				s.advance()
				continue
			}
			s.advance() // #
			s.advance() // {
			s.interp++
			return s.token(opening, start)

		case '\\':
			escStart := s.off
			s.advance()
			switch s.ch {
			case 'n', 't', 'r', 'a', 'b', 'f', 'v', '\\', '"', '0', '#':
				s.advance()
			case 'x':
				s.advance()
				s.scanHexEscape(escStart, 2, 2)
			case 'u':
				s.advance()
				if s.ch == '{' {
					s.advance()
					s.scanHexEscape(escStart, 1, 6)
					if s.ch != '}' {
						s.diags.Errorf(source.Span{Start: escStart, End: s.off},
							"unterminated \\u{...} escape")
					} else {
						s.advance()
					}
				} else {
					s.scanHexEscape(escStart, 4, 4)
				}
			case eof, '\n':
				// Let the next loop turn report the unterminated string.
			default:
				s.diags.Errorf(source.Span{Start: escStart, End: s.next},
					"unknown escape sequence \\%c", s.ch)
				s.advance()
			}

		default:
			s.advance()
		}
	}
}

// scanHexEscape consumes between least and most hex digits and checks that what
// they spell is a codepoint. It reports rather than returning a value: the
// parser decodes the escape, and it can only be reached for input the scanner
// already accepted.
//
// The value is a rune, not a byte, even for \xNN. Aria strings are UTF-8 and
// index by rune, so writing a raw byte above 0x7F would be a way to build a
// string the rest of the language cannot read.
func (s *Scanner) scanHexEscape(escStart source.Pos, least, most int) {
	var v rune
	n := 0
	for n < most && isHexDigit(s.ch) {
		v = v*16 + rune(hexValue(s.ch))
		n++
		s.advance()
	}

	switch {
	case n < least:
		s.diags.Errorf(source.Span{Start: escStart, End: s.off},
			"escape needs at least %d hex digits, found %d", least, n)
	case v > unicode.MaxRune || (v >= 0xD800 && v <= 0xDFFF):
		s.diags.Errorf(source.Span{Start: escStart, End: s.off},
			"0x%X is not a codepoint", v)
	}
}

func hexValue(ch rune) int {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0')
	case ch >= 'a' && ch <= 'f':
		return int(ch-'a') + 10
	default:
		return int(ch-'A') + 10
	}
}

// scanRawString reads a `...` literal. It spans lines and processes no escapes,
// which is one form covering both gaps: there was no way to write a string over
// more than one line, and every backslash in a regex passed to String.match?
// had to be doubled.
func (s *Scanner) scanRawString(start source.Pos) token.Token {
	s.advance() // opening backtick

	for {
		switch s.ch {
		case eof:
			// A raw string spans lines, so running out of input with one open
			// means more input could close it.
			s.diags.MarkIncomplete()
			s.diags.Errorf(source.Span{Start: start, End: s.off}, "unterminated raw string")
			return s.token(token.Invalid, start)
		case '`':
			s.advance()
			return s.token(token.String, start)
		default:
			s.advance()
		}
	}
}

// scanLineComment reads to end of line. The leading "//" has had its first
// slash consumed.
func (s *Scanner) scanLineComment(start source.Pos) (token.Token, bool) {
	s.advance() // second '/'
	for s.ch != '\n' && s.ch != eof {
		s.advance()
	}
	return s.comment(start)
}

// scanBlockComment reads a /* ... */ comment. Nesting is not supported, which
// matches the old lexer.
func (s *Scanner) scanBlockComment(start source.Pos) (token.Token, bool) {
	s.advance() // '*'
	for {
		switch s.ch {
		case eof:
			s.diags.MarkIncomplete()
			s.diags.Errorf(source.Span{Start: start, End: s.off}, "unterminated block comment")
			return s.token(token.Invalid, start), true
		case '*':
			s.advance()
			if s.ch == '/' {
				s.advance()
				return s.comment(start)
			}
		default:
			s.advance()
		}
	}
}

func (s *Scanner) comment(start source.Pos) (token.Token, bool) {
	if s.mode&ScanComments == 0 {
		return token.Token{}, false
	}
	return s.token(token.Comment, start), true
}

func isDigit(ch rune) bool       { return ch >= '0' && ch <= '9' }
func isHexDigit(ch rune) bool    { return isDigit(ch) || (lower(ch) >= 'a' && lower(ch) <= 'f') }
func isOctalDigit(ch rune) bool  { return ch >= '0' && ch <= '7' }
func isBinaryDigit(ch rune) bool { return ch == '0' || ch == '1' }

// isNameStart reports whether ch can begin an identifier. Digits are excluded
// so a number is never mistaken for a name.
func isNameStart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

func isNamePart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

// lower folds an ASCII letter to lowercase, leaving everything else alone.
func lower(ch rune) rune { return ('a' - 'A') | ch }
