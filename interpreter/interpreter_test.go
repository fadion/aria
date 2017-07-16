package interpreter

import (
	"testing"
	"github.com/fadion/aria/lexer"
	"github.com/fadion/aria/reader"
	"github.com/fadion/aria/parser"
	"github.com/fadion/aria/reporter"
)

func TestInterpreterString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`"hello"+"world"`, "helloworld"},
		{`"hello"+" "+"world"`, "hello world"},
	}

	for _, test := range tests {
		lex := lexer.New(reader.New([]byte(test.input)))
		parse := parser.New(lex)
		program := parse.Parse()
		runner := New()
		actual := runner.Interpret(program, NewScope())
		checkForErrors(t)

		testStringType(t, actual, test.expected)
	}
}

func TestInterpreterInteger(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`10`, 10},
		{`1234567`, 1234567},
		{`1 + 1`, 2},
		{`-10`, -10},
		{`-10 + 10`, 0},
		{`5 * 2`, 10},
		{`5 * (2 + 2)`, 20},
		{`2 ** 8`, 256},
		{`5 % 2`, 1},
	}

	for _, test := range tests {
		lex := lexer.New(reader.New([]byte(test.input)))
		parse := parser.New(lex)
		program := parse.Parse()
		runner := New()
		actual := runner.Interpret(program, NewScope())
		checkForErrors(t)

		testIntegerType(t, actual, test.expected)
	}
}

func TestInterpreterFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{`10.0`, 10.0},
		{`10.0 + 1.2`, 11.2},
		{`1 - 0.5`, 0.5},
		{`4.5 * 2`, 9.0},
		{`-5.2`, -5.2},
		{`9.0 / 3`, 3.0},
	}

	for _, test := range tests {
		lex := lexer.New(reader.New([]byte(test.input)))
		parse := parser.New(lex)
		program := parse.Parse()
		runner := New()
		actual := runner.Interpret(program, NewScope())
		checkForErrors(t)

		testFloatType(t, actual, test.expected)
	}
}

func TestInterpreterBoolean(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`true`, true},
		{`false`, false},
		{`!false`, true},
		{`1 == 1`, true},
		{`1 == 2`, false},
		{`1 != 2`, true},
		{`1 != 1`, false},
		{`5 > 1`, true},
		{`5 >= 5`, true},
		{`10 > 100`, false},
		{`(1 < 2) == (2 > 1)`, true},
		{`5.3 > 5.2`, true},
		{`"four" > "one"`, true},
		{`"hello" == "world"`, false},
		{`[1, 2] == [3, 4]`, false},
		{`[1, 2] == [1, 2]`, true},
		{`["a": "b", "c": "d"] == ["a": "b", "c": "d"]`, true},
		{`true == !false`, true},
		{`true && true`, true},
		{`true && false`, false},
		{`false || false`, false},
		{`false || true`, true},
	}

	for _, test := range tests {
		lex := lexer.New(reader.New([]byte(test.input)))
		parse := parser.New(lex)
		program := parse.Parse()
		runner := New()
		actual := runner.Interpret(program, NewScope())
		checkForErrors(t)

		testBooleanType(t, actual, test.expected)
	}
}

func TestInterpreterIf(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{`if 5 > 2 then 10 end`, 10},
		{`if 5 < 2 then 10 else 15 end`, 15},
		{`if true then 10 end`, 10},
	}

	for _, test := range tests {
		lex := lexer.New(reader.New([]byte(test.input)))
		parse := parser.New(lex)
		program := parse.Parse()
		runner := New()
		actual := runner.Interpret(program, NewScope())
		checkForErrors(t)

		result, ok := test.expected.(int)
		if !ok {
			t.Errorf("Expected Integer but got %T", test.expected)
		}

		testIntegerType(t, actual, int64(result))
	}
}

func TestInterpreterSwitch(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{`switch 1 do case 1 then 10 case 2 then 20 end`, 10},
		{`switch 2 do case 1 then 10 case 2 then 20 end`, 20},
		{`switch 3 do case 1 then 10 default then 20 end`, 20},
		{`switch do case 1 == 1 then 10 end`, 10},
	}

	for _, test := range tests {
		lex := lexer.New(reader.New([]byte(test.input)))
		parse := parser.New(lex)
		program := parse.Parse()
		runner := New()
		actual := runner.Interpret(program, NewScope())
		checkForErrors(t)

		result, ok := test.expected.(int)
		if !ok {
			t.Errorf("Expected Integer but got %T", test.expected)
		}

		testIntegerType(t, actual, int64(result))
	}
}

func testStringType(t *testing.T, tp DataType, expected string) bool {
	result, ok := tp.(*StringType)
	if !ok {
		t.Errorf("Expected StringType but got %t", tp)
		return false
	}

	if result.Value != expected {
		t.Errorf("Expected %s but got %s", expected, result.Value)
		return false
	}

	return true
}

func testIntegerType(t *testing.T, tp DataType, expected int64) bool {
	result, ok := tp.(*IntegerType)
	if !ok {
		t.Errorf("Expected IntegerType but got %t", tp)
		return false
	}

	if result.Value != expected {
		t.Errorf("Expected %d but got %d", expected, result.Value)
		return false
	}

	return true
}

func testFloatType(t *testing.T, tp DataType, expected float64) bool {
	result, ok := tp.(*FloatType)
	if !ok {
		t.Errorf("Expected FloatType but got %t", tp)
		return false
	}

	if result.Value != expected {
		t.Errorf("Expected %f but got %f", expected, result.Value)
		return false
	}

	return true
}

func testBooleanType(t *testing.T, tp DataType, expected bool) bool {
	result, ok := tp.(*BooleanType)
	if !ok {
		t.Errorf("Expected BooleanType but got %t", tp)
		return false
	}

	if result.Value != expected {
		t.Errorf("Expected %t but got %t", expected, result.Value)
		return false
	}

	return true
}

func checkForErrors(t *testing.T) {
	if reporter.HasErrors() {
		t.Errorf("Parse Errors: ")
		for _, v := range reporter.GetErrors() {
			t.Errorf(v)
		}
	}
}