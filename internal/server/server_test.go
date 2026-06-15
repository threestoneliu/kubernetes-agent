package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/threestoneliu/kubernetes-agent/internal/agent"
	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"github.com/threestoneliu/kubernetes-agent/internal/tools/k8s"
)

// noopClient satisfies llm.Client; the stubbed run never calls it
// because the script pushes events directly on the channel.
type noopClient struct{}

func (noopClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (llm.Stream, error) {
	return noopStream{}, nil
}

type noopStream struct{}

func (noopStream) Next(ctx context.Context) (llm.Event, error) { return llm.Event{}, io.EOF }
func (noopStream) Close() error                                { return nil }

// noopMsgStore satisfies the agent.MessageStore interface the
// runner needs. The stubbed run never reaches persistence, so the
// methods are inert.
type noopMsgStore struct{}

func (noopMsgStore) BatchInsertMessages(ctx context.Context, msgs []store.Message) error {
	return nil
}

// testDeps builds a Deps wired to a fresh SQLite file, a real
// AEAD, and an empty RunnerFactory. Tests that exercise the chat
// handler must replace RunnerFactory with a scriptedRunnerFactory
// that has a session-id-keyed event sequence.
func testDeps(t *testing.T) Deps {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := store.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate())

	key := make([]byte, 32)
	_, err = rand.Read(key)
	require.NoError(t, err)
	aead, err := crypto.NewAEAD(key)
	require.NoError(t, err)

	factory := k8s.NewClientFactory(db, aead)
	return Deps{
		DB:            db,
		AEAD:          aead,
		Engine:        &policy.Engine{Rules: policy.DefaultRules()},
		LLM:           &llm.Registry{},
		Factory:       factory,
		RunnerFactory: &scriptedRunnerFactory{},
	}
}

// --- /healthz ---

func TestHealthz_ReturnsOK(t *testing.T) {
	d := testDeps(t)
	d.LLM = &llm.Registry{
		Providers: []llm.Provider{{Name: "anthropic"}, {Name: "openai"}},
		Health: map[string]llm.PingStatus{
			"anthropic": {Name: "anthropic", OK: true},
			"openai":    {Name: "openai", OK: false, Reason: "401"},
		},
	}
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		OK        bool               `json:"ok"`
		Providers []llm.ProviderStatus `json:"providers"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, body.OK)
	require.Len(t, body.Providers, 2)
	assert.Equal(t, "anthropic", body.Providers[0].Name)
	assert.Equal(t, "enabled", body.Providers[0].Status)
	assert.Equal(t, "openai", body.Providers[1].Name)
	assert.Equal(t, "disabled", body.Providers[1].Status)
}

// --- /api/clusters CRUD ---

func TestClusters_CRUD(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	// POST a new cluster with a minimal but valid kubeconfig.
	postBody := bytes.NewReader([]byte(`{
		"name": "dev",
		"kubeconfig": "apiVersion: v1\nkind: Config\nclusters:\n- name: dev\n  cluster:\n    server: https://example.com\ncontexts:\n- name: dev\n  context:\n    cluster: dev\n    user: dev\ncurrent-context: dev\nusers:\n- name: dev\n  user: {}\n"
	}`))
	resp, err := http.Post(ts.URL+"/api/clusters", "application/json", postBody)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created clusterView
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()
	assert.Equal(t, "dev", created.Name)
	assert.Equal(t, "https://example.com", created.Server)
	assert.NotEmpty(t, created.ID)

	// GET should list it.
	resp, err = http.Get(ts.URL + "/api/clusters")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var listResp struct {
		Clusters []clusterView `json:"clusters"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listResp))
	resp.Body.Close()
	require.Len(t, listResp.Clusters, 1)
	assert.Equal(t, created.ID, listResp.Clusters[0].ID)
	// The blob must NEVER be in the response.
	raw, _ := json.Marshal(listResp)
	assert.NotContains(t, string(raw), "kubeconfig")

	// DELETE then list should be empty.
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/clusters/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/api/clusters")
	require.NoError(t, err)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listResp))
	resp.Body.Close()
	assert.Empty(t, listResp.Clusters)

	// DELETE a second time is 404.
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/clusters/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestClusters_RejectsBadKubeconfig(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	body := bytes.NewReader([]byte(`{"name": "broken", "kubeconfig": "not yaml at all: [[["}`))
	resp, err := http.Post(ts.URL+"/api/clusters", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var errResp errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Equal(t, "invalid_kubeconfig", errResp.Code)
}

// --- /api/policies ---

func TestPolicies_Toggle(t *testing.T) {
	d := testDeps(t)
	require.NoError(t, d.DB.SeedDefaultPolicies(t.Context()))

	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	// GET /api/policies -> seeded defaults
	resp, err := http.Get(ts.URL + "/api/policies")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var listResp struct {
		Policies []policyView `json:"policies"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listResp))
	resp.Body.Close()
	require.NotEmpty(t, listResp.Policies)
	id := listResp.Policies[0].ID
	assert.True(t, listResp.Policies[0].Enabled)

	// PATCH enabled=false
	patch := bytes.NewReader([]byte(`{"enabled": false}`))
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/policies/"+id+"/enabled", patch)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// GET -> first policy should be disabled
	resp, err = http.Get(ts.URL + "/api/policies")
	require.NoError(t, err)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listResp))
	resp.Body.Close()
	for _, p := range listResp.Policies {
		if p.ID == id {
			assert.False(t, p.Enabled)
		}
	}
}

// --- /api/sessions ---

func TestSessions_CreateAndList(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	// First create a real cluster so the cluster_id FK on
	// sessions is satisfied. Encrypt a placeholder blob because
	// the schema marks kubeconfig_blob NOT NULL.
	blob, err := d.AEAD.Encrypt([]byte("placeholder"))
	require.NoError(t, err)
	require.NoError(t, d.DB.CreateCluster(t.Context(), store.Cluster{
		ID: "cl-1", Name: "cl1", Server: "https://x", User: "u",
		KubeconfigBlob: blob,
	}))

	body := bytes.NewReader([]byte(`{"title": "my chat", "cluster_id": "cl-1"}`))
	resp, err := http.Post(ts.URL+"/api/sessions", "application/json", body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created sessionView
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	resp.Body.Close()
	assert.Equal(t, "my chat", created.Title)
	require.NotNil(t, created.ClusterID)
	assert.Equal(t, "cl-1", *created.ClusterID)

	resp, err = http.Get(ts.URL + "/api/sessions")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var listResp struct {
		Sessions []sessionView `json:"sessions"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listResp))
	resp.Body.Close()
	require.Len(t, listResp.Sessions, 1)
	assert.Equal(t, created.ID, listResp.Sessions[0].ID)

	// GET /api/sessions/{id}/messages returns an empty list for
	// a fresh session.
	resp, err = http.Get(ts.URL + "/api/sessions/" + created.ID + "/messages")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var msgResp struct {
		Messages []messageView `json:"messages"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&msgResp))
	resp.Body.Close()
	assert.Empty(t, msgResp.Messages)
}

// --- /api/chat SSE ---

func TestChat_SSEHeaders(t *testing.T) {
	d := testDeps(t)

	// Create the session first so the handler's session lookup succeeds.
	require.NoError(t, d.DB.CreateSession(t.Context(), store.Session{ID: "s1", Title: "x"}))

	// Script: emit session_meta first (mimicking production), then
	// one token + message_end.
	tokenEv, _ := agent.NewEvent(agent.EventToken, agent.Token{Text: "hi"})
	endEv, _ := agent.NewEvent(agent.EventMessageEnd, agent.MessageEnd{InputTokens: 1, OutputTokens: 1})
	metaEv, _ := agent.NewEvent(agent.EventSessionMeta, agent.SessionMeta{SessionID: "s1"})

	stub := &scriptedRunnerFactory{}
	stub.Set("s1", []agent.Event{metaEv, tokenEv, endEv})
	d.RunnerFactory = stub

	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	body := bytes.NewReader([]byte(`{"session_id": "s1", "message": "hello"}`))
	resp, err := http.Post(ts.URL+"/api/chat", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Contains(t, resp.Header.Get("X-Accel-Buffering"), "no")

	// Read at least one event frame and confirm the shape.
	rdr := bufio.NewReader(resp.Body)
	frame, err := readSSEFrame(rdr, 5*time.Second)
	require.NoError(t, err, "should receive at least one SSE frame")
	assert.Contains(t, frame, "id: ")
	assert.Contains(t, frame, "event: ")
	// session_meta is the first frame in our script.
	assert.Contains(t, frame, "event: "+agent.EventSessionMeta)
}

func TestChat_MissingMessage(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	body := bytes.NewReader([]byte(`{"message": ""}`))
	resp, err := http.Post(ts.URL+"/api/chat", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Equal(t, "validation_error", errResp.Code)
	assert.Contains(t, errResp.Message, "message is required")
	assert.False(t, errResp.Retryable)
}

// --- helpers ---

// scriptedRunnerFactory is a per-session stub factory. Each test
// seeds the scripts map with the events the runner should emit.
type scriptedRunnerFactory struct {
	mu      sync.Mutex
	scripts map[string][]agent.Event
}

func (f *scriptedRunnerFactory) Set(sessionID string, evs []agent.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.scripts == nil {
		f.scripts = map[string][]agent.Event{}
	}
	f.scripts[sessionID] = evs
}

func (f *scriptedRunnerFactory) NewRunner(sessionID, clusterID string) *agent.Runner {
	f.mu.Lock()
	defer f.mu.Unlock()
	events := make(chan agent.Event, 64)
	r := &agent.Runner{
		Client:  noopClient{},
		Store:   noopMsgStore{},
		Events:  events,
		Session: agent.NewSession(sessionID),
	}
	if clusterID != "" {
		r.Session.ClusterID = clusterID
	}
	script := f.scripts[sessionID]
	go func() {
		defer close(events)
		for _, e := range script {
			events <- e
		}
	}()
	return r
}

// readSSEFrame reads one full SSE frame (the empty-line
// terminator) from rdr with a hard timeout. Returns the raw frame
// text including the trailing \n\n.
func readSSEFrame(rdr *bufio.Reader, timeout time.Duration) (string, error) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		var b strings.Builder
		for {
			line, err := rdr.ReadString('\n')
			b.WriteString(line)
			if err != nil {
				ch <- result{b.String(), err}
				return
			}
			if line == "\n" || line == "\r\n" {
				ch <- result{b.String(), nil}
				return
			}
		}
	}()
	select {
	case r := <-ch:
		return r.line, r.err
	case <-time.After(timeout):
		return "", io.EOF
	}
}
