package reporter

import (
	"fmt"
	"github.com/fadion/aria/token"
	"testing"
)

func TestError(t *testing.T) {
	errors = []string{}
	Error(PARSE, token.Location{1, 1}, "Test error 1")
	Error(PARSE, token.Location{1, 1}, "Test error 2")

	if len(errors) != 2 {
		t.Errorf("Expected %d but got %d", 2, len(errors))
	}
}

func TestGetErrors(t *testing.T) {
	errors = []string{}
	Error(PARSE, token.Location{1, 1}, "Test error 1")
	Error(RUNTIME, token.Location{2, 1}, "Test error 2")

	expected := []string{
		fmt.Sprintf("%s [Line %d:%d]: %s", PARSE, 1, 1, "Test error 1"),
		fmt.Sprintf("%s [Line %d:%d]: %s", RUNTIME, 2, 1, "Test error 2"),
	}

	for i, k := range errors {
		if k != expected[i] {
			t.Errorf("Expected %s but got %s", expected[i], k)
		}
	}
}

func TestClearErrors(t *testing.T) {
	errors = []string{}
	Error(PARSE, token.Location{1, 1}, "Test error 1")
	Error(PARSE, token.Location{1, 1}, "Test error 2")

	ClearErrors()

	if len(errors) != 0 {
		t.Errorf("Expected %d but got %d", 0, len(errors))
	}
}
