package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatic_RootServesIndexNoCache(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	staticHandler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
}

func TestStatic_IndexHTMLNoCache(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	staticHandler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
}

func TestStatic_HashedAssetLongCache(t *testing.T) {
	// Vite emits content-hashed filenames; the actual hash changes
	// per build, so look up whatever is currently embedded rather
	// than hardcoding a name.
	req := httptest.NewRequest(http.MethodGet, hashedAssetPath(t), nil)
	rec := httptest.NewRecorder()
	staticHandler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age=31536000")
	assert.Contains(t, rec.Header().Get("Cache-Control"), "immutable")
}

func TestStatic_UnknownPathFallsBackToIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	staticHandler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestStatic_APIRouteNotShadowedByMountOrder(t *testing.T) {
	// Mounting staticHandler() with r.Handle("/*", ...) must NOT
	// intercept /api/* routes; the router ordering test verifies
	// the explicit /api routes still win.
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/chat", "application/json", http.NoBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// hashedAssetPath returns a /assets/... path that exists in the
// embedded web_dist. The test fails loudly if no asset is
// embedded — the signal that `make copy-web` was not run before
// `go test`.
func hashedAssetPath(t *testing.T) string {
	t.Helper()
	entries, err := webDistFS.ReadDir("web_dist/assets")
	require.NoError(t, err, "web_dist/assets must exist; did you run `make copy-web`?")
	for _, e := range entries {
		if !e.IsDir() {
			return "/assets/" + e.Name()
		}
	}
	t.Fatal("no embedded asset files in web_dist/assets/")
	return ""
}