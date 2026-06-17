package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/threestoneliu/kubernetes-agent/internal/agent"
	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

func newSessionsTestServer(t *testing.T) (*httptest.Server, *store.DB, *agent.SessionManager) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate())

	key := make([]byte, 32)
	aead, err := crypto.NewAEAD(key)
	require.NoError(t, err)

	deps := Deps{
		DB:       db,
		AEAD:     aead,
		Engine:   &policy.Engine{Rules: policy.DefaultRules()},
		LLM:      &llm.Registry{},
		Sessions: agent.NewSessionManager(),
	}
	ts := httptest.NewServer(NewRouter(deps))
	t.Cleanup(ts.Close)
	return ts, db, deps.Sessions
}

func mustCreateSession(t *testing.T, db *store.DB, id, title string) {
	t.Helper()
	require.NoError(t, db.CreateSession(context.Background(), store.Session{ID: id, Title: title}))
}

func ptrString(s string) *string { return &s }

func TestPutSessionHandler_RenameOK(t *testing.T) {
	ts, db, _ := newSessionsTestServer(t)
	mustCreateSession(t, db, "s1", "old")

	req, _ := http.NewRequest("PUT", ts.URL+"/api/sessions/s1",
		strings.NewReader(`{"title":"new name"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	got, err := db.GetSession(context.Background(), "s1")
	require.NoError(t, err)
	assert.Equal(t, "new name", got.Title)
}

func TestPutSessionHandler_EmptyTitle422(t *testing.T) {
	ts, db, _ := newSessionsTestServer(t)
	mustCreateSession(t, db, "s1", "old")

	req, _ := http.NewRequest("PUT", ts.URL+"/api/sessions/s1",
		strings.NewReader(`{"title":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	assert.Equal(t, 422, resp.StatusCode)
}

func TestPutSessionHandler_NotFound(t *testing.T) {
	ts, _, _ := newSessionsTestServer(t)
	req, _ := http.NewRequest("PUT", ts.URL+"/api/sessions/missing",
		strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

func TestDeleteSessionHandler_OK(t *testing.T) {
	ts, db, _ := newSessionsTestServer(t)
	mustCreateSession(t, db, "s1", "x")

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/sessions/s1", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.EqualValues(t, 1, body["deleted"])
}

func TestDeleteSessionHandler_Active409(t *testing.T) {
	ts, db, sm := newSessionsTestServer(t)
	mustCreateSession(t, db, "s1", "x")
	sm.Set("s1", agent.NewSession("s1"))

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/sessions/s1", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	assert.Equal(t, 409, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "session_active", body["code"])
}

func TestDeleteSessionHandler_NotFound(t *testing.T) {
	ts, _, _ := newSessionsTestServer(t)
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/sessions/missing", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

func TestBulkDeleteSessionsHandler(t *testing.T) {
	ts, db, _ := newSessionsTestServer(t)
	mustCreateSession(t, db, "a", "a")
	mustCreateSession(t, db, "b", "b")
	mustCreateSession(t, db, "c", "c")

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/sessions", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.EqualValues(t, 3, body["deleted"])
}

func TestListSessionsHandler_QueryParams(t *testing.T) {
	ts, db, _ := newSessionsTestServer(t)
	mustCreateSession(t, db, "a", "demo-A")
	mustCreateSession(t, db, "b", "Demo-B")
	mustCreateSession(t, db, "c", "unrelated")

	resp, _ := http.Get(ts.URL + "/api/sessions?q=demo&sort=title&order=asc&limit=10&offset=0")
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	var body struct {
		Sessions []struct {
			ID    string
			Title string
		} `json:"sessions"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Len(t, body.Sessions, 2)
	assert.Equal(t, "a", body.Sessions[0].ID)
	assert.Equal(t, "b", body.Sessions[1].ID)
}

func TestListSessionsHandler_InvalidSort(t *testing.T) {
	ts, _, _ := newSessionsTestServer(t)
	resp, _ := http.Get(ts.URL + "/api/sessions?sort=password")
	defer resp.Body.Close()
	assert.Equal(t, 400, resp.StatusCode)
}

func TestExportSessionHandler_Markdown(t *testing.T) {
	ts, db, _ := newSessionsTestServer(t)
	mustCreateSession(t, db, "s1", "demo")
	require.NoError(t, db.BatchInsertMessages(context.Background(), []store.Message{
		{ID: "m1", SessionID: "s1", Role: "user", Content: ptrString("hello")},
		{ID: "m2", SessionID: "s1", Role: "assistant", Reasoning: ptrString("thinking"), Content: ptrString("hi")},
	}))

	resp, _ := http.Get(ts.URL + "/api/sessions/s1/export?format=md")
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/markdown")
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	assert.Contains(t, body, "# 会话: demo")
	assert.Contains(t, body, "## user")
	assert.Contains(t, body, "## assistant")
	assert.Contains(t, body, "<details>")
}

func TestExportSessionHandler_NotFound(t *testing.T) {
	ts, _, _ := newSessionsTestServer(t)
	resp, _ := http.Get(ts.URL + "/api/sessions/missing/export?format=md")
	defer resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
}

func TestExportSessionHandler_InvalidFormat(t *testing.T) {
	ts, db, _ := newSessionsTestServer(t)
	mustCreateSession(t, db, "s1", "x")
	resp, _ := http.Get(ts.URL + "/api/sessions/s1/export?format=xml")
	defer resp.Body.Close()
	assert.Equal(t, 400, resp.StatusCode)
}