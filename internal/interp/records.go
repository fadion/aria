package interp

import (
	"github.com/fadion/aria/internal/ast"
	"github.com/fadion/aria/internal/source"
	"github.com/fadion/aria/internal/value"
)

// RecordDef is a declared record, bound to its own name.
//
// Calling it constructs an instance, which is why a record needs no
// construction syntax: its fields are a parameter list, so `Point(1, 2)` is an
// ordinary call, with the same arity check, type hints and defaults every other
// call has.
//
// Anything shaped like a domain object used to have no home. A module holds only
// `let` and cannot be instantiated; a dictionary carries data but no identity, so
// `typeof` said "Dictionary" for every one of them and `is` could not tell a
// point from a config.
type RecordDef struct {
	Decl *ast.Record
	Def  *value.RecordType
	// File is where the declaration was written, so a fault building an
	// instance is reported there.
	File *source.File
}

func (*RecordDef) Type() value.Type       { return value.TRecord }
func (d *RecordDef) String() string       { return "record " + d.Def.Name }
func (d *RecordDef) Inspect() string      { return d.String() }
func (d *RecordDef) FieldNames() []string { return d.Def.Fields }

// evalRecord declares a record. The name is bound like a module's: a record is
// an ordinary value that happens to be callable.
func (i *Interp) evalRecord(n *ast.Record, e *env) value.Value {
	if _, exists := i.records[n.Name.Value]; exists {
		i.fail(n.Name.Span(), "record '%s' is already declared", n.Name.Value)
	}

	fields := make([]string, 0, len(n.Fields))
	seen := map[string]bool{}
	for _, f := range n.Fields {
		if f == nil || f.Name == nil {
			continue
		}
		if seen[f.Name.Value] {
			i.fail(f.Name.Span(), "record '%s' declares '%s' twice", n.Name.Value, f.Name.Value)
		}
		seen[f.Name.Value] = true
		fields = append(fields, f.Name.Value)
	}

	def := &RecordDef{
		Decl: n,
		Def:  &value.RecordType{Name: n.Name.Value, Fields: fields},
		File: i.curFile(),
	}
	i.records[n.Name.Value] = def
	e.define(n.Name.Value, def)
	return def
}

// construct builds an instance from a call's arguments.
//
// The arity and type rules are a function's, because the fields are a parameter
// list: a trailing field with a default may be omitted, and a hint is enforced.
func (i *Interp) construct(def *RecordDef, args []value.Value, span source.Span) value.Value {
	callerFile := i.curFile()
	fields := def.Decl.Fields

	required := 0
	for idx, f := range fields {
		if f.Default == nil {
			required = idx + 1
		}
	}
	if len(args) < required {
		i.failIn(callerFile, span, "%s expects at least %d field(s), got %d",
			def.Def.Name, required, len(args))
	}
	if len(args) > len(fields) {
		i.failIn(callerFile, span, "%s expects at most %d field(s), got %d",
			def.Def.Name, len(fields), len(args))
	}

	values := make([]value.Value, len(fields))
	for idx, f := range fields {
		if idx < len(args) {
			values[idx] = args[idx]
		} else {
			// A default is evaluated in the file the record was declared in,
			// which is where a fault in one belongs.
			outer := i.file
			i.file = def.File
			values[idx] = i.eval(f.Default, i.globals)
			i.file = outer
		}
		i.checkFieldType(def, f, values[idx], callerFile, span)
	}
	return value.NewRecord(def.Def, values)
}

func (i *Interp) checkFieldType(def *RecordDef, f *ast.FunctionParameter, v value.Value, file *source.File, span source.Span) {
	if f.Type == nil || f.Type.Value == value.Any {
		return
	}
	if value.TypeName(v) != f.Type.Value {
		i.failIn(file, span, "%s field '%s' expects %s, got %s",
			def.Def.Name, f.Name.Value, f.Type.Value, value.TypeName(v))
	}
}
