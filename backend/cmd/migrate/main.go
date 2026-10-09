package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leaf482/sonora/backend/internal/postgres"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 1 || (args[0] != "up" && args[0] != "status") {
		return errors.New("usage: go run ./cmd/migrate {up|status} (from backend/)")
	}
	cfg, err := postgres.ParseURL(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	timeout, err := postgres.ConnectTimeout()
	if err != nil {
		return err
	}
	// Bound the entire operator command, including lock acquisition and SQL.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	db, err := postgres.Open(ctx, cfg, timeout)
	if err != nil {
		return err
	}
	defer db.Close()
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return errors.New("migration lock setup failed")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"),
		goose.WithSessionLocker(locker), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return errors.New("migration setup failed; check backend/migrations SQL files and run from backend/")
	}
	if args[0] == "up" {
		results, err := provider.Up(ctx)
		if err != nil {
			return migrationError(ctx)
		}
		_, err = fmt.Fprintf(out, "applied %d migration(s)\n", len(results))
		return err
	}
	statuses, err := provider.Status(ctx)
	if err != nil {
		return migrationError(ctx)
	}
	for _, status := range statuses {
		if _, err := fmt.Fprintf(out, "%05d %s %s\n", status.Source.Version, status.State, status.Source.Path); err != nil {
			return err
		}
	}
	return nil
}

func migrationError(ctx context.Context) error {
	if ctx.Err() != nil {
		return fmt.Errorf("migration command: %w", ctx.Err())
	}
	// Database/server errors can contain secrets. Never print raw driver errors.
	return errors.New("migration command failed; check database permissions, availability and migration SQL; inspect status before retrying")
}
