package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Code is a machine-readable error code.
type Code string

const (
	CodeInternal         Code = "INTERNAL_SERVER_ERROR"
	CodeInvalidRequest   Code = "VALIDATION_FAILED"
	CodeNotFound         Code = "NOT_FOUND"
	CodeUnauthorized     Code = "UNAUTHORIZED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeConflict         Code = "CONFLICT"
	CodeTooManyRequests  Code = "TOO_MANY_REQUESTS"
	CodeMethodNotAllowed Code = "METHOD_NOT_ALLOWED"
)

// ErrorDetail describes a single field-level issue.
type ErrorDetail struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// FieldError is an alias for ErrorDetail for backward compatibility.
type FieldError = ErrorDetail

// ErrorPayload describes the error in an error response.
type ErrorPayload struct {
	Code    Code          `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// Error is the canonical API error envelope representation.
type Error struct {
	Code      Code          `json:"code"`
	Message   string        `json:"message"`
	RequestID string        `json:"request_id,omitempty"`
	Details   []ErrorDetail `json:"details,omitempty"`
}

// Envelope constructs an Error with the given code, message, and request id.
func Envelope(code Code, message string, requestID string) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}
}

// AddDetail appends a field-level issue.
func (e *Error) AddDetail(field, issue string) {
	e.Details = append(e.Details, ErrorDetail{Field: field, Issue: issue})
}

// AddField appends a field-level validation issue (alias for AddDetail).
func (e *Error) AddField(field, message string) {
	e.AddDetail(field, message)
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
	e.writeJSON(w, nil)
}

func (e *Error) writeJSON(w http.ResponseWriter, r *http.Request) {
	reqID := e.RequestID
	if reqID == "" {
		reqID = extractRequestID(w, r)
	}

	meta := Meta{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: reqID,
	}

	payload := Response[any]{
		Success: false,
		Data:    nil,
		Meta:    meta,
		Error: &ErrorPayload{
			Code:    e.Code,
			Message: e.Message,
			Details: e.Details,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode())
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, `{"success":false,"data":null,"meta":{"timestamp":"","requestId":""},"error":{"code":"INTERNAL_SERVER_ERROR","message":"failed to write error response"}}`, http.StatusInternalServerError)
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
