package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"

	"github.com/threestoneliu/kubernetes-agent/internal/agent"
	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"github.com/threestoneliu/kubernetes-agent/internal/tools/k8s"
)

// --- scripted LLM ---

// scriptResponse is one turn of the LLM script. The script is
// declared as a slice of builders; each builder is called once,
// the first time the response is consumed, so later turns can
// read state captured by earlier turns (e.g. the plan_id emitted
// by k8s_plan_write in turn 0 is consumed by the test, then fed
// into the turn-1 builder via closure).
type scriptResponse struct {
	build func() []llm.Event
}

// scriptedLLM returns a pre-recorded event sequence for each Chat
// call. Tests describe what the LLM "does" without hitting a real
// provider. Each call advances an internal cursor; once the cursor
// runs off the end, every subsequent Chat returns an empty stream
// (so the runner cleanly terminates).
type scriptedLLM struct {
	mu        sync.Mutex
	script    [][]llm.Event     // legacy static script (used when responses is nil)
	responses []scriptResponse  // dynamic builders (preferred)
	cursor    int
}

func (s *scriptedLLM) Chat(ctx context.Context, msgs []llm.Message, tools []llm.Tool) (llm.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.responses != nil {
		if s.cursor >= len(s.responses) {
			return &sliceStream{}, nil
		}
		resp := s.responses[s.cursor]
		s.cursor++
		var evs []llm.Event
		if resp.build != nil {
			evs = resp.build()
		}
		return &sliceStream{evs: evs}, nil
	}
	if s.cursor >= len(s.script) {
		return &sliceStream{}, nil
	}
	evs := s.script[s.cursor]
	s.cursor++
	return &sliceStream{evs: evs}, nil
}

type sliceStream struct {
	evs []llm.Event
	i   int
}

func (s *sliceStream) Next(ctx context.Context) (llm.Event, error) {
	if s.i >= len(s.evs) {
		return llm.Event{}, io.EOF
	}
	ev := s.evs[s.i]
	s.i++
	return ev, nil
}

func (s *sliceStream) Close() error { return nil }

// scriptBuilder is a small fluent helper for constructing LLM
// scripts. The builder methods return the receiver so callers can
// chain.
type scriptBuilder []llm.Event

func (b *scriptBuilder) token(text string) *scriptBuilder {
	*b = append(*b, llm.Event{Type: llm.EventToken, Text: text})
	return b
}

func (b *scriptBuilder) toolCall(id, name string, input any) *scriptBuilder {
	raw, _ := json.Marshal(input)
	*b = append(*b, llm.Event{Type: llm.EventToolCall, Call: llm.ToolCall{
		ID:    id,
		Name:  name,
		Input: raw,
	}})
	return b
}

func (b *scriptBuilder) end() *scriptBuilder {
	*b = append(*b, llm.Event{Type: llm.EventMessageEnd, In: 1, Out: 1})
	return b
}

// --- fake k8s factory ---

// fakeK8sFactory hands out a *dynfake.FakeDynamicClient for every
// cluster id, wrapped in a small version-inferring shim. The
// shim fills in the Group/Version for a handful of well-known
// resources (pods, deployments, services, …) because the k8s
// tools construct GVRs with an empty Version and rely on the
// production dynamic client to do server-side discovery; the
// fake client does not.
type fakeK8sFactory struct {
	mu      sync.Mutex
	scheme  *runtime.Scheme
	clients map[string]*dynfake.FakeDynamicClient
}

func newFakeK8sFactory() *fakeK8sFactory {
	return &fakeK8sFactory{
		scheme:  runtime.NewScheme(),
		clients: map[string]*dynfake.FakeDynamicClient{},
	}
}

// wellKnownGV maps the resource names the k8s tools accept to
// the (group, version) the dynfake client expects. New resources
// added to the tools need a row here.
var wellKnownGV = map[string]schema.GroupVersion{
	"pods":                   {Group: "", Version: "v1"},
	"events":                 {Group: "", Version: "v1"},
	"namespaces":             {Group: "", Version: "v1"},
	"services":               {Group: "", Version: "v1"},
	"configmaps":             {Group: "", Version: "v1"},
	"nodes":                  {Group: "", Version: "v1"},
	"deployments":            {Group: "apps", Version: "v1"},
	"statefulsets":           {Group: "apps", Version: "v1"},
	"daemonsets":             {Group: "apps", Version: "v1"},
	"replicasets":            {Group: "apps", Version: "v1"},
	"jobs":                   {Group: "batch", Version: "v1"},
	"cronjobs":               {Group: "batch", Version: "v1"},
}

func (f *fakeK8sFactory) clientFor(_ context.Context, clusterID string) (*dynfake.FakeDynamicClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.clients[clusterID]; ok {
		return c, nil
	}
	gvrToListKind := map[schema.GroupVersionResource]string{}
	for res, gv := range wellKnownGV {
		gvrToListKind[schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: res}] = strings.Title(res) + "List"
	}
	c := dynfake.NewSimpleDynamicClientWithCustomListKinds(f.scheme, gvrToListKind)
	f.clients[clusterID] = c
	return c, nil
}

// versionShim wraps a dynfake client to translate empty-Version
// GVR lookups into the well-known group/version map. Calls with
// a non-empty Version pass through unchanged.
type versionShim struct {
	inner *dynfake.FakeDynamicClient
}

func (s *versionShim) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	if gvr.Version == "" {
		if gv, ok := wellKnownGV[gvr.Resource]; ok {
			gvr.Group = gv.Group
			gvr.Version = gv.Version
		}
	}
	return s.inner.Resource(gvr)
}

func (f *fakeK8sFactory) Get(ctx context.Context, clusterID string) (dynamic.Interface, error) {
	c, err := f.clientFor(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	return &versionShim{inner: c}, nil
}

func (f *fakeK8sFactory) Invalidate(clusterID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.clients, clusterID)
}

// Resolver returns a Resolver pre-populated with the e2e test's
// built-in GV map. dynfake does not implement the discovery API,
// so we hand the resolver a static cache rather than a discovery
// client.
func (f *fakeK8sFactory) Resolver(clusterID string) *k8s.Resolver {
	return k8s.ResolverFromMap(e2eGVCache())
}

// e2eGVCache maps the lowercase plural resources the e2e tests
// touch to their canonical Group/Version, mirroring the map that
// the production Resolver would build from the cluster's discovery
// payload.
func e2eGVCache() map[string]schema.GroupVersionResource {
	m := map[string]schema.GroupVersionResource{}
	for res, gv := range e2eKnownGV {
		m[res] = schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: res}
	}
	return m
}

var e2eKnownGV = map[string]schema.GroupVersion{
	"pods":        {Group: "", Version: "v1"},
	"deployments": {Group: "apps", Version: "v1"},
}

func (f *fakeK8sFactory) client(clusterID string) *dynfake.FakeDynamicClient {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clients[clusterID]
}

func (f *fakeK8sFactory) seedPod(t *testing.T, clusterID, namespace, name string) {
	t.Helper()
	c := f.client(clusterID)
	if c == nil {
		_, _ = f.clientFor(context.Background(), clusterID)
		c = f.client(clusterID)
	}
	pod := &unstructured.Unstructured{}
	pod.SetKind("Pod")
	pod.SetAPIVersion("v1")
	pod.SetName(name)
	pod.SetNamespace(namespace)
	_, err := c.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}).
		Namespace(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)
}

func (f *fakeK8sFactory) seedDeployment(t *testing.T, clusterID, namespace, name string, replicas int) {
	t.Helper()
	c := f.client(clusterID)
	if c == nil {
		_, _ = f.clientFor(context.Background(), clusterID)
		c = f.client(clusterID)
	}
	dep := &unstructured.Unstructured{}
	dep.SetKind("Deployment")
	dep.SetAPIVersion("apps/v1")
	dep.SetName(name)
	dep.SetNamespace(namespace)
	_ = unstructured.SetNestedField(dep.Object, int64(replicas), "spec", "replicas")
	_, err := c.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace(namespace).Create(context.Background(), dep, metav1.CreateOptions{})
	require.NoError(t, err)
}

// --- e2e runner factory ---

// e2eRunnerFactory is a *server.RunnerFactory that returns runners
// backed by a scripted LLM and the six k8s tools wired to a fake
// dynamic client factory.
type e2eRunnerFactory struct {
	scripted *scriptedLLM
	db       *store.DB
	factory  k8s.ClientFactory
	engine   *policy.Engine
}

func (f *e2eRunnerFactory) NewRunner(sessionID, clusterID string) *agent.Runner {
	// Build the runner first so the tool handlers (registered
	// below) can observe the same ToolDeps the agent loop
	// mutates — Run wires deps.Emit and deps.Session lazily on
	// the first Chat call so plan / ask can surface events and
	// block on the per-session resume channels.
	r := &agent.Runner{Client: f.scripted, Store: f.db}
	r.Deps = agent.ToolDeps{Factory: f.factory, Engine: f.engine, Store: f.db}
	r.Tools = agent.RegisterK8sTools(&r.Deps)
	return r
}

// --- test environment ---

type testEnv struct {
	t         *testing.T
	server    *httptest.Server
	factory   *fakeK8sFactory
	script    *scriptedLLM
	clusterID string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate())
	t.Cleanup(func() { _ = db.Close() })

	key := make([]byte, 32)
	_, err = rand.Read(key)
	require.NoError(t, err)
	aead, err := crypto.NewAEAD(key)
	require.NoError(t, err)

	engine := &policy.Engine{Rules: policy.DefaultRules()}
	fk := newFakeK8sFactory()
	scripted := &scriptedLLM{}
	rf := &e2eRunnerFactory{
		scripted: scripted,
		db:       db,
		factory:  fk,
		engine:   engine,
	}
	deps := Deps{
		DB:            db,
		AEAD:          aead,
		Engine:        engine,
		LLM:           &llm.Registry{},
		Factory:       fk,
		RunnerFactory: rf,
		Sessions:      agent.NewSessionManager(),
	}
	ts := httptest.NewServer(NewRouter(deps))
	t.Cleanup(ts.Close)

	clusterID := "test-cluster"
	// Sessions.cluster_id is a FK to clusters(id) — create a
	// placeholder row so the chat handler can persist a session
	// pointing at our test cluster id. The encrypted blob is
	// never decrypted by these tests (the k8s factory is a
	// fake); the placeholder is enough to satisfy the FK.
	blob, err := aead.Encrypt([]byte("placeholder-kubeconfig"))
	require.NoError(t, err)
	require.NoError(t, db.CreateCluster(context.Background(), store.Cluster{
		ID: clusterID, Name: "test",
		Server: "https://test", User: "u",
		KubeconfigBlob: blob,
	}))
	_, _ = fk.clientFor(context.Background(), clusterID)
	return &testEnv{
		t: t, server: ts,
		factory: fk, script: scripted, clusterID: clusterID,
	}
}

// --- HTTP + SSE helpers ---

type sseFrame struct {
	Event string
	Data  string
}

// readSSE consumes the entire text/event-stream body and returns
// the parsed frames. Each frame is one or more "event: x\ndata:
// y\n\n" records; trailing CRLF is tolerated. The reader is closed
// when EOF is reached.
func readSSE(t *testing.T, body io.Reader) []sseFrame {
	t.Helper()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var (
		frames []sseFrame
		cur    sseFrame
	)
	flush := func() {
		if cur.Event != "" || cur.Data != "" {
			frames = append(frames, cur)
			cur = sseFrame{}
		}
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "event: "):
			cur.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.Data = strings.TrimPrefix(line, "data: ")
		}
	}
	flush()
	require.NoError(t, scanner.Err(), "scanner error")
	return frames
}

// streamFrames runs readSSE in a goroutine and returns:
//   - framesCh: receives every parsed frame in order
//
// The chat handler keeps the response open while the runner is
// blocked on WaitPlan / WaitAsk, so the read goroutine will sit
// idle (not error) until the test resumes. Use waitForFrame to
// time the "plan_awaiting_confirm arrived" event.
type sseStream struct {
	framesCh chan sseFrame
}

func startSSEStream(t *testing.T, body io.Reader) *sseStream {
	t.Helper()
	out := &sseStream{
		framesCh: make(chan sseFrame, 128),
	}
	go func() {
		defer close(out.framesCh)
		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var cur sseFrame
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				if cur.Event != "" || cur.Data != "" {
					out.framesCh <- cur
					cur = sseFrame{}
				}
			case strings.HasPrefix(line, "event: "):
				cur.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				cur.Data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return out
}

// waitForFrame returns the first frame matching pred, polling
// framesCh until timeout. Non-matching frames it consumes along
// the way are returned in the prefix slice so callers can
// inspect the entire sequence after the wait.
func waitForFrame(framesCh <-chan sseFrame, pred func(sseFrame) bool, timeout time.Duration) (sseFrame, []sseFrame, bool) {
	deadline := time.After(timeout)
	var prefix []sseFrame
	for {
		select {
		case f, ok := <-framesCh:
			if !ok {
				return sseFrame{}, prefix, false
			}
			if pred(f) {
				return f, prefix, true
			}
			prefix = append(prefix, f)
		case <-deadline:
			return sseFrame{}, prefix, false
		}
	}
}

// drainRemaining reads every remaining frame from framesCh until
// it closes, returning the slice.
func drainRemaining(framesCh <-chan sseFrame) []sseFrame {
	var out []sseFrame
	for f := range framesCh {
		out = append(out, f)
	}
	return out
}

// drainWithDeadline reads frames from framesCh until the channel
// closes OR the deadline elapses, whichever comes first. Used by
// tests that need to assert on partial output (e.g. plan_ready
// arrives, the plan is fully denied, the runner blocks on
// WaitPlan, and we don't want to resume).
func drainWithDeadline(framesCh <-chan sseFrame, deadline time.Duration) []sseFrame {
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	var out []sseFrame
	for {
		select {
		case f, ok := <-framesCh:
			if !ok {
				return out
			}
			out = append(out, f)
		case <-timer.C:
			return out
		}
	}
}

func (e *testEnv) postChatRaw(t *testing.T, body string) (*http.Response, *sseStream) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, e.server.URL+"/api/chat",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("chat returned status %d: %s", resp.StatusCode, string(b))
	}
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	return resp, startSSEStream(t, resp.Body)
}

func (e *testEnv) postChat(t *testing.T, body string) []sseFrame {
	t.Helper()
	resp, stream := e.postChatRaw(t, body)
	defer resp.Body.Close()
	frames := drainRemaining(stream.framesCh)
	return frames
}

func (e *testEnv) resumePlan(t *testing.T, sessionID, planID string, approved bool) {
	t.Helper()
	body := fmt.Sprintf(`{"kind":"plan","plan_id":%q,"approved":%v}`, planID, approved)
	req, _ := http.NewRequest(http.MethodPost,
		e.server.URL+"/api/sessions/"+sessionID+"/resume",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func (e *testEnv) resumeAsk(t *testing.T, sessionID, answer string) {
	t.Helper()
	body := fmt.Sprintf(`{"kind":"ask_user","answer":%q}`, answer)
	req, _ := http.NewRequest(http.MethodPost,
		e.server.URL+"/api/sessions/"+sessionID+"/resume",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func framesToMap(frames []sseFrame) map[string]int {
	out := map[string]int{}
	for _, f := range frames {
		out[f.Event]++
	}
	return out
}

// findSessionMeta returns the session_id from the first
// session_meta frame in the slice.
func findSessionMeta(frames []sseFrame) string {
	for _, f := range frames {
		if f.Event == agent.EventSessionMeta {
			var m agent.SessionMeta
			if err := json.Unmarshal([]byte(f.Data), &m); err == nil {
				return m.SessionID
			}
		}
	}
	return ""
}

// --- E2E 1: list pods ---

// E2E 1: the model issues a single k8s_list tool call (no plan
// confirm, no ask_user) and the chat stream reports the result.
// The fake k8s client has 2 pods in default.
func TestE2E_ListPods(t *testing.T) {
	env := newTestEnv(t)
	env.factory.seedPod(t, env.clusterID, "default", "pod-a")
	env.factory.seedPod(t, env.clusterID, "default", "pod-b")

	var sb scriptBuilder
	sb.toolCall("tc1", "k8s_list", map[string]any{
		"resource":   "pods",
		"namespace":  "default",
		"cluster_id": env.clusterID,
	})
	sb.end()
	env.script.script = [][]llm.Event{sb}

	frames := env.postChat(t, fmt.Sprintf(
		`{"message":"list default pods","cluster_id":%q}`, env.clusterID))

	counts := framesToMap(frames)
	assert.Equal(t, 1, counts[agent.EventToolCall], "expected one tool_call event")
	assert.Equal(t, 1, counts[agent.EventToolResult], "expected one tool_result event")
	assert.Equal(t, 1, counts[agent.EventMessageEnd], "expected one message_end")
	assert.Equal(t, 1, counts[agent.EventSessionMeta], "session_meta always emitted first")

	var resultPayload struct {
		Output json.RawMessage `json:"output"`
		Error  string          `json:"error"`
	}
	for _, f := range frames {
		if f.Event != agent.EventToolResult {
			continue
		}
		require.NoError(t, json.Unmarshal([]byte(f.Data), &resultPayload))
	}
	require.Empty(t, resultPayload.Error, "tool result should not be an error")
	var listOut struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(resultPayload.Output, &listOut))
	assert.Len(t, listOut.Items, 2, "should see both seeded pods")
}

// --- E2E 2: plan preview + execute (scale deployment) ---

// E2E 2: the model calls k8s_plan_write with a scale op. The
// agent loop emits plan_ready + plan_awaiting_confirm and blocks
// on WaitPlan. The test confirms via /resume. The runner's next
// Chat call (per the script) issues k8s_execute_plan, which
// scales the deployment to 1.
//
// The script captures the plan_id by querying the store for the
// most-recent plan on this session (the k8s_plan_write handler
// persists it before blocking). The capture happens after
// plan_awaiting_confirm arrives, so the runner has already
// written the row.
func TestE2E_PlanPreviewAndExecute(t *testing.T) {
	env := newTestEnv(t)
	env.factory.seedDeployment(t, env.clusterID, "default", "nginx", 3)

	// Two-step dynamic script. Step 1 is fixed; step 2 closes
	// over the plan_id we discover at runtime.
	var capturedPlanID string
	env.script.responses = []scriptResponse{
		{build: func() []llm.Event {
			var sb scriptBuilder
			sb.toolCall("tc-plan", "k8s_plan_write", map[string]any{
				"operations": []map[string]any{{
					"action":     "scale",
					"resource":   "deployments",
					"name":       "nginx",
					"namespace":  "default",
					"replicas":   1,
					"cluster_id": env.clusterID,
				}},
			})
			sb.end()
			return sb
		}},
		{build: func() []llm.Event {
			var sb scriptBuilder
			sb.toolCall("tc-exec", "k8s_execute_plan", map[string]any{
				"plan_id":       capturedPlanID,
				"confirm_token": "test-token",
			})
			sb.end()
			return sb
		}},
		{build: func() []llm.Event {
			var sb scriptBuilder
			sb.token("Scaled nginx to 1 replica.")
			sb.end()
			return sb
		}},
	}

	resp, stream := env.postChatRaw(t, fmt.Sprintf(
		`{"message":"scale nginx to 1","cluster_id":%q}`, env.clusterID))
	defer resp.Body.Close()

	// Pull session_id from the first frame.
	metaFrame, metaPrefix, ok := waitForFrame(stream.framesCh,
		func(f sseFrame) bool { return f.Event == agent.EventSessionMeta },
		2*time.Second)
	require.True(t, ok, "session_meta must arrive first")
	var meta agent.SessionMeta
	require.NoError(t, json.Unmarshal([]byte(metaFrame.Data), &meta))

	// Wait for plan_awaiting_confirm. Any frames we consume
	// along the way (notably plan_ready, tool_call, tool_result)
	// are kept so the final assertion sees the full sequence.
	confirmFrame, confirmPrefix, ok := waitForFrame(stream.framesCh,
		func(f sseFrame) bool { return f.Event == agent.EventPlanAwaitingConfirm },
		5*time.Second)
	require.True(t, ok, "plan_awaiting_confirm must arrive within 5s")
	var pac agent.PlanAwaitingConfirm
	require.NoError(t, json.Unmarshal([]byte(confirmFrame.Data), &pac))
	require.NotEmpty(t, pac.PlanID, "plan_id must be non-empty")
	capturedPlanID = pac.PlanID

	// Resume the plan.
	env.resumePlan(t, meta.SessionID, pac.PlanID, true)

	// Drain the remaining frames.
	frames := drainRemaining(stream.framesCh)
	all := []sseFrame{}
	all = append(all, metaPrefix...)
	all = append(all, metaFrame)
	all = append(all, confirmPrefix...)
	all = append(all, confirmFrame)
	all = append(all, frames...)

	counts := framesToMap(all)
	assert.GreaterOrEqual(t, counts[agent.EventToolCall], 2, "expected 2 tool calls: plan_write + execute_plan")
	assert.GreaterOrEqual(t, counts[agent.EventToolResult], 2, "expected 2 tool results")
	assert.GreaterOrEqual(t, counts[agent.EventPlanReady], 1, "expected plan_ready")
	assert.GreaterOrEqual(t, counts[agent.EventPlanAwaitingConfirm], 1, "expected plan_awaiting_confirm")
	assert.GreaterOrEqual(t, counts[agent.EventMessageEnd], 3, "expected 3 message_end events (one per Chat call)")

	// The fake k8s client's Deployment should now have replicas=1.
	dc := env.factory.client(env.clusterID)
	require.NotNil(t, dc, "fake client should exist for test cluster")
	got, err := dc.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace("default").Get(context.Background(), "nginx", metav1.GetOptions{})
	require.NoError(t, err)
	replicasGot, found, err := unstructured.NestedInt64(got.Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found, "spec.replicas should be present")
	assert.Equal(t, int64(1), replicasGot, "deployment should be scaled to 1 replica")
}

// --- E2E 3: deny delete in kube-system ---

// E2E 3: k8s_plan_write is called with a delete op on a pod in
// kube-system. The default policy rule deny-delete-system-ns
// fires, the PlanReady event carries a non-empty Denied list,
// and the tool_result reports back the partial plan (no diffs,
// denied=1). The runner blocks on WaitPlan afterwards (the
// frontend would still need to acknowledge the partial plan);
// this test reads with a deadline so it doesn't hang.
func TestE2E_DenySystemNamespace(t *testing.T) {
	env := newTestEnv(t)
	env.factory.seedPod(t, env.clusterID, "kube-system", "coredns-xxx")

	var sb scriptBuilder
	sb.toolCall("tc-deny", "k8s_plan_write", map[string]any{
		"operations": []map[string]any{{
			"action":     "delete",
			"resource":   "pods",
			"name":       "coredns-xxx",
			"namespace":  "kube-system",
			"cluster_id": env.clusterID,
		}},
	})
	sb.end()
	env.script.script = [][]llm.Event{sb}

	// Read with a deadline — the runner blocks on WaitPlan after
	// the plan is emitted because the test does not resume.
	resp, stream := env.postChatRaw(t, fmt.Sprintf(
		`{"message":"delete kube-system pod","cluster_id":%q}`, env.clusterID))
	defer resp.Body.Close()
	frames := drainWithDeadline(stream.framesCh, 2*time.Second)

	counts := framesToMap(frames)
	assert.GreaterOrEqual(t, counts[agent.EventPlanReady], 1, "expected plan_ready event")

	var planReadyPayload agent.PlanReady
	var foundReady bool
	for _, f := range frames {
		if f.Event == agent.EventPlanReady {
			require.NoError(t, json.Unmarshal([]byte(f.Data), &planReadyPayload))
			foundReady = true
			break
		}
	}
	require.True(t, foundReady, "must see plan_ready")
	require.Len(t, planReadyPayload.Denied, 1, "default rule should deny delete in kube-system")
	assert.Equal(t, "delete", planReadyPayload.Denied[0].Operation.Action())
	assert.Equal(t, "kube-system", planReadyPayload.Denied[0].Operation.Namespace())
	assert.Empty(t, planReadyPayload.Diffs, "denied op should produce no diffs")
}

// --- E2E 4: ask_user answer ---

// E2E 4: the model calls k8s_ask_user. The agent loop emits
// ask_user and blocks on WaitAsk. The test resumes with the
// answer "production". The runner's next Chat call (per the
// script) issues k8s_list for the production namespace.
func TestE2E_AskUserAnswer(t *testing.T) {
	env := newTestEnv(t)
	// No k8s objects needed.

	var sb1 scriptBuilder
	sb1.toolCall("tc-ask", "k8s_ask_user", map[string]any{
		"question": "目标 namespace?",
		"options":  []string{"default", "production"},
	})
	sb1.end()
	var sb2 scriptBuilder
	sb2.toolCall("tc-list", "k8s_list", map[string]any{
		"resource":   "pods",
		"namespace":  "production",
		"cluster_id": env.clusterID,
	})
	sb2.end()
	env.script.script = [][]llm.Event{sb1, sb2}

	resp, stream := env.postChatRaw(t, fmt.Sprintf(
		`{"message":"I want to deploy an app","cluster_id":%q}`, env.clusterID))
	defer resp.Body.Close()

	// Wait for session_meta to capture the session id.
	metaFrame, metaPrefix, ok := waitForFrame(stream.framesCh,
		func(f sseFrame) bool { return f.Event == agent.EventSessionMeta },
		2*time.Second)
	require.True(t, ok, "session_meta must arrive first")
	var meta agent.SessionMeta
	require.NoError(t, json.Unmarshal([]byte(metaFrame.Data), &meta))

	// Wait for ask_user.
	askFrame, askPrefix, ok := waitForFrame(stream.framesCh,
		func(f sseFrame) bool { return f.Event == agent.EventAskUser },
		5*time.Second)
	require.True(t, ok, "ask_user must arrive within 5s")
	var askPayload agent.AskUserPayload
	require.NoError(t, json.Unmarshal([]byte(askFrame.Data), &askPayload))
	assert.Equal(t, "目标 namespace?", askPayload.Question)

	// Resume with the answer.
	env.resumeAsk(t, meta.SessionID, "production")

	// Drain the rest.
	tail := drainRemaining(stream.framesCh)
	all := []sseFrame{}
	all = append(all, metaPrefix...)
	all = append(all, metaFrame)
	all = append(all, askPrefix...)
	all = append(all, askFrame)
	all = append(all, tail...)

	counts := framesToMap(all)
	assert.GreaterOrEqual(t, counts[agent.EventAskUser], 1, "expected ask_user event")
	assert.GreaterOrEqual(t, counts[agent.EventToolCall], 1, "expected at least 1 tool call after resume")

	// The post-resume tool_call should be for k8s_list with
	// namespace=production.
	var listCallInput struct {
		Namespace string `json:"namespace"`
	}
	var foundList bool
	for _, f := range all {
		if f.Event != agent.EventToolCall {
			continue
		}
		var tc agent.ToolCall
		require.NoError(t, json.Unmarshal([]byte(f.Data), &tc))
		if tc.Name == "k8s_list" {
			require.NoError(t, json.Unmarshal(tc.Input, &listCallInput))
			foundList = true
		}
	}
	require.True(t, foundList, "should have seen k8s_list tool call after resume")
	assert.Equal(t, "production", listCallInput.Namespace,
		"the answer to ask_user should have threaded through to the next k8s_list call")
}
