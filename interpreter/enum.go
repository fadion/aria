package interpreter

import (
	"fmt"
	"math/rand"
	"time"
)

// Enum.size(Array) -> Integer
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

// Enum.reverse(Array) -> Array
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

// Enum.first(Array) -> Any
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

// Enum.last(Array) -> Any
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

// Enum.insert(Arramy, element Any) -> Array
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

// Enum.delete(Array, index Integer) -> Array
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

// Enum.map(Array, fn Function(element)) -> Array
// Map a function to every element of the array.
func enumMap(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Enum.map expects exactly 2 arguments")
	}

	if args[1].Type() != FUNCTION_TYPE {
		return nil, fmt.Errorf("Enum.map expects a Function")
	}

	object := []DataType{}

	switch obj := args[0].(type) {
	case *ArrayType:
		object = obj.Elements
	case *StringType:
		for _, v := range obj.Value {
			object = append(object, &StringType{Value: string(v)})
		}
	default:
		return nil, fmt.Errorf("Enum.map expects an Array or String")
	}

	function := args[1].(*FunctionType)

	if len(function.Parameters) != 1 {
		return nil, fmt.Errorf("Enum.map expects a function with exactly 1 parameter")
	}

	runner := New()
	array := []DataType{}
	for _, v := range object {
		function.Scope.Write(function.Parameters[0].Value, v)
		result := runner.Interpret(function.Body, function.Scope)

		if result == nil {
			return nil, nil
		}

		array = append(array, result)
	}

	return &ArrayType{Elements: array}, nil
}

// Enum.filter(Array, fn Function(element)) -> Array
// Filter with a function every element of the array.
func enumFilter(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Enum.filter expects exactly 2 arguments")
	}

	if args[1].Type() != FUNCTION_TYPE {
		return nil, fmt.Errorf("Enum.filter expects a Function")
	}

	object := []DataType{}

	switch obj := args[0].(type) {
	case *ArrayType:
		object = obj.Elements
	case *StringType:
		for _, v := range obj.Value {
			object = append(object, &StringType{Value: string(v)})
		}
	default:
		return nil, fmt.Errorf("Enum.filter expects an Array or String")
	}

	function := args[1].(*FunctionType)

	if len(function.Parameters) != 1 {
		return nil, fmt.Errorf("Enum.filter expects a function with exactly 1 parameter")
	}

	runner := New()
	array := []DataType{}
	for _, v := range object {
		function.Scope.Write(function.Parameters[0].Value, v)
		result := runner.Interpret(function.Body, function.Scope)

		if result == nil {
			return nil, nil
		}

		if result.Type() == BOOLEAN_TYPE {
			filter := result.(*BooleanType).Value
			if filter {
				array = append(array, v)
			}
		}
	}

	return &ArrayType{Elements: array}, nil
}

// Enum.empty(Array) -> Boolean
// Check if the array is empty.
func enumEmpty(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Enum.empty expects exactly 1 argument")
	}

	isempty := true

	switch object := args[0].(type) {
	case *ArrayType:
		isempty = len(object.Elements) == 0
	case *StringType:
		isempty = len(object.Value) == 0
	default:
		return nil, fmt.Errorf("Enum.empty expects an Array or String")
	}

	return &BooleanType{Value: isempty}, nil
}

// Enum.random(Array) -> Any
// Get a random element from the array.
func enumRandom(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Enum.random expects exactly 1 argument")
	}

	elements := []DataType{}

	switch object := args[0].(type) {
	case *ArrayType:
		elements = object.Elements
	case *StringType:
		for _, v := range object.Value {
			elements = append(elements, &StringType{Value: string(v)})
		}
	default:
		return nil, fmt.Errorf("Enum.random expects an Array or String")
	}

	if len(elements) == 0 {
		return nil, fmt.Errorf("Random value can't be retrieved from an empty enumerable")
	}

	rand.Seed(time.Now().UnixNano())
	random := rand.Intn(len(elements))

	return elements[random], nil
}

// Enum.find(Array, fn Function(element)) -> Array
// Get the first value the function returns true.
func enumFind(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Enum.find expects exactly 2 arguments")
	}

	if args[1].Type() != FUNCTION_TYPE {
		return nil, fmt.Errorf("Enum.find expects a Function")
	}

	object := []DataType{}

	switch obj := args[0].(type) {
	case *ArrayType:
		object = obj.Elements
	case *StringType:
		for _, v := range obj.Value {
			object = append(object, &StringType{Value: string(v)})
		}
	default:
		return nil, fmt.Errorf("Enum.find expects an Array or String")
	}

	function := args[1].(*FunctionType)

	if len(function.Parameters) != 1 {
		return nil, fmt.Errorf("Enum.find expects a function with exactly 1 parameter")
	}

	runner := New()
	for _, v := range object {
		function.Scope.Write(function.Parameters[0].Value, v)
		result := runner.Interpret(function.Body, function.Scope)

		if result == nil {
			return nil, nil
		}

		if result.Type() == BOOLEAN_TYPE {
			found := result.(*BooleanType).Value
			if found {
				return v, nil
			}
		}
	}

	return NIL, nil
}

// Enum.contains?(Array, search Any) -> Boolean
// Check if the array contains the search.
func enumContains(args ...DataType) (DataType, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Enum.contains? expects exactly 2 arguments")
	}

	object := []DataType{}
	search := args[1]

	switch obj := args[0].(type) {
	case *ArrayType:
		object = obj.Elements
	case *StringType:
		for _, v := range obj.Value {
			object = append(object, &StringType{Value: string(v)})
		}
	default:
		return nil, fmt.Errorf("Enum.contains? expects an Array or String")
	}

	for _, v := range object {
		if v.Type() == search.Type() && v.Inspect() == search.Inspect() {
			return &BooleanType{Value: true}, nil
		}
	}

	return &BooleanType{Value: false}, nil
}

// Enum.unique(Array) -> Array
// Remove duplicates from the array.
func enumUnique(args ...DataType) (DataType, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Enum.unique expects exactly 1 argument")
	}

	object := []DataType{}

	switch obj := args[0].(type) {
	case *ArrayType:
		object = obj.Elements
	case *StringType:
		for _, v := range obj.Value {
			object = append(object, &StringType{Value: string(v)})
		}
	default:
		return nil, fmt.Errorf("Enum.unique expects an Array or String")
	}

	unique := []DataType{}
	found := map[string]DataType{}
	for _, v := range object {
		if _, ok := found[v.Inspect()]; !ok {
			unique = append(unique, v)
			found[v.Inspect()] = v
		}
	}

	return &ArrayType{Elements: unique}, nil
}

// Enum.reduce(Array, start Any, fn Function(element, accumulator)) -> Any
// Run the function on each iteration and pass the result to the accumulator.
func enumReduce(args ...DataType) (DataType, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("Enum.reduce expects exactly 3 arguments")
	}

	if args[2].Type() != FUNCTION_TYPE {
		return nil, fmt.Errorf("Enum.reduce expects a Function")
	}

	object := []DataType{}

	switch obj := args[0].(type) {
	case *ArrayType:
		object = obj.Elements
	case *StringType:
		for _, v := range obj.Value {
			object = append(object, &StringType{Value: string(v)})
		}
	default:
		return nil, fmt.Errorf("Enum.reduce expects an Array or String")
	}

	accumulator := args[1]
	function := args[2].(*FunctionType)

	if len(function.Parameters) != 2 {
		return nil, fmt.Errorf("Enum.reduce expects a function with exactly 2 parameters")
	}

	runner := New()
	for _, v := range object {
		function.Scope.Write(function.Parameters[0].Value, v)
		function.Scope.Write(function.Parameters[1].Value, accumulator)
		result := runner.Interpret(function.Body, function.Scope)

		if result == nil {
			return nil, nil
		}

		accumulator = result
	}

	return accumulator, nil
}