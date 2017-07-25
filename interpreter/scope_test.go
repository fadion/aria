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

func TestScopeUpdate(t *testing.T) {
	sp := NewScope()
	sp.Write("dec", &IntegerType{Value: 10})
	s := NewScopeFrom(sp)

	s.Update("dec", &IntegerType{Value: 20})

	val, ok := s.Read("dec")
	valP, okP := sp.Read("dec")
	if !ok || !okP {
		t.Errorf("Expected a value but got nothing")
	}

	value, ok := val.(*IntegerType)
	valueP, okP := valP.(*IntegerType)
	if !ok || !okP {
		t.Errorf("Expected an IntegerType but got %T", val)
	}

	if value.Value != 20 || valueP.Value != 20 {
		t.Errorf("Expected %d but got %d", 20, value.Value)
	}
}

func TestScopeMerge(t *testing.T) {
	sp := NewScope()
	sp.Write("dec", &IntegerType{Value: 10})

	s := NewScope()
	s.Write("str", &StringType{Value: "test"})
	s.Merge(sp)

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