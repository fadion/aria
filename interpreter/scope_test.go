package interpreter

import "testing"

func TestScopeReadWrite(t *testing.T) {
	s := NewScope()
	s.Write("dec", &IntegerType{Value: 10})
	val, ok := s.Read("dec")

	if !ok {
		t.Errorf("Expected a value but got nothing")
	}

	value, ok := val.(*IntegerType)
	if !ok {
		t.Errorf("Expected an IntegerType but got %T", val)
	}

	if value.Value != 10 {
		t.Errorf("Expected %d but got %d", 10, value.Value)
	}
}

func TestScopeParent(t *testing.T) {
	sp := NewScope()
	sp.Write("dec", &IntegerType{Value: 10})
	s := NewScopeFrom(sp)

	val, ok := s.Read("dec")
	if !ok {
		t.Errorf("Expected a value but got nothing")
	}

	value, ok := val.(*IntegerType)
	if !ok {
		t.Errorf("Expected an IntegerType but got %T", val)
	}

	if value.Value != 10 {
		t.Errorf("Expected %d but got %d", 10, value.Value)
	}
}
