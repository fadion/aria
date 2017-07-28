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
    rand(min, max)
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
}