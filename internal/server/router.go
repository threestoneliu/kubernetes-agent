package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/threestoneliu/kubernetes-agent/internal/agent"
	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"github.com/threestoneliu/kubernetes-agent/internal/tools/k8s"
)

// Deps is the bag of backend handles the HTTP layer needs. A
// concrete-value struct (not a pointer) so the caller can pass it
// around by value into each handler constructor without the usual
// nil-pointer anxiety.
type Deps struct {
	DB      *store.DB
	AEAD    *crypto.AEAD
	Engine  *policy.Engine
	LLM     *llm.Registry
	Factory k8s.ClientFactory
	// RunnerFactory builds a *agent.Runner for a given session id
	// and cluster id. The chat handler invokes it once per request.
	// It is an interface (not the concrete *agent.Runner) so tests
	// can inject stub runners without wiring up an llm.Client.
	RunnerFactory RunnerFactory
	// Sessions tracks active in-flight agent.Session values so the
	// /resume endpoint can unblock a plan confirm or ask_user
	// response. The chatHandler registers each new session here
	// before kicking off the runner goroutine.
	Sessions *agent.SessionManager
}

// RunnerFactory returns a ready-to-Run agent.Runner. The chat
// handler passes the supplied sessionID + clusterID so the runner
// can hydrate its Session and use the correct cluster as the default
// target for tool calls.
type RunnerFactory interface {
	NewRunner(sessionID, clusterID string) *agent.Runner
}

// NewRouter wires every route and middleware. It is the single
// construction site for the HTTP surface.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(requestIDMiddleware)
	r.Use(loggerMiddleware)
	r.Use(recovererMiddleware)
	r.Use(corsMiddleware)

	r.Get("/healthz", healthHandler(d))
	r.Post("/api/chat", chatHandler(d))

	r.Route("/api/clusters", func(r chi.Router) {
		r.Get("/", listClustersHandler(d))
		r.Post("/", createClusterHandler(d))
		r.Delete("/{id}", deleteClusterHandler(d))
	})

	r.Route("/api/policies", func(r chi.Router) {
		r.Get("/", listPoliciesHandler(d))
		r.Put("/{id}", updatePolicyHandler(d))
		r.Patch("/{id}/enabled", togglePolicyHandler(d))
	})

	r.Route("/api/sessions", func(r chi.Router) {
		r.Get("/", listSessionsHandler(d))
		r.Post("/", createSessionHandler(d))
		r.Get("/{id}", getSessionHandler(d))
		r.Get("/{id}/messages", listMessagesHandler(d))
		r.Post("/{id}/resume", resumeHandler(d))
	})

	// SPA fallback is mounted last so the explicit /api/* and
	// /healthz routes above take precedence. Anything not matched
	// by an earlier route is served by staticHandler, which
	// resolves to either a real file or index.html for client-side
	// routing.
	r.Handle("/*", staticHandler())
	return r
}
