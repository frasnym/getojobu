package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	contractUsecase "github.com/frasnym/getojobu/apps/backend/internal/contract/usecase"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUserService is a test double for ucUsers.User.
type mockUserService struct {
	createCalled bool
	updateCalled bool
	deleteCalled bool
	createErr    error
	updateErr    error
	deleteErr    error
	lastClerkID  string
	lastEmail    string
	lastFullName string
	lastRole     models.Role
	lastUpdateID string
	lastDeleteID string
}

func (m *mockUserService) FindByClerkUserID(ctx context.Context, clerkUserID string) (*models.Principal, error) {
	return nil, nil
}

func (m *mockUserService) FindByID(ctx context.Context, id string) (*models.User, error) {
	return nil, nil
}

func (m *mockUserService) Create(ctx context.Context, clerkUserID string, email, fullName string, role models.Role) (*models.User, error) {
	m.createCalled = true
	m.lastClerkID = clerkUserID
	m.lastEmail = email
	m.lastFullName = fullName
	m.lastRole = role
	return nil, m.createErr
}

func (m *mockUserService) Update(ctx context.Context, id string, email, fullName string) (*models.User, error) {
	m.updateCalled = true
	m.lastUpdateID = id
	m.lastEmail = email
	m.lastFullName = fullName
	return nil, m.updateErr
}

func (m *mockUserService) SoftDelete(ctx context.Context, id string) (*models.User, error) {
	m.deleteCalled = true
	m.lastDeleteID = id
	return nil, m.deleteErr
}

func newHandlerWithMock(mock *mockUserService) *handler {
	return NewHandler(&contract.Contract{
		Usecase: &contractUsecase.Contract{Users: mock},
	})
}

func TestServeWebhook_user_created(t *testing.T) {
	// Arrange.
	mock := &mockUserService{}
	h := newHandlerWithMock(mock)
	body := strings.NewReader(`{"type":"user.created","data":{"id":"user_123","primary_email_address_id":"ea_1","deleted_at":"","email_addresses":[{"id":"ea_1","email_address":"alice@example.com"}],"first_name":"Alice","last_name":"Smith"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clerk", body)
	w := httptest.NewRecorder()

	// Act.
	err := h.serveWebhook(w, req)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	var createdResp httpx.Response[StatusResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createdResp))
	assert.True(t, createdResp.Success)
	assert.Equal(t, "created", createdResp.Data.Status)
	assert.True(t, mock.createCalled)
	assert.Equal(t, "user_123", mock.lastClerkID)
	assert.Equal(t, "alice@example.com", mock.lastEmail)
	assert.Equal(t, "Alice Smith", mock.lastFullName)
	assert.Equal(t, models.RoleUser, mock.lastRole)
}

func TestServeWebhook_user_updated(t *testing.T) {
	// Arrange.
	mock := &mockUserService{}
	h := newHandlerWithMock(mock)
	body := strings.NewReader(`{"type":"user.updated","data":{"id":"user_123","primary_email_address_id":"ea_1","deleted_at":"","email_addresses":[{"id":"ea_1","email_address":"bob@example.com"}],"first_name":"Bob","last_name":"Jones"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clerk", body)
	w := httptest.NewRecorder()

	// Act.
	err := h.serveWebhook(w, req)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	var updatedResp httpx.Response[StatusResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updatedResp))
	assert.True(t, updatedResp.Success)
	assert.Equal(t, "updated", updatedResp.Data.Status)
	assert.True(t, mock.updateCalled)
	assert.Equal(t, "user_123", mock.lastUpdateID)
	assert.Equal(t, "bob@example.com", mock.lastEmail)
	assert.Equal(t, "Bob Jones", mock.lastFullName)
}

func TestServeWebhook_user_deleted(t *testing.T) {
	// Arrange.
	mock := &mockUserService{}
	h := newHandlerWithMock(mock)
	body := strings.NewReader(`{"type":"user.deleted","data":{"id":"user_123","primary_email_address_id":"ea_1","deleted_at":"2026-01-01T00:00:00Z","email_addresses":[],"first_name":"","last_name":""}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clerk", body)
	w := httptest.NewRecorder()

	// Act.
	err := h.serveWebhook(w, req)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	var deletedResp httpx.Response[StatusResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &deletedResp))
	assert.True(t, deletedResp.Success)
	assert.Equal(t, "deleted", deletedResp.Data.Status)
	assert.True(t, mock.deleteCalled)
	assert.Equal(t, "user_123", mock.lastDeleteID)
}

func TestServeWebhook_invalid_payload(t *testing.T) {
	// Arrange.
	mock := &mockUserService{}
	h := newHandlerWithMock(mock)
	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clerk", body)
	w := httptest.NewRecorder()

	// Act.
	err := h.serveWebhook(w, req)

	// Assert. When called directly (not via HandlerFunc.ServeHTTP), an
	// error is returned without writing to the response writer.
	require.Error(t, err)
	assert.False(t, mock.createCalled)
	assert.False(t, mock.updateCalled)
	assert.False(t, mock.deleteCalled)
}

func TestRoute_registers_clerk_under_group(t *testing.T) {
	// Arrange.
	mock := &mockUserService{}
	h := newHandlerWithMock(mock)
	rt := httpx.New()
	group := rt.Group("/v1/webhooks")

	// Act.
	h.Route(group)

	// Assert via introspection.
	routes := rt.Routes()
	require.Len(t, routes, 1, "expected 1 route registered under webhooks group")
	assert.Equal(t, http.MethodPost, routes[0].Method, "expected POST method")
	assert.Equal(t, "/v1/webhooks/clerk", routes[0].Pattern, "expected compounded group prefix")
}
