package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespondJSON_writes_success_envelope(t *testing.T) {
	// Arrange.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	req = req.WithContext(ctxval.WithRequestID(req.Context(), "req_123"))

	// Act.
	err := RespondJSON(w, req, http.StatusOK, map[string]string{"id": "usr_12345", "name": "Jane Doe"})

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, http.StatusOK, w.Code, "expected 200")

	var resp Response[map[string]string]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "usr_12345", resp.Data["id"])
	assert.Equal(t, "Jane Doe", resp.Data["name"])
	assert.Equal(t, "req_123", resp.Meta.RequestID)
	assert.NotEmpty(t, resp.Meta.Timestamp)
	assert.Nil(t, resp.Meta.Pagination)
	assert.Nil(t, resp.Error)
}

func TestRespondJSON_writes_paginated_envelope(t *testing.T) {
	// Arrange.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/items", nil)
	items := []map[string]any{
		{"id": 1, "name": "Item A"},
		{"id": 2, "name": "Item B"},
	}

	// Act.
	err := RespondJSON(w, req, http.StatusOK, items, WithPaginationParams(1, 20, 145))

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response[[]map[string]any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 2)
	require.NotNil(t, resp.Meta.Pagination)
	assert.Equal(t, 1, resp.Meta.Pagination.Page)
	assert.Equal(t, 20, resp.Meta.Pagination.Limit)
	assert.Equal(t, 145, resp.Meta.Pagination.TotalItems)
	assert.Equal(t, 8, resp.Meta.Pagination.TotalPages)
	assert.True(t, resp.Meta.Pagination.HasNext)
	assert.False(t, resp.Meta.Pagination.HasPrev)
	assert.Nil(t, resp.Error)
}

func TestRespondError_writes_error_envelope(t *testing.T) {
	// Arrange.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/test", nil)
	req = req.WithContext(ctxval.WithRequestID(req.Context(), "req_99abc123"))

	errEnvelope := Envelope(CodeInvalidRequest, "The input provided is invalid.", "")
	errEnvelope.AddDetail("email", "Format is invalid.")

	// Act.
	err := RespondError(w, req, errEnvelope)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response[any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Nil(t, resp.Data)
	assert.Equal(t, "req_99abc123", resp.Meta.RequestID)
	assert.NotEmpty(t, resp.Meta.Timestamp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, CodeInvalidRequest, resp.Error.Code)
	assert.Equal(t, "The input provided is invalid.", resp.Error.Message)
	require.Len(t, resp.Error.Details, 1)
	assert.Equal(t, "email", resp.Error.Details[0].Field)
	assert.Equal(t, "Format is invalid.", resp.Error.Details[0].Issue)
}

func TestParseJSON_returns_error_on_malformed_json(t *testing.T) {
	// Arrange.
	req := httptest.NewRequest(http.MethodPost, "/v1/test", nil)
	req.Body = http.NoBody

	// Act.
	err := ParseJSON(req, &struct{}{})

	// Assert.
	require.Error(t, err, "expected error for malformed body")
	require.IsType(t, &Error{}, err, "expected *Error")
	apiErr := err.(*Error)
	assert.Equal(t, CodeInvalidRequest, apiErr.Code, "expected CodeInvalidRequest")
}
