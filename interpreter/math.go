package interpreter

import (
	"fmt"
	"math"
)

// Math.pi() -> Float
// Pi constant.
func mathPi(args ...DataType) (DataType, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("Math.pi doesn't expect arguments")
	}

	return &FloatType{Value: math.Pi}, nil
}

// Math.ceil(float) -> Integer
// Round up the float.
func mathCeil(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.ceil expects exactly 1 argument")
	}
	if args[0].Type() != FLOAT_TYPE {
		return nil, fmt.Errorf("Math.ceil expects a Float")
	}

	arg1 := args[0].(*FloatType).Value

	return &IntegerType{Value: int64(math.Ceil(arg1))}, nil
}

// Math.floor(float) -> Integer
// Round down the float.
func mathFloor(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.floor expects exactly 1 argument")
	}
	if args[0].Type() != FLOAT_TYPE {
		return nil, fmt.Errorf("Math.floor expects a Float")
	}

	arg1 := args[0].(*FloatType).Value

	return &IntegerType{Value: int64(math.Floor(arg1))}, nil
}

// Math.max(float | integer, float | integer) -> Float | Integer
// Get the biggest value between two floats or integers.
func mathMax(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Math.max expects exactly 2 arguments")
	}

	switch {
	case args[0].Type() == INTEGER_TYPE && args[1].Type() == INTEGER_TYPE:
		arg1 := args[0].(*IntegerType).Value
		arg2 := args[1].(*IntegerType).Value
		result := arg1

		if arg2 > arg1 {
			result = arg2
		}

		return &IntegerType{Value: result}, nil
	case args[0].Type() == FLOAT_TYPE && args[1].Type() == FLOAT_TYPE:
		arg1 := args[0].(*FloatType).Value
		arg2 := args[1].(*FloatType).Value
		result := arg1

		if arg2 > arg1 {
			result = arg2
		}

		return &FloatType{Value: result}, nil
	default:
		return nil, fmt.Errorf("Math.max can't compare '%s' with '%s'", args[0].Type(), args[1].Type())
	}
}

// Math.min(float | integer, float | integer) -> Float | Integer
// Get the smallest value between two floats or integers.
func mathMin(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Math.min expects exactly 2 arguments")
	}

	switch {
	case args[0].Type() == INTEGER_TYPE && args[1].Type() == INTEGER_TYPE:
		arg1 := args[0].(*IntegerType).Value
		arg2 := args[1].(*IntegerType).Value
		result := arg1

		if arg2 < arg1 {
			result = arg2
		}

		return &IntegerType{Value: result}, nil
	case args[0].Type() == FLOAT_TYPE && args[1].Type() == FLOAT_TYPE:
		arg1 := args[0].(*FloatType).Value
		arg2 := args[1].(*FloatType).Value
		result := arg1

		if arg2 < arg1 {
			result = arg2
		}

		return &FloatType{Value: result}, nil
	default:
		return nil, fmt.Errorf("Math.min can't compare '%s' with '%s'", args[0].Type(), args[1].Type())
	}
}
