package interpreter

// Library represents the standard library.
type Library struct {
	store map[string]libraryFunc
}

// Alias to a func.
type libraryFunc func(args ...DataType) (DataType, error)

// NewLibrary initializes a Library.
func NewLibrary() *Library {
	return &Library{
		store: map[string]libraryFunc{},
	}
}

// Register bootstraps the registration process.
func (l *Library) Register() {
	l.store["Math.pi"] = mathPi
	l.store["Math.ceil"] = mathCeil
	l.store["Math.floor"] = mathFloor
	l.store["Math.max"] = mathMax
	l.store["Math.min"] = mathMin

	l.store["Type.of"] = typeOf
	l.store["Type.toString"] = typeToString
	l.store["Type.toInt"] = typeToInt
	l.store["Type.toFloat"] = typeToFloat
	l.store["Type.toArray"] = typeToArray

	l.store["Enum.size"] = enumSize
	l.store["Enum.reverse"] = enumReverse
	l.store["Enum.first"] = enumFirst
	l.store["Enum.last"] = enumLast
	l.store["Enum.insert"] = enumInsert
	l.store["Enum.delete"] = enumDelete
	l.store["Enum.map"] = enumMap

	l.store["Dict.size"] = dictSize
	l.store["Dict.has"] = dictHas
	l.store["Dict.insert"] = dictInsert
	l.store["Dict.delete"] = dictDelete

	l.store["String.count"] = stringCount
	l.store["String.countBytes"] = stringCountBytes
	l.store["String.lower"] = stringLower
	l.store["String.upper"] = stringUpper
	l.store["String.capitalize"] = stringCapitalize
	l.store["String.trim"] = stringTrim
	l.store["String.trimLeft"] = stringTrimLeft
	l.store["String.trimRight"] = stringTrimRight
	l.store["String.replace"] = stringReplace
	l.store["String.join"] = stringJoin
	l.store["String.split"] = stringSplit
	l.store["String.contains?"] = stringContains
	l.store["String.reverse"] = stringReverse
	l.store["String.slice"] = stringSlice
	l.store["String.match?"] = stringMatch
	l.store["String.starts?"] = stringStarts
	l.store["String.ends?"] = stringEnds
	l.store["String.first"] = stringFirst

	l.store["IO.puts"] = ioPuts
	l.store["IO.write"] = ioWrite
}

// Get returns a function from the library.
func (l *Library) Get(function string) (libraryFunc, bool) {
	_, ok := l.store[function]
	if ok {
		return l.store[function], true
	}

	return nil, false
}
