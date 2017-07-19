package interpreter

import (
	"fmt"
	"strings"
	"regexp"
)

// String.count(String) -> Integer
// Count the number of unicode characters in a string.
func stringCount(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.count expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.count expects a String")
	}

	object := args[0].(*StringType).Value
	return &IntegerType{Value: int64(len([]rune(object)))}, nil
}

// String.countBytes(String) -> Integer
// Count the number of bytes in a string.
func stringCountBytes(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.count expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.count expects a String")
	}

	object := args[0].(*StringType).Value
	return &IntegerType{Value: int64(len(object))}, nil
}

// String.lower(String) -> String
// Make all the characters of a string lowercase.
func stringLower(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.lower expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.lower expects a String")
	}

	object := args[0].(*StringType).Value
	return &StringType{Value: strings.ToLower(object)}, nil
}

// String.upper(String) -> String
// Make all the characters of a string uppercase.
func stringUpper(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.upper expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.upper expects a String")
	}

	object := args[0].(*StringType).Value
	return &StringType{Value: strings.ToUpper(object)}, nil
}

// String.capitalize(String) -> String
// Make the first character of words uppercase.
func stringCapitalize(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.capitalize expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.capitalize expects a String")
	}

	object := args[0].(*StringType).Value
	return &StringType{Value: strings.Title(object)}, nil
}

// String.trim(String, subset String) -> String
// Remove all subset characters from the string.
func stringTrim(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.trim expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.trim expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.trim expects a String as subset")
	}

	object := args[0].(*StringType).Value
	subset := args[1].(*StringType).Value

	return &StringType{Value: strings.Trim(object, subset)}, nil
}

// String.trimLeft(String, subset String) -> String
// Remove all subset characters from the start of the string.
func stringTrimLeft(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.trimLeft expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.trimLeft expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.trimLeft expects a String as subset")
	}

	object := args[0].(*StringType).Value
	subset := args[1].(*StringType).Value

	return &StringType{Value: strings.TrimLeft(object, subset)}, nil
}

// String.trimLeft(String, subset String) -> String
// Remove all subset characters from the end of the string.
func stringTrimRight(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.trimRight expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.trimRight expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.trimRight expects a String as subset")
	}

	object := args[0].(*StringType).Value
	subset := args[1].(*StringType).Value

	return &StringType{Value: strings.TrimRight(object, subset)}, nil
}

// String.replace(String, search String, replace String) -> String
// Replace a substring with another string.
func stringReplace(args ...DataType) (DataType, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("String.replace expects exactly 3 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.replace expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.replace expects a String as the search")
	}

	if args[2].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.replace expects a String as the replace")
	}

	object := args[0].(*StringType).Value
	search := args[1].(*StringType).Value
	replace := args[2].(*StringType).Value

	return &StringType{Value: strings.Replace(object, search, replace, -1)}, nil
}

// String.join(Array, glue String) -> String
// Join every element of the array with glue in a string.
func stringJoin(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.join expects exactly 2 arguments")
	}

	if args[0].Type() != ARRAY_TYPE {
		return nil, fmt.Errorf("String.join expects an Array")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.join expects a String as the glue")
	}

	object := args[0].(*ArrayType).Elements
	glue := args[1].(*StringType).Value
	result := []string{}

	for _, v := range object {
		if v.Type() == STRING_TYPE {
			value := v.(*StringType).Value
			result = append(result, value)
		} else {
			return nil, fmt.Errorf("String.join expects an Array of Strings")
		}
	}

	return &StringType{Value: strings.Join(result, glue)}, nil
}

// String.split(String, separator String) -> Array
// Split a string by the separator into an array.
func stringSplit(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.split expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.split expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.split expects a String as the separator")
	}

	object := args[0].(*StringType).Value
	sep := args[1].(*StringType).Value
	result := []DataType{}

	for _, v := range strings.Split(object, sep) {
		result = append(result, &StringType{Value: v})
	}

	return &ArrayType{Elements: result}, nil
}

// String.contains?(String, search String) -> Bool
// Check if a string has a substring.
func stringContains(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.contains? expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.contains? expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.contains? expects a String as search")
	}

	object := args[0].(*StringType).Value
	search := args[1].(*StringType).Value

	return &BooleanType{Value: strings.Contains(object, search)}, nil
}

// String.reverse(String) -> String
// Reverse the characters of a string.
func stringReverse(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.reverse expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.reverse expects a String")
	}

	object := args[0].(*StringType).Value
	// Implemented from:
	// https://github.com/golang/example/blob/master/stringutil/reverse.go
	rev := []rune(object)
	for i, j := 0, len(rev)-1; i < len(rev)/2; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}

	return &StringType{Value: string(rev)}, nil
}

// String.slice(String, start Integer, length Integer) -> String
// Take a "length" part of the string from "start".
func stringSlice(args ...DataType) (DataType, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("String.slice expects exactly 3 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.slice expects a String")
	}

	if args[1].Type() != INTEGER_TYPE {
		return nil, fmt.Errorf("String.slice expects an Integer as start")
	}

	if args[2].Type() != INTEGER_TYPE {
		return nil, fmt.Errorf("String.slice expects an Integer as length")
	}

	object := args[0].(*StringType).Value
	start := args[1].(*IntegerType).Value
	length := args[2].(*IntegerType).Value
	end := start + length

	if start < 0 || end > int64(len(object)) {
		return nil, fmt.Errorf("Length out of bounds")
	}

	return &StringType{Value: object[start:end]}, nil
}

// String.match(String, regex String) -> Boolean
// Matches the string with a regular expression.
func stringMatch(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.match expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.match expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.match expects a String regex")
	}

	object := args[0].(*StringType).Value
	match := args[1].(*StringType).Value

	regx, err := regexp.Compile(match)
	if err != nil {
		return nil, fmt.Errorf("Check the syntax of the regular expression")
	}

	return &BooleanType{Value: regx.Find([]byte(object)) != nil}, nil
}

// String.starts?(String, prefix String) -> Bool
// Check if a string starts with a prefix.
func stringStarts(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.starts? expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.starts? expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.starts? expects a String as prefix")
	}

	object := args[0].(*StringType).Value
	prefix := args[1].(*StringType).Value

	return &BooleanType{Value: strings.HasPrefix(object, prefix)}, nil
}

// String.ends?(String, suffix String) -> Bool
// Check if a string ends with a suffic.
func stringEnds(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.starts? expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.starts? expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.starts? expects a String as suffix")
	}

	object := args[0].(*StringType).Value
	suffix := args[1].(*StringType).Value

	return &BooleanType{Value: strings.HasSuffix(object, suffix)}, nil
}

// String.first(String) -> String
// First character of the string.
func stringFirst(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.first expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.first expects a String")
	}

	object := args[0].(*StringType).Value

	if len(object) > 0 {
		return &StringType{Value: string(object[0])}, nil
	}

	return &StringType{Value: ""}, nil
}

// String.last(String) -> String
// Last character of the string.
func stringLast(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.last expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.last expects a String")
	}

	object := args[0].(*StringType).Value

	if len(object) > 0 {
		return &StringType{Value: string(object[len(object) - 1])}, nil
	}

	return &StringType{Value: ""}, nil
}