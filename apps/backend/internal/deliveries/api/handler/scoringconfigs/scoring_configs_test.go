package scoringConfigs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/scoringConfigs/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

func newTestHandler(ctrl *gomock.Controller) (*handler, *ucMocks.MockScoringConfig) {
	mockUC := ucMocks.NewMockScoringConfig(ctrl)
	cfg := &config.Config{
		MachineAuth: config.MachineAuth{
			APIKeys: map[string]config.MachineKey{
				"test-key": {ActorType: "worker", ActorID: "python-worker"},
			},
		},
	}
	h := &handler{
		cfg:             cfg,
		ucScoringConfig: mockUC,
	}
	return h, mockUC
}

func machinePrincipalCtx() context.Context {
	p := &models.Principal{ActorType: "worker", ActorID: "python-worker"}
	return ctxval.ContextWithPrincipal(context.Background(), p)
}

func TestCreateScoringConfig_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, mockUC := newTestHandler(ctrl)

	body := `{
		"family": "technology",
		"version": 1,
		"categories": [
			{"name": "technical_skills", "weight": 0.6, "description": "skills"},
			{"name": "experience", "weight": 0.4, "description": "exp"}
		],
		"is_active": true
	}`

	expected := &models.ScoringConfig{
		ID:       uuid.New(),
		Family:   "technology",
		Version:  1,
		IsActive: true,
	}

	mockUC.EXPECT().Create(gomock.Any(), gomock.Any()).Return(expected, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/scoring-configs", strings.NewReader(body))
	req = req.WithContext(machinePrincipalCtx())
	w := httptest.NewRecorder()

	err := h.CreateScoringConfig(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp httpx.Response[*models.ScoringConfig]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, expected.ID, resp.Data.ID)
}

func TestGetActiveScoringConfig_MissingFamilyParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, _ := newTestHandler(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs/active", nil)
	req = req.WithContext(machinePrincipalCtx())
	w := httptest.NewRecorder()

	err := h.GetActiveScoringConfig(w, req)
	assert.Error(t, err)
	var httpErr *httpx.Error
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, httpx.CodeInvalidRequest, httpErr.Code)
}

func TestGetActiveScoringConfig_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, mockUC := newTestHandler(ctrl)

	expected := &models.ScoringConfig{
		ID:       uuid.New(),
		Family:   "technology",
		Version:  1,
		IsActive: true,
	}

	mockUC.EXPECT().GetActiveByFamily(gomock.Any(), "technology").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs/active?family=technology", nil)
	req = req.WithContext(machinePrincipalCtx())
	w := httptest.NewRecorder()

	err := h.GetActiveScoringConfig(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp httpx.Response[*models.ScoringConfig]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, expected.ID, resp.Data.ID)
}

func TestGetScoringConfigByID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, mockUC := newTestHandler(ctrl)
	id := uuid.New()

	mockUC.EXPECT().GetByID(gomock.Any(), id).Return(nil, constants.ErrScoringConfigNotFound)

	req := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs/"+id.String(), nil)
	req.SetPathValue("id", id.String())
	req = req.WithContext(machinePrincipalCtx())
	w := httptest.NewRecorder()

	err := h.GetScoringConfigByID(w, req)
	assert.Error(t, err)
	var httpErr *httpx.Error
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, httpx.CodeNotFound, httpErr.Code)
}

func TestUpdateScoringConfig_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, mockUC := newTestHandler(ctrl)
	id := uuid.New()

	body := `{"is_active": false}`
	expected := &models.ScoringConfig{
		ID:       id,
		Family:   "technology",
		Version:  1,
		IsActive: false,
	}

	mockUC.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(expected, nil)

	req := httptest.NewRequest(http.MethodPatch, "/v1/scoring-configs/"+id.String(), strings.NewReader(body))
	req.SetPathValue("id", id.String())
	req = req.WithContext(machinePrincipalCtx())
	w := httptest.NewRecorder()

	err := h.UpdateScoringConfig(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp httpx.Response[*models.ScoringConfig]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.False(t, resp.Data.IsActive)
}

func TestCreateScoringConfig_DuplicateVersion_Conflict409(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, mockUC := newTestHandler(ctrl)

	body := `{
		"family": "technology",
		"version": 1,
		"categories": [
			{"name": "technical_skills", "weight": 1.0, "description": "skills"}
		]
	}`

	mockUC.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, constants.ErrDuplicateScoringConfigVersion)

	req := httptest.NewRequest(http.MethodPost, "/v1/scoring-configs", strings.NewReader(body))
	req = req.WithContext(machinePrincipalCtx())
	w := httptest.NewRecorder()

	err := h.CreateScoringConfig(w, req)
	assert.Error(t, err)

	var httpErr *httpx.Error
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, httpx.CodeConflict, httpErr.Code)
}

func TestListScoringConfigs_InvalidIsActiveQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, _ := newTestHandler(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs?is_active=notabool", nil)
	req = req.WithContext(machinePrincipalCtx())
	w := httptest.NewRecorder()

	err := h.ListScoringConfigs(w, req)
	assert.Error(t, err)

	var httpErr *httpx.Error
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, httpx.CodeInvalidRequest, httpErr.Code)
}

type stubUserUC struct{}

func (stubUserUC) FindByClerkUserID(context.Context, string) (*models.Principal, error) {
	return nil, nil
}
func (stubUserUC) FindByID(context.Context, string) (*models.User, error) { return nil, nil }
func (stubUserUC) Create(context.Context, string, string, string, models.Role) (*models.User, error) {
	return nil, nil
}
func (stubUserUC) Update(context.Context, string, string, string) (*models.User, error) {
	return nil, nil
}
func (stubUserUC) SoftDelete(context.Context, string) (*models.User, error) { return nil, nil }

func TestRoute_ValidAPIKey_Enforcement(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := ucMocks.NewMockScoringConfig(ctrl)
	cfg := &config.Config{
		MachineAuth: config.MachineAuth{
			APIKeys: map[string]config.MachineKey{
				"valid-machine-key": {ActorType: "worker", ActorID: "worker-1"},
			},
		},
	}
	h := &handler{
		cfg:             cfg,
		userUC:          stubUserUC{},
		ucScoringConfig: mockUC,
	}

	router := httpx.New()
	h.Route(router.Group("/v1/scoring-configs"))

	// 1. Missing Authorization header -> 401
	reqMissing := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs", nil)
	wMissing := httptest.NewRecorder()
	router.Mux().ServeHTTP(wMissing, reqMissing)
	assert.Equal(t, http.StatusUnauthorized, wMissing.Code)

	// 2. Invalid or Clerk Bearer token -> 401
	reqInvalid := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs", nil)
	reqInvalid.Header.Set("Authorization", "Bearer invalid-clerk-token")
	wInvalid := httptest.NewRecorder()
	router.Mux().ServeHTTP(wInvalid, reqInvalid)
	assert.Equal(t, http.StatusUnauthorized, wInvalid.Code)

	// 3. Valid Machine API Key -> Success (calls handler -> mockUC)
	mockUC.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*models.ScoringConfig{}, nil)
	reqValid := httptest.NewRequest(http.MethodGet, "/v1/scoring-configs", nil)
	reqValid.Header.Set("Authorization", "Bearer valid-machine-key")
	wValid := httptest.NewRecorder()
	router.Mux().ServeHTTP(wValid, reqValid)
	assert.Equal(t, http.StatusOK, wValid.Code)
}
