package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	contractUsecase "github.com/frasnym/getojobu/apps/backend/internal/contract/usecase"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth_returns_200_with_status_ok(t *testing.T) {
	// Arrange.
	h := New(&contract.Contract{
		Common:  &common.Contract{},
		Usecase: &contractUsecase.Contract{},
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Act.
	err := h.Health(w, req)

	// Assert.
	require.NoError(t, err, "unexpected error")
	assert.Equal(t, http.StatusOK, w.Code, "expected 200")

	var resp httpx.Response[StatusResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "ok", resp.Data.Status)
	assert.Nil(t, resp.Error)
	assert.NotEmpty(t, resp.Meta.Timestamp)
}
