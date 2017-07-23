package interpreter

// Scope represents the variable scope.
type Scope struct {
	store  map[string]DataType
	parent *Scope
}

// NewScope initializes an empty scope.
func NewScope() *Scope {
	return &Scope{
		store: make(map[string]DataType),
	}
}

// NewScopeFrom initializes a scope by inheriting
// from a parent.
func NewScopeFrom(parent *Scope) *Scope {
	return &Scope{
		store:  make(map[string]DataType),
		parent: parent,
	}
}

// Read returns a variable from the scope.
func (s *Scope) Read(name string) (DataType, bool) {
	value, ok := s.store[name]
	if !ok && s.parent != nil {
		value, ok = s.parent.Read(name)
	}

	return value, ok
}

// Write saves a variable to the scope.
func (s *Scope) Write(name string, value DataType) {
	s.store[name] = value
}

// Adds scope to the current scope.
func (s *Scope) Merge(scope *Scope) {
	for k, v := range scope.store {
		if _, ok := s.store[k]; !ok {
			s.store[k] = v
		}
	}
}
