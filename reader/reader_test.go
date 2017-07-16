package reader

import "testing"

func TestReader(t *testing.T) {
	r := New([]byte("testing"))
	next, _ := r.Advance()
	if next != 't' {
		t.Errorf("Expected t but got %s", string(next))
	}

	next, _ = r.Advance()
	if next != 'e' {
		t.Errorf("Expected e but got %s", string(next))
	}
}

func TestPeek(t *testing.T) {
	r := New([]byte("testing"))
	next, _ := r.Peek()
	if next != 't' {
		t.Errorf("Expected t but got %s", string(next))
	}

	next, _ = r.Advance()
	if next != 't' {
		t.Errorf("Expected t but got %s", string(next))
	}
}

func TestUnread(t *testing.T) {
	r := New([]byte("testing"))
	r.Advance()
	r.Advance()
	r.Unread()

	next, _ := r.Advance()
	if next != 'e' {
		t.Errorf("Expected e but got %s", string(next))
	}
}
