package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDatabaseStartupTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Accept TCP, but never finish the PostgreSQL handshake.
		_, _ = io.Copy(io.Discard, conn)
	}()
	cfg, err := parseDatabaseURL("postgres://test:secret@" + listener.Addr().String() + "/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	db, err := openDatabase(context.Background(), cfg, 50*time.Millisecond)
	if db != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected nil DB and deadline, got db=%v err=%v", db != nil, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("database startup was not bounded")
	}
	_ = listener.Close()    // Unblock Accept if cancellation won the connection race.
	waitSignal(t, peerDone) // The failed connection was closed, not leaked.
}

func TestDatabaseStartupCancellation(t *testing.T) {
	cfg, err := parseDatabaseURL("postgres://test:secret@127.0.0.1:5432/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db, err := openDatabase(ctx, cfg, time.Second)
	if db != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected nil DB and cancellation, got db=%v err=%v", db != nil, err)
	}
}

func TestDatabaseStartupFailureDoesNotExposeCredentials(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()
	const secret = "do-not-log-this-password"
	cfg, err := parseDatabaseURL("postgres://test:" + secret + "@" + listener.Addr().String() + "/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := openDatabase(context.Background(), cfg, time.Second)
	if db != nil || err == nil {
		t.Fatal("expected failed startup")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "postgres://") {
		t.Fatal("database error exposed connection credentials")
	}
	_ = listener.Close()
	waitSignal(t, done)
}

// Explicit opt-in: never silently use a developer's DATABASE_URL for tests.
func TestPostgresLive(t *testing.T) {
	raw, set := os.LookupEnv("SONORA_TEST_DATABASE_URL")
	if !set {
		t.Skip("set SONORA_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	cfg, err := parseDatabaseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	db, err := openDatabase(context.Background(), cfg, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal("closing database failed")
	}
	if db.Stats().OpenConnections != 0 {
		t.Fatal("database connections remained open after Close")
	}

	// Verify authentication failure too; no application SQL or schema changes.
	u, _ := url.Parse(raw)
	u.User = url.UserPassword(u.User.Username(), "deliberately-wrong-smoke-password")
	bad, err := parseDatabaseURL(u.String())
	if err != nil {
		t.Fatal(err)
	}
	unexpected, err := openDatabase(context.Background(), bad, 5*time.Second)
	if unexpected != nil {
		_ = unexpected.Close()
		t.Fatal("wrong password was accepted; use a password-authenticated disposable database")
	}
	if err == nil {
		t.Fatal("wrong password did not fail")
	}
}
