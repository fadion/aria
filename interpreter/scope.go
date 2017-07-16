package interpreter

// Scope.
type Scope struct {
	store  map[string]DataType
	parent *Scope
}

// Initializes an empty scope.
func NewScope() *Scope {
	return &Scope{
		store: make(map[string]DataType),
	}
}

// Initializes a scope by inheriting from
// a parent.
func NewScopeFrom(parent *Scope) *Scope {
	return &Scope{
		store:  make(map[string]DataType),
		parent: parent,
	}
}

// Read a variable from the scope.
func (s *Scope) Read(name string) (DataType, bool) {
	value, ok := s.store[name]
	if !ok && s.parent != nil {
		value, ok = s.parent.Read(name)
	}

	return value, ok
}

// Write a variable to the scope.
func (s *Scope) Write(name string, value DataType) {
	s.store[name] = value
}
