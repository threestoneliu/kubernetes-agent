package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingProvider_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	st, err := PingProvider(context.Background(), Provider{Type: "openai-compatible", BaseURL: srv.URL, Model: "x"}, 1)
	require.NoError(t, err)
	assert.True(t, st.OK)
}

func TestPingProvider_Timeout(t *testing.T) {
	st, err := PingProvider(context.Background(), Provider{Type: "openai-compatible", BaseURL: "http://127.0.0.1:1", Model: "x"}, 1)
	require.NoError(t, err)
	assert.False(t, st.OK)
	assert.NotEmpty(t, st.Reason)
}

func TestPingProvider_5xxIsFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	st, err := PingProvider(context.Background(), Provider{Type: "openai-compatible", BaseURL: srv.URL, Model: "x"}, 1)
	require.NoError(t, err)
	assert.False(t, st.OK)
	assert.Contains(t, st.Reason, "503")
}

func TestPingProvider_4xxIsOK(t *testing.T) {
	// 4xx is "reachable but request was bad" — server is up.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	st, err := PingProvider(context.Background(), Provider{Type: "openai-compatible", BaseURL: srv.URL, Model: "x"}, 1)
	require.NoError(t, err)
	assert.True(t, st.OK)
}

func TestPingProvider_EmptyBaseURL(t *testing.T) {
	st, err := PingProvider(context.Background(), Provider{Type: "openai-compatible", Model: "x"}, 1)
	require.NoError(t, err)
	assert.False(t, st.OK)
	assert.Contains(t, st.Reason, "base_url")
}
