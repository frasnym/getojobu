package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httperr "github.com/frasnym/getojobu/apps/backend/internal/utils/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJSON_returns_error_on_malformed_json(t *testing.T) {
	// Arrange.
	base := &Base{}
	req := httptest.NewRequest(http.MethodPost, "/v1/test", nil)
	req.Body = http.NoBody

	// Act.
	err := base.ParseJSON(req, &struct{}{})

	// Assert.
	require.Error(t, err, "expected error for malformed body")
	require.IsType(t, &httperr.Error{}, err, "expected *httperr.Error")
	apiErr := err.(*httperr.Error)
	assert.Equal(t, httperr.CodeInvalidRequest, apiErr.Code, "expected CodeInvalidRequest")
}

func TestRespondJSON_writes_status_and_body(t *testing.T) {
	// Arrange.
	base := &Base{}
	w := httptest.NewRecorder()

	// Act.
	base.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code, "expected 200")
	assert.Equal(t, `{"status":"ok"}`+"\n", w.Body.String(), "expected JSON body")
}
