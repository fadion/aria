// Package source holds the text being compiled and converts byte offsets into
// human-readable positions.
//
// Everything downstream — tokens, AST nodes, diagnostics — refers to source
// text by byte offset rather than by line and column. Offsets are cheap to
// store (two int32 per span), exact, and cannot drift out of sync with the
// text. Line and column are derived here, only when a message is actually
// rendered for a human.
package source

import (
	"strings"
	"unicode/utf8"
)

// Pos is a byte offset into a File's contents. The zero value is the first
// byte of the file; NoPos marks "no position".
type Pos int32

// NoPos is the absence of a position. It is negative so that a zero-valued
// Span is not mistaken for a real one at offset 0.
const NoPos Pos = -1

// A Span is a half-open byte range [Start, End) of a File.
type Span struct {
	Start Pos
	End   Pos
}

// SpanAt returns a zero-width span at off, for errors that point at a single
// place rather than covering a range of text.
func SpanAt(off Pos) Span { return Span{Start: off, End: off} }

// IsValid reports whether s refers to real text.
func (s Span) IsValid() bool { return s.Start >= 0 && s.End >= s.Start }

// Position is a human-facing location: 1-based line, 1-based column counted in
// runes rather than bytes, so a caret lines up under the character a reader
// sees rather than under the middle of a multi-byte one.
type Position struct {
	Line int
	Col  int
}

// A File is a unit of source text and its line index.
type File struct {
	Name string
	src  []byte
	// lineStarts[i] is the offset of the first byte of line i+1. It always
	// begins with 0, so a file with no newlines still has one line.
	lineStarts []Pos
}

// NewFile indexes src for position lookups. It does not copy src, and the
// caller must not modify it afterwards.
func NewFile(name string, src []byte) *File {
	// One entry per line, plus the leading 0. Most source averages well under
	// 64 bytes a line; over-allocating slightly beats regrowing repeatedly.
	starts := make([]Pos, 1, len(src)/32+2)
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, Pos(i+1))
		}
	}
	return &File{Name: name, src: src, lineStarts: starts}
}

// Bytes returns the file contents.
func (f *File) Bytes() []byte { return f.src }

// Size is the length of the file in bytes.
func (f *File) Size() Pos { return Pos(len(f.src)) }

// LineCount is the number of lines in the file, counting a trailing newline as
// ending its line rather than starting an empty one.
func (f *File) LineCount() int { return len(f.lineStarts) }

// Text returns the source text covered by s. An invalid or out-of-range span
// yields the empty string rather than panicking, since spans often reach here
// from error paths where something has already gone wrong.
func (f *File) Text(s Span) string {
	if !s.IsValid() {
		return ""
	}
	start, end := s.Start, s.End
	if start > f.Size() {
		start = f.Size()
	}
	if end > f.Size() {
		end = f.Size()
	}
	return string(f.src[start:end])
}

// Position converts a byte offset to a line and column. Offsets past the end
// of the file clamp to the final position, so an error reported at EOF still
// names a real place.
func (f *File) Position(off Pos) Position {
	if off < 0 {
		off = 0
	}
	if off > f.Size() {
		off = f.Size()
	}

	line := f.lineIndex(off)
	lineStart := f.lineStarts[line]

	// Column counts runes, not bytes.
	col := utf8.RuneCount(f.src[lineStart:off]) + 1
	return Position{Line: line + 1, Col: col}
}

// lineIndex returns the 0-based index of the line containing off, by binary
// search over the line table.
func (f *File) lineIndex(off Pos) int {
	lo, hi := 0, len(f.lineStarts)
	for lo < hi {
		mid := (lo + hi) / 2
		if f.lineStarts[mid] <= off {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo - 1
}

// LineText returns the text of a 1-based line number, without its terminating
// newline. Out-of-range lines yield the empty string.
func (f *File) LineText(line int) string {
	if line < 1 || line > len(f.lineStarts) {
		return ""
	}
	start := f.lineStarts[line-1]
	end := f.Size()
	if line < len(f.lineStarts) {
		end = f.lineStarts[line]
	}
	return strings.TrimRight(string(f.src[start:end]), "\r\n")
}
