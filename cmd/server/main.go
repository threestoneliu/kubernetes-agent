package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/threestoneliu/kubernetes-agent/internal/agent"
	"github.com/threestoneliu/kubernetes-agent/internal/config"
	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/llm"
	"github.com/threestoneliu/kubernetes-agent/internal/logging"
	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/server"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
	"github.com/threestoneliu/kubernetes-agent/internal/tools/k8s"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logging.Setup(cfg.Logging.Level, cfg.Logging.Format)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, aead, err := startup(cfg, ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	deps := buildDeps(cfg, db, aead)
	router := server.NewRouter(deps)

	addr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.Port))
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("http listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http listen: %w", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("http shutdown", "err", err)
		}
	}

	slog.Info("shutdown")
	return nil
}

// buildDeps assembles the dependency bag the HTTP layer needs. The
// chat route's RunnerFactory is wired to a real *agent.Runner
// builder that resolves the configured LLM provider from the
// registry, constructs the six k8s tools, and hands the runner a
// per-session agent.Session.
func buildDeps(cfg *config.Config, db *store.DB, aead *crypto.AEAD) server.Deps {
	registry := buildRegistry(cfg)
	factory := k8s.NewClientFactory(db, aead)
	engine := &policy.Engine{Rules: policy.DefaultRules()}
	rf := newRunnerFactory(registry, db, factory, engine, cfg.LLM.Default)
	return server.Deps{
		DB:            db,
		AEAD:          aead,
		Engine:        engine,
		LLM:           registry,
		Factory:       factory,
		RunnerFactory: rf,
		Sessions:      agent.NewSessionManager(),
	}
}

// buildRegistry turns the on-disk LLM config into a populated
// llm.Registry. Providers missing the prerequisites for their type
// (api key, model, base URL) are skipped with a warning so a single
// broken entry doesn't take down the whole agent.
func buildRegistry(cfg *config.Config) *llm.Registry {
	reg := &llm.Registry{
		Providers: make([]llm.Provider, 0, len(cfg.LLM.Providers)),
		Clients:   map[string]llm.Client{},
		Health:    map[string]llm.PingStatus{},
	}
	for _, p := range cfg.LLM.Providers {
		prov := llm.Provider{
			Name: p.Name, Type: p.Type, APIKey: p.APIKey,
			BaseURL: p.BaseURL, Model: p.Model,
		}
		reg.Providers = append(reg.Providers, prov)
		c, err := buildClient(prov)
		if err != nil {
			slog.Warn("llm provider skipped", "name", p.Name, "err", err)
			continue
		}
		reg.Clients[p.Name] = c
	}
	return reg
}

func buildClient(p llm.Provider) (llm.Client, error) {
	switch p.Type {
	case "anthropic":
		return llm.NewAnthropicClient(p)
	case "openai":
		return llm.NewOpenAIClient(p)
	case "openai-compatible", "openai_compat":
		return llm.NewOpenAICompatClient(p)
	default:
		return nil, fmt.Errorf("unknown provider type %q", p.Type)
	}
}

// runnerFactory resolves the active LLM client on each request (or
// once at build time — clients are stateless and safe to share
// across goroutines) and returns a fresh *agent.Runner configured
// with the six k8s tools, the policy engine, the store, and the
// per-session state.
type runnerFactory struct {
	registry     *llm.Registry
	defaultName  string
	defaultCli   llm.Client
	db           *store.DB
	factory      k8s.ClientFactory
	engine       *policy.Engine
}

func newRunnerFactory(reg *llm.Registry, db *store.DB, factory k8s.ClientFactory, engine *policy.Engine, defaultName string) *runnerFactory {
	cli := pickDefaultClient(reg, defaultName)
	return &runnerFactory{
		registry:    reg,
		defaultName: defaultName,
		defaultCli:  cli,
		db:          db,
		factory:     factory,
		engine:      engine,
	}
}

func (rf *runnerFactory) NewRunner(sessionID, clusterID string) *agent.Runner {
	cli := rf.defaultCli
	if cli == nil {
		// Fall back to picking a client at request time so
		// late-registered providers (tests) are picked up.
		cli = pickDefaultClient(rf.registry, rf.defaultName)
	}
	// Build the runner first so the tool handlers (registered
	// below) can observe the same ToolDeps the agent loop
	// mutates — Run wires deps.Emit and deps.Session lazily
	// on the first Chat call so plan / ask can surface events
	// and block on the per-session resume channels.
	r := &agent.Runner{Client: cli, Store: rf.db}
	r.Deps = agent.ToolDeps{Factory: rf.factory, Engine: rf.engine, Store: rf.db}
	r.Tools = agent.RegisterK8sTools(&r.Deps)
	return r
}

// pickDefaultClient returns the LLM client the agent loop should
// drive. It honours the configured default name first, then the
// first registered client in declaration order, and finally nil
// (the runner will surface a clean "no LLM configured" error).
func pickDefaultClient(reg *llm.Registry, defaultName string) llm.Client {
	if defaultName != "" {
		if c, ok := reg.Clients[defaultName]; ok {
			return c
		}
	}
	for _, p := range reg.Providers {
		if c, ok := reg.Clients[p.Name]; ok {
			return c
		}
	}
	return nil
}

// startup runs the master-key + DB + migrate + seed sequence and returns the
// opened DB and AEAD. It is split out from run() so the integration test in
// main_test.go can exercise the wiring without blocking on a signal.
func startup(cfg *config.Config, ctx context.Context) (*store.DB, *crypto.AEAD, error) {
	key, err := crypto.LoadMasterKey()
	if err != nil {
		return nil, nil, fmt.Errorf("master key: %w", err)
	}
	aead, err := crypto.NewAEAD(key)
	if err != nil {
		return nil, nil, fmt.Errorf("aead: %w", err)
	}

	db, err := store.Open(cfg.Storage.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}
	if err := db.SeedDefaultPolicies(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("seed default policies: %w", err)
	}
	return db, aead, nil
}
