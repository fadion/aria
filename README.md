[![GoDoc](https://godoc.org/github.com/fadion/aria?status.svg)](https://godoc.org/github.com/fadion/aria)
[![Go Report Card](https://goreportcard.com/badge/github.com/fadion/aria)](https://goreportcard.com/report/github.com/fadion/aria)

# Aria Language

Aria is an expressive, interpreted, toy language built as an exercise on designing and interpreting a programming language. It has a noiseless syntax, free of useless semi colons, braces or parantheses, and treats everything as an expression. Technically, it's built with a hand written lexer and parser, a recursive decent one (Pratt), and a tree-walk interpreter. I have never set any goals for it to be either fast, nor bulletproof, so don't expect neither of them.

If features mutable and immutable values, if and switch conditionals, functions, for loops, modules, the pipe operator, imports and many more. More importantly, it's getting expanded frequently with new features, more functions for the standard library and bug fixes. All of that while retaining it's expressiveness, clean syntax and easy of use.

```swift
var name = "aria language"
let expressive? = fn x
  if x != ""
    return "expressive " + x
  end
  "sorry, what?"
end

let pipe = name |> expressive?() |> String.capitalize()
IO.puts(pipe) // "Expressive Aria Language"
```

## Table of Contents

* [Usage](#usage)
    * [Run a Source File](#run-a-source-file)
    * [REPL](#repl)
* [Variables](#variables)
* [Data Types](#data-types)
    * [String](#string)
    * [Atom](#atom)
    * [Integer](#integer)
    * [Float](#float)
    * [Boolean](#boolean)
    * [Array](#array)
    * [Dictionary](#dictionary)
    * [Nil](#nil)
* [Operators](#operators)
* [Functions](#functions)
    * [Return Statement](#return-statement)
    * [Closures](#closures)
    * [Recursion](#recursion)
    * [Arrow Functions](#arrow-functions)
    * [Tricks](#tricks)
* [Conditionals](#conditionals)
    * [If](#if)
    * [Ternary Operator](#ternary-operator)
    * [Switch](#switch)
    * [Pattern Matching](#pattern-matching)
* [For Loop](#foor-loop)
* [Range Operator](#range-operator)
* [Pipe Operator](#pipe-operator)
* [Immutability](#immutability)
* [Modules](#modules)
* [Imports](#imports)
* [Comments](#comments)
* [Standard Library](#standard-library)

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

## Variables

Variable declaration in Aria comes in two flavours: `let` and `var`, respectively for immutable and mutable values. The language and the [standard library](https://github.com/fadion/aria/wiki/Standard-Library) will help you enough to aim for immutability, something explained in more detail later on. However, there are occassions where a mutable value is unavoidable, so you have that option too.

Immutable values aren't just for basic data types like Integers or String, but for data structures too. Elements of an array or dictionary, once declared with a `let`, can't be changed, added or deleted.

```swift
let name = "John"
let married = false
var age = 40
```

Variable names have to start with an alphabetic character and continue either with alphanumeric, underscores, questions marks or exclamation marks. When you see a question mark, don't confuse them with optionals like in some other languages. In here they have no special lexical meaning except that they allow for some nice variable names like `is_empty?` or `do_it!`.

## Data Types

Aria supports 7 data types: `String`, `Atom`, `Integer`, `Float`, `Boolean`, `Array` and `Dictionary`.

### String

Strings are UTF-8 encoded, meaning that you can stuff in there anything, even emojis.

```swift
let weather = "Hot"
let price = "円500"
```

String concatenation is handled with the `+` operator. Concats between a string and another data type will resut in a runtime error. You'll have to use the `Type` library module to convert a data type to string.

```swift
let name = "Tony" + " " + "Stark" 
```

Additionally, strings are treated as enumerables. They support subscript, iteration in `for in` loops and most of the library's array functions.

```swift
"howdy"[2] // "w" 
```

Escape sequences are there too if you need them: `\"`, `\n`, `\t`, `\r`, `\a`, `\b`, `\f` and `\v`. Nothing changes from other languages, so I'm sure you can figure out by yourself what every one of them does.

```swift
let code = "if(name == \"ben\"){\n\tprint(10)\n}"
```

### Atom

Atoms, or symbols as some languages refer to them, are constants where the name is their value. Although they behave a lot like strings and can generally be interchanged, internally they are treated as their own type. As the language progresses, Atoms will be put to better use.

```swift
let eq = :dog == :cat
let arr = ["dog", :cat, :mouse]
let concat = "hello" + :world
```

They're interesting to use as control conditions, emulating enums as a fixed, already-known value:

```swift
let os = "linux"
switch os
case :linux
  IO.puts("FREE")
case :windows
  IO.puts("!FREE")
end
```

### Integer

Integers are whole numbers that support most of the arithmetic and bitwise operators, as you'll see later. They can be represented also as: binary with the 0b prefix, hexadecimal with the 0x prefix and octal with the 0o prefix.

```swift
let dec = 27
let oct = 0o33
let hex = 0x1B
let bin = 0b11011
let arch = 2 ** 32
```

A sugar feature both in Integer and Float is the underscore:
 
```swift
let big = 27_000_000
```

It has no special meaning, as it will be ignored since the lexing phase. Writing `1_000` and `1000` is the same thing to the interpreter.

### Float

Floating point numbers are used in a very similar way to Integers. In fact, they can be mixed and matched, like `3 + 0.2` or `5.0 + 2`, where the result will always be a Float.

```swift
let pi = 3.14_159_265
let e = 2.71828182
```

Scientific notation is also supported via the `e` modifier:

```swift
let sci = 0.1e3
let negsci = 25e-5
```

### Boolean

It would be strange if this data type included anything else except `true` and `false`.

```swift
let mad = true
let genius = false
```

This is a dynamic language and as and such, expressions that aren't actual Booleans may evaluate to `true` or `false`. Integers and Floats will be checked if they're equal to 0, and Strings, Arrays and Dictionaries if they're empty. These are called `truthy` expressions and internally, will be evaluated to `true`.

### Array

Arrays are ordered collections of any data type. You can mix and match strings with integers, or floats with other arrays.
 
```swift
let multi = [5, "Hi", ["Hello", "World"]]
let names = ["John", "Ben", 1337]

let john = names[0]
let leet = names[-1]
```
 
Individual array elements can be accessed via subscripting with a 0-based index:

```swift
let names = ["Kirk", "Bones", "Spock"]
let first = names[0] // "Kirk"
let last = names[-1] // "Spock"
```

In the same style, an index can be used to check if it exists. It will return `nil` if it doesn't:

```swift
if names[10]
  // handle it
end
```

Individual elements can be reassigned on mutable arrays:

```swift
var numbers = [5, 8, 10, 15]
numbers[1] = 7
```

Appended with an empty or placeholder index:

```swift
numbers[] = 100
numbers[_] = 200 // Same.
```

Arrays can be compared with the `==` and `!=` operators, which will check the position and value of every element of both arrays. Equal arrays should have the same exact values in the same position.

They can also be combined with the `+` operator, which adds the element of the right side to the array on the left side.

```swift
let concat = ["an", "array"] + ["and", "another"]
// ["an", "array", "and", "another"]
```

Oh and if you're that lazy, you can ommit commas too:

```swift
let nocomma = [5 7 9 "Hi"]
```
 
### Dictionary
 
Dictionaries are hashes with a key and a value of any data type. They're good to hold unordered, structured data:

```swift
let user = ["name": "Dr. Unusual", "proffesion": "Illusionist", "age": 150]
```

Unlike arrays, internally their order is irrelevant, so you can't rely on index-based subscripting. They only support key-based subscripting:
 
```swift
user["name"] // "Dr. Unusual"
```

Values can be reassigned or inserted by key on mutable dictionaries:

```swift
var numbers = ["one": 1, "two": 2]
numbers["one"] = 5
numbers["three"] = 3 // new key:value
```

To check for a key's existence, you can access it as normal and check if it's `nil` or truthy:

```swift
if user["location"]
  // do smth
end
```

### Nil

Aria has a Nil type and yes, I'm totally aware of its problems. This was a choice for simplicity, at least for the time being. In the future, I plan to experiment with optionals and hopefully integrate them into the language.

```swift
let empty = nil
```

## Operators

You can't expect to run some calculations without a good batch of operators, right? Well, Aria has a range of arithmetic, boolean and bitwise operators to match your needs.

By order of precedence:

```swift
Boolean: && || (AND, OR)
Bitwise: & | ~ (Bitwise AND, OR, NOT)
Equality: == != (Equal, Not equal)
Comparison: < <= > >=
Bitshift: << >> (Bitshift left and right)
Arithmetic: + - * / % ** (addition, substraction, multiplication, division, modulo, power)
```

Arithmetic expressions can be safely used for Integers and Floats:

```swift
1 + 2 * 3 / 4.2
2 ** 8
3 % 2 * (5 - 3)
```

Addition can be used to concatenate Strings or combine Arrays and Dictionaries:

```swift
"obi" + " " + "wan"
[1, 2] + [3, 4]
["a": 1, "b": 2] + ["c": 3]
```

Comparison operators can compare Integers and Float by exact value, Strings, Arrays and Dictionaries by length:

```swift
5 > 2
3.2 <= 4.5
"one" < "three"
[1, 2] > [5]
["a": 1] < ["b": 2, "c": 3]
```

Equality and inequality can be used for most data types. Integers, Floats and Booleans will be compared by exact value, Strings by length, Arrays by the value and position of the elements, and Dictionaries by the the combination of key and value.

```swift
1 != 4
1.0 != 2.5
true == true
"one" == "three"
[1, 2, 3] != [1, 2]
["a": 1, "b": 2] != ["a": 5, "b": 6]
```

Boolean operators can only be used with Boolean values, namely `true` or `false`. Other data types will not be converted to truthy values.

```swift
true == true
false != true
```

Bitwise and bitshift operator apply only to Integers. Float values can't be used, even those that "look" as Integers, like `1.0` or `5.0`.

```swift
10 >> 1
12 & 5 | 3
5 ~ 2
```

## Functions

Aria treats functions as first class, like any sane language should. It checks all the boxes: they can be passed to variables, as arguments to other functions, and as elements to data structures. They also support recursion, closures, currying, you name it.

```swift
let add = fn x, y
  x + y
end
```

Some people may be used so much to parantheses that find it hard to read without them. For those people, I've left parantheses optional. The above function definition could be also written as `fn (x, y)`. Calling the function needs the parantheses though:

```swift
let sum = add(1335, 2)
```

### Return Statement

Until now we haven't seen a single `return` statement. Functions are expressions, so the last line is considered its return value. In most cases, especially with small functions, you don't have to bother. However, there are scenarios with multiple return points that need to explicitly tell the interpreter.

```swift
let even = fn n
  if n % 2 == 0
    return true
  end
  
  false
end
``` 

The last statement doesn't need a `return`, as it's the last line and will be automatically inferred. With the `if` on the other hand, the interpreter can't understand the intention, as it's just another expression. It needs the explicit `return` to stop the other statements from being interpreted.

In the case of multiple return points, I'd advise to always use `return`, no matter if it's the first or last statement. It will make for clearer intentions. 

### Closures

Closures are functions inside functions that hold on to values from the parent and "close" them when executed. This allows for some interesting side effects, like currying:

```swift
let add = fn x
  fn y
    x + y
  end
end

add(5)(7) // 12
```

Some would prefer more explicit way of calling:

```swift
let add_5 = add(5) // returns a function
let add_5_7 = add_5(7) // 12
```

You could nest a virtually unlimited amount of functions inside other functions, and all of them will have the scope of the parents.

### Recursion

Recursive functions calculate results by calling themselves. Although loops are probably easier to mentally visualize, recursion provides for some highly expressive and clean code. Technically, they build an intermediate stack and rewind it with the correct values in place when a finishing, non-recursive result is met. It's easier to understand them if you think of how they're executed. Let's see the classic factorial example:

```swift
let fac = fn n
  if n == 0
    return 1
  end
  
  n * fac(n - 1)
end
``` 

Keep in mind that Aria doesn't provide tail call optimization, as Go still doesn't support it. That would allow for more memory efficient recursion, especially when creating large stacks.

### Arrow Functions

Very useful when passing short functions as arguments, arrow functions provide a very clean syntax. They're handled internally exactly like normal functions. The only difference is that they're meant as a single line of code, while normal functions can handle blocks.

This normal function:

```swift
let sub = fn x
  x - 5
end
```

Is equivalent to:

```swift
let sub = (x) -> x - 5
```

They're not that useful to just spare a couple lines of code. They shine when passed as arguments:

```
Enum.map([1, 2, 3, 4], (x) -> x * 2)
Enum.reduce(1..10, 0, (x, acc) -> x + acc)
```

### Tricks

As first class, functions have their share of tricks. First, they can self-execute and return their result immediately:

```swift
let pow_2 = fn x
  x ** 2
end(2)
```

Not sure how useful, but they can be passed as elements to data structures, like arrays and dictionaries:

```swift
let add = fn x, y do x + y end
let list = [1, 2, add]
list[2](5, 7) 
```

Finally, like you may have guessed from previous examples, they can be passed as parameters to other functions:

```swift
let add = fn x, factor
  x + factor(x)
end
add(5, (x) -> x * 2)
```

## Conditionals

Aria provides two types of conditional statements. The `if/else` is limited to just an `if` and/or `else` block, without support for multiple `else if` blocks. That's because it advocates the use of the much better looking and flexible `switch` statement.

### If

An `if/else` block looks pretty familiar:

```swift
if 1 == 2
  IO.puts("Not calling me.")
else
  IO.puts("1 isn't equal to 2. Duh!")
end
```

Sometimes it's useful to inline it for simple checks:

```swift
let married = true
let free_time = if married then 0 else 100_000_000 end
```

### Ternary Operator

The ternary operator `?:` is a short-hand `if/else`, mostly useful when declaring variables based on a condition or when passing function parameters. It's behaviour is exactly as that of an `if/else`.

```swift
let price = 100
let offer = 120
let status = offer > price ? "sold" : "bidding"
```

Although multiple ternary operators can be nested, I wouldn't say that would be the most readable code. Actually, except for simple checks, it generally makes for unreadable code.

### Switch

`Switch` expressions on the other hand are way more interesting. They can have multiple cases with multiple conditions that break automatically on each successful case, act as generic if/else, and match array elements.

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

### Pattern Matching

When fed arrays as the control condition, the `switch` can pattern match its elements. Every argument to the switch case is compared to the respective element of the array. Off course, for a match, the number of arguments should match the size of the array.

```swift
switch ["game", "of", "thrones"]
case "game", "thrones"
  IO.puts("no match")
case "game", "of", "thrones"
  IO.puts("yep!")
end
```

That's probably useful from time to time, but it's totally achievable with array cases. The `switch` can do much better than that.

```swift
switch ["John", "Lick", 2]
case "John", _, _
  IO.puts("John Something")
case _, _ 2
  IO.puts("Something 2")
default
  IO.puts("Lame movie pun not found")
end
```

The `_` is a placeholder that will match any type and value. That makes it powerful to compare arrays where you don't need to know every element. You can mix and match values with placeholders in any position, as long as they match the size of the array.

## For Loop

There's an abundance of `for` loop variations around so Aria takes the short way: a single `for in` loop that's useful to iterate arrays, strings or dictionaries, but that does nothing else. It looks boring, but it does more than meets the eye.

```swift
for v in [1, 2, 3, 4]
  IO.puts(v)
end
```

Obviously, the result of the loop can be passed to a variable, and that's what makes them interesting to manipulate enumerables.

```swift
let plus_one = for v in [1, 2, 3, 4]
  v + 1
end
IO.puts(plus_one) // [2, 3, 4, 5]
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

With that power, you could build a function like `map` in no time:

```swift
let map = fn x, f
  for v in x
    f(v)
  end
end

let plus_one = map([1, 2, 3, 4], (x) -> x + 1)

IO.puts(plus_one) // [2, 3, 4, 5]
```

The `break` and `continue` keywords, well break or skip the iteration. They function exactly like you're used to.

```swift
for i in 1..10
  if i == 5
    continue
  end
end
```

## Range Operator

The range operator is a special type of sugar to quickly generate an array of integers or strings. 

```swift
let numbers = 0..9
let huge = 999..100
let alphabet = "a".."z"
```

As it creates an enumerable, it can be put into a `for in` loop or any other function that expects an array.

```swift
for v in 10..20
  IO.puts(v)
end
```

Although its bounds are inclusive, meaning that the left and right expressions are included in the generated array, nothing stops you from doing calculations. This is completely valid:

```swift
let numbers = [1, 2, 3, 4]
for i in 0..Enum.size(numbers) - 1
  IO.puts(i)
end
```

## Pipe Operator

The pipe operator, inspired by [Elixir](https://elixir-lang.org/), is a very expressive way of chaining functions calls. Instead of ugly code like the one below, where the order of operations is from the inner function to the outers ones:

```swift
subtract(pow(add(2, 1)))
```

You'll be writing beauties like this one:

```swift
add(2, 1) |> pow() |> substract()
```

The pipe starts from left to right, evaluating each left expression and passing it automatically as the first parameter to the function on the right side. Basically, the result of `add` is passed to `pow`, and finally the result of `pow` to `substract`.

It gets even more interesting when combined with standard library functions:

```swift
["hello", "world"] |> String.join(" ") |> String.capitalize()
```

Enumerable functions too:

```swift
Enum.map([1, 2, 3], (x) -> x + 1) |> Enum.filter((x) -> x % 2 == 1)

// or even nicer

[1, 2, 3] |> Enum.map((x) -> x + 1) |> Enum.filter((x) -> x % 2 == 1)
```

Such a simple operator hides so much power and flexibility into making more readable code. Almost always, if you have a chain of functions, think that they could be put into a pipe.

## Immutability

Now that you've seen most of the language constructs, it's time to fight the dragon. Immutability is something you may not agree with immediately, but it makes a lot of sense the more you think about it. What you'll earn is increased clarity and programs that are easier to reason about.

Iterators are typical examples where mutability is seeked for. The dreaded `i` variable shows itself in almost every language's `for` loop. Aria keeps it simple with the `for in` loop that tracks the index and value. Even if it looks like it, the index and value aren't mutable, but instead arguments to each iteration of the loop.

```swift
let numbers = [10, 5, 9]
for k, v in numbers
  IO.puts(v) 
  IO.puts(numbers[k]) // same thing
end
```

But there may be more complicated scenarios, like wanting to modify an array's values. Sure, you can do it with the `for in` loop as we've seen earlier, but higher order functions play even better:

```swift
let plus_one = Enum.map([1, 2, 3], (x) -> x + 1)
IO.puts(plus_one) // [2, 3, 4]
```

What about accumulators? Let's say you want the product of all the integer elements of an array (factorial) and obviously, you'll need a mutable variable to hold it. Fortunately we have `reduce`:

```swift
let product = Enum.reduce(1..5, 1, (x, acc) -> x * acc)
IO.puts(product)
```

Think first of how you would write the problem with immutable values and only move to mutable ones when it's impossible, hard or counter-intuitive. In most cases, immutability is the better choice.

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

Module properties and functions have access to each other. The module is interpreted and cached before-hand to allow for such behavious. In contrast to modules, everything else in Aria is single pass and as such, it will only recognize calls to a module that has already been declared.

## Imports

Source file imports are a good way of breaking down projects into smaller, easily digestible files. There's no special syntax or rules to imported files. They're included in the caller's scope and treated as if they were originally there.

```swift
// dog.ari
let name = "Charlie"
let bark_to = fn x
  "woof-woof " + x
end
```

```javascript
// main.ari
import "dog"

let phrase = name + " " + bark_to("John")
IO.puts(phrase) // "Charlie woof-woof John"
```

The file is relatively referenced from the caller and this case, both `main.ari` and `dog.ari` reside in the same folder. As the long as the extension is `.ari`, there's no need to write it in the import statement.

A more useful pattern would be to wrap imported files into a module. That would make for a more intuitive system and prevent scope leakage. The dog case above would be written simply into:

```swift
// dog.ari
module Dog
  let name = "Charlie"
  let bark_to = fn x
    "woof-woof " + x
  end
end
```

```javascript
// main.ari
import "dog"

let phrase = Dog.name + " " + Dog.bark_to("John")
```

## Comments

Nothing ground breaking in here. You can write either single line or multi line comments:

```
// an inline comment
/*
  I'm spanning multiple
  lines.
*/
```

## Standard Library

Right now it's a small library of functions, but it's expending continually. Head over to the [documentation](https://github.com/fadion/aria/wiki/Standard-Library).

## Future Plans

Although this is a language made purely for fun and experimentation, it doesn't mean I will abandon it in it's first release. Adding other features means I'll learn even more!

In the near future, hopefully, I plan to:

- Improve the Standard Library with more functions.
- ~~Support closures and recursion~~.
- ~~Add a short syntax for functions in the form of: (x) -> x~~.
- ~~Add importing of other files~~.
- ~~Add the pipe operator!~~
- Support optional values for null returns.
- Write more tests!
- Write some useful benchmarks with non-trivial programs.

## Credits

Aria was developed by Fadion Dashi, a freelance web and mobile developer from Tirana.

The implementation is based on the fantastic [Writing an Interpreter in Go](https://interpreterbook.com/). If you're even vaguely interested in interpreters, with Golang or not, I highly suggest that book.

The `reader.Buffer` package is a "fork" of Golang's `bytes.Buffer`, in which I had to add a method that reads a rune without moving the internal cursor. I hate doing that, but unfortunately couldn't find a way out of it. That package has its own BSD-style [license](https://github.com/golang/go/blob/master/LICENSE).