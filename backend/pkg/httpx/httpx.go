// Package httpx provides JSON response and error helpers using the API's
// standard envelope: { "data": ... } on success, { "error": {...} } on failure.
package httpx

import (
	"encoding/json"
	"net/http"
)

// Error codes per backend spec §11.
const (
	CodeValidation     = "validation_error"
	CodeUnauthorized   = "unauthorized"
	CodeForbidden      = "forbidden"
	CodeNotFound       = "not_found"
	CodeConflict       = "conflict"
	CodeInsufficient   = "insufficient_balance"
	CodeBusinessRule   = "business_rule"
	CodeInternal       = "internal"
)

type envelope struct {
	Data  interface{} `json:"data,omitempty"`
	Error *apiError   `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes a success envelope with the given status code.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: data})
}

// Err writes an error envelope with the given status code and error code.
func Err(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: &apiError{Code: code, Message: message}})
}

// StatusForCode maps an error code to its conventional HTTP status.
func StatusForCode(code string) int {
	switch code {
	case CodeValidation:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeInsufficient:
		return http.StatusPaymentRequired
	case CodeBusinessRule:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// DecodeJSON decodes the request body into dst, returning false (and writing a
// validation error) if the body is malformed.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		Err(w, http.StatusBadRequest, CodeValidation, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}
