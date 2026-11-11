package httpx

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
)

// Pagination holds pagination metadata for list responses.
type Pagination struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	TotalItems int  `json:"totalItems"`
	TotalPages int  `json:"totalPages"`
	HasNext    bool `json:"hasNext"`
	HasPrev    bool `json:"hasPrev"`
}

// Meta holds metadata for every API response.
type Meta struct {
	Timestamp  string      `json:"timestamp"`
	RequestID  string      `json:"requestId"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Response is the canonical API response contract.
type Response[T any] struct {
	Success bool          `json:"success"`
	Data    T             `json:"data"`
	Meta    Meta          `json:"meta"`
	Error   *ErrorPayload `json:"error"`
}

// ResponseOption customizes the response metadata.
type ResponseOption func(*Meta)

// WithPagination attaches pagination metadata.
func WithPagination(pagination Pagination) ResponseOption {
	return func(m *Meta) {
		m.Pagination = &pagination
	}
}

// WithPaginationParams calculates and attaches pagination metadata.
func WithPaginationParams(page, limit, totalItems int) ResponseOption {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}
	hasNext := page < totalPages
	hasPrev := page > 1 && totalPages > 1
	return func(m *Meta) {
		m.Pagination = &Pagination{
			Page:       page,
			Limit:      limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    hasNext,
			HasPrev:    hasPrev,
		}
	}
}

// Base provides shared request/response helpers for HTTP handlers.
type Base struct{}

// ParseJSON decodes the request body into v. Returns a 400 error envelope on malformed JSON.
func (b *Base) ParseJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return Envelope(CodeInvalidRequest, "malformed request body", "")
	}
	return nil
}

// RespondJSON writes data wrapped in the canonical response envelope with the given status code.
func (b *Base) RespondJSON(w http.ResponseWriter, r *http.Request, code int, data any, opts ...ResponseOption) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	meta := Meta{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: extractRequestID(w, r),
	}
	for _, opt := range opts {
		opt(&meta)
	}

	resp := Response[any]{
		Success: true,
		Data:    data,
		Meta:    meta,
		Error:   nil,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, `{"success":false,"data":null,"meta":{"timestamp":"","requestId":""},"error":{"code":"INTERNAL_SERVER_ERROR","message":"failed to write response"}}`, http.StatusInternalServerError)
		return err
	}
	return nil
}

// RespondError writes an *Error as the canonical error response envelope.
func (b *Base) RespondError(w http.ResponseWriter, r *http.Request, err *Error) error {
	err.writeJSON(w, r)
	return nil
}

// ExtractPathParam returns the named path parameter from the request.
func (b *Base) ExtractPathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}

// RespondJSON writes data wrapped in the canonical response envelope with the given status code.
func RespondJSON(w http.ResponseWriter, r *http.Request, code int, data any, opts ...ResponseOption) error {
	return (&Base{}).RespondJSON(w, r, code, data, opts...)
}

// RespondError writes an *Error as the canonical error response envelope.
func RespondError(w http.ResponseWriter, r *http.Request, err *Error) error {
	return (&Base{}).RespondError(w, r, err)
}

// ParseJSON decodes the request body into v.
func ParseJSON(r *http.Request, v any) error {
	return (&Base{}).ParseJSON(r, v)
}

// ExtractPathParam returns the named path parameter from the request.
func ExtractPathParam(r *http.Request, name string) string {
	return (&Base{}).ExtractPathParam(r, name)
}

// HandlerFunc is an http.HandlerFunc that returns an error.
// This lets handlers express logic that can fail (DB calls, validation,
// etc.) and return errors instead of writing responses inline.
// The ServeHTTP method converts a returned error into the standard JSON
// error envelope.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// ServeHTTP implements http.Handler.
func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		RespondError(w, r, Wrap(err, CodeInternal))
	}
}

func extractRequestID(w http.ResponseWriter, r *http.Request) string {
	if r != nil {
		if reqID := ctxval.RequestIDFromContext(r.Context()); reqID != "" {
			return reqID
		}
		if reqID := r.Header.Get("X-Request-Id"); reqID != "" {
			return reqID
		}
		if reqID := r.Header.Get("X-Correlation-Id"); reqID != "" {
			return reqID
		}
	}
	if w != nil {
		if reqID := w.Header().Get("X-Request-Id"); reqID != "" {
			return reqID
		}
	}
	return ""
}
