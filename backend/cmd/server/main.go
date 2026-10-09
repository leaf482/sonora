package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/leaf482/sonora/backend/internal/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.database != nil {
		db, err := postgres.Open(ctx, cfg.database, cfg.databaseTimeout)
		if err != nil {
			return err
		}
		defer db.Close()
		logger.Info("database connected")
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.port)))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	server := newServer(logger)
	logger.Info("server listening", "address", listener.Addr().String())
	if err := serve(ctx, server, listener, 10*time.Second); err != nil {
		return err
	}
	logger.Info("server stopped")
	return nil
}
