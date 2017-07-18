package interpreter

import (
	"fmt"
)

// Enum.size(array) -> Integer
// Size of the array.
func enumSize(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Enum.size expects exactly 1 argument")
	}

	switch object := args[0].(type) {
	case *ArrayType:
		return &IntegerType{Value: int64(len(object.Elements))}, nil
	case *StringType:
		return &IntegerType{Value: int64(len(object.Value))}, nil
	default:
		return nil, fmt.Errorf("Enum.size expects an Array or String")
	}
}

// Enum.reverse(array) -> Array
// Reverse the elements of the array.
func enumReverse(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Enum.reverse expects exactly 1 argument")
	}

	switch object := args[0].(type) {
	case *ArrayType:
		result := []DataType{}
		for i := len(object.Elements) - 1; i >= 0; i-- {
			result = append(result, object.Elements[i])
		}
		return &ArrayType{Elements: result}, nil
	case *StringType:
		result := []DataType{}
		for i := len(object.Value) - 1; i >= 0; i-- {
			result = append(result, &StringType{Value: string(object.Value[i])})
		}
		return &ArrayType{Elements: result}, nil
	default:
		return nil, fmt.Errorf("Enum.reverse expects an Array or String")
	}
}

// Enum.first(array) -> any
// Get the first element of the array.
func enumFirst(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Enum.first expects exactly 1 argument")
	}

	switch object := args[0].(type) {
	case *ArrayType:
		if len(object.Elements) == 0 {
			return nil, fmt.Errorf("Enum.first expects a non-empty array or string")
		}
		return object.Elements[0], nil
	case *StringType:
		if len(object.Value) == 0 {
			return nil, fmt.Errorf("Enum.first expects a non-empty array or string")
		}
		return &StringType{Value: string(object.Value[0])}, nil
	default:
		return nil, fmt.Errorf("Enum.first expects an Array or String")
	}
}

// Enum.last(array) -> any
// Get the last element of the array.
func enumLast(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Enum.last expects exactly 1 argument")
	}

	switch object := args[0].(type) {
	case *ArrayType:
		if len(object.Elements) == 0 {
			return nil, fmt.Errorf("Enum.last expects a non-empty array or string")
		}
		return object.Elements[len(object.Elements)-1], nil
	case *StringType:
		if len(object.Value) == 0 {
			return nil, fmt.Errorf("Enum.last expects a non-empty array or string")
		}
		return &StringType{Value: string(object.Value[len(object.Value)-1])}, nil
	default:
		return nil, fmt.Errorf("Enum.last expects an Array or String")
	}
}

// Enum.insert(array, element [any]) -> Array
// Insert an element at the end of the array.
func enumInsert(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Enum.insert expects exactly 2 arguments")
	}

	switch object := args[0].(type) {
	case *ArrayType:
		elem := append(object.Elements, args[1])
		return &ArrayType{Elements: elem}, nil
	default:
		return nil, fmt.Errorf("Enum.insert expects an Array")
	}
}

// Enum.delete(array, index [integer]) -> Array
// Delete an element from the array.
func enumDelete(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Enum.delete expects exactly 2 arguments")
	}

	if args[1].Type() != INTEGER_TYPE {
		return nil, fmt.Errorf("Enum.delete expects an Integer index")
	}

	idx := args[1].(*IntegerType).Value

	switch object := args[0].(type) {
	case *ArrayType:
		if int(idx) > len(object.Elements)-1 || idx < 0 {
			return nil, fmt.Errorf("Index supplied to Enum.delete doesn't exist in the Array")
		}

		elem := append(object.Elements[:idx], object.Elements[idx+1:]...)
		return &ArrayType{Elements: elem}, nil
	default:
		return nil, fmt.Errorf("Enum.delete expects an Array")
	}
}
