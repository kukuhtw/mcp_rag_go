// internal/util/errors.go
// Definisi error aplikasi standar (stub)

package util

import "fmt"

type AppError struct {
	Code    string // e.g., "bad_input", "not_found", "internal"
	Message string
}

func (e AppError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func BadInput(msg string) AppError   { return AppError{Code: "bad_input", Message: msg} }
func NotFound(msg string) AppError   { return AppError{Code: "not_found", Message: msg} }
func Internal(msg string) AppError   { return AppError{Code: "internal", Message: msg} }
