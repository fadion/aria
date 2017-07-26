package interpreter

import (
	"bytes"
	"fmt"
	"github.com/fadion/aria/ast"
	"strings"
)

// Data types.
const (
	INTEGER_TYPE     = "Integer"
	FLOAT_TYPE       = "Float"
	STRING_TYPE      = "String"
	ATOM_TYPE        = "Atom"
	BOOLEAN_TYPE     = "Boolean"
	ARRAY_TYPE       = "Array"
	DICTIONARY_TYPE  = "Dictionary"
	NIL_TYPE         = "Nil"
	FUNCTION_TYPE    = "Function"
	RETURN_TYPE      = "Return"
	BREAK_TYPE       = "Break"
	CONTINUE_TYPE    = "Continue"
	MODULE_TYPE      = "Module"
	PLACEHOLDER_TYPE = "Placeholder"
)

// A NIL, TRUE or FALSE don't have any literal
// meaning. A TRUE is always the same instance.
var (
	NIL   = &NilType{}
	TRUE  = &BooleanType{Value: true}
	FALSE = &BooleanType{Value: false}
)

// DataType interface.
type DataType interface {
	Type() string
	Inspect() string
}

// ModuleType for modules.
type ModuleType struct {
	Name *ast.Identifier
	Body *ast.BlockStatement
}

func (t *ModuleType) Type() string { return MODULE_TYPE }
func (t *ModuleType) Inspect() string {
	var out bytes.Buffer

	out.WriteString("Module ")
	out.WriteString(t.Name.Inspect())
	out.WriteString(" { ")
	out.WriteString(t.Body.Inspect())
	out.WriteString(" } ")

	return out.String()
}

// IntegerType for integers.
type IntegerType struct {
	Value int64
}

func (t *IntegerType) Type() string    { return INTEGER_TYPE }
func (t *IntegerType) Inspect() string { return fmt.Sprintf("%d", t.Value) }

// FloatType for floating point numbers.
type FloatType struct {
	Value float64
}

func (t *FloatType) Type() string    { return FLOAT_TYPE }
func (t *FloatType) Inspect() string { return fmt.Sprintf("%f", t.Value) }

// StringType for strings.
type StringType struct {
	Value string
}

func (t *StringType) Type() string    { return STRING_TYPE }
func (t *StringType) Inspect() string { return "\"" + t.Value + "\"" }

// AtomType for atoms.
type AtomType struct {
	Value string
}

func (t *AtomType) Type() string    { return ATOM_TYPE }
func (t *AtomType) Inspect() string { return ":" + t.Value }

// BooleanType for boolean.
type BooleanType struct {
	Value bool
}

func (t *BooleanType) Type() string    { return BOOLEAN_TYPE }
func (t *BooleanType) Inspect() string { return fmt.Sprintf("%t", t.Value) }

// ArrayType for arrays.
type ArrayType struct {
	Elements []DataType
}

func (t *ArrayType) Type() string { return ARRAY_TYPE }
func (t *ArrayType) Inspect() string {
	var out bytes.Buffer

	elements := []string{}
	for _, e := range t.Elements {
		elements = append(elements, e.Inspect())
	}

	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}

// DictionaryType for dictionaries.
type DictionaryType struct {
	Pairs map[DataType]DataType
}

func (t *DictionaryType) Type() string { return DICTIONARY_TYPE }
func (t *DictionaryType) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}
	for key, value := range t.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s:%s", key.Inspect(), value.Inspect()))
	}

	out.WriteString("[")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("]")

	return out.String()
}

// NilType for nil.
type NilType struct{}

func (t *NilType) Type() string    { return NIL_TYPE }
func (t *NilType) Inspect() string { return "nil" }

// FunctionType for functions.
type FunctionType struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Scope      *Scope
}

func (t *FunctionType) Type() string { return FUNCTION_TYPE }
func (t *FunctionType) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range t.Parameters {
		params = append(params, p.Inspect())
	}

	out.WriteString("fn")
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(t.Body.Inspect())
	out.WriteString("\n}")

	return out.String()
}

// ReturnType for return.
type ReturnType struct {
	Value DataType
}

func (t *ReturnType) Type() string    { return RETURN_TYPE }
func (t *ReturnType) Inspect() string { return t.Value.Inspect() }

// BreakType for break.
type BreakType struct{}

func (t *BreakType) Type() string    { return BREAK_TYPE }
func (t *BreakType) Inspect() string { return BREAK_TYPE }

// ContinueType for continue.
type ContinueType struct{}

func (t *ContinueType) Type() string    { return CONTINUE_TYPE }
func (t *ContinueType) Inspect() string { return CONTINUE_TYPE }

// PlaceholderType for continue.
type PlaceholderType struct{}

func (t *PlaceholderType) Type() string    { return PLACEHOLDER_TYPE }
func (t *PlaceholderType) Inspect() string { return PLACEHOLDER_TYPE }
