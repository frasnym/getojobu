package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouter_registers_and_dispatches_GET_and_POST_to_different_handlers(t *testing.T) {
	// Arrange.
	rt := New()
	rt.Handle(http.MethodGet, "/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("get"))
	})
	rt.Handle(http.MethodPost, "/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("post"))
	})

	// Act GET.
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	rt.Mux().ServeHTTP(w, req)

	// Assert GET.
	assert.Equal(t, http.StatusOK, w.Code, "GET expected 200")

	// Act POST.
	req = httptest.NewRequest(http.MethodPost, "/v1/health", nil)
	w = httptest.NewRecorder()
	rt.Mux().ServeHTTP(w, req)

	// Assert POST.
	assert.Equal(t, http.StatusCreated, w.Code, "POST expected 201")
}

func TestRouter_405_for_known_path_wrong_method(t *testing.T) {
	// Arrange.
	rt := New()
	rt.Handle(http.MethodGet, "/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Act: POST to a GET-only endpoint.
	req := httptest.NewRequest(http.MethodPost, "/v1/health", nil)
	w := httptest.NewRecorder()
	rt.Mux().ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "expected 405 for wrong method")
}

func TestRouter_404_for_unknown_path(t *testing.T) {
	// Arrange.
	rt := New()
	rt.Handle(http.MethodGet, "/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Act.
	req := httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	w := httptest.NewRecorder()
	rt.Mux().ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusNotFound, w.Code, "expected 404 for unknown path")
}

func TestRouter_Introspection_lists_registered_routes(t *testing.T) {
	// Arrange.
	rt := New()
	noop := func(w http.ResponseWriter, r *http.Request) {}
	rt.Handle(http.MethodGet, "/v1/health", noop)
	rt.Handle(http.MethodPost, "/v1/health", noop)
	rt.Handle(http.MethodGet, "/v1/jobs", noop)

	// Act.
	routes := rt.Routes()

	// Assert.
	require.Len(t, routes, 3, "expected 3 routes")
	assert.Equal(t, http.MethodGet, routes[0].Method, "unexpected first route method")
	assert.Equal(t, "/v1/health", routes[0].Pattern, "unexpected first route pattern")
	assert.Equal(t, http.MethodPost, routes[1].Method, "unexpected second route method")
	assert.Equal(t, "/v1/health", routes[1].Pattern, "unexpected second route pattern")
}
