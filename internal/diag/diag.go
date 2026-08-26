// Package diag collects and renders compiler diagnostics.
//
// Diagnostics are values held by whoever is compiling, not package-level state.
// That lets two compilations run at once, lets tests run in parallel without
// leaking errors into each other, and makes the packages usable as a library.
package diag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fadion/aria/internal/source"
)

// Severity classifies a diagnostic.
type Severity uint8

const (
	Error Severity = iota
	Warning
	// Note attaches context to the diagnostic before it — where a name was
	// first declared, say. A note is not itself a problem, so it does not make
	// HasErrors true and does not count against the cap.
	Note
)

func (s Severity) String() string {
	switch s {
	case Warning:
		return "warning"
	case Note:
		return "note"
	}
	return "error"
}

// A Diagnostic is one problem found in the source, located by span rather than
// by pre-formatted line and column so the renderer can show the offending text.
type Diagnostic struct {
	Severity Severity
	Span     source.Span
	Message  string
}

// DefaultMax is how many diagnostics a Bag keeps before it stops recording.
// Past a certain point extra messages describe the confusion caused by earlier
// ones rather than new problems.
const DefaultMax = 20

// A Bag accumulates diagnostics for one compilation.
type Bag struct {
	file   *source.File
	list   []Diagnostic
	max    int
	capped bool
}

// New returns a Bag for file, keeping at most DefaultMax diagnostics.
func New(file *source.File) *Bag { return &Bag{file: file, max: DefaultMax} }

// SetMax changes the cap. A cap of zero or less means unlimited.
func (b *Bag) SetMax(n int) { b.max = n }

// Errorf records an error covering span.
func (b *Bag) Errorf(span source.Span, format string, args ...any) {
	b.add(Error, span, format, args...)
}

// Warnf records a warning covering span.
func (b *Bag) Warnf(span source.Span, format string, args ...any) {
	b.add(Warning, span, format, args...)
}

// Notef attaches context to the preceding diagnostic. Notes are exempt from the
// cap, so a capped run does not leave an error stripped of its explanation.
func (b *Bag) Notef(span source.Span, format string, args ...any) {
	b.add(Note, span, format, args...)
}

func (b *Bag) add(sev Severity, span source.Span, format string, args ...any) {
	if sev != Note && b.max > 0 && len(b.list) >= b.max {
		b.capped = true
		return
	}
	b.list = append(b.list, Diagnostic{
		Severity: sev,
		Span:     span,
		Message:  fmt.Sprintf(format, args...),
	})
}

// HasErrors reports whether any diagnostic is an error.
func (b *Bag) HasErrors() bool {
	for _, d := range b.list {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// Len is the number of diagnostics recorded.
func (b *Bag) Len() int { return len(b.list) }

// Capped reports whether diagnostics were dropped because the cap was reached.
func (b *Bag) Capped() bool { return b.capped }

// All returns the diagnostics in source order. Callers producing them out of
// order — a parser that reports a missing `end` only on reaching EOF — still
// get a list a reader can follow top to bottom.
//
// Notes travel with the diagnostic they follow rather than sorting on their own
// span, which is usually earlier in the file: "declared here" has to stay under
// the error it explains.
func (b *Bag) All() []Diagnostic {
	type group struct {
		head  Diagnostic
		notes []Diagnostic
	}

	var groups []group
	for _, d := range b.list {
		if d.Severity == Note && len(groups) > 0 {
			last := &groups[len(groups)-1]
			last.notes = append(last.notes, d)
			continue
		}
		groups = append(groups, group{head: d})
	}

	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].head.Span.Start < groups[j].head.Span.Start
	})

	out := make([]Diagnostic, 0, len(b.list))
	for _, g := range groups {
		out = append(out, g.head)
		out = append(out, g.notes...)
	}
	return out
}

// Render formats every diagnostic, each with the offending line and a caret
// under the span:
//
//	main.ari:3:11: error: unterminated string
//	  let s = "oops
//	          ^
func (b *Bag) Render() string {
	var sb strings.Builder
	for _, d := range b.All() {
		sb.WriteString(b.render1(d))
	}
	if b.capped {
		fmt.Fprintf(&sb, "(further diagnostics suppressed after %d)\n", b.max)
	}
	return sb.String()
}

func (b *Bag) render1(d Diagnostic) string {
	var sb strings.Builder

	pos := b.file.Position(d.Span.Start)
	fmt.Fprintf(&sb, "%s:%d:%d: %s: %s\n", b.file.Name, pos.Line, pos.Col, d.Severity, d.Message)

	line := b.file.LineText(pos.Line)
	if line == "" {
		return sb.String()
	}
	// Tabs would shift the caret out of alignment, so render them as single
	// spaces in both the source line and the caret line.
	fmt.Fprintf(&sb, "  %s\n", strings.ReplaceAll(line, "\t", " "))

	// Caret width is the span's rune length, clamped to what remains on the
	// line so a span reaching to EOF does not draw a runaway underline.
	width := 1
	if d.Span.End > d.Span.Start {
		width = len([]rune(b.file.Text(d.Span)))
		if nl := strings.IndexByte(b.file.Text(d.Span), '\n'); nl >= 0 {
			width = len([]rune(b.file.Text(d.Span)[:nl]))
		}
	}
	if width < 1 {
		width = 1
	}

	fmt.Fprintf(&sb, "  %s%s\n", strings.Repeat(" ", pos.Col-1), strings.Repeat("^", width))
	return sb.String()
}
