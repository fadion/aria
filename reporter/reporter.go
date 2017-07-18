package reporter

import (
	"fmt"
	"github.com/fadion/aria/token"
)

// String alias.
type ErrorType string

// Error types.
const (
	PARSE   ErrorType = "Parse Error"
	RUNTIME ErrorType = "Runtime Error"
)

// Error store.
var errors []string

// Add a new error to the store.
func Error(errortype ErrorType, location token.Location, message string) {
	errors = append(errors, fmt.Sprintf("%s [Line %d]: %s", errortype, location.Row, message))
}

// Check if there are errors.
func HasErrors() bool {
	return len(errors) > 0
}

// Get the array of errors.
func GetErrors() []string {
	return errors
}

// Clear the errors.
func ClearErrors() {
	errors = []string{}
}
