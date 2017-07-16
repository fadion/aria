package interpreter

// Standart library.
type Library struct {
	store map[string]libraryFunc
}

// Alias to a func.
type libraryFunc func(args ...DataType) (DataType, error)

// Initializes a Library.
func NewLibrary() *Library {
	return &Library{
		store: map[string]libraryFunc{},
	}
}

// Bootstrap the registration process.
func (l *Library) Register() {
	l.store["Math.pi"] = math_pi
	l.store["Math.ceil"] = math_ceil
	l.store["Math.floor"] = math_floor
	l.store["Math.max"] = math_max
	l.store["Math.min"] = math_min

	l.store["Type.of"] = type_of
	l.store["Type.toString"] = type_toString
	l.store["Type.toInt"] = type_toInt
	l.store["Type.toFloat"] = type_toFloat

	l.store["Enum.size"] = enum_size
	l.store["Enum.reverse"] = enum_reverse
	l.store["Enum.first"] = enum_first
	l.store["Enum.last"] = enum_last
	l.store["Enum.insert"] = enum_insert
	l.store["Enum.delete"] = enum_delete

	l.store["Dict.size"] = dict_size
	l.store["Dict.has"] = dict_has
	l.store["Dict.insert"] = dict_insert
	l.store["Dict.delete"] = dict_delete

	l.store["String.count"] = string_count
	l.store["String.lower"] = string_lower
	l.store["String.upper"] = string_upper
	l.store["String.capitalize"] = string_capitalize
	l.store["String.trim"] = string_trim
	l.store["String.replace"] = string_replace
	l.store["String.join"] = string_join
	l.store["String.split"] = string_split
	l.store["String.has"] = string_has

	l.store["IO.puts"] = io_puts
	l.store["IO.write"] = io_write
}

// Get a function from the library.
func (l *Library) Get(function string) (libraryFunc, bool) {
	_, ok := l.store[function]
	if ok {
		return l.store[function], true
	} else {
		return nil, false
	}
}
