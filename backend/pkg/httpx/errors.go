package httpx

import (
	"errors"
	"net/http"
)

// AppError is a domain error carrying an API error code and message. Services
// return these; WriteError translates them into the JSON envelope.
type AppError struct {
	Code    string
	Message string
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }

// NewError builds an AppError.
func NewError(code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// Convenience constructors.
func Validation(msg string) *AppError   { return NewError(CodeValidation, msg) }
func NotFound(msg string) *AppError     { return NewError(CodeNotFound, msg) }
func Forbidden(msg string) *AppError    { return NewError(CodeForbidden, msg) }
func Conflict(msg string) *AppError     { return NewError(CodeConflict, msg) }
func BusinessRule(msg string) *AppError { return NewError(CodeBusinessRule, msg) }
func Insufficient(msg string) *AppError { return NewError(CodeInsufficient, msg) }
func ServiceUnavailable(msg string) *AppError {
	return NewError(CodeServiceUnavailable, msg)
}

// WriteError inspects err: if it is an *AppError it uses its code/status,
// otherwise it returns a generic 500.
func WriteError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		Err(w, StatusForCode(appErr.Code), appErr.Code, appErr.Message)
		return
	}
	Err(w, http.StatusInternalServerError, CodeInternal, "unexpected error")
}
