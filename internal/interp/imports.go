package interp

import (
	"os"
	"path/filepath"

	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/diag"
	"github.com/fadion/aria/internal/parser"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/value"
)

// A unit is one file of a program. The entry file and everything it imports,
// directly or not, are units of the same compilation — which is what lets the
// resolver see all of it.
//
// Before this, an imported file was parsed and evaluated and never resolved, so
// inside one an undefined name was a runtime error, `let` immutability was
// unenforced, and every guarantee the resolver provides was absent. And a single
// `import` anywhere in a file turned undefined-name checking off for the whole
// of the importing file too, because the resolver could not see what the import
// might have brought in.
type unit struct {
	file *source.File
	prog *ast.Program
	bag  *diag.Bag
	// alias is the name an `import ... as Name` gives the file's own top-level
	// bindings. Empty for a plain import, whose names join the importing scope.
	alias string
}

// loadUnits parses everything prog imports, depth first, in the order their
// names have to become visible: an imported file's own imports come first.
//
// The entry unit is not included; it is the caller's, already parsed.
func loadUnits(file *source.File, prog *ast.Program, bag *diag.Bag) ([]unit, bool) {
	l := &loader{seen: map[string]bool{}}
	if abs, err := filepath.Abs(file.Name); err == nil {
		l.seen[abs] = true
	}
	if !l.walk(filepath.Dir(file.Name), prog, bag) {
		return nil, false
	}
	return l.units, true
}

type loader struct {
	// seen holds the absolute path of every file already pulled in, which is
	// what makes a cycle terminate. A second import of the same file is a
	// no-op: its names are already in scope, so there is nothing to do and
	// nothing to report.
	seen  map[string]bool
	units []unit
}

func (l *loader) walk(dir string, prog *ast.Program, bag *diag.Bag) bool {
	for _, n := range prog.Nodes {
		imp, ok := n.(*ast.Import)
		if !ok {
			continue
		}
		if !l.load(dir, imp, bag) {
			return false
		}
	}
	return true
}

func (l *loader) load(dir string, imp *ast.Import, bag *diag.Bag) bool {
	name := imp.File
	if filepath.Ext(name) == "" {
		name += ".ari"
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, name)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if l.seen[abs] {
		return true
	}
	l.seen[abs] = true

	src, err := os.ReadFile(path)
	if err != nil {
		bag.Errorf(imp.Span(), "cannot read imported file '%s'", imp.File)
		return false
	}

	file := source.NewFile(path, src)
	unitBag := diag.New(file)
	unitProg := parser.New(file, unitBag).Parse()
	if unitBag.HasErrors() {
		l.units = append(l.units, unit{file: file, prog: unitProg, bag: unitBag})
		return false
	}

	// Depth first: what this file imports has to be in scope before it is.
	if !l.walk(filepath.Dir(path), unitProg, unitBag) {
		l.units = append(l.units, unit{file: file, prog: unitProg, bag: unitBag})
		return false
	}

	alias := ""
	if imp.Alias != nil {
		alias = imp.Alias.Value
	}
	l.units = append(l.units, unit{file: file, prog: unitProg, bag: unitBag, alias: alias})
	return true
}

// evalUnits runs each imported unit before the program that imported it.
//
// The same interpreter runs all of them, rather than a sub-interpreter per file.
// A separate Interp had its own signal field, so a stray top-level `return` or
// `break` in an imported file set a copy nobody read — it neither propagated nor
// errored. The resolver rejects those now, and there is no second copy to lose
// them in either way.
func (i *Interp) evalUnits(units []unit) error {
	outer := i.file
	defer func() { i.file = outer }()

	for _, u := range units {
		i.file = u.file

		scope := i.globals
		if u.alias != "" {
			// An aliased import gets a scope of its own, which then becomes a
			// module. A module IS a namespace, so aliasing needs no machinery
			// the language does not already have.
			scope = newEnv(i.globals)
		}

		if _, err := i.runNodes(u.prog, scope); err != nil {
			return err
		}

		if u.alias != "" {
			i.defineModule(u.alias, u.prog, scope)
		}
	}
	return nil
}

// defineModule turns an aliased unit's top-level bindings into a module.
func (i *Interp) defineModule(name string, prog *ast.Program, scope *env) {
	m := &Module{Name: name, members: map[string]value.Value{}}
	for _, n := range prog.Nodes {
		var bound *ast.Identifier
		switch n := n.(type) {
		case *ast.Let:
			bound = n.Name
		case *ast.Var:
			bound = n.Name
		}
		if bound == nil {
			continue
		}
		if v, ok := scope.vars[bound.Value]; ok {
			if _, seen := m.members[bound.Value]; !seen {
				m.order = append(m.order, bound.Value)
			}
			m.members[bound.Value] = v
		}
	}
	i.modules[name] = m
	i.globals.define(name, m)
}
