package projects

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newHandlerWithMock(ctrl *gomock.Controller) (*handler, *mocks.MockProjects, *mocks.MockProjectSkills) {
	mockProj := mocks.NewMockProjects(ctrl)
	mockProjSkills := mocks.NewMockProjectSkills(ctrl)
	h := &handler{
		cfg:          &config.Config{},
		ucProj:       mockProj,
		ucProjSkills: mockProjSkills,
	}
	return h, mockProj, mockProjSkills
}

func principalCtx(id string) *models.Principal {
	return &models.Principal{ID: id, Role: models.RoleUser}
}

func bodyAsMap(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestCreateProject_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockProj, _ := newHandlerWithMock(ctrl)

	mockProj.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(&models.Project{ID: "proj_1", UserID: "user_1"}, nil)

	body := strings.NewReader(`{"name":"Website Redesign","description":"Redesign the site","is_current":false}`)
	req := httptest.NewRequest(http.MethodPost, "/projects", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateProject).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusCreated, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "proj_1", m["ID"])
	assert.Equal(t, "user_1", m["UserID"])
}

func TestCreateProject_unauthenticated(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"name":"Project X"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects", body)
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateProject).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateProject_malformed_body(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPost, "/projects", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateProject).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateProject_missing_name(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"description":"No name"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.CreateProject).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetProject_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockProj, _ := newHandlerWithMock(ctrl)

	mockProj.EXPECT().
		GetByID(gomock.Any(), models.GetProjectParams{UserID: "user_1", ID: "proj_1"}).
		Return(&models.Project{ID: "proj_1", UserID: "user_1"}, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /projects/{id}", httpx.HandlerFunc(h.GetProject))

	req := httptest.NewRequest(http.MethodGet, "/projects/proj_1", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "proj_1", m["ID"])
	assert.Equal(t, "user_1", m["UserID"])
}

func TestGetProject_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockProj, _ := newHandlerWithMock(ctrl)

	mockProj.EXPECT().
		GetByID(gomock.Any(), models.GetProjectParams{UserID: "user_1", ID: "proj_1"}).
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("GET /projects/{id}", httpx.HandlerFunc(h.GetProject))

	req := httptest.NewRequest(http.MethodGet, "/projects/proj_1", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetProject_missing_id(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("GET /projects/{id}", httpx.HandlerFunc(h.GetProject))

	req := httptest.NewRequest(http.MethodGet, "/projects/", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert. "/projects/" does not match "/projects/{id}" so ServeMux returns 404.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListProjects_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockProj, _ := newHandlerWithMock(ctrl)

	mockProj.EXPECT().
		ListByUser(gomock.Any(), "user_1").
		Return([]*models.Project{{ID: "proj_1", UserID: "user_1"}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	httpx.HandlerFunc(h.ListProjects).ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	var arr []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	require.Len(t, arr, 1)
	assert.Equal(t, "proj_1", arr[0]["ID"])
	assert.Equal(t, "user_1", arr[0]["UserID"])
}

func TestUpdateProject_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockProj, _ := newHandlerWithMock(ctrl)

	mockProj.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(&models.Project{ID: "proj_1", UserID: "user_1"}, nil)

	mux := http.NewServeMux()
	mux.Handle("PUT /projects/{id}", httpx.HandlerFunc(h.UpdateProject))

	body := strings.NewReader(`{"name":"Updated Project","description":"New desc","is_current":false}`)
	req := httptest.NewRequest(http.MethodPut, "/projects/proj_1", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "proj_1", m["ID"])
	assert.Equal(t, "user_1", m["UserID"])
}

func TestUpdateProject_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, mockProj, _ := newHandlerWithMock(ctrl)

	mockProj.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil, constants.ErrNotFound)

	mux := http.NewServeMux()
	mux.Handle("PUT /projects/{id}", httpx.HandlerFunc(h.UpdateProject))

	body := strings.NewReader(`{"name":"Project X"}`)
	req := httptest.NewRequest(http.MethodPut, "/projects/proj_1", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateProject_missing_id(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PUT /projects/{id}", httpx.HandlerFunc(h.UpdateProject))

	body := strings.NewReader(`{"name":"Project X"}`)
	req := httptest.NewRequest(http.MethodPut, "/projects/", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert. "/projects/" does not match "/projects/{id}" so ServeMux returns 404.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateProject_missing_name(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PUT /projects/{id}", httpx.HandlerFunc(h.UpdateProject))

	body := strings.NewReader(`{"description":"No name"}`)
	req := httptest.NewRequest(http.MethodPut, "/projects/proj_1", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateProject_malformed_body(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("PUT /projects/{id}", httpx.HandlerFunc(h.UpdateProject))

	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPut, "/projects/proj_1", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLinkProjectSkills_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, mockProjSkills := newHandlerWithMock(ctrl)

	mockProjSkills.EXPECT().
		LinkSkills(gomock.Any(), models.LinkProjectSkillsParams{
			UserID:    "user_1",
			ProjectID: "proj_1",
			SkillIDs:  []string{"skill_a", "skill_b"},
		}).
		Return(nil)

	mux := http.NewServeMux()
	mux.Handle("POST /projects/{id}/skills", httpx.HandlerFunc(h.LinkProjectSkills))

	body := strings.NewReader(`{"skill_ids":["skill_a","skill_b"]}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/proj_1/skills", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	m := bodyAsMap(t, w)
	assert.Equal(t, "linked", m["status"])
}

func TestLinkProjectSkills_missing_skill_ids(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("POST /projects/{id}/skills", httpx.HandlerFunc(h.LinkProjectSkills))

	body := strings.NewReader(`{"skill_ids":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/proj_1/skills", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLinkProjectSkills_missing_id(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	body := strings.NewReader(`{"skill_ids":["skill_a"]}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/proj_1/skills", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act. Call handler directly without ServeMux routing so PathValue("id") returns "".
	httpx.HandlerFunc(h.LinkProjectSkills).ServeHTTP(w, req)

	// Assert. Handler validates the missing id and returns 400.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLinkProjectSkills_malformed_body(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)

	mux := http.NewServeMux()
	mux.Handle("POST /projects/{id}/skills", httpx.HandlerFunc(h.LinkProjectSkills))

	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPost, "/projects/proj_1/skills", body)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListProjectSkills_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, mockProjSkills := newHandlerWithMock(ctrl)

	mockProjSkills.EXPECT().
		ListSkillsForProject(gomock.Any(), models.ListProjectSkillsParams{UserID: "user_1", ProjectID: "proj_1"}).
		Return([]*models.UserSkill{{ID: "skill_1", UserID: "user_1", SkillName: "Go"}}, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /projects/{id}/skills", httpx.HandlerFunc(h.ListProjectSkills))

	req := httptest.NewRequest(http.MethodGet, "/projects/proj_1/skills", nil)
	req = req.WithContext(ctxval.ContextWithPrincipal(req.Context(), principalCtx("user_1")))
	w := httptest.NewRecorder()

	// Act.
	mux.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code)
	var arr []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	require.Len(t, arr, 1)
	assert.Equal(t, "skill_1", arr[0]["ID"])
	assert.Equal(t, "user_1", arr[0]["UserID"])
	assert.Equal(t, "Go", arr[0]["SkillName"])
}

func TestRoute_registers_projects_under_group(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	h, _, _ := newHandlerWithMock(ctrl)
	rt := httpx.New()
	group := rt.Group("/v1/projects")

	// Act.
	h.Route(group)

	// Assert via introspection. Patterns are compounded with the group prefix.
	routes := rt.Routes()
	require.Len(t, routes, 6, "expected 6 project routes under /v1/projects")
	expected := []struct {
		method  string
		pattern string
	}{
		{http.MethodPost, "/v1/projects"},
		{http.MethodGet, "/v1/projects"},
		{http.MethodGet, "/v1/projects/{id}"},
		{http.MethodPut, "/v1/projects/{id}"},
		{http.MethodPost, "/v1/projects/{id}/skills"},
		{http.MethodGet, "/v1/projects/{id}/skills"},
	}
	for i, e := range expected {
		assert.Equal(t, e.method, routes[i].Method, "route %d method", i)
		assert.Equal(t, e.pattern, routes[i].Pattern, "route %d pattern", i)
	}
}
