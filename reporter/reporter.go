package reporter

import (
	"fmt"
	"github.com/fadion/aria/token"
)

// ErrorType is the type of error.
type ErrorType string

// Error types.
const (
	PARSE   ErrorType = "Parse Error"
	RUNTIME ErrorType = "Runtime Error"
)

// Error store.
var errors []string

// Error adds a new error to the store.
func Error(errortype ErrorType, location token.Location, message string) {
	errors = append(errors, fmt.Sprintf("%s [Line %d]: %s", errortype, location.Row, message))
}

// HasErrors checks if there are errors.
func HasErrors() bool {
	return len(errors) > 0
}

// GetErrors returns the array of errors.
func GetErrors() []string {
	return errors
}

// ClearErrors clears the errors.
func ClearErrors() {
	errors = []string{}
}
