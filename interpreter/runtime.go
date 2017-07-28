package interpreter

import (
	"fmt"
	"time"
	"math/rand"
	"strconv"
	"strings"
	"regexp"
)

type runtimeFunc func(args ...DataType) (DataType, error)

var runtime = map[string]runtimeFunc{

	// println(Any)
	"println": func(args ...DataType) (DataType, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("println() expects exactly 1 argument")
		}

		fmt.Println(args[0].Inspect())

		// Return a dummy string just to suppress errors,
		// as there's nothing to return.
		return &StringType{Value: ""}, nil
	},

	// print(Any)
	"print": func(args ...DataType) (DataType, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("print() expects exactly 1 argument")
		}

		fmt.Print(args[0].Inspect())

		// Return a dummy string just to suppress errors,
		// as there's nothing to return.
		return &StringType{Value: ""}, nil
	},

	// panic(Any)
	"panic": func(args ...DataType) (DataType, error) {
		var message string
		if len(args) > 0 {
			message = args[0].Inspect()
		}

		return nil, fmt.Errorf(message)
	},

	// typeof(Any) -> Any
	"typeof": func(args ...DataType) (DataType, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("typeof() expects exactly 1 argument")
		}

		return &StringType{Value: args[0].Type()}, nil
	},

	// String(Any) -> String
	"String": func(args ...DataType) (DataType, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("String() expects exactly 1 argument")
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
			return nil, fmt.Errorf("String() can't convert '%s' to String", object.Type())
		}
	},

	// Int(Any) -> Integer
	"Int": func(args ...DataType) (DataType, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("Int() expects exactly 1 argument")
		}

		switch object := args[0].(type) {
		case *StringType:
			i, err := strconv.Atoi(object.Value)
			if err != nil {
				return nil, fmt.Errorf("Int() can't convert '%s' to Integer", object.Value)
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
			return nil, fmt.Errorf("Int() can't convert '%s' to Integer", object.Type())

		}
	},

	// Float(Any) -> Float
	"Float": func(args ...DataType) (DataType, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("Float() expects exactly 1 argument")
		}

		switch object := args[0].(type) {
		case *StringType:
			i, err := strconv.ParseFloat(object.Value, 64)
			if err != nil {
				return nil, fmt.Errorf("Float() can't convert '%s' to Integer", object.Value)
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
			return nil, fmt.Errorf("Float() can't convert '%s' to Integer", object.Type())
		}
	},

	// runtime_rand(min Integer, max Integer) -> Integer
	"runtime_rand": func(args ...DataType) (DataType, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("runtime_rand() expects exactly 2 arguments")
		}

		if args[0].Type() != INTEGER_TYPE || args[1].Type() != INTEGER_TYPE {
			return nil, fmt.Errorf("runtime_rand() expects min and max as Integers")
		}

		min := int(args[0].(*IntegerType).Value)
		max := int(args[1].(*IntegerType).Value)

		if max < min {
			return nil, fmt.Errorf("runtime_rand() expects max higher than min")
		}

		rand.Seed(time.Now().UnixNano())
		random := rand.Intn(max - min) + min

		return &IntegerType{Value: int64(random)}, nil
	},

	// runtime_tolower(String)
	"runtime_tolower": func(args ...DataType) (DataType, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("runtime_tolower() expects exactly 1 argument")
		}

		if args[0].Type() != STRING_TYPE {
			return nil, fmt.Errorf("runtime_tolower() expects a String")
		}

		str := args[0].(*StringType).Value

		return &StringType{Value: strings.ToLower(str)}, nil
	},

	// runtime_toupper(String)
	"runtime_toupper": func(args ...DataType) (DataType, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("runtime_toupper() expects exactly 1 argument")
		}

		if args[0].Type() != STRING_TYPE {
			return nil, fmt.Errorf("runtime_toupper() expects a String")
		}

		str := args[0].(*StringType).Value

		return &StringType{Value: strings.ToUpper(str)}, nil
	},

	// runtime_regex_match(String, regex String) -> Bool
	"runtime_regex_match": func(args ...DataType) (DataType, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("runtime_regex_match() expects exactly 2 arguments")
		}

		if args[0].Type() != STRING_TYPE {
			return nil, fmt.Errorf("runtime_regex_match() expects a String")
		}

		if args[1].Type() != STRING_TYPE {
			return nil, fmt.Errorf("runtime_regex_match() expects a String regex")
		}

		object := args[0].(*StringType).Value
		match := args[1].(*StringType).Value

		regx, err := regexp.Compile(match)
		if err != nil {
			return nil, fmt.Errorf("runtime_regex_match() couldn't compile the regular expression")
		}

		return &BooleanType{Value: regx.Find([]byte(object)) != nil}, nil
	},

}