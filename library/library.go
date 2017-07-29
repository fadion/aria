package library

// There are alternatives to include static assets
// in the Go binary, but didn't want to use external
// libraries. For the moment, this is the only good
// solution I can think of.
var Modules = []string{
	`module Enum

  let size = func (array: Array) -> Int
    var count = 0
    for v in array
      count = count + 1
    end
    count
  end

  let empty? = func (array: Array) -> Bool
    size(array) == 0
  end

  let reverse = func (array: Array) -> Array
    var reversed = []
    for i in size(array)-1..0
      reversed[] = array[i]
    end
    reversed
  end

  let first = func (array: Array)
    array[0]
  end

  let last = func array: Array
    array[size(array) - 1]
  end

  let insert = func (array: Array, el) -> Array
    array[] = el
  end

  let delete = func (array: Array, index) -> Array
    var purged = []
    for i, v in array
      if i != index
        purged[] = v
      end
    end
    purged
  end

  let map = func (array: Array, fn: Function) -> Array
    for v in array
      fn(v)
    end
  end

  let filter = func (array: Array, fn: Function) -> Array
    var filtered = []
    for v in array
      if fn(v)
        filtered[] = v
      end
    end
    filtered
  end

  let reduce = func (array: Function, start, fn: Function)
    var acc = start
    for v in array
      acc = fn(v, acc)
    end
    return acc
  end

  let find = func (array: Array, fn: Function)
    for v in array
      if fn(v)
        return v
      end
    end
    nil
  end

  let contains? = func (array: Array, search) -> Bool
    for v in array
      if v == search
        return true
      end
    end
    false
  end

  let unique = func (array: Array) -> Array
    var filtered = []
    var hash = [=>]
    for i, v in array
      if hash[v] == nil
        hash[v] = i
        filtered[] = v
      end
    end
    filtered
  end

  let random = func (array: Array)
    var rnd = rand(0, size(array) - 1)
    array[rnd]
  end

end
`,

	`module Math

  let pi = 3.14159265359
  let e = 2.718281828459

  let floor = func (nr: Float) -> Int
    Int(nr - nr % 1)
  end

  let ceil = func (nr: Float) -> Int
    let rem = nr % 1
    if rem == 0
      return Int(nr)
    end
    nr > 0 ? Int(nr + (1 - rem)) : Int(nr - (1 + rem))
  end

  let max = func (nr1, nr2)
    if !Type.isNumber(nr1) || !Type.isNumber(nr2)
      panic("Math.max() expects a Float or Int")
    end

    return nr1 > nr2 ? nr1 : nr2
  end

  let min = func (nr1, nr2)
    if !Type.isNumber(nr1) || !Type.isNumber(nr2)
      panic("Math.min() expects a Float or Int")
    end

    return nr1 > nr2 ? nr2 : nr1
  end

  let random = func (min: Int, max: Int) -> Int
    runtime_rand(min, max)
  end

  let abs = func (nr)
    if !Type.isNumber(nr)
      panic("Math.abs() expects a Float or Int")
    end

    if nr < 0
      return -nr
    end
    nr
  end

  let pow = func (nr, exp)
    if !Type.isNumber(nr) || !Type.isNumber(exp)
      panic("Math.pow() expects a Float or Int")
    end

    nr ** exp
  end

end`,


	`module Type

  let of = func x
    typeof(x)
  end

  let isNumber = func x
    if typeof(x) == "Float" || typeof(x) == "Int"
      return true
    end
    false
  end

  let toString = func x
    String(x)
  end

  let toInt = func x
    Int(x)
  end

  let toFloat = func x
    Float(x)
  end

  let toArray = func x
    [x]
  end

end`,

	`module Dict
  let size = func (dict: Dictionary) -> Int
    var count = 0
    for v in dict
      count = count + 1
    end
    count
  end

  let contains? = func (dict: Dictionary, key) -> Bool
    for k, v in dict
      if k == key
        return true
      end
    end
    false
  end

  let empty? = func (dict: Dictionary) -> Bool
    size(dict) == 0
  end

  let insert = func (dict: Dictionary, key, value) -> Dictionary
    if dict[key] != nil
      panic("Dictionary key '" + String(key) + "' already exists")
    end

    dict[key] = value
  end

  let update = func (dict: Dictionary, key, value) -> Dictionary
    if dict[key] == nil
      panic("Dictionary key '" + String(key) + "' doesn't exist")
    end

    dict[key] = value
  end

  let delete = func (dict: Dictionary, key) -> Dictionary
    if dict[key] == nil
      panic("Dictionary key '" + String(key) + "' doesn't exist")
    end

    var purged = [=>]
    for k, v in dict
      if k != key
        purged[k] = v
      end
    end
    purged
  end
end`,

	`module String

  let count = func (str: String) -> Int
    var cnt = 0
    for v in str
      cnt = cnt + 1
    end
    cnt
  end

  let first = func (str: String) -> String
    str[0]
  end

  let last = func (str: String) -> String
    str[String.count(str) - 1]
  end

  let lower = func (str: String) -> String
    runtime_tolower(str)
  end

  let upper = func (str: String) -> String
    runtime_toupper(str)
  end

  let capitalize = func (str: String) -> String
    var title = str
    for i, v in str
      if i == 0 || str[i - 1] != nil && str[i - 1] == " "
        title[i] = String.upper(v)
      end
    end
    title
  end

  let reverse = func (str: String) -> String
    var reversed = ""
    for i in String.count(str)-1..0
      reversed = reversed + str[i]
    end
    reversed
  end

  let slice = func (str: String, start: Int, length: Int) -> String
    if start < 0 || length < 0
      panic("String.slice() expects positive start and length parameters")
    end

    var sliced = ""
    var chars = 0
    for i, v in str
      if i >= start && chars < length
        sliced = sliced + v
        chars = chars + 1
      end
    end
    sliced
  end

  let trim = func (str: String, subset: String) -> String
    var trimmed = str
    var left = false
    var right = false

    for i, v in subset
      if trimmed[0] == v && !left
        trimmed = String.slice(trimmed, 1, String.count(trimmed))
        left = true
      end

      if String.last(trimmed) == v && !right
        trimmed = String.slice(trimmed, 0, String.count(trimmed) - 1)
        right = true
      end

      if left && right
        break
      end
    end

    trimmed
  end

  let trimLeft = func (str: String, subset: String) -> String
    var trimmed = str
    for v in subset
      if trimmed[0] == v
        trimmed = String.slice(trimmed, 1, String.count(trimmed))
        break
      end
    end

    trimmed
  end

  let trimRight = func (str: String, subset: String) -> String
    var trimmed = str
    for v in subset
      if String.last(trimmed) == v
        trimmed = String.slice(trimmed, 0, String.count(trimmed) - 1)
        break
      end
    end

    trimmed
  end

  let join = func (array: Array, glue: String) -> String
    var glued = ""
    for v in array
      glued = glued + v + glue
    end

    if String.count(glued) > String.count(glue)
      return String.slice(glued, 0, String.count(glued) - String.count(glue))
    end

    glued
  end

  let split  = func (str: String, separator: String) -> Array
    let count_sep = String.count(separator)
    var array = []
    var last_index = 0

    for i, v in str
        if String.slice(str, i, count_sep) == separator
          var curr = String.slice(str, last_index, i - last_index)
          if curr != ""
            array[] = curr
          end
          last_index = i + count_sep
        end
    end

    array[] = String.slice(str, last_index, String.count(str))

    array
  end

  let starts? = func (str: String, prefix: String) -> Bool
    if String.count(str) < String.count(prefix)
      return false
    end

    if String.slice(str, 0, String.count(prefix)) == prefix
      return true
    end

    false
  end

  let ends? = func (str: String, suffix: String) -> Bool
    if String.count(str) < String.count(suffix)
      return false
    end

    if String.slice(str, String.count(str) - String.count(suffix), String.count(str)) == suffix
      return true
    end

    false
  end

  let contains? = func (str: String, search: String) -> Bool
    for i, v in str
      if String.slice(str, i, String.count(search)) == search
        return true
      end
    end
    false
  end

  let replace = func (str: String, search: String, replace: String) -> String
    let count_search = String.count(search)
    var rpl = ""
    var last_index = 0

    for i, v in str
      if String.slice(str, i, count_search) == search
        rpl = rpl + String.slice(str, last_index, i - last_index) + replace
        last_index = i + count_search
      end
    end

    rpl + String.slice(str, last_index, String.count(str))
  end

  let match? = func (str: String, regex: String) -> Bool
    runtime_regex_match(str, regex)
  end

end`,
}