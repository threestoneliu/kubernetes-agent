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

	deps := buildDeps(db, aead)
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
// chat route's RunnerFactory is nil in this minimal wiring — the
// chat handler returns a 400 on validation before touching the
// factory, and the SSE stream emits an internal error if it ever
// gets that far. Task 13 (full LLM + agent wiring) replaces this
// stub with a real factory.
func buildDeps(db *store.DB, aead *crypto.AEAD) server.Deps {
	registry := &llm.Registry{
		Providers: []llm.Provider{},
		Clients:   map[string]llm.Client{},
		Health:    map[string]llm.PingStatus{},
	}
	factory := k8s.NewClientFactory(db, aead)
	return server.Deps{
		DB:      db,
		AEAD:    aead,
		Engine:  &policy.Engine{Rules: policy.DefaultRules()},
		LLM:     registry,
		Factory: factory,
		Sessions: agent.NewSessionManager(),
	}
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
