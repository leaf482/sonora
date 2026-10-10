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
	if len(args) != 1 || args[0] != "development" {
		return errors.New("usage: go run ./cmd/seed development (from backend/; development database only)")
	}
	assets, err := loadFixtures("../fixtures/audio")
	if err != nil {
		return err
	}
	cfg, err := postgres.ParseURL(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	timeout, err := postgres.ConnectTimeout()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	db, err := postgres.Open(ctx, cfg, timeout)
	if err != nil {
		return err
	}
	defer db.Close()
	inserted, err := seed(ctx, db, assets, ".")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "development fixtures: %d new assets, %d preserved; new tracks draft, files staged; storage .local/audio\n", inserted, len(assets)-inserted)
	return err
}
