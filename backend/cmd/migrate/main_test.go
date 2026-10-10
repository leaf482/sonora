package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leaf482/sonora/backend/internal/postgres"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

func TestCommandValidation(t *testing.T) {
	for _, args := range [][]string{nil, {"down"}, {"up", "extra"}} {
		if err := run(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Fatalf("expected usage error for %v", args)
		}
	}
	t.Setenv("DATABASE_URL", "")
	if err := run(context.Background(), []string{"up"}, &bytes.Buffer{}); err == nil {
		t.Fatal("missing DATABASE_URL must fail")
	}
	t.Setenv("DATABASE_URL", "postgres://test:secret%@localhost/test")
	if err := run(context.Background(), []string{"status"}, &bytes.Buffer{}); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("malformed URL must fail without disclosing credentials")
	}
	t.Setenv("DATABASE_URL", "postgres://test:secret@localhost/test")
	t.Setenv("DB_CONNECT_TIMEOUT", "0s")
	if err := run(context.Background(), []string{"up"}, &bytes.Buffer{}); err == nil {
		t.Fatal("invalid timeout must fail")
	}
}

// This opt-in test requires an empty disposable database and never resets one.
func TestMigrationsLive(t *testing.T) {
	raw, set := os.LookupEnv("SONORA_TEST_DATABASE_URL")
	if !set {
		t.Skip("set SONORA_TEST_DATABASE_URL to a fresh disposable PostgreSQL database")
	}
	t.Setenv("DATABASE_URL", raw)
	t.Setenv("DB_CONNECT_TIMEOUT", "5s")
	t.Chdir(filepath.Join("..", ".."))
	cfg, err := postgres.ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := postgres.Open(ctx, cfg, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&count); err != nil {
		t.Fatal("cannot inspect fresh database")
	}
	if count != 0 {
		t.Fatal("live migration test requires an empty public schema; no reset performed")
	}
	check := func(command, want string) {
		t.Helper()
		var output bytes.Buffer
		if err := run(ctx, []string{command}, &output); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), want) {
			t.Fatalf("%s output %q, want %q", command, output.String(), want)
		}
		t.Log(strings.TrimSpace(output.String()))
	}
	check("status", "00001 pending")
	check("status", "00002 pending")
	check("up", "applied 2 migration(s)")
	check("status", "00001 applied")
	check("status", "00002 applied")
	var before int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM goose_db_version").Scan(&before); err != nil {
		t.Fatal("cannot inspect migration bookkeeping")
	}
	check("up", "applied 0 migration(s)")
	check("status", "00001 applied")
	check("status", "00002 applied")
	var after int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM goose_db_version").Scan(&after); err != nil || before != after {
		t.Fatal("repeated apply changed bookkeeping")
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name NOT IN ('goose_db_version', 'artists', 'albums')").Scan(&count); err != nil || count != 0 {
		t.Fatal("migrations created unexpected tables")
	}
	checkCatalogSchema(t, ctx, db)
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"),
		goose.WithSessionLocker(locker), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Down(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source.Version != 2 {
		t.Fatalf("rolled back version %d, want 2", result.Source.Version)
	}
	check("status", "00001 applied")
	check("status", "00002 pending")
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name <> 'goose_db_version'").Scan(&count); err != nil || count != 0 {
		t.Fatal("rollback left catalog tables")
	}
	t.Log("rollback removed artists/albums and preserved baseline")
	check("up", "applied 1 migration(s)")
	check("status", "00002 applied")
	checkCatalogSchema(t, ctx, db)
	t.Log("reapply restored schema and constraints")

}
