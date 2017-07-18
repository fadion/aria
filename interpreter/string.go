package interpreter

import (
	"fmt"
	"strings"
)

// String.count(string) -> Integer
// Count the number of characters in a string.
func stringCount(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("String.count expects exactly 1 argument")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.count expects a String")
	}

	object := args[0].(*StringType).Value
	return &IntegerType{Value: int64(len(object))}, nil
}

// String.lower(string) -> String
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

// String.upper(string) -> String
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

// String.capitalize(string) -> String
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

// String.trim(string, subset [string]) -> String
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

// String.replace(string, search [string], replace [string]) -> String
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

// String.join(array, glue [string]) -> String
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

// String.split(string, separator) -> Array
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

// String.has(string, search [string]) -> Bool
// Check if a string has a substring.
func stringHas(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("String.has expects exactly 2 arguments")
	}

	if args[0].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.has expects a String")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("String.has expects a String as search")
	}

	object := args[0].(*StringType).Value
	search := args[1].(*StringType).Value

	return &BooleanType{Value: strings.Contains(object, search)}, nil
}
