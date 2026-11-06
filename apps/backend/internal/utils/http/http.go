package http

import (
	"encoding/json"
	"net/http"
)

// Base provides shared request/response helpers for HTTP handlers.
type Base struct{}

// ParseJSON decodes the request body into v. Returns a 400 error envelope on malformed JSON.
func (b *Base) ParseJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return Envelope(CodeInvalidRequest, "malformed request body", "")
	}
	return nil
}

// RespondJSON writes v as a JSON response with the given status code.
// Returns nil so callers can write: return httperf.RespondJSON(w, code, v).
func (b *Base) RespondJSON(w http.ResponseWriter, code int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"code":"internal","message":"failed to write response"}`, http.StatusInternalServerError)
		return err
	}
	return nil
}

// RespondError writes an *Error as a JSON response.
// Returns nil so callers can write: return httperf.RespondError(w, err).
func (b *Base) RespondError(w http.ResponseWriter, err *Error) error {
	err.WriteJSON(w)
	return nil
}

// ExtractPathParam returns the named path parameter from the request.
func (b *Base) ExtractPathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}

// RespondJSON writes v as a JSON response with the given status code.
// Returns nil so callers can write: return httperf.RespondJSON(w, code, v).
func RespondJSON(w http.ResponseWriter, code int, v any) error {
	return (&Base{}).RespondJSON(w, code, v)
}

// RespondError writes an *Error as a JSON response.
// Returns nil so callers can write: return httperf.RespondError(w, err).
func RespondError(w http.ResponseWriter, err *Error) error {
	return (&Base{}).RespondError(w, err)
}

// ParseJSON decodes the request body into v. Returns a 400 error envelope
// on malformed JSON. Convenience wrapper around Base.ParseJSON.
func ParseJSON(r *http.Request, v any) error {
	return (&Base{}).ParseJSON(r, v)
}

// ExtractPathParam returns the named path parameter from the request.
// Convenience wrapper around Base.ExtractPathParam.
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
		RespondError(w, Wrap(err, CodeInternal))
	}
}
