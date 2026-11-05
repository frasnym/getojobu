package http

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Code is a machine-readable error code.
type Code string

const (
	CodeInternal         Code = "internal"
	CodeInvalidRequest   Code = "invalid_request"
	CodeNotFound         Code = "not_found"
	CodeUnauthorized     Code = "unauthorized"
	CodeForbidden        Code = "forbidden"
	CodeConflict         Code = "conflict"
	CodeTooManyRequests  Code = "too_many_requests"
	CodeMethodNotAllowed Code = "method_not_allowed"
)

// FieldError describes a single invalid request field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error is the canonical API error envelope.
type Error struct {
	Code      Code         `json:"code"`
	Message   string       `json:"message"`
	RequestID string       `json:"request_id,omitempty"`
	Fields    []FieldError `json:"fields,omitempty"`
}

// Envelope constructs an Error with the given code, message, and request id.
func Envelope(code Code, message string, requestID string) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}
}

// AddField appends a field-level validation error.
func (e *Error) AddField(field, message string) {
	e.Fields = append(e.Fields, FieldError{Field: field, Message: message})
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s: %s (request_id=%s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// StatusCode maps an error code to an HTTP status code.
func (e *Error) StatusCode() int {
	switch e.Code {
	case CodeInvalidRequest:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeConflict:
		return http.StatusConflict
	case CodeTooManyRequests:
		return http.StatusTooManyRequests
	case CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	default:
		return http.StatusInternalServerError
	}
}

// WriteJSON writes the error as a JSON response with the mapped status code.
func (e *Error) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode())
	if err := json.NewEncoder(w).Encode(e); err != nil {
		http.Error(w, `{"code":"internal","message":"failed to write error response"}`, http.StatusInternalServerError)
	}
}

// IsAPIError reports whether err is an *Error.
func IsAPIError(err error) bool {
	_, ok := err.(*Error)
	return ok
}

// AsAPIError returns err as *Error, or nil if err is not an *Error.
func AsAPIError(err error) *Error {
	if e, ok := err.(*Error); ok {
		return e
	}
	return nil
}

// Wrap returns err if it is already an *Error, otherwise wraps it as CodeInternal.
func Wrap(err error, code Code) *Error {
	if err == nil {
		return nil
	}
	if e := AsAPIError(err); e != nil {
		return e
	}
	return Envelope(code, err.Error(), "")
}

// Ensure compile-time interface satisfaction.
var _ error = (*Error)(nil)
