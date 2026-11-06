package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespondJSON_writes_status_and_body(t *testing.T) {
	// Arrange.
	w := httptest.NewRecorder()

	// Act.
	err := RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, http.StatusOK, w.Code, "expected 200")
	assert.Equal(t, `{"status":"ok"}`+"\n", w.Body.String(), "expected JSON body")
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
