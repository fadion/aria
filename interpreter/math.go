package interpreter

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Math.pi() -> Float
// Pi constant.
func mathPi(args ...DataType) (DataType, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("Math.pi doesn't expect arguments")
	}

	return &FloatType{Value: math.Pi}, nil
}

// Math.ceil(Float) -> Integer
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

// Math.floor(Float) -> Integer
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

// Math.max(Float | Integer, Float | Integer) -> Float | Integer
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

// Math.min(Float | Integer, Float | Integer) -> Float | Integer
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

// Math.random(min Integer, max Integer) -> Float | Integer
// Generate a random number between min and max.
func mathRandom(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Math.random expects exactly 2 arguments")
	}

	if args[0].Type() != INTEGER_TYPE || args[1].Type() != INTEGER_TYPE {
		return nil, fmt.Errorf("Math.random expects min and max as Integers")
	}

	min := int(args[0].(*IntegerType).Value)
	max := int(args[1].(*IntegerType).Value)

	if max < min {
		return nil, fmt.Errorf("Max should be higher than min")
	}

	rand.Seed(time.Now().UnixNano())
	random := rand.Intn(max - min) + min

	return &IntegerType{Value: int64(random)}, nil
}