package http

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
	rt.GET("/v1/health", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("get"))
		return nil
	})
	rt.POST("/v1/health", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("post"))
		return nil
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
	rt.GET("/v1/health", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
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
	rt.GET("/v1/health", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	// Act.
	req := httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	w := httptest.NewRecorder()
	rt.Mux().ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusNotFound, w.Code, "expected 404 for unknown path")
}

func TestRouter_EnvelopeHandler_rewrites_404_as_json(t *testing.T) {
	// Arrange.
	rt := New()
	rt.GET("/v1/health", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})
	h := rt.EnvelopeHandler(rt.Mux())

	// Act.
	req := httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusNotFound, w.Code, "expected 404")
	assert.JSONEq(t, `{"code":"not_found","message":"route not found"}`, w.Body.String(), "expected JSON envelope")
}

func TestRouter_EnvelopeHandler_rewrites_405_as_json(t *testing.T) {
	// Arrange.
	rt := New()
	rt.GET("/v1/health", func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})
	h := rt.EnvelopeHandler(rt.Mux())

	// Act.
	req := httptest.NewRequest(http.MethodPost, "/v1/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Assert.
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "expected 405")
	assert.JSONEq(t, `{"code":"method_not_allowed","message":"method not allowed"}`, w.Body.String(), "expected JSON envelope")
}

func TestRouter_Introspection_lists_registered_routes(t *testing.T) {
	// Arrange.
	rt := New()
	noop := HandlerFunc(func(w http.ResponseWriter, r *http.Request) error { return nil })
	rt.GET("/v1/health", noop)
	rt.POST("/v1/health", noop)
	rt.GET("/v1/jobs", noop)

	// Act.
	routes := rt.Routes()

	// Assert.
	require.Len(t, routes, 3, "expected 3 routes")
	assert.Equal(t, http.MethodGet, routes[0].Method, "unexpected first route method")
	assert.Equal(t, "/v1/health", routes[0].Pattern, "unexpected first route pattern")
	assert.Equal(t, http.MethodPost, routes[1].Method, "unexpected second route method")
	assert.Equal(t, "/v1/health", routes[1].Pattern, "unexpected second route pattern")
}
