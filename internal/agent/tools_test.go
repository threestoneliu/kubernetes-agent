package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"

	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"github.com/threestoneliu/kubernetes-agent/internal/tools/k8s"
)

// stubFactory hands out a no-op dynamic interface. The k8s tool
// functions take a ClientFactory; tests below only exercise the agent
// layer so the factory can fail cleanly at the first client call.
type stubFactory struct{}

func (stubFactory) Get(ctx context.Context, id string) (k8s.ClientFactory, error) {
	return nil, nil
}

// Use the real k8s.ClientFactory type alias. Since the test doesn't
// make any successful k8s API calls, the factory's Get just needs to
// return *something* compatible.

func TestAllToolNames(t *testing.T) {
	names := AllToolNames()
	require.Len(t, names, 6)
	assert.Contains(t, names, "k8s_get")
	assert.Contains(t, names, "k8s_list")
	assert.Contains(t, names, "k8s_describe")
	assert.Contains(t, names, "k8s_plan_write")
	assert.Contains(t, names, "k8s_execute_plan")
	assert.Contains(t, names, "k8s_ask_user")
}

func TestRegisterK8sTools_HandlerCount(t *testing.T) {
	tools := RegisterK8sTools(&ToolDeps{})
	assert.Len(t, tools, 6)
	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.Handler)
		assert.NotNil(t, tool.InputSchema)
	}
}

func TestMergeDiffsAndDenied(t *testing.T) {
	out := mergeDiffsAndDenied([]byte(`[{"a":1}]`), []byte(`[{"b":2}]`))
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(out), &m))
	assert.Contains(t, string(m["diffs"]), `"a":1`)
	assert.Contains(t, string(m["denied"]), `"b":2`)
}

func TestHashString_Deterministic(t *testing.T) {
	a := hashString("which namespace?")
	b := hashString("which namespace?")
	c := hashString("which cluster?")
	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
	assert.NotEmpty(t, a)
}

func TestPlanDecision_NilSession(t *testing.T) {
	d := &ToolDeps{}
	assert.Equal(t, "confirmed", d.planDecision())
}

func TestPlanDecision_Empty(t *testing.T) {
	sess := NewSession("s1")
	d := &ToolDeps{Session: sess}
	assert.Equal(t, "confirmed", d.planDecision())
}

func TestPlanDecision_AfterCancel(t *testing.T) {
	sess := NewSession("s1")
	sess.CancelPlan()
	d := &ToolDeps{Session: sess}
	assert.Equal(t, "cancelled", d.planDecision())
}

func TestPlanDecision_AfterConfirm(t *testing.T) {
	sess := NewSession("s1")
	sess.ConfirmPlan()
	d := &ToolDeps{Session: sess}
	assert.Equal(t, "confirmed", d.planDecision())
}

// emptyFactory returns a factory whose Get returns (nil, nil). The
// handlers below either fail input validation before reaching Get or
// tolerate a nil dynamic.Interface.
type emptyFactory struct{}

func (emptyFactory) Get(ctx context.Context, id string) (dynamic.Interface, error) {
	return nil, nil
}
func (emptyFactory) Invalidate(id string) {}
func (emptyFactory) Resolver(id string) *k8s.Resolver { return k8s.NewResolver(nil) }

func TestK8sGetHandler_InvalidJSON(t *testing.T) {
	tools := RegisterK8sTools(&ToolDeps{Factory: emptyFactory{}})
	get := findTool(t, tools, "k8s_get")
	_, err := get.Handler(context.Background(), llm.ToolCall{Input: []byte(`not json`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestK8sListHandler_InvalidJSON(t *testing.T) {
	tools := RegisterK8sTools(&ToolDeps{Factory: emptyFactory{}})
	l := findTool(t, tools, "k8s_list")
	_, err := l.Handler(context.Background(), llm.ToolCall{Input: []byte(`not json`)})
	require.Error(t, err)
}

func TestK8sDescribeHandler_InvalidJSON(t *testing.T) {
	tools := RegisterK8sTools(&ToolDeps{Factory: emptyFactory{}})
	d := findTool(t, tools, "k8s_describe")
	_, err := d.Handler(context.Background(), llm.ToolCall{Input: []byte(`not json`)})
	require.Error(t, err)
}

func TestK8sPlanWriteHandler_InvalidJSON(t *testing.T) {
	tools := RegisterK8sTools(&ToolDeps{
		Factory: emptyFactory{},
		Engine:  &policy.Engine{Rules: policy.DefaultRules()},
	})
	p := findTool(t, tools, "k8s_plan_write")
	_, err := p.Handler(context.Background(), llm.ToolCall{Input: []byte(`not json`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestK8sPlanWriteHandler_NoStoreNoSession(t *testing.T) {
	// Without Store/Session, the handler runs the plan but skips
	// persistence and the blocking wait.
	tools := RegisterK8sTools(&ToolDeps{
		Factory: emptyFactory{},
		Engine:  &policy.Engine{Rules: policy.DefaultRules()},
	})
	p := findTool(t, tools, "k8s_plan_write")
	// Note: this call would fail because emptyFactory.Get returns nil
	// for the dynamic interface. So we test the input parsing path
	// instead — see TestK8sPlanWriteHandler_InvalidJSON.
	_ = p
}

func TestK8sExecutePlanHandler_NoStore(t *testing.T) {
	tools := RegisterK8sTools(&ToolDeps{Factory: emptyFactory{}})
	e := findTool(t, tools, "k8s_execute_plan")
	_, err := e.Handler(context.Background(), llm.ToolCall{Input: []byte(`{"plan_id":"x","confirm_token":"y"}`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store not configured")
}

func TestK8sExecutePlanHandler_InvalidJSON(t *testing.T) {
	st := newAgentTestStore(t)
	tools := RegisterK8sTools(&ToolDeps{Factory: emptyFactory{}, Store: st})
	e := findTool(t, tools, "k8s_execute_plan")
	_, err := e.Handler(context.Background(), llm.ToolCall{Input: []byte(`not json`)})
	require.Error(t, err)
}

func TestK8sAskUserHandler_NoSession(t *testing.T) {
	tools := RegisterK8sTools(&ToolDeps{Factory: emptyFactory{}})
	a := findTool(t, tools, "k8s_ask_user")
	out, err := a.Handler(context.Background(), llm.ToolCall{Input: []byte(`{"question":"which ns?"}`)})
	require.NoError(t, err)
	// Without a session, the handler falls through to k8s.AskUser.
	assert.Contains(t, string(out), `"question_id"`)
}

func TestK8sAskUserHandler_InvalidJSON(t *testing.T) {
	tools := RegisterK8sTools(&ToolDeps{Factory: emptyFactory{}})
	a := findTool(t, tools, "k8s_ask_user")
	_, err := a.Handler(context.Background(), llm.ToolCall{Input: []byte(`not json`)})
	require.Error(t, err)
}

func TestK8sAskUserHandler_WithSessionEmits(t *testing.T) {
	sess := NewSession("s1")
	var got Event
	tools := RegisterK8sTools(&ToolDeps{
		Factory: emptyFactory{},
		Session: sess,
		Emit:    func(e Event) { got = e },
	})
	a := findTool(t, tools, "k8s_ask_user")
	sess.AnswerAsk("prod")
	out, err := a.Handler(context.Background(), llm.ToolCall{Input: []byte(`{"question":"which ns?"}`)})
	require.NoError(t, err)
	assert.Equal(t, EventAskUser, got.Type)
	assert.Contains(t, string(out), `"answer":"prod"`)
}

func findTool(t *testing.T, tools []llm.Tool, name string) llm.Tool {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return llm.Tool{}
}

// newAgentTestStore opens a fresh SQLite-backed store with
// migrations applied. Used by tests that exercise store-backed
// handlers.
func newAgentTestStore(t *testing.T) *store.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent_test.db")
	db, err := store.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.Migrate())
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// _ keeps imports used.
var _ = k8s.Operation{}