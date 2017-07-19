[![GoDoc](https://godoc.org/github.com/fadion/aria?status.svg)](https://godoc.org/github.com/fadion/aria)
[![Go Report Card](https://goreportcard.com/badge/github.com/fadion/aria)](https://goreportcard.com/report/github.com/fadion/aria)

# Aria Language

Aria is an expressive, interpreted, toy language built as an exercise on designing and interpreting a programming language. It has a noiseless syntax, free of useless semi colons, braces or parantheses, and treats everything as an expression. Technically, it's built with a hand written lexer and parser, a recursive decent one (Pratt), and a tree-walk interpreter. I have never set any goals for it to be either fast, nor bulletproof, so don't expect neither of them.

## Usage

If you want to play with the language, but have no interest in toying with its code, you can download a built binary for your operating system. Just head to the [latest release](https://github.com/fadion/aria/releases/latest) and download one of the archives.

The other option, where you get to play with the code and run your changes, is to `go get github.com/fadion/aria` and install it as a local binary with `go install`. Obviously, you'll need `GOROOT` in your path, but I guess you already know what you're doing.

### Run a source file

To run an Aria source file, give it a path relative to the current directory.

```
aria run path/to/file.ari
```

### REPL

As any serious language, Aria provides a REPL too:

```
aria repl
```

## Basic Syntax

As you'd expect, there's a way to declaring variables:

```swift
let name = "John"
let age = 40
```

Once declared, a variable is locked to that value and can't be changed. You guessed it right, they're immutable! We could argue all day, but immutability advocates for safier code. It isn't that hard to pass a modified value to a new variable, isn't it?

Variables have to start with an alphabetic character and then continue either with alphanumeric, underscores, question mark or exclamation mark.

As anything is an expression, except for variable declaration, there are some pretty funny consequences. Everything can be passed to a variable as a value, even block statements like Ifs or Fors:

```swift
let old = if age > 40
  true
else
  false
end
```

Sometimes it's even nicer to inline the If completely, something you can do with almost every block expression. I'm not sure if that's actually readable for anyone, but it's an option:

```swift
let old = if age > 40 then true else false end
```

You've noticed there are no semi colons, braces or stuff like that? To me, it makes for code that's easier to read and scan. Don't confuse it with languages like Python however; in here, whitespace has absolutely no importance. Blocks of code are either inferred where they start, or delimited with keywords like `do`, `then` and `end`.

## Data Types

Aria supports 6 data types: `String`, `Integer`, `Float`, `Boolean`, `Array` and `Dictionary`.

### String

Strings are UTF-8 encoded, meaning that you can stuff in there anything, even emojis.

```swift
let weather = "Hot"
let code = "if\nthen\t\"yes\""
let price = "円500"
let concat = "Hello" + " " + "World"
let subscript = "aname"[2]
```

String concatenation is handled with the `+` operator but trying to concat a string with some other data type will result in a runtime error. Additionally, strings are treated as enumerables. They support subscript, iteration in `for in` loops and most of the array functions.

For the sake of it, there are some escape sequences too: \n, \t, \r, \a, \b, \f and \v. I'm sure you can figure out by yourself what every of them does.

### Integer & Float

Integers and Floats use mostly the same operators, with some minor differences. They can be used in the same expression, for example: 3 + 0.2, where the result is always cast to a Float.

Integers can be represented also as: binary with the 0b prefix, hexadecimal with the 0x prefix and octal with the 0o prefix. They'll be checked for validity at runtime.

```swift
let dec = 27
let oct = 0o33
let hex = 0x1B
let bin = 0b11011
let big = 27_000_000
let arch = 2 ** 32
let ratio = 1.61
let pi = 3.14_159_265
```

### Boolean

Just `true` or `false`, nothing else!

```swift
let mad = true
let genius = false
```

### Array

Arrays are ordered collections of any data types. You can mix and match strings with integers, or floats with other arrays.
 
 ```swift
 let multi = [5, "Hi", ["Hello", "World"]]
 let names = ["John", "Ben", 1337]
 let john = names[0]
 let concat = ["an", "array"] + ["and", "another"]
 let compare = [1, 2] == [1, 2]
 let nocomma = [5 7 9 "Hi"]
 ```
 
They support subscript with a 0-based index, combining with the `+` operator and comparison with `==` and `!=` that checks every element of both arrays for equality. Obviously, they're enumerables that can be used in `for in` loops and enumerable functions.
 
### Dictionary
 
Dictionaries are hashes with a forced string key and a value of any data type. Unlike arrays, internally their order is irrelevant, so you can't rely on index-based subscripting. They only support key-based subscripting.
 
```swift
let user = ["name": "John", "age": 40]
user["name"]
```

Just to be clear, keys should be string only. Other data types, at least for the moment, are not supported.

## Operators

You can't expect to run some calculations without a good batch of operators, right? Well, Aria has a good range of arithmetic, boolean and bitwise operators to match your needs.

By order of precedence:

```
Boolean: && || (AND, OR)
Bitwise: & | ~ (Bitwise AND, OR, NOT)
Equality: == != (Equal, Not equal)
Comparison: < <= > >=
Range: ..
Bitshift: << >> (Bitshift left and right)
Arithmetic: + - * / % ** (addition, substraction, multiplication, division, modulo, power)
```

Not all operators will work with any data type and I'm sure you don't expect that. I've touched on some of them for the special cases, like the `+` for string concatenation or array combining. I'm sure you'll figure them out.

## Functions

Functions in Aria are pure expressions that are passed to variables or as arguments to other functions. A function is essentially treated the same as an integer, string or any other expression.

```swift
let add = fn x, y
  x + y
end
```

I've omitted the parantheses too! Of course, you can write the function as `fn (x, y)`, but where's the beauty in that? Calling the function needs the parantheses though:

```swift
let sum = add(1335, 2)
```

Notice the lack of a `return` statement. I doubt you'll ever need it, but if you do, it is there. Let's see a stupid example that also inlines the function.

```swift
let mad_genius? = fn mad, genius do return mad && genius end
mad_genius?(true, false)
```

Yes, you can inline functions just by adding a `do` keyword. It's up to you how readable that is.

Finally, there's the self-executing function syntax for all of you Javascripters:

```swift
let pow_2 = fn x
  x ** 2
end(2)
```

## Conditionals

Aria provides two types of conditional expressions: 1) An `if/else` that doesn't support multiple `else if` statements and that's good for simple checks, and 2) A `switch` for anything else. Every block of conditional code has it's own scope, like any other block in Aria; meaning that it can access the previously declared variables, but anything declared inside of them doesn't persist to the rest of the code.

An `if` is pretty simple:

```swift
if 1 == 1
  IO.puts("YES!")
end
```

With the ever present `else` block:

```swift
if 1 == 2
  IO.puts("Not calling me.")
else
  IO.puts("1 isn't equal to 2. Duh!")
end
```

`Switch` expressions on the other hand are more interesting. They can have multiple cases with multiple conditions that break automatically on each successful case. When was the last time you didn't need to break? Exactly!

```swift
let a = 5
switch a
case 2, 3
  IO.puts("Is it 2 or 3?")
case 5
  IO.puts("It is 5. Magic!")
default
  IO.puts("No idea, sorry.")
end
```

Not only that, but a `switch` can behave as a typical if/else when no control condition is provided. It basically becomes a `switch true`.

```swift
let a = "John"
switch
case a == "John"
  IO.puts("John")
case a == "Ben"
  IO.puts("Ben") 
default
  IO.puts("Nobody")
end
```

## For Loop

There's an abundance of `for` loop variations around so Aria takes the short way: a single `for in` loop that's useful to iterate arrays, strings or dictionaries, but that does nothing else.

```swift
for v in [1, 2, 3, 4]
  IO.puts(v)
end
```

Obviously, the result of the loop can be pass to a variable, and that's what makes them interesting to manipulate enumerables.

```swift
let plus_one = for v in [1, 2, 3, 4]
  v + 1
end
```

Passing two arguments for arrays or strings will return the current index and value. For dictionaries, the first argument will be the key.

```swift
for i, v in "abcd"
  IO.puts(i + ":" + v)
end
```

```swift
for k, v in ["name": "John", "age": 40]
  IO.puts(k)
  IO.puts(v)
end
```

## Range Operator

The range operator is a special type of sugar to generate an array of integers or strings. Without a flexible `for` loop, it surely comes in handy.

```swift
let numbers = 0..9
let huge = 999..100
let alphabet = "a".."z"
```

More interesting is using them in a `for in` loop:

```swift
for v in 10..20
  IO.puts(v)
end
```

## Modules

Modules are very simple containers of data and nothing more. They're not an imitation of classes, as they can't be initialized, don't have any type of access control, inheritance or whatever. If you need to think in Object Oriented terms, they're like a class with only static properties and methods. They're good to give some structure to a program, but not to represent cars, trees and cats.

```swift
module Color
  let white = "#fff"
  let grey = "#666"
  let hexToRGB = fn hex
    // some calculations
  end
end

let background = Color.white
let font_color = Color.hexToRGB(Color.grey)
```

There can't be any other statement in modules except `let`, but those variables can have any expression possible. The dot syntax of calling a module property or function may remind you of classes, but still, they're not!

Keep in mind that the Aria interpreter is single pass and as such, it will only recognize calls to a module that has already been declared. 

## Comments

Nothing fancy in here! You can comment your code using both inline or block comments:

```
// an inline comment
/*
  I'm spanning multiple
  lines.
*/
```

## Standard Library

Not sure if we can call it that, as it's just a few functions I quickly rolled out to have something to work with. I expect to increase the functionality in the future.

### IO

```
IO.puts(Any)
IO.write(Any)
```

### Type

```
Type.of(Any) -> String
Type.toString(Any) -> String
Type.toInt(Any) -> Integer
Type.toFloat(Any) -> Float
Type.toArray(Any) -> Array
```

### String

```
String.count(String) -> Integer
String.countBytes(String) -> Integer
String.lower(String) -> String
String.upper(String) -> String
String.capitalize(String) -> String
String.trim(String, subset String) -> String
String.trimLeft(String, subset String) -> String
String.trimRight(String, subset String) -> String
String.reverse(String) -> String 
String.replace(String, search String, replace String) -> String
String.slice(String, start Integer, length Integer) -> String 
String.join(Array, glue String) -> String
String.split(String, separator String) -> Array
String.first(String) -> String
String.last(String) -> String
String.contains?(String, search String) -> Boolean 
String.starts?(String, prefix String) -> Boolean 
String.ends?(String, suffix String) -> Boolean 
String.match?(String, regex String) -> Boolean 
```

### Enum

```
Enum.size(Array|String) -> Integer
Enum.reverse(Array|String) -> Array
Enum.first(Array|String) -> Any
Enum.last(Array|String) -> Any
Enum.insert(Array, value Any) -> Array
Enum.delete(Array, index Integer) -> Array
Enum.map(Array|String, fn Function) -> Array
Enum.filter(Array|String, fn Function) -> Array
```

### Dict

```
Dict.size(Dictionary) -> Integer
Dict.has(Dictionary, key String) -> Boolean
Dict.insert(Dictionary, key String, value Any) -> Dictionary
Dict.delete(Dictionary, key String) -> Dictionary
```

### Math

```
Math.pi() -> Float
Math.ceil(Float) -> Integer
Math.floor(Float) -> Integer
Math.max(Float|Integer, Float|Integer) -> Float|Integer
Math.min(Float|Integer, Float|Integer) -> Float|Integer
```

## Future Plans

Although this is a language made purely for fun and experimentation, it doesn't mean I will abandon it in it's first release. Adding other features means I'll learn even more!

In the near future, hopefully, I plan to:

- Improve the Standard Library with more functions.
- Write more tests!
- Write some useful benchmarks with non-trivial programs.
- Find a way to support closures and recursion.
- Support optional values for null returns. Although right now there's only a few things that can return null and immutability makes it far more sane, I would still be a nice thing to have.
- Fixing bugs, which I'm sure there are plenty.

## Credits

Aria was developed by Fadion Dashi, a freelance web and mobile developer from Tirana.

The implementation is based on the fantastic [Writing an Interpreter in Go](https://interpreterbook.com/). If you're even vaguely interested in interpreters, with Golang or not, I highly suggest that book.

The `reader.Buffer` package is a "fork" of Golang's `bytes.Buffer`, in which I had to add a method that reads a rune without moving the internal cursor. I hate doing that, but unfortunately couldn't find a way out of it. That package has its own BSD-style [license](https://github.com/golang/go/blob/master/LICENSE).