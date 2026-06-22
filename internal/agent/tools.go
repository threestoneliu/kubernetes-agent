package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/skills"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"github.com/threestoneliu/kubernetes-agent/internal/tools/agent"
	k8s "github.com/threestoneliu/kubernetes-agent/internal/tools/k8s"
)

// ToolDeps bundles everything a k8s tool handler needs: the dynamic
// client factory, policy engine, store (for plan persistence), the
// per-session state (for blocking on plan confirm / ask), and the
// emitter (for pushing plan / ask events to the SSE channel).
//
// One ToolDeps instance is shared across the six handlers. The Runner
// constructs it once when it starts a turn and passes it down.
type ToolDeps struct {
	Factory          k8s.ClientFactory
	Engine           *policy.Engine
	Store            *store.DB
	Session          *Session
	FSReadAllowedDir string // Directory for fs_read tool to restrict access
	Skills           []*skills.SkillEntry // Loaded skills for load_skill tool
	// Emit is called by handlers that need to surface side-channel
	// events before returning their tool output. In particular,
	// plan_write emits PlanReady + PlanAwaitingConfirm via Emit
	// before blocking on the session's ResumePlan.
	Emit func(Event)
}

// RegisterK8sTools returns the six llm.Tool entries the agent loop
// hands to the LLM: k8s_get, k8s_list, k8s_describe, k8s_plan_write,
// k8s_execute_plan, k8s_ask_user.
//
// d is taken by pointer so the returned tool handlers observe the
// same ToolDeps the agent loop mutates — Run wires d.Emit and
// d.Session lazily on the first Chat call, and the plan / ask
// handlers need those mutations to surface events and block on
// the per-session resume channels.
//
// Each handler's input is JSON of the tool's typed input struct (e.g.
// k8s.GetInput). The agent loop deserialises the model's tool call
// input and hands it to the handler. Handlers return JSON of the
// corresponding output struct.
func RegisterK8sTools(d *ToolDeps) []llm.Tool {
	return []llm.Tool{
		{
			Name:        "k8s_get",
			Description: "Fetch a single Kubernetes resource by name. Returns the resource as a JSON object.",
			InputSchema: getSchema,
			Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
				var in k8s.GetInput
				if err := json.Unmarshal(call.Input, &in); err != nil {
					return nil, fmt.Errorf("invalid input: %w", err)
				}
				d.fillClusterID(&in.ClusterID)
				out, err := k8s.Get(ctx, d.Factory, in)
				if err != nil {
					return nil, err
				}
				return json.Marshal(out)
			},
		},
		{
			Name:        "k8s_list",
			Description: "List Kubernetes resources in table format (kubectl get style). Columns are provided by the API server. Empty namespace means all namespaces.",
			InputSchema: listSchema,
			Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
				var in k8s.ListInput
				if err := json.Unmarshal(call.Input, &in); err != nil {
					return nil, fmt.Errorf("invalid input: %w", err)
				}
				d.fillClusterID(&in.ClusterID)
				out, err := k8s.ListTable(ctx, d.Factory, in)
				if err != nil {
					return nil, err
				}
				return json.Marshal(out)
			},
		},
		{
			Name:        "k8s_describe",
			Description: "Describe a Kubernetes resource: returns the object, related events, owner references, and diagnosis hints derived from conditions / container state.",
			InputSchema: describeSchema,
			Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
				var in k8s.DescribeInput
				if err := json.Unmarshal(call.Input, &in); err != nil {
					return nil, fmt.Errorf("invalid input: %w", err)
				}
				d.fillClusterID(&in.ClusterID)
				out, err := k8s.Describe(ctx, d.Factory, in)
				if err != nil {
					return nil, err
				}
				return json.Marshal(out)
			},
		},
		{
			Name:        "k8s_plan_write",
			Description: "Build a plan for one or more write operations (apply/delete/scale). Returns a plan_id and diffs. The agent must then call k8s_execute_plan with the plan_id after the user confirms.",
			InputSchema: planWriteSchema,
			Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
				var in k8s.PlanInput
				if err := json.Unmarshal(call.Input, &in); err != nil {
					return nil, fmt.Errorf("invalid input: %w", err)
				}
				// Auto-fill cluster_id per operation from the
				// session's bound cluster when the LLM omitted it.
				// k8s.Operation keeps clusterID unexported, so we
				// round-trip through JSON using its wire format.
				if d.Session != nil && d.Session.ClusterID != "" {
					bound := d.Session.ClusterID
					for i := range in.Operations {
						raw, err := json.Marshal(in.Operations[i])
						if err != nil {
							continue
						}
						var m map[string]any
						if err := json.Unmarshal(raw, &m); err != nil {
							continue
						}
						if cid, _ := m["cluster_id"].(string); cid == "" {
							m["cluster_id"] = bound
							fixed, err := json.Marshal(m)
							if err != nil {
								continue
							}
							_ = json.Unmarshal(fixed, &in.Operations[i])
						}
					}
				}
				out, err := k8s.PlanWrite(ctx, d.Factory, d.Engine, in)
				if err != nil {
					return nil, err
				}
				// Persist the plan so execute_plan can retrieve the
				// original ops at execute time.
				if d.Store != nil && d.Session != nil {
					opsJSON, _ := json.Marshal(in.Operations)
					diffsJSON, _ := json.Marshal(out.Diffs)
					deniedJSON, _ := json.Marshal(out.Denied)
					risk := "low"
					if len(out.Diffs) > 0 {
						risk = out.Diffs[0].Risk
					}
					_ = d.Store.CreatePlan(ctx, store.Plan{
						ID:        out.PlanID,
						SessionID: d.Session.ID,
						OpsJSON:   string(opsJSON),
						DiffsJSON: mergeDiffsAndDenied(diffsJSON, deniedJSON),
						Risk:      risk,
						Status:    store.PlanStatusPending,
					})
				}
				// Surface the plan to the frontend so it can render
				// the diffs in a modal, then block on user decision.
				if d.Emit != nil {
					ev, _ := NewEvent(EventPlanReady, PlanReady{
						PlanID:  out.PlanID,
						Summary: out.Summary,
						Diffs:   out.Diffs,
						Denied:  out.Denied,
					})
					d.Emit(ev)
					ev, _ = NewEvent(EventPlanAwaitingConfirm, PlanAwaitingConfirm{PlanID: out.PlanID})
					d.Emit(ev)
				}
				if d.Session != nil {
						d.Session.ResetPlan() // clear stale cancelled/confirmed state
						if err := d.Session.WaitPlan(ctx); err != nil {
							return nil, fmt.Errorf("plan %s: %w", out.PlanID, err)
						}
					}
				return json.Marshal(map[string]any{
					"plan_id":  out.PlanID,
					"decision": d.planDecision(),
				})
			},
		},
		{
			Name:        "k8s_execute_plan",
			Description: "Execute a previously planned set of operations. Requires the plan_id returned by k8s_plan_write and a confirm_token. Operations are re-evaluated against policy at execute time.",
			InputSchema: executePlanSchema,
			Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
				var in k8s.ExecuteInput
				if err := json.Unmarshal(call.Input, &in); err != nil {
					return nil, fmt.Errorf("invalid input: %w", err)
				}
				ops, err := loadOpsForPlan(ctx, d.Store, in.PlanID)
				if err != nil {
					return nil, err
				}
				// Apply session-bound cluster_id to operations that
				// were planned before the user switched clusters.
				// Same JSON round-trip as in k8s_plan_write because
				// k8s.Operation.clusterID is unexported.
				if d.Session != nil && d.Session.ClusterID != "" {
					bound := d.Session.ClusterID
					for i := range ops {
						raw, err := json.Marshal(ops[i])
						if err != nil {
							continue
						}
						var m map[string]any
						if err := json.Unmarshal(raw, &m); err != nil {
							continue
						}
						if cid, _ := m["cluster_id"].(string); cid == "" {
							m["cluster_id"] = bound
							fixed, err := json.Marshal(m)
							if err != nil {
								continue
							}
							_ = json.Unmarshal(fixed, &ops[i])
						}
					}
				}
				out, err := k8s.ExecutePlan(ctx, d.Factory, d.Engine, d.Store, in, ops)
				if err != nil {
					return nil, err
				}
				if d.Store != nil {
					_ = d.Store.MarkExecuted(ctx, in.PlanID)
				}
				return json.Marshal(out)
			},
		},
		{
			Name:        "k8s_ask_user",
			Description: "Ask the user a clarifying question. The frontend renders an input form. The next turn the user provides an answer; until then the agent loop blocks.",
			InputSchema: askUserSchema,
			Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
				var in k8s.AskUserInput
				if err := json.Unmarshal(call.Input, &in); err != nil {
					return nil, fmt.Errorf("invalid input: %w", err)
				}
				if d.Emit != nil {
					ev, _ := NewEvent(EventAskUser, AskUserPayload{
						Question:    in.Question,
						Options:     in.Options,
						MultiSelect: in.MultiSelect,
					})
					d.Emit(ev)
				}
				if d.Session != nil {
					if err := d.Session.WaitAsk(ctx); err != nil {
						return nil, fmt.Errorf("ask_user: %w", err)
					}
					return json.Marshal(map[string]any{
						"question_id": hashString(in.Question),
						"answer":      d.Session.AskAnswer,
					})
				}
				out := k8s.AskUser(in)
				return json.Marshal(out)
			},
		},
		// fs_read tool - only registered if FSReadAllowedDir is set
		{
			Name:        "fs_read",
			Description: "Read a file from the local filesystem. Access is restricted to ~/.kubernetes-agent/ directory.",
			InputSchema: agent.FSReadSchema,
			Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
				if d.FSReadAllowedDir == "" {
					slog.Warn("fs_read: not configured (FSReadAllowedDir is empty)")
					return json.Marshal(map[string]string{"error": "fs_read not configured"})
				}
				slog.Debug("fs_read: called", "allowed_dir", d.FSReadAllowedDir, "input", string(call.Input))
				tool := agent.NewFSReadTool(d.FSReadAllowedDir)
				return tool.Handle(ctx, call.Input)
			},
		},
		// load_skill tool - loads a SKILL.md by name and returns its content.
		// The LLM calls this when its task matches a skill description.
		// Unlike fs_read, this is a name-based API so the LLM never has
		// to construct a file path.
		{
			Name:        "load_skill",
			Description: "Load a SKILL.md file by its name. Use this when the user's task matches a skill description from the system prompt. Pass the skill name exactly as it appears in the <name> tag (e.g. \"k8s-debug-pod\"). Returns the skill's workflow instructions.",
			InputSchema: agent.LoadSkillSchema,
			Handler: func(ctx context.Context, call llm.ToolCall) ([]byte, error) {
				if d.Skills == nil {
					slog.Warn("load_skill: not configured (Skills is nil)")
					return json.Marshal(map[string]string{"error": "load_skill not configured"})
				}
				slog.Debug("load_skill: called", "input", string(call.Input))
				tool := agent.NewSkillTool(d.Skills)
				return tool.Handle(ctx, call.Input)
			},
		},
	}
}

// fillClusterID assigns the session-bound cluster_id to *cid when
// the caller did not provide one. This lets the LLM omit cluster_id
// from tool arguments without breaking the call: the agent loop
// already knows which cluster the user is talking to via the
// session. If the LLM did pass one (e.g. for an explicit cluster
// switch), keep the LLM's value.
func (d *ToolDeps) fillClusterID(cid *string) {
	if *cid != "" || d.Session == nil {
		return
	}
	d.Session.mu.Lock()
	bound := d.Session.ClusterID
	d.Session.mu.Unlock()
	if bound != "" {
		*cid = bound
	}
}

// planDecision reports the user's plan decision ("confirmed" /
// "cancelled") for the most recent plan. Used to feed back to the
// LLM in the tool result.
func (d *ToolDeps) planDecision() string {
	if d.Session == nil {
		return "confirmed"
	}
	d.Session.mu.Lock()
	defer d.Session.mu.Unlock()
	if d.Session.PlanResult == "" {
		return "confirmed"
	}
	return d.Session.PlanResult
}

// mergeDiffsAndDenied stores the diffs and denied lists in the
// plans.DiffsJSON column as a JSON object {"diffs": ..., "denied": ...}
// so execute_plan can read them later (we keep diffs for audit).
func mergeDiffsAndDenied(diffsJSON, deniedJSON []byte) string {
	var diffs, denied json.RawMessage
	_ = json.Unmarshal(diffsJSON, &diffs)
	_ = json.Unmarshal(deniedJSON, &denied)
	out, _ := json.Marshal(map[string]json.RawMessage{
		"diffs":  diffs,
		"denied": denied,
	})
	return string(out)
}

// loadOpsForPlan reads a plan's persisted operations back from the
// store. The plan_write handler wrote them as JSON; execute_plan
// needs the typed []Operation to re-evaluate against policy.
func loadOpsForPlan(ctx context.Context, st *store.DB, planID string) ([]k8s.Operation, error) {
	if st == nil {
		return nil, fmt.Errorf("store not configured")
	}
	plan, err := st.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}
	var ops []k8s.Operation
	if err := json.Unmarshal([]byte(plan.OpsJSON), &ops); err != nil {
		return nil, fmt.Errorf("decode plan ops: %w", err)
	}
	return ops, nil
}

// hashString is a small stable hash for ask_user question IDs. It
// mirrors k8s.hashQ but is duplicated here to avoid the dependency
// (the agent package does not own the tool layer's helpers).
func hashString(s string) string {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%x", h)
}

// --- JSON schemas for each tool's input. Hand-written and minimal —
// JSON Schema 2020-12 with the subset our LLM providers understand.
// `required` lists the keys we treat as mandatory; properties that
// are optional use omitempty in the typed input struct and are not
// marked required here.

var getSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"resource":   map[string]any{"type": "string", "description": "[REQUIRED] lowercase plural resource name (e.g. pods, deployments)"},
		"name":       map[string]any{"type": "string", "description": "[REQUIRED] resource name"},
		"namespace":  map[string]any{"type": "string", "description": "namespace (defaults to 'default')"},
		"cluster_id": map[string]any{"type": "string", "description": "cluster id (UUID). Optional: when omitted, the session-bound cluster is used automatically."},
	},
	"required": []string{"resource", "name"},
}

var listSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"resource":       map[string]any{"type": "string", "description": "[REQUIRED] lowercase plural resource name"},
		"namespace":      map[string]any{"type": "string", "description": "namespace (empty = all)"},
		"label_selector": map[string]any{"type": "string", "description": "label selector (e.g. app=nginx)"},
		"cluster_id":     map[string]any{"type": "string", "description": "cluster id (UUID). Optional: when omitted, the session-bound cluster is used automatically."},
	},
	"required": []string{"resource"},
}

var describeSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"resource":   map[string]any{"type": "string", "description": "[REQUIRED] lowercase plural resource name"},
		"name":       map[string]any{"type": "string", "description": "[REQUIRED] resource name"},
		"namespace":  map[string]any{"type": "string", "description": "namespace (defaults to 'default')"},
		"cluster_id": map[string]any{"type": "string", "description": "cluster id (UUID). Optional: when omitted, the session-bound cluster is used automatically."},
	},
	"required": []string{"resource", "name"},
}

var planWriteSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"operations": map[string]any{
			"type":        "array",
			"description": "[REQUIRED] List of write operations to plan. Each op has action (apply|delete|scale), resource, name, namespace, and for apply the manifest.",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action":     map[string]any{"type": "string", "enum": []string{"apply", "delete", "scale"}},
					"resource":   map[string]any{"type": "string"},
					"name":       map[string]any{"type": "string"},
					"namespace":  map[string]any{"type": "string"},
					"kind":       map[string]any{"type": "string"},
					"replicas":   map[string]any{"type": "integer"},
					"manifest":   map[string]any{"type": "object", "additionalProperties": true},
					"cluster_id": map[string]any{"type": "string", "description": "cluster id (UUID). Optional: when omitted, the session-bound cluster is used automatically."},
				},
				"required": []string{"action", "resource", "name", "namespace"},
			},
		},
	},
	"required": []string{"operations"},
}

var executePlanSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"plan_id":       map[string]any{"type": "string", "description": "[REQUIRED] plan_id returned by k8s_plan_write"},
		"confirm_token": map[string]any{"type": "string", "description": "[REQUIRED] opaque token that the user must approve"},
	},
	"required": []string{"plan_id", "confirm_token"},
}

var askUserSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"question":     map[string]any{"type": "string", "description": "[REQUIRED] question to ask the user"},
		"options":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"multi_select": map[string]any{"type": "boolean"},
	},
	"required": []string{"question"},
}

// AllToolNames returns the canonical list of tool names registered by
// RegisterK8sTools. Used by tests.
func AllToolNames() []string {
	return []string{
		"k8s_get", "k8s_list", "k8s_describe",
		"k8s_plan_write", "k8s_execute_plan", "k8s_ask_user",
	}
}

