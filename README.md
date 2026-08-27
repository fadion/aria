[![CI](https://github.com/fadion/aria/actions/workflows/ci.yml/badge.svg)](https://github.com/fadion/aria/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/fadion/aria.svg)](https://pkg.go.dev/github.com/fadion/aria)

# Aria Language

Aria is an expressive, interpreted, toy language built as an exercise on designing and interpreting a programming language. It has a noiseless syntax, free of useless semi colons, braces or parantheses, and treats everything as an expression. Technically, it's built with a hand written lexer and parser, a recursive decent one (Pratt), and a tree-walk interpreter. I have never set any goals for it to be either fast, nor bulletproof, so don't expect neither of them.

It features mutable and immutable values, if and switch conditionals with pattern matching, functions, type hinting, for and while loops, records, modules, the pipe operator, recoverable errors, imports and many more. More importantly, it's getting expanded frequently with new features, more functions for the standard library and bug fixes. All of that while retaining it's expressiveness, clean syntax and easy of use.

```swift
record Book
  title: String
  year: Int
end

let shelf = [
  Book("Neuromancer", 1984),
  Book("Dune", 1965),
  Book("Snow Crash", 1992),
  Book("Cryptonomicon", 1999)
]

let era = func (b: Book) -> String
  switch b.year
  case 1900..1979 then "a classic"
  case 1980..1989 then "early cyberpunk"
  default then "late cyberpunk"
  end
end

shelf
  |> Enum.sortBy((b) -> b.year)
  |> Enum.map((b) -> "#{b.title} (#{b.year}), #{era(b)}")
  |> Enum.each((line) -> println(line))
```

## Table of Contents

* [Usage](#usage)
    * [Run a Source File](#run-a-source-file)
    * [Run a One-liner](#run-a-one-liner)
    * [Check Without Running](#check-without-running)
    * [REPL](#repl)
* [Variables](#variables)
    * [Constants](#constants)
    * [Type Lock](#type-lock)
* [Data Types](#data-types)
    * [String](#string)
    * [Atom](#atom)
    * [Int](#int)
    * [Float](#float)
    * [Boolean](#boolean)
    * [Array](#array)
    * [Dictionary](#dictionary)
    * [Nil](#nil)
    * [Type Conversion](#type-conversion)
    * [Type Checking](#type-checking)
* [Operators](#operators)
    * [Shorthand Assignment](#shorthand-assignment)
* [Functions](#functions)
    * [Type Hinting](#type-hinting)
    * [Default Parameters](#default-parameters)
    * [Return Statement](#return-statement)
    * [Variadic](#variadic)
    * [Arrow Functions](#arrow-functions)
    * [Closures](#closures)
    * [Recursion](#recursion)
    * [Tricks](#tricks)
* [Line Breaks](#line-breaks)
* [Destructuring](#destructuring)
* [Blocks](#blocks)
* [Conditionals](#conditionals)
    * [If](#if)
    * [Ternary Operator](#ternary-operator)
    * [Switch](#switch)
    * [Guards, Types and Ranges](#guards-types-and-ranges)
    * [Pattern Matching](#pattern-matching)
* [For Loop](#for-loop)
* [While and Until](#while-and-until)
* [Range Operator](#range-operator)
* [Pipe Operator](#pipe-operator)
* [Immutability](#immutability)
* [Modules](#modules)
* [Records](#records)
* [Imports](#imports)
* [Comments](#comments)
* [Errors](#errors)
    * [Tagged Results](#tagged-results)
    * [Try and Rescue](#try-and-rescue)
* [Files, Arguments, Environment and Time](#files-arguments-environment-and-time)
* [Standard Library](#standard-library)

_Working on the interpreter itself? [docs/architecture.md](docs/architecture.md) covers how it's put together and why the design decisions went the way they did. [docs/compatibility.md](docs/compatibility.md) says what a version number promises and what it doesn't._

## Usage

If you want to play with the language, but have no interest in toying with its code, you can download a built binary for your operating system. Just head to the [latest release](https://github.com/fadion/aria/releases/latest) and download one of the archives.

The other option, where you get to play with the code and run your changes, is to install it from source with `go install github.com/fadion/aria@latest`. That drops an `aria` binary in `$GOBIN`, or `$HOME/go/bin` if you haven't set one, so make sure that directory is in your path. From a clone, a plain `go build .` in the checkout gives you the same binary in place.

### Run a source file

To run an Aria source file, give it a path relative to the current directory.

```
aria run path/to/file.ari
```

A `-` reads from standard input, so aria can sit in a pipeline:

```
cat path/to/file.ari | aria run -
```

### Run a one-liner

```
aria -e 'println(1 + 2)'
```

### Check without running

`check` runs a file through the whole pipeline except the evaluation, so it parses and resolves and then exits non-zero if anything is off. That's what you'd want in CI or in an editor, and it catches everything the resolver knows about: undefined names, immutable rebinding, unknown type hints and mistyped module members.

```
aria check path/to/file.ari
```

### REPL

As any serious language, Aria provides a REPL too:

```
aria repl
```

It reads whole constructs instead of lines, so you can type a multi-line `func`, `module`, `if` or `for` straight into it. The prompt changes to `..` while one is still open and an empty line abandons it. A statement that produced nothing will print nothing.

`:help` lists the commands: `:load` evaluates a file into the session, `:vars` and `:modules` show what's in scope, and `:quit` gets you out.

## Variables

Variables in Aria start with the keyword `var`. Accessing an undeclared variable, in contrast with some languages, will not create it. Names are checked before the program runs, so a typo gets reported before anything has had the chance to happen.

```swift
var name = "John"
var married = false

var age = 40
age = 41
```

Names have to start with an alphabetic character and continue either with alphanumeric, underscores, questions marks or exclamation marks. When you see a question mark, don't confuse them with optionals like in some other languages. In here they have no special lexical meaning except that they allow for some nice variable names like `is_empty?` or `do_it!`.

### Constants

Constants have the same traits as variables, except that they start with `let` and are immutable. Once declared, reassigning a constant is an error and it's reported before the program runs. Even data structures are locked into immutability, so elements of an Array or Dictionary can't be added, updated or removed through a `let` name.

Writing to an element is really a rebinding, as `xs[] = v` gives you a new collection and points the name at it, so it needs a `var` just like any other reassignment.

```swift
let name = "Ben"
name = "John" // error: name is bound with let
```

### Type Lock

Type lock is a safety feature of mutable variables. Once they're declared with a certain data type, they can only be assigned to that same type. This makes for more predictable results, as an integer variable can't be assigned to a string or array. In this regard, Aria works as a strong typed language.

This will work:

```swift
var nr = 10
nr = 15
```

This won't:

```swift
var nr = 10
nr = "ten" // error: nr holds Int
```

## Data Types

Aria supports 7 data types: `String`, `Atom`, `Int`, `Float`, `Bool`, `Array`, `Dictionary` and `Nil`.

### String

Strings are UTF-8 encoded, meaning that you can stuff in there anything, even emojis.

```swift
let weather = "Hot"
let price = "円500"
```

String concatenation is handled with the `+` operator. Concats between a string and another data type will result in a runtime error.

```swift
let name = "Tony" + " " + "Stark"
```

Additionally, strings are treated as enumerables. They support subscripting and iteration in `for in` loops.

```swift
"howdy"[2] // "w"
```

Subscripting counts characters, not bytes, so a string with accents or emoji indexes the way it reads:

```swift
"héllo"[1] // "é"
```

Escape sequences are there too if you need them: `\"`, `\n`, `\t`, `\r`, `\a`, `\b`, `\f` and `\v`. Nothing changes from other languages, so I'm sure you can figure out by yourself what every one of them does.

```swift
let code = "if(name == \"ben\"){\n\tprint(10)\n}"
```

You can write a rune by its codepoint with `\xNN`, `\uNNNN` or `\u{N...}`. All three take codepoints and not bytes, seeing as strings index by rune anyway:

```swift
println("\x41")      // "A"
println("\u{1F600}") // an emoji, one character long
```

Strings interpolate with `#{}`. Every hole takes a whole expression and the value comes out rendered exactly like `println` would render it:

```swift
let name = "Ada"
let items = 3
println("user #{name} has #{items} items")
```

A `#` is only special right before a `{`, and `\#` opts out of even that.

`println` and `print` also take any number of arguments, joined with a space:

```swift
println("count:", 3)
```

Backticks make a raw string. It spans lines and processes no escapes at all, interpolation included, which is exactly what you want for a block of text or a regex:

```swift
let block = `line one
line two`
println(block)

println(String.match?("abc123", `\d+`))
```

### Atom

Atoms, or symbols as some languages refer to them, are constants where the name is their value. Although they behave a lot like strings and can generally be interchanged, internally they are treated as their own type. As the language progresses, Atoms will be put to better use.

Interchangeable really does mean interchangeable. `:a == "a"` is true and the two are the same dictionary key, so a dictionary written with atoms can be read with strings and the other way around. Whatever spelling you gave it, it keeps.

```swift
let eq = :dog == :cat
let arr = ["dog", :cat, :mouse]
let dict = [:name => "John", :age => 40]
let concat = "hello" + :world
```

They're interesting to use as control conditions, emulating enums as a fixed, already-known value:

```swift
let os = "linux"
switch os
case :linux
  println("FREE")
case :windows
  println("!FREE")
end
```

### Int

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

It has no special meaning, as it will be ignored in the lexing phase. Writing `1_000` and `1000` is the same thing to the interpreter.

Arithmetic between two Integers always produces an Integer, and that includes division, which truncates toward zero:

```swift
10 / 5  // 2
10 / 4  // 2, not 2.5
1 / 3   // 0
2 ** -1 // 0, for the same reason
```

The operand *types* decide the result type, never the operand values. That's what makes a declared return type like `func (n: Int) -> Int` something you can check by reading the code, instead of by running it with the wrong numbers and finding out the hard way. When you want real division, give it a Float:

```swift
10 / 4.0 // 2.5
```

Integer arithmetic never wraps. An `Int` is a 64-bit signed integer and an operation whose result doesn't fit in one is an error, not a plausible looking negative number:

```swift
2 ** 62 // 4611686018427387904
```

Floats are IEEE 754 and left exactly as they are, so overflow there reaches `Inf` instead. Division and modulo by zero are errors on both.

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

### Bool

It would be strange if this data type included anything else except `true` and `false`.

```swift
let mad = true
let genius = false
```

Expressions like the `if/else`, as you'll see later, will check for values that aren't necessarily boolean. Integers and Floats will be checked if they're equal to 0, and Strings, Arrays and Dictionaries if they're empty. These are called `truthy` expressions and internally, will be evaluated to boolean.

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
  nil
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
let user = ["name" => "Dr. Unusual", "proffesion" => "Illusionist", "age" => 150]
```

I'd argue that using Atoms for keys would make them look cleaner:

```swift
let user = [:name => "Dr. Unusual", :proffesion => "Illusionist", :age => 150]
```

Unlike arrays, internally their order is irrelevant, so you can't rely on index-based subscripting. They only support key-based subscripting:

```swift
user["name"] // "Dr. Unusual"
```

Values can be reassigned or inserted by key on mutable dictionaries:

```swift
var numbers = ["one" => 1, "two" => 2]
numbers["one"] = 5
numbers["three"] = 3 // new key:value
```

To check for a key's existence, you can access it as normal and check if it's `nil` or truthy:

```swift
if user["location"] == nil
  // do smth
  nil
end
```

### Nil

Aria has a Nil type and yes, I'm totally aware of its problems. This was a choice for simplicity, at least for the time being. In the future, I plan to experiment with optionals and hopefully integrate them into the language.

```swift
let empty = nil
```

### Type Conversion

Converting between types is handled in a few ways that produce exactly the same results. The `as` operator is probably the more convenient and more expressive of the bunch. It converts to `String`, `Int`, `Float`, `Bool`, `Array` and `Dictionary`:

```swift
let nr = 10
nr as String
nr as Int
nr as Float
nr as Bool
nr as Array
```

`as Bool` follows the same truthiness rule as a condition, while `as Dictionary` is the inverse of `as Array` on a dictionary, taking `[key, value]` pairs:

```swift
println([[:a, 1], [:b, 2]] as Dictionary)
```

Provided by the runtime are the appropriately named functions: `String()`, `Int()`, `Float()` and `Array()`.

```swift
let str = String(10)
let int = Int("10")
let fl = Float(10)
let arr = Array(10)
```

The `Type` module of the Standard Library provides interfaces to those same functions and even adds some more, like `Type.of()` and `Type.isNumber()`.

```swift
let str = Type.toString(10)
let int = Type.toInt("10")
let fl = Type.toFloat(10)
let arr = Type.toArray(10)
```

Which method you choose to use is strictly preferential and depends on your background.

### Type Checking

There will be more than one occassion where you'll need to type check a variable. Aria provides a few ways to achieve that.

The `is` operator is specialized in checking types and should be the one you'll want to use practically everywhere.

```swift
let nr = 10
if nr is Int
  println("Yes, an integer")
end
```

There's also the `typeof()` runtime function and `Type.of()` from the Standard Library. They essentially do the same thing, but not only they're longer to write, but return strings. The above would be equivalent to:

```swift
if Type.of(nr) == "Int"
  println("Yes, an integer")
end
```

## Operators

You can't expect to run some calculations without a good batch of operators, right? Well, Aria has a range of arithmetic, boolean and bitwise operators to match your needs.

By order of precedence, loosest first:

```
||                    OR
??                    nil-coalescing
&&                    AND
== != < <= > >=       equality and comparison
|                     bitwise OR
^                     bitwise XOR
&                     bitwise AND
..                    range
<< >>                 bitshift left and right
+ -                   addition, subtraction
* / %                 multiplication, division, modulo
**                    power (right associative)
! ~ -                 prefix NOT, bitwise NOT, negation
```

`??` gives you its left side unless it's `nil`, and it short-circuits. It tests for `nil` and not for truthiness, which is the whole reason it exists next to `||`:

```swift
let config = [:retries => 0]
println(config["port"] ?? 8080) // 8080
println(config[:retries] ?? 3)  // 0, because 0 is not nil
println(0 || 5)                 // true, which is why || can't do this job
```

A dot can be written as `?.`, which gives you `nil` as soon as a link in the chain is `nil`, instead of failing on the next access:

```swift
let cfg = [:db => [:host => "localhost"]]
println(cfg?.db?.host)
println(cfg?.missing?.host ?? "none")
```

Two of these are worth a second look, because they read the other way round in some languages. `&&` binds tighter than `||`, so `a && b || c` is `(a && b) || c`. And bitwise binds tighter than comparison, so `6 & 3 == 3` is `(6 & 3) == 3`, the same way Python does it and the opposite of C.

`**` is right associative and outranks a leading minus, but not a minus on its exponent:

```swift
2 ** 3 ** 2 // 512, not 64
-2 ** 2     // -4, the negation of 2 ** 2
2 ** -1     // 0, see Int division below
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
["a" => 1, "b" => 2] + ["c" => 3]
```

Comparison operators compare Integers and Floats by value, and Strings lexicographically:

```swift
5 > 2
3.2 <= 4.5
"one" < "three"
```

Arrays and Dictionaries have no order, so `<`, `<=`, `>` and `>=` aren't defined on them. If it's sizes you want to compare, say so:

```swift
Enum.size([1, 2]) > Enum.size([5])
Dict.size(["a" => 1]) < Dict.size(["b" => 2, "c" => 3])
```

Equality and inequality can be used for most data types. Integers, Floats and Booleans will be compared by exact value, Strings by their text, Arrays by the value and position of the elements, and Dictionaries by the the combination of key and value.

```swift
1 != 4
1.0 != 2.5
true == true
"one" == "three"
[1, 2, 3] != [1, 2]
["a" => 1, "b" => 2] != ["a" => 5, "b" => 6]
```

Boolean operators can only be used with Boolean values, namely `true` or `false`. Other data types will not be converted to truthy values.

```swift
true == true
false != true
```

Bitwise and bitshift operator apply only to Integers. Float values can't be used, even those that "look" as Integers, like `1.0` or `5.0`. A shift count has to be between 0 and 63, and a left shift that would push bits past the sign bit is an error like any other overflow.

```swift
10 >> 1
12 & 5 | 3
~5
```

### Shorthand Assignment

Operators like `+`, `-`, `*` and `/` support shorthand assignment to variables. Basically, statements like this:

```swift
count = count + 1
```

Can be expressed as:

```swift
count += 1
```

## Functions

Aria treats functions as first class, like any sane language should. It checks all the boxes: they can be passed to variables, as arguments to other functions, and as elements to data structures. They also support recursion, closures, currying, variadic parameters, you name it.

```swift
let add = func x, y
  x + y
end
```

Parantheses are optional and for simple functions like the above, I'd omit them. Calling the function needs the parantheses though:

```swift
let sum = add(1335, 2)
```

### Type Hinting

Like in strong typed languages, type hinting can be a very useful feature to validate function arguments and its return type. It's extra useful for library functions that have no assurance of the data types they're going to get.

This function call will produce output:

```swift
let add = func (x: Int, y: Int) -> Int
  x + y
end
println(add(5, 2))
```

This however, will cause a type missmatch runtime error:

```swift
println(add(5, "two"))
```

Aria is not a strong typed language, so type hinting is completely optional. Generally, it's a good idea to use it as a validation measure. Once you enforce a certain type, you'll be sure of how the function executes.

A hint names one of `Nil`, `Bool`, `Int`, `Float`, `String`, `Atom`, `Array`, `Dictionary`, `Function`, `Module` or `Any`. Anything else is an error before the program runs, and the same goes for `is` and `as`. `Any` accepts everything, which is how you say "anything" out loud instead of by leaving the hint off:

```swift
let identity = func (v: Any) -> Any
  v
end
println(identity([1, 2]))
```

A default is checked against its own parameter's hint, so `func (n: Int = "oops")` is an error instead of a hint that lies to every caller who omits the argument.

### Default Parameters

Function parameters can have default values, used when the parameters are omitted from function calls.

```swift
let architecture = func bits = 6
  2 ** bits
end

architecture() // 64
architecture(4) // 16
```

They can be combined with type hinting and, obviously, need to be of the same declared type.

```swift
let architecture = func bits: Int = 6
  2 ** bits
end
```

### Return Statement

Until now we haven't seen a single `return` statement. Functions are expressions, so the last line is considered its return value. In most cases, especially with small functions, you don't have to bother. However, there are scenarios with multiple return points that need to explicitly tell the interpreter.

```swift
let even = func n
  if n % 2 == 0
    return true
  end
  false
end
```

The last statement doesn't need a `return`, as it's the last line and will be automatically inferred. With the `if` on the other hand, the interpreter can't understand the intention, as it's just another expression. It needs the explicit `return` to stop the other statements from being interpreted.

In the case of multiple return points, I'd advise to always use `return`, no matter if it's the first or last statement. It will make for clearer intentions.

### Variadic

Variadic functions take an indefinite number of parameters and merge them all into a single, Array argument. Their first use would be as a sugar:

```swift
let add = func ...nums
  var count = 0
  for n in nums
    count = count + n
  end
  count
end

add(1, 2, 3, 4, 5) // 10
```

Even better, they can be used for functions that respond differently based on the number of arguments:

```swift
let structure = func ...args
  if Enum.size(args) == 2
    let key = args[0]
    let val = args[1]
    return [key => val]
  end
  if Enum.size(args) > 2
    return args
  end
  args[0]
end

structure("name", "John") // dictionary
structure(1, 2, 3) // array
structure(5) // integer
```

Functions may have as many parameters as needed, as long the variadic argument is the last parameter:

```swift
let calc = func mult, ...nums
  mult * Enum.reduce(nums, 0, func x, acc do x + acc end)
end
calc(10, 1, 2, 3, 4) // 100
```

Variadic arguments can even have default values:

```swift
let join = func (glue: String, ...words = ["hello", "there"])
  String.join(words, glue)
end

join(" ") // "hello there"
```

### Arrow Functions

Very useful when passing short functions as arguments, arrow functions provide a very clean syntax. They're handled internally exactly like normal functions. The only difference is that they're meant as a single line of code, while normal functions can handle blocks.

This normal function:

```swift
let sub = func x
  x - 5
end
```

Is equivalent to:

```swift
let sub = (x) -> x - 5
```

They're not that useful to just spare a couple lines of code. They shine when passed as arguments:

```swift
Enum.map([1, 2, 3, 4], (x) -> x * 2)
Enum.reduce(1..10, 0, (x, acc) -> x + acc)
```

### Closures

Closures are functions inside functions that hold on to values from the parent and "close" them when executed. This allows for some interesting side effects, like currying:

```swift
let add = func x
  func y
    x + y
  end
end

add(5)(7) // 12
```

Some would prefer a more explicit way of calling:

```swift
let add_5 = add(5) // returns a function
let add_5_7 = add_5(7) // 12
```

You could nest a virtually unlimited amount of functions inside other functions, and all of them will have the scope of the parents.

### Recursion

Recursive functions calculate results by calling themselves. Although loops are probably easier to mentally visualize, recursion provides for some highly expressive and clean code. Technically, they build an intermediate stack and rewind it with the correct values in place when a finishing, non-recursive result is met. It's easier to understand them if you think of how they're executed. Let's see the classic factorial example:

```swift
let fac = func n
  if n == 0
    return 1
  end

  n * fac(n - 1)
end
```

Keep in mind that Aria doesn't provide tail call optimization, as Go still doesn't support it. That would allow for more memory efficient recursion, especially when creating large stacks.

### Tricks

As first class, functions have their share of tricks. First, they can self-execute and return their result immediately:

```swift
let pow_2 = func x
  x ** 2
end(2)
```

Not sure how useful, but they can be passed as elements to data structures, like arrays and dictionaries:

```swift
let add = func x, y do x + y end
let list = [1, 2, add]
list[2](5, 7)
```

Finally, like you may have guessed from previous examples, they can be passed as parameters to other functions:

```swift
let add = func x, factor
  x + factor(x)
end
add(5, (x) -> x * 2)
```

## Line Breaks

A newline ends a statement and that's how Aria gets away without semicolons. An expression can still span lines though, in either of the two shapes that read well: a line ending with an operator, or a line beginning with one:

```swift
let total = 1 +
  2 +
  3

let data = [1, -2, 3, -4]
println(data
  |> Enum.filter((x) -> x > 0)
  |> Enum.map((x) -> x * 2))
```

The exception is a line beginning with `-`, `(` or `[`, as each of those could start an expression of its own, be it a negation, a call or a subscript. Those are new statements, like they always were.

A parameter list in parentheses can span lines too:

```swift
let add = func (
  a: Int,
  b: Int = 10
) -> Int
  a + b
end
```

## Destructuring

An array can be taken apart by its shape in a `let` or a `var`. The `_` is a hole, `...name` takes whatever is left, and patterns can nest:

```swift
let [a, b] = [1, 2]
let [_, second] = ["skip", "keep"]
let [head, ...tail] = [1, 2, 3, 4]
let [first, ...middle, last] = [1, 2, 3, 4, 5]
let [x, [y, z]] = [1, [2, 3]]
println(middle) // [2, 3, 4]
```

Without a `...`, the shape has to match exactly. A pattern that doesn't fit is an error and not a partial bind.

In a `switch` arm, `let name` captures what matched:

```swift
let describe = func (result)
  switch result
  case [:ok, let value] then "ok: #{value}"
  case [:error, let message] then "error: #{message}"
  default then "not a result"
  end
end
println(describe([:ok, 42]))
```

A bare identifier in a case is still a reference compared against the control, like it always was. The `let` is what says "bind this", the same way it does at the front of a statement.

## Blocks

`do ... end` is an expression. It returns its last value and has a scope of its own, which is handy to name something that takes a few steps to build, without leaking those steps around:

```swift
let area = do
  let width = 6
  let height = 7
  width * height
end
println(area) // 42
```

A body has to hold something. An empty `if`, `else`, `for`, `while`, `until`, `func`, `do`, `try`, `rescue` or `switch` arm is an error, because a block that runs nothing is almost always an unfinished edit rather than an intention. Comments don't count, so a body holding only a `// todo` is empty as far as the parser is concerned:

```swift
let stub = func x
  // work this out later
  nil
end
```

Writing `nil` is how you say "deliberately nothing", and it says it where a reader will notice. A `module` and a `record` are containers rather than bodies, so an empty one of those is fine.

## Conditionals

Aria provides two types of conditional statements. The `if/else` is limited to just an `if` and/or `else` block, without support for multiple `else if` blocks. That's because it advocates the use of the much better looking and flexible `switch` statement.

### If

An `if/else` block looks pretty familiar:

```swift
if 1 == 2
  println("Not calling me.")
else
  println("1 isn't equal to 2. Duh!")
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
  println("Is it 2 or 3?")
case 5
  println("It is 5. Magic!")
default
  println("No idea, sorry.")
end
```

Not only that, but a `switch` can behave as a typical if/else when no control condition is provided. It basically becomes a `switch true`.

```swift
let a = "John"
switch
case a == "John"
  println("John")
case a == "Ben"
  println("Ben")
default
  println("Nobody")
end
```

### Guards, Types and Ranges

A `when` clause guards an arm. It's only tested once one of the arm's values has matched, which is what lets a `switch` without a control condition replace an else-if chain without repeating the subject:

```swift
let describe = func (n)
  switch n
  case 1..9 when n % 2 == 0 then "even digit"
  case 1..9 then "odd digit"
  default then "not a digit"
  end
end
println(describe(4))
```

`is` works in case position too, matching on the control's type, while a range matches membership:

```swift
let classify = func (v)
  switch v
  case is Int then "a number"
  case is String then "some text"
  case is Any then "something else"
  end
end
println(classify("hi"))
```

A guard that fails falls through to the next arm, not straight to `default`.

### Pattern Matching

An array literal in case position pattern matches its elements. Of course, for a match the pattern and the array have to be the same size:

```swift
switch ["game", "of", "thrones"]
case ["game", "thrones"]
  println("no match")
case ["game", "of", "thrones"]
  println("yep!")
end
```

The `_` is a placeholder that will match any type and value, so you can compare arrays where you don't need to know every element:

```swift
switch ["John", "Lick", 2]
case ["John", _, _]
  println("John Something")
case [_, _, 2]
  println("Something 2")
default
  println("Lame movie pun not found")
end
```

A comma separated case list is a list of *alternatives* and not a pattern, so `case 1, 2` means "1 or 2" no matter what the control is. That's why a pattern gets the array literal spelling instead. The same syntax used to mean both and the runtime type of the subject decided which one you got.

## For Loop

Aria takes a modern approach to the `for` loop, evading from the traditional, 3-parts `for` we've been using for decades. Instead, it focuses on a flexible `for in` loop that iterates arrays, dictionaries, and as you'll see later, ranges.

```swift
for v in [1, 2, 3, 4]
  println(v)
end
```

Obviously, the result of the loop can be passed to a variable, and that's what makes them interesting to manipulate enumerables.

```swift
let plus_one = for v in [1, 2, 3, 4]
  v + 1
end
println(plus_one) // [2, 3, 4, 5]
```

Passing two arguments for arrays or strings will return the current index and value. For dictionaries, the first argument will be the key.

```swift
for i, v in "abcd"
  println((i as String) + "=>" + v)
end
```

```swift
for k, v in ["name" => "John", "age" => 40]
  println(k)
  println(v)
end
```

With that power, you could build a function like `map` in no time:

```swift
let map = func x, f
  for v in x
    f(v)
  end
end

let plus_one = map([1, 2, 3, 4], (x) -> x + 1)

println(plus_one) // [2, 3, 4, 5]
```

Without arguments, the `for` loop can behave as an infite loop, much like a traditional `while`. Although there's not too many usecases, it does its job when needed. An example would be prompting the user for input and only breaking the infinite loop on a specific text.

```swift
for
  let pass = prompt("Enter the password: ")
  if pass == "123"
    println("Good, strong password!")
    break
  end
end
```

The `break` and `continue` keywords, well break or skip the iteration. They function exactly like you're used to.

```swift
for i in 1..10
  if i == 5
    continue
  end
end
```

`break` also takes a count, so a nested loop can break outward without a flag variable to carry the intention:

```swift
let rows = [[1, 2], [3, 4]]
for row in rows
  for cell in row
    if cell == 3
      println("found it")
      break 2
    end
  end
end
```

## While and Until

`while` repeats a body for as long as its condition holds and `until` for as long as it doesn't. Both take the same optional `do` and the same `end` terminated shape as the `for`:

```swift
var i = 0
while i < 5
  i += 1
end

var j = 10
until j <= 0 do
  j -= 2
end
```

Unlike the `for`, they evaluate to `nil`. A `for` collects every iteration's value into an array, which is great when you want it and wasteful when you don't. These two are for when you don't.

*The `for` loop is currently naively parsed. It works for most cases, but still, it's not robust enough. I'm working to find a better solution.*

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
  println(v)
end
```

Although its bounds are inclusive, meaning that the left and right expressions are included in the generated array, nothing stops you from doing calculations. This is completely valid:

```swift
let numbers = [1, 2, 3, 4]
for i in 0..Enum.size(numbers) - 1
  println(i)
end
```

A range written directly as a loop's enumerable just counts, instead of building the whole array first, so `for i in 1..10000000` costs nothing up front.

A range used as a subscript slices, on arrays and strings alike. The bounds are inclusive here too, negative ones count from the end, and anything outside the collection gets clamped instead of raising an error:

```swift
let a = [1, 2, 3, 4, 5]
println(a[1..3])   // [2, 3, 4]
println(a[-2..-1]) // [4, 5]
println(a[3..1])   // [4, 3, 2]
println(a[0..99])  // the whole thing
println("héllo"[1..3])
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

A bare name works too, seeing as an empty argument list on a function that takes an argument looks like a mistake anyway:

```swift
let double = func (x) do x * 2 end
println(4 |> double)
```

And when the piped value doesn't belong first, a `_` among the arguments marks where it goes. Only one of them is allowed:

```swift
let subtract = func (a, b) do a - b end
println(3 |> subtract(10, _)) // 7
```

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
  println(v)
  println(numbers[k]) // same thing
end
```

But there may be more complicated scenarios, like wanting to modify an array's values. Sure, you can do it with the `for in` loop as we've seen earlier, but higher order functions play even better:

```swift
let plus_one = Enum.map([1, 2, 3], (x) -> x + 1)
println(plus_one) // [2, 3, 4]
```

What about accumulators? Let's say you want the product of all the integer elements of an array (factorial) and obviously, you'll need a mutable variable to hold it. Fortunately we have `reduce`:

```swift
let product = Enum.reduce(1..5, 1, (x, acc) -> x * acc)
println(product)
```

Think first of how you would write the problem with immutable values and only move to mutable ones when it's impossible, hard or counter-intuitive. In most cases, immutability is the better choice.

## Modules

Modules are very simple containers of data and nothing more. They're not an imitation of classes, as they can't be initialized, don't have any type of access control, inheritance or whatever. If you need to think in Object Oriented terms, they're like a class with only static properties and methods. They're good to give some structure to a program, but not to represent cars, trees and cats. That's what [records](#records) are for.

```swift
module Color
  let white = "#fff"
  let grey = "#666"
  let hexToRGB = func hex
    // some calculations
    nil
  end
end

let background = Color.white
let font_color = Color.hexToRGB(Color.grey)
```

Because modules are interpreted and cached before-hand, properties and functions have access to each other. Top level functions hoist for the same reason, so two of them can call each other without having to wrap one in a module:

```swift
let isEven = func (n) do
  if n == 0 then return true end
  isOdd(n - 1)
end

let isOdd = func (n) do
  if n == 0 then return false end
  isEven(n - 1)
end

println(isEven(10)) // true
```

Everything else is still single pass. Only a top level `let` whose value is a function literal hoists, while any other name has to be declared before it's read.

A module is an ordinary value, so its name can be assigned, passed to a function and returned from one:

```swift
let C = Color
println(C.white)
println(typeof(Color)) // "Module"
```

And the `.` is an operator over expressions and not a form over two names, so it chains over anything you throw at it, be it a member, a call or a subscript:

```swift
let config = [:db => [:host => "localhost"]]
println(config.db.host)
```

## Records

Modules give a program structure but can't be instantiated, while a dictionary carries data with no identity at all, seeing as `typeof` says `Dictionary` for every single one of them. A record is the shape for a car, a tree or a cat.

```swift
record Point
  x: Int
  y: Int
end

let p = Point(1, 2)
println(p.x)
println(typeof(p))  // "Point"
println(p is Point) // true
```

A record's fields are a parameter list, so constructing one is an ordinary call with the same arity check, the same type hints and the same defaults:

```swift
record Config
  host: String
  port: Int = 8080
end
println(Config("localhost"))
```

Two records with the same fields are still different types, which is pretty much the whole point of having them:

```swift
record Point
  x: Int
end
record Size
  x: Int
end
println(Point(1) == Size(1)) // false
```

Records are immutable like everything else, so writing a field rebinds the name, exactly the way `a[0] = v` already does:

```swift
record Point
  x: Int
  y: Int
end
var p = Point(1, 2)
let before = p
p.x = 5
println(p)      // Point(x: 5, y: 2)
println(before) // Point(x: 1, y: 2)
```

That works through dictionaries too, and to any depth you need.

## Imports

Source file imports are a good way of breaking down projects into smaller, easily digestible files. There's no special syntax or rules to imported files. They're included in the caller's scope and treated as if they were originally there. Imports are cached, so in multiple imports, only the first one is actually interpreted.

```swift
// cat.ari
let name = "Bella"
let hi = func x
  "moew " + x
end
```

```javascript
// main.ari
import "cat"

let phrase = name + " " + hi("John")
println(phrase) // "Bella moew John"
```

The file is relatively referenced from the caller and in this case, both `main.ari` and `dog.ari` reside in the same folder. As the long as the extension is `.ari`, there's no need to write it in the import statement. Even the quotes can be omited and the file written as an identifier, as long as it doesn't include a dot (as in `cat.ari`) and isn't a reserved keyword.

An imported file is part of the same compilation as the file that imports it, so a mistake in one gets reported before anything runs, in the file where it was actually made. An import belongs at the top level of a file, because no pass can see through a conditional one.

Two files can both define `size`, because `as` namespaces what an import brings in:

```javascript
// geometry.ari
let size = 10
let area = func (w, h)
  w * h
end
```

```swift
import "geometry" as Geo

println(Geo.size)
println(Geo.area(3, 4))
```

An alias is a module, so it's checked like any other one and a member that isn't there gets you a diagnostic instead of a surprise at runtime. Import cycles are fine, as a file that's already been pulled in won't be pulled in twice.

A more useful pattern would be to wrap imported files into a module. That would make for a more intuitive system and prevent scope leakage. The cat case above could be written simply into:

```swift
// cat.ari
module Cat
  let name = "Bella"
  let hi = func x
    "moew " + x
  end
end
```

```javascript
// main.ari
import cat

let phrase = Cat.name + " " + Cat.hi("John")
```

Imports are expressions too! Technically, they can be used anywhere else an Integer or String can, even though it probably wouldn't make for the classiest code ever.

```javascript
// exp.ari
let x = 10
let y = 15
x + y
```

```javascript
// main.ari
let value = import exp
println(value) // 25

if import exp == 25
  println("Yay")
end
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

## Errors

A failure halts the program and exits non-zero. That's the right rule for a genuine fault and the wrong one for "this key isn't in this dictionary", so Aria gives you two shapes for recovering, each of them answering a different question.

### Tagged results

An operation that can fail answers with `[:ok, value]` or `[:error, reason]`. No special syntax needed, as a `switch` with array patterns and `let` capture takes one apart just fine:

```swift
let user = [:name => "Ada"]
println(switch Dict.insert(user, :name, "Bob")
case [:ok, let updated] then updated
case [:error, let why] then "could not: #{why}"
end)
```

The `Result` module gives that shape a name and the usual ways to consume it: `Result.ok?`, `Result.unwrap` with a fallback, `Result.expect` and `Result.reason`:

```swift
let user = [:name => "Ada"]
println(Result.unwrap(Dict.delete(user, :missing), user))
```

### try and rescue

`try` catches a failure and, like everything else around here, it's an expression that evaluates to whichever block ran. The rescued value is a dictionary with `:message`, `:file`, `:line` and `:column`:

```swift
println(try
  1 / 0
rescue e
  "caught: #{e.message}"
end)
```

Every runtime error is catchable, including the ones the runtime raised itself. Whether a division by zero is a bug or a validation outcome is the caller's call and not the language's. The name after `rescue` is optional. A `return` inside a `try` isn't a failure, so it unwinds to its function as usual.

`Result.attempt` bridges the two shapes:

```swift
println(Result.attempt(() -> 1 / 0))
```

The library draws the line at data versus misuse. `Dict.insert`, `Dict.update` and `Dict.delete` answer with tagged results, because a key that is or isn't there is an ordinary outcome. Passing the wrong kind of thing, like `Math.max("a", 1)`, still raises, because that one is a mistake in the caller.

## Files, Arguments, Environment and Time

`prompt` used to be the only way an Aria program could reach the outside world. `File`, `OS` and `Time` are the rest of it.

Everything that can fail answers with a tagged result, so a file that isn't there is something you handle instead of something that ends the program:

```swift
let path = "config.txt"
println(switch File.read(path)
case [:ok, let contents] then "read #{String.count(contents)} characters"
case [:error, let why] then "could not read: #{why}"
end)
```

`File` has `read`, `lines`, `write`, `append`, `remove` and `exists?`. `OS.args()` is everything after the source file on the command line, and `OS.env(name, fallback)` reads the environment:

```
aria run script.ari one two
```

```swift
println(OS.args())
println(OS.env("HOME", "unknown"))
```

`Time` has two clocks, because they answer different questions. `Time.now()` is milliseconds since the Unix epoch and that's the one you write down. `Time.monotonic()` is nanoseconds from an arbitrary origin and that's the one you subtract, seeing as a wall clock can move backwards on you:

```swift
let start = Time.monotonic()
var total = 0
for i in 1..1000
  total += i
end
println(Time.since(start) > 0)
```

There's no sandbox here. A program reads and writes any path the process can, so running untrusted Aria source is exactly the same as running untrusted code.

## Standard Library

The Standard Library is fully written in Aria with the help of a few essential functions provided by the runtime. That is currently the best source to check out some "production" Aria code and see what it's capable of. [Read the documentation](https://github.com/fadion/aria/wiki/Standard-Library).

Nine modules come with it:

**`String`**: `count`, `isEmpty?`, `first`, `last`, `lower`, `upper`, `capitalize`, `reverse`, `slice`, `repeat`, `padLeft`, `padRight`, `trim`, `trimLeft`, `trimRight`, `join`, `split`, `lines`, `words`, `indexOf`, `lastIndexOf`, `starts?`, `ends?`, `contains?`, `replace`, `match?`.

`trim` and its two halves strip whitespace, unless you hand them a set of characters to strip instead, and they strip every leading or trailing occurrence of it:

```swift
String.trim("  hi  ")        // "hi"
String.trimLeft("xxhi", "x") // "hi"
```

`indexOf` and `lastIndexOf` answer with `-1` when there's nothing to find, so "not found" doesn't get confused with "found at 0".

**`Math`**: `pi`, `e`, `infinity`, `nan`, `floor`, `ceil`, `round`, `trunc`, `max`, `min`, `clamp`, `random`, `abs`, `sign`, `pow`, `sqrt`, `cbrt`, `exp`, `log`, `log2`, `log10`, `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `isNaN?`, `isInfinite?`.

`floor`, `ceil`, `round` and `trunc` answer with an Integer and raise instead of converting when out of range. `max` and `min` are variadic:

```swift
Math.round(-2.5)     // -3, halves round away from zero
Math.max(3, 7, 1)    // 7
Math.clamp(15, 0, 10) // 10
```

**`Enum`**: `size`, `empty?`, `first`, `last`, `reverse`, `insert`, `delete`, `map`, `filter`, `reduce`, `find`, `contains?`, `indexOf`, `unique`, `random`, `sort`, `sortBy`, `each`, `sum`, `min`, `max`, `count`, `any?`, `all?`, `take`, `drop`, `takeWhile`, `dropWhile`, `zip`, `concat`, `flatten`, `groupBy`, `chunk`.

Sorting orders with the language's own `<`, so numbers order among numbers and text among text, and a pair that `<` can't compare is an error instead of some invented ordering across types. `sortBy` takes a key function:

```swift
Enum.sort([3, 1, 2])                                  // [1, 2, 3]
Enum.sortBy(["bbb", "a", "cc"], (s) -> String.count(s)) // ["a", "cc", "bbb"]
```

**`Dict`**: `size`, `empty?`, `keys`, `values`, `get`, `has?`, `contains?`, `insert`, `update`, `delete`, `merge`, `map`, `filter`, `toPairs`, `fromPairs`.

`get` takes a fallback, which `dict[key]` can't express, seeing as it has no way to tell a missing key from one whose value is `nil`:

```swift
let user = [:name => "Ada"]
println(Dict.get(user, :city, "unknown"))
```

**`Result`**: `ok`, `error`, `ok?`, `error?`, `unwrap`, `expect`, `reason`, `attempt`. See [Errors](#errors).

**`File`**: `read`, `lines`, `write`, `append`, `remove`, `exists?`. **`OS`**: `args`, `env`, `env?`. **`Time`**: `now`, `monotonic`, `since`, `milliseconds`, `seconds`.

And **`Type`**, which covers type inspection and conversion, as you've seen earlier.
