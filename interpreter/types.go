package interpreter

import (
	"bytes"
	"fmt"
	"github.com/fadion/aria/ast"
	"strings"
)

const (
	INTEGER_TYPE    = "Integer"
	FLOAT_TYPE      = "Float"
	STRING_TYPE     = "String"
	BOOLEAN_TYPE    = "Boolean"
	ARRAY_TYPE      = "Array"
	DICTIONARY_TYPE = "Dictionary"
	NIL_TYPE        = "nil"
	FUNCTION_TYPE   = "Function"
	RETURN_TYPE     = "Return"
	BREAK_TYPE      = "Break"
	CONTINUE_TYPE   = "Continue"
	MODULE_TYPE     = "Module"
)

// A NIL, TRUE or FALSE don't have any literal
// meaning. A TRUE is always the same instance.
var (
	NIL   = &NilType{}
	TRUE  = &BooleanType{Value: true}
	FALSE = &BooleanType{Value: false}
)

// Data Type interface.
type DataType interface {
	Type() string
	Inspect() string
}

// Module.
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

// Integer.
type IntegerType struct {
	Value int64
}

func (t *IntegerType) Type() string    { return INTEGER_TYPE }
func (t *IntegerType) Inspect() string { return fmt.Sprintf("%d", t.Value) }

// Floating point number.
type FloatType struct {
	Value float64
}

func (t *FloatType) Type() string    { return FLOAT_TYPE }
func (t *FloatType) Inspect() string { return fmt.Sprintf("%f", t.Value) }

// String.
type StringType struct {
	Value string
}

func (t *StringType) Type() string    { return STRING_TYPE }
func (t *StringType) Inspect() string { return "\"" + t.Value + "\"" }

// Boolean.
type BooleanType struct {
	Value bool
}

func (t *BooleanType) Type() string    { return BOOLEAN_TYPE }
func (t *BooleanType) Inspect() string { return fmt.Sprintf("%t", t.Value) }

// Array.
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

// Dictionary.
type DictionaryType struct {
	Pairs map[*StringType]DataType
}

func (t *DictionaryType) Type() string { return DICTIONARY_TYPE }
func (t *DictionaryType) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}
	for key, value := range t.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s:%s", key.Value, value.Inspect()))
	}

	out.WriteString("[")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("]")

	return out.String()
}

// NIL.
type NilType struct{}

func (t *NilType) Type() string    { return NIL_TYPE }
func (t *NilType) Inspect() string { return "nil" }

// Function.
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

// Return.
type ReturnType struct {
	Value DataType
}

func (t *ReturnType) Type() string    { return RETURN_TYPE }
func (t *ReturnType) Inspect() string { return t.Value.Inspect() }

// Break.
type BreakType struct{}

func (t *BreakType) Type() string    { return BREAK_TYPE }
func (t *BreakType) Inspect() string { return BREAK_TYPE }

// Continue.
type ContinueType struct{}

func (t *ContinueType) Type() string    { return CONTINUE_TYPE }
func (t *ContinueType) Inspect() string { return CONTINUE_TYPE }
