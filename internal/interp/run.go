package interp

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/parser"
	"github.com/fadion/aria/internal/resolver"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/stdlib"
	"github.com/fadion/aria/internal/value"
)

// Options configure a run.
type Options struct {
	Out io.Writer
	Err io.Writer
	In  io.Reader
	// NoStdlib skips loading the standard library, for tests that want a bare
	// interpreter.
	NoStdlib bool
}

// Run compiles and evaluates one source file.
//
// It returns false if anything went wrong, having already written the reason to
// Options.Err. The phases are deliberately separate and ordered: nothing is
// evaluated until the whole file has parsed and resolved, so a name error in a
// branch that never executes is still reported.
func Run(file *source.File, opts Options) bool {
	if opts.Out == nil || opts.Err == nil {
		panic("interp.Run: Out and Err are required")
	}

	bag := diag.New(file)

	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		fmt.Fprint(opts.Err, bag.Render())
		return false
	}

	i := New(file, nil)
	i.Out, i.Err, i.In = opts.Out, opts.Err, opts.In

	// The standard library is compiled and evaluated first, into the same
	// globals the program will use. Doing it once here rather than on every
	// node visit is the fix for the original's importLibraryModules, which was
	// branch-checked on every single AST node.
	if !opts.NoStdlib {
		if !i.loadStdlib(opts.Err) {
			return false
		}
	}

	// Globals first, then modules: predeclaring a name replaces its binding, and
	// a module's binding is the one its member list is attached to. A module IS
	// a global, so the other order silently dropped the member checking.
	r := resolver.New(file, bag)
	for name := range i.globals.vars {
		r.Predeclare(name)
	}
	i.predeclareModules(r)
	info := r.Resolve(prog)
	if bag.HasErrors() {
		fmt.Fprint(opts.Err, bag.Render())
		return false
	}
	i.info = info

	if _, err := i.Run(prog); err != nil {
		var re *Error
		if !errors.As(err, &re) {
			fmt.Fprintln(opts.Err, err)
			return false
		}
		i.Report(re)
		return false
	}
	return true
}

// Check compiles file without evaluating it, reporting into Options.Err.
//
// It is the same pipeline Run uses, stopped after resolution — which is what an
// editor or a CI step wants, and what Run already did internally before
// evaluating anything.
func Check(file *source.File, opts Options) bool {
	if opts.Err == nil {
		panic("interp.Check: Err is required")
	}

	bag := diag.New(file)
	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		fmt.Fprint(opts.Err, bag.Render())
		return false
	}

	// The standard library has to be loaded, not just named: resolution checks
	// module members, and those are only known once it has been evaluated.
	i := New(file, nil)
	i.Out, i.Err, i.In = io.Discard, opts.Err, strings.NewReader("")
	if !opts.NoStdlib && !i.loadStdlib(opts.Err) {
		return false
	}

	r := resolver.New(file, bag)
	for name := range i.globals.vars {
		r.Predeclare(name)
	}
	i.predeclareModules(r)
	r.Resolve(prog)
	if bag.HasErrors() {
		fmt.Fprint(opts.Err, bag.Render())
		return false
	}
	return true
}

// predeclareModules tells a resolver about every module that exists, with the
// members each one declares, so an access to a missing member is a diagnostic
// rather than a runtime error. The standard library is already evaluated by the
// time this runs, so its members are real values rather than a guess.
func (i *Interp) predeclareModules(r *resolver.Resolver) {
	for _, name := range stdlib.Names() {
		if m, ok := i.modules[name]; ok {
			r.PredeclareModule(name, m.Names()...)
			continue
		}
		r.PredeclareModule(name)
	}
	for name, m := range i.modules {
		r.PredeclareModule(name, m.Names()...)
	}
}

// loadStdlib parses, resolves and evaluates the embedded library.
//
// A failure here is a bug in the library rather than in the user's program, so
// it says so: the original reported such failures against a line number in the
// embedded source, which pointed nowhere the reader could look.
func (i *Interp) loadStdlib(errOut io.Writer) bool {
	mods := stdlib.Modules()

	for _, m := range mods {
		file := source.NewFile(m.Path, []byte(m.Src))
		bag := diag.New(file)

		prog := parser.New(file, bag).Parse()
		if !bag.HasErrors() {
			r := resolver.New(file, bag)
			for _, name := range stdlib.Names() {
				r.PredeclareModule(name)
			}
			r.Resolve(prog)
		}
		if bag.HasErrors() {
			fmt.Fprintf(errOut, "internal error: the standard library failed to compile\n%s", bag.Render())
			return false
		}

		sub := &Interp{
			file: file, info: nil,
			Out: i.Out, Err: i.Err, In: i.In,
			globals: i.globals, modules: i.modules,
			dir: ".", imported: i.imported,
		}
		if _, err := sub.Run(prog); err != nil {
			var re *Error
			if errors.As(err, &re) {
				sub.Report(re)
			} else {
				fmt.Fprintln(errOut, err)
			}
			return false
		}
	}
	return true
}

// Eval compiles and evaluates src, returning its value. It is the entry point
// tests use, and skips the standard library unless asked for it.
func Eval(name, src string, opts Options) (value.Value, error) {
	file := source.NewFile(name, []byte(src))
	bag := diag.New(file)

	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		return nil, fmt.Errorf("%s", bag.Render())
	}

	i := New(file, nil)
	if opts.Out != nil {
		i.Out = opts.Out
	}
	if opts.Err != nil {
		i.Err = opts.Err
	}
	if opts.In != nil {
		i.In = opts.In
	}

	if !opts.NoStdlib {
		var sink discard
		if !i.loadStdlib(&sink) {
			return nil, fmt.Errorf("standard library failed to load: %s", sink.String())
		}
	}

	r := resolver.New(file, bag)
	for n := range i.globals.vars {
		r.Predeclare(n)
	}
	i.predeclareModules(r)
	info := r.Resolve(prog)
	if bag.HasErrors() {
		return nil, fmt.Errorf("%s", bag.Render())
	}
	i.info = info

	return i.Run(prog)
}

// Nodes exposes a parsed program's nodes, for tests that need the tree.
func Nodes(prog *ast.Program) []ast.Node { return prog.Nodes }

// discard collects anything written to it, so a failure can still be reported.
type discard struct{ b []byte }

func (d *discard) Write(p []byte) (int, error) { d.b = append(d.b, p...); return len(p), nil }
func (d *discard) String() string              { return string(d.b) }

// ---------------------------------------------------------------------------
// Interactive sessions
// ---------------------------------------------------------------------------

// A Session evaluates one line at a time against persistent state, for a REPL.
//
// Each line is a complete compilation: it is parsed and resolved before it runs,
// so an undefined name is reported without the line having any effect. Names
// declared by earlier lines are carried forward and predeclared for later ones.
type Session struct {
	interp *Interp
	// declared accumulates the top-level names earlier lines introduced.
	declared map[string]bool
	line     int
}

// Declared lists the names earlier fragments introduced, in sorted order, so a
// REPL can show what is in scope.
func (s *Session) Declared() []string {
	out := make([]string, 0, len(s.declared))
	for name := range s.declared {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Modules lists the modules in scope, standard library included.
func (s *Session) Modules() []string {
	out := make([]string, 0, len(s.interp.modules))
	for name := range s.interp.modules {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// NewSession starts a session with the standard library loaded.
func NewSession(opts Options) (*Session, error) {
	file := source.NewFile("<repl>", nil)
	i := New(file, nil)
	if opts.Out != nil {
		i.Out = opts.Out
	}
	if opts.Err != nil {
		i.Err = opts.Err
	}
	if opts.In != nil {
		i.In = opts.In
	}
	i.dir = "."

	if !opts.NoStdlib {
		var sink discard
		if !i.loadStdlib(&sink) {
			return nil, fmt.Errorf("standard library failed to load: %s", sink.String())
		}
	}

	return &Session{interp: i, declared: map[string]bool{}}, nil
}

// ErrIncomplete says the source given to Session.Eval opened a construct it did
// not close, so more input could complete it. A REPL buffers on this rather than
// reporting it.
var ErrIncomplete = errors.New("incomplete input")

// Eval runs one fragment. It returns its value, or an error describing why it
// could not run — already formatted with a caret, as a file would be.
//
// A REPL that reads one line at a time cannot enter a multi-line func, module,
// if or for at all, which rules out most of what a REPL is for. Eval takes a
// whole fragment and says ErrIncomplete when more input could complete it.
func (s *Session) Eval(src string) (value.Value, error) {
	s.line++
	file := source.NewFile(fmt.Sprintf("<repl:%d>", s.line), []byte(src))
	bag := diag.New(file)

	prog := parser.New(file, bag).Parse()
	if bag.HasErrors() {
		if bag.Incomplete() {
			// The fragment opened something it did not close. A REPL reads this
			// as "keep typing" rather than as a mistake.
			s.line--
			return nil, ErrIncomplete
		}
		return nil, fmt.Errorf("%s", strings.TrimRight(bag.Render(), "\n"))
	}

	r := resolver.New(file, bag)
	for name := range s.interp.globals.vars {
		r.Predeclare(name)
	}
	for name := range s.declared {
		r.Predeclare(name)
	}
	// A module declared on an earlier line is still declared, and its members
	// are known, so an access to one is checked like any other.
	s.interp.predeclareModules(r)

	info := r.Resolve(prog)
	if bag.HasErrors() {
		return nil, fmt.Errorf("%s", strings.TrimRight(bag.Render(), "\n"))
	}

	s.interp.file = file
	s.interp.info = info

	v, err := s.interp.Run(prog)
	if err != nil {
		var re *Error
		if errors.As(err, &re) {
			at := re.File
			if at == nil {
				at = file
			}
			b := diag.New(at)
			b.Errorf(re.Span, "%s", re.Msg)
			return nil, fmt.Errorf("%s", strings.TrimRight(b.Render(), "\n"))
		}
		return nil, err
	}

	// Remember what this line declared, so the next one can see it.
	for _, n := range prog.Nodes {
		switch n := n.(type) {
		case *ast.Let:
			if n.Name != nil {
				s.declared[n.Name.Value] = true
			}
		case *ast.Var:
			if n.Name != nil {
				s.declared[n.Name.Value] = true
			}
		}
	}
	return v, nil
}
