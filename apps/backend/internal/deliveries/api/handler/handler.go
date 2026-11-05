package handler

import (
	"encoding/json"
	"net/http"

	httperr "github.com/frasnym/getojobu/apps/backend/internal/utils/http"
)

// Base provides shared request/response helpers for HTTP handlers.
type Base struct{}

// ParseJSON decodes the request body into v. Returns a 400 error envelope on malformed JSON.
func (b *Base) ParseJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return httperr.Envelope(httperr.CodeInvalidRequest, "malformed request body", "")
	}
	return nil
}

// RespondJSON writes v as a JSON response with the given status code.
func (b *Base) RespondJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"code":"internal","message":"failed to write response"}`, http.StatusInternalServerError)
	}
}

// RespondError writes an *httperr.Error as a JSON response.
func (b *Base) RespondError(w http.ResponseWriter, err *httperr.Error) {
	err.WriteJSON(w)
}

// ExtractPathParam returns the named path parameter from the request.
func (b *Base) ExtractPathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}
