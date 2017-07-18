package interpreter

import "testing"

func TestLibraryGet(t *testing.T) {
	l := NewLibrary()
	l.Register()

	_, ok := l.Get("Math.pi")
	if !ok {
		t.Errorf("Expected a value but got nothing")
	}
}
