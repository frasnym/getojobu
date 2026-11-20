package httpx

import (
	"encoding/json"
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
	var resp Response[any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Nil(t, resp.Data)
	require.NotNil(t, resp.Error)
	assert.Equal(t, CodeNotFound, resp.Error.Code)
	assert.Equal(t, "route not found", resp.Error.Message)
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
	var resp Response[any]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Nil(t, resp.Data)
	require.NotNil(t, resp.Error)
	assert.Equal(t, CodeMethodNotAllowed, resp.Error.Code)
	assert.Equal(t, "method not allowed", resp.Error.Message)
}

func TestRouter_EnvelopeHandler_passes_through_successful_response(t *testing.T) {
	// Arrange.
	rt := New()
	rt.GET("/v1/static/", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "text/html")
		// Write without calling WriteHeader first, like a static file server.
		_, _ = w.Write([]byte("<html>ok</html>"))
		return nil
	})
	h := rt.EnvelopeHandler(rt.Mux())

	// Act.
	req := httptest.NewRequest(http.MethodGet, "/v1/static/index.html", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Assert: body passes through and status is 200 (not 0).
	assert.Equal(t, http.StatusOK, w.Code, "expected 200 for successful response")
	assert.Equal(t, "<html>ok</html>", w.Body.String(), "expected body to pass through")
}

func TestRouter_trailing_slash_pattern_matches_subpaths(t *testing.T) {
	// Arrange.
	rt := New()
	rt.GET("/swagger/", func(w http.ResponseWriter, r *http.Request) error {
		_, _ = w.Write([]byte("swagger:" + r.URL.Path))
		return nil
	})

	// Act & Assert.
	for _, path := range []string{"/swagger/doc.json", "/swagger/index.html"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		rt.Mux().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "expected 200 for %s", path)
		assert.Equal(t, "swagger:"+path, w.Body.String(), "unexpected body for %s", path)
	}
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

func TestRouter_Group_prefixes_patterns(t *testing.T) {
	// Arrange.
	rt := New()
	v1 := rt.Group("/v1")
	noop := HandlerFunc(func(w http.ResponseWriter, r *http.Request) error { return nil })
	v1.GET("/health", noop)
	v1.POST("/webhooks/clerk", noop)

	// Act: dispatch through the shared mux.
	getReq := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	getW := httptest.NewRecorder()
	rt.Mux().ServeHTTP(getW, getReq)

	postReq := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clerk", nil)
	postW := httptest.NewRecorder()
	rt.Mux().ServeHTTP(postW, postReq)

	// Unprefixed path must not match.
	rawReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	rawW := httptest.NewRecorder()
	rt.Mux().ServeHTTP(rawW, rawReq)

	// Assert.
	assert.Equal(t, http.StatusOK, getW.Code, "GET /v1/health via group")
	assert.Equal(t, http.StatusOK, postW.Code, "POST /v1/webhooks/clerk via group")
	assert.Equal(t, http.StatusNotFound, rawW.Code, "GET /health unprefixed should be 404")

	// Introspection should reflect the prefixed patterns.
	routes := rt.Routes()
	require.Len(t, routes, 2, "expected 2 introspected routes")
	assert.Equal(t, "/v1/health", routes[0].Pattern, "introspected pattern should include group prefix")
	assert.Equal(t, "/v1/webhooks/clerk", routes[1].Pattern, "introspected pattern should include group prefix")
}

func TestRouter_NestedGroup_compounds_prefixes(t *testing.T) {
	// Arrange.
	rt := New()
	v1 := rt.Group("/v1")
	webhooks := v1.Group("/webhooks")
	noop := HandlerFunc(func(w http.ResponseWriter, r *http.Request) error { return nil })
	webhooks.POST("/clerk", noop)

	// Act: dispatch through the shared mux.
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clerk", nil)
	w := httptest.NewRecorder()
	rt.Mux().ServeHTTP(w, req)

	// Parent-only and unprefixed paths must not match.
	parentReq := httptest.NewRequest(http.MethodPost, "/v1/clerk", nil)
	parentW := httptest.NewRecorder()
	rt.Mux().ServeHTTP(parentW, parentReq)

	rawReq := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", nil)
	rawW := httptest.NewRecorder()
	rt.Mux().ServeHTTP(rawW, rawReq)

	// Assert.
	assert.Equal(t, http.StatusOK, w.Code, "POST /v1/webhooks/clerk via nested group")
	assert.Equal(t, http.StatusNotFound, parentW.Code, "POST /v1/clerk should be 404 (missing webhooks segment)")
	assert.Equal(t, http.StatusNotFound, rawW.Code, "POST /webhooks/clerk unprefixed should be 404")

	// Introspection should reflect compounded prefix.
	routes := rt.Routes()
	require.Len(t, routes, 1, "expected 1 introspected route")
	assert.Equal(t, "/v1/webhooks/clerk", routes[0].Pattern, "introspected pattern should include compounded prefixes")
}
