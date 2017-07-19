package interpreter

import (
	"fmt"
)

// Dict.size(Dictionary) -> Integer
// Size of the dictionary.
func dictSize(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Dict.size expects exactly 1 argument")
	}

	if args[0].Type() != DICTIONARY_TYPE {
		return nil, fmt.Errorf("Dict.size expects a Dictionary")
	}

	object := args[0].(*DictionaryType)
	return &IntegerType{Value: int64(len(object.Pairs))}, nil
}

// Dict.has(Dictionary, key String) -> Boolean
// Check if the dictionary has a key.
func dictContains(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Dict.has expects exactly 2 arguments")
	}

	if args[0].Type() != DICTIONARY_TYPE {
		return nil, fmt.Errorf("Dict.has expects a Dictionary")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("Dict.has expects a String key")
	}

	object := args[0].(*DictionaryType)
	key := args[1].(*StringType)
	found := false

	for k := range object.Pairs {
		if k.Value == key.Value {
			found = true
		}
	}

	return &BooleanType{Value: found}, nil
}

// Dict.insert(Dictionary, key String, value Any) -> Dictionary
// Insert a key:value in the dictionary.
func dictInsert(args ...DataType) (DataType, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("Dict.size expects exactly 3 arguments")
	}

	if args[0].Type() != DICTIONARY_TYPE {
		return nil, fmt.Errorf("Dict.size expects a Dictionary")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("Dict.size expects a String key")
	}

	object := args[0].(*DictionaryType)
	key := args[1].(*StringType)
	object.Pairs[key] = args[2]

	return &DictionaryType{Pairs: object.Pairs}, nil
}

// Dict.delete(Dictionary, key String) -> Dictionary
// Delete a key from the dictionary.
func dictDelete(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Dict.size expects exactly 2 arguments")
	}

	if args[0].Type() != DICTIONARY_TYPE {
		return nil, fmt.Errorf("Dict.size expects a Dictionary")
	}

	if args[1].Type() != STRING_TYPE {
		return nil, fmt.Errorf("Dict.size expects a String key")
	}

	object := args[0].(*DictionaryType)
	key := args[1].(*StringType)
	for k := range object.Pairs {
		if k.Value == key.Value {
			delete(object.Pairs, k)
		}
	}

	return &DictionaryType{Pairs: object.Pairs}, nil
}

// Dict.empty?(Dictionary) -> Boolean
// Check if the dictionary is empty.
func dictEmpty(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Dict.empty expects exactly 1 argument")
	}

	if args[0].Type() != DICTIONARY_TYPE {
		return nil, fmt.Errorf("Dict.empty expects a Dictionary")
	}

	object := args[0].(*DictionaryType)

	return &BooleanType{Value: len(object.Pairs) == 0}, nil
}
