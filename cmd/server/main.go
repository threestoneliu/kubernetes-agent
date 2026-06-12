package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/threestoneliu/kubernetes-agent/internal/config"
	"github.com/threestoneliu/kubernetes-agent/internal/logging"
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
	slog.Info("startup", "host", cfg.Server.Host, "port", cfg.Server.Port)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()
	slog.Info("shutdown")
	return nil
}
