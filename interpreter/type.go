package interpreter

import (
	"fmt"
	"strconv"
)

// Type.of(Any) -> String
// Get the type of a value.
func typeOf(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Type.of expects exactly 1 argument")
	}

	return &StringType{Value: args[0].Type()}, nil
}

// Type.toString(Any) -> String
// Convert a value to string.
func typeToString(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Type.toString expects exactly 1 argument")
	}

	switch object := args[0].(type) {
	case *IntegerType:
		return &StringType{Value: fmt.Sprintf("%d", object.Value)}, nil
	case *FloatType:
		return &StringType{Value: fmt.Sprintf("%f", object.Value)}, nil
	case *BooleanType:
		return &StringType{Value: fmt.Sprintf("%t", object.Value)}, nil
	case *StringType:
		return object, nil
	default:
		return nil, fmt.Errorf("Type.toString can't convert '%s' to String", object.Type())
	}
}

// Type.toInt(Any) -> Integer
// Convert a value to integer.
func typeToInt(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Type.toInt expects exactly 1 argument")
	}

	switch object := args[0].(type) {
	case *StringType:
		i, err := strconv.Atoi(object.Value)
		if err != nil {
			return nil, fmt.Errorf("Type.toInt can't convert '%s' to Integer", object.Value)
		}
		return &IntegerType{Value: int64(i)}, nil
	case *FloatType:
		return &IntegerType{Value: int64(object.Value)}, nil
	case *BooleanType:
		result := 0
		if object.Value {
			result = 1
		}
		return &IntegerType{Value: int64(result)}, nil
	case *IntegerType:
		return object, nil
	default:
		return nil, fmt.Errorf("Type.toInt can't convert '%s' to Integer", object.Type())
	}
}

// Type.toFloat(Any) -> Float
// Convert a value to float.
func typeToFloat(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Type.toFloat expects exactly 1 argument")
	}

	switch object := args[0].(type) {
	case *StringType:
		i, err := strconv.ParseFloat(object.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("Type.toFloat can't convert '%s' to Integer", object.Value)
		}
		return &FloatType{Value: i}, nil
	case *IntegerType:
		return &FloatType{Value: float64(object.Value)}, nil
	case *BooleanType:
		result := 0
		if object.Value {
			result = 1
		}
		return &FloatType{Value: float64(result)}, nil
	case *FloatType:
		return &FloatType{Value: object.Value}, nil
	default:
		return nil, fmt.Errorf("Type.toFloat can't convert '%s' to Integer", object.Type())
	}
}

// Type.toArray(Any) -> String
// Convert a value to string.
func typeToArray(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Type.toArray expects exactly 1 argument")
	}

	switch object := args[0].(type) {
	case *StringType:
		result := []DataType{}
		for _, k := range object.Value {
			result = append(result, &StringType{Value: string(k)})
		}

		return &ArrayType{Elements: result}, nil
	case *IntegerType, *FloatType:
		return &ArrayType{Elements: []DataType{object}}, nil
	default:
		return nil, fmt.Errorf("Type.toArray can't convert '%s' to Array", object.Type())
	}
}