package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/threestoneliu/kubernetes-agent/internal/config"
	"github.com/threestoneliu/kubernetes-agent/internal/crypto"
	"github.com/threestoneliu/kubernetes-agent/internal/logging"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
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
	_ = aead // unused until Task 5 wires clusters through crypto

	slog.Info("startup complete",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"db", cfg.Storage.DBPath,
	)

	<-ctx.Done()
	slog.Info("shutdown")
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
	if err := db.SeedDefaultsIfEmpty(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("seed default policies: %w", err)
	}
	return db, aead, nil
}
