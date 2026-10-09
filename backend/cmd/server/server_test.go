package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestHealth(t *testing.T) {
	server := httptest.NewServer(newServer(testLogger()).Handler)
	defer server.Close()

	for _, tt := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodHead, "/health", http.StatusOK},
		{http.MethodPost, "/health", http.StatusMethodNotAllowed},
		{http.MethodGet, "/health/extra", http.StatusNotFound},
		{http.MethodGet, "/", http.StatusNotFound},
	} {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			request, err := http.NewRequest(tt.method, server.URL+tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.status)
			}
			if tt.status == http.StatusOK {
				if got := response.Header.Get("Content-Type"); got != "application/json" {
					t.Fatalf("Content-Type = %q", got)
				}
				if tt.method == http.MethodHead {
					if len(body) != 0 {
						t.Fatalf("HEAD returned a body: %q", body)
					}
				} else {
					var payload map[string]string
					if err := json.Unmarshal(body, &payload); err != nil {
						t.Fatalf("invalid JSON: %v", err)
					}
					if len(payload) != 1 || payload["status"] != "ok" {
						t.Fatalf("unexpected health response: %s", body)
					}
				}
			}
			if tt.status == http.StatusMethodNotAllowed && response.Header.Get("Allow") != "GET, HEAD" {
				t.Fatalf("Allow = %q", response.Header.Get("Allow"))
			}
		})
	}
}

func startTestServer(t *testing.T, server *http.Server, timeout time.Duration) (context.CancelFunc, string, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, listener, timeout) }()
	t.Cleanup(func() {
		cancel()
		_ = server.Close()
	})
	return cancel, "http://" + listener.Addr().String(), done
}

func waitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server lifecycle event")
	}
}

func waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server/client result")
		return nil
	}
}

func getAsync(url string) <-chan error {
	result := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		defer client.CloseIdleConnections()
		response, err := client.Get(url)
		if err == nil {
			defer response.Body.Close()
			var body []byte
			body, err = io.ReadAll(response.Body)
			if err == nil && (response.StatusCode != http.StatusOK || string(body) != "done") {
				err = errors.New("request did not complete with the expected response")
			}
		}
		result <- err
	}()
	return result
}

func TestShutdownDrainsActiveRequest(t *testing.T) {
	entered, release, shuttingDown := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)

	server := newServer(testLogger())
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "done")
	})
	server.RegisterOnShutdown(func() { close(shuttingDown) })
	cancel, url, done := startTestServer(t, server, 2*time.Second)
	result := getAsync(url)
	waitSignal(t, entered)

	cancel()
	waitSignal(t, shuttingDown)
	select {
	case err := <-done:
		t.Fatalf("server returned before the active request finished: %v", err)
	default:
	}
	unblock()
	if err := waitResult(t, result); err != nil {
		t.Fatalf("in-flight request failed: %v", err)
	}
	if err := waitResult(t, done); err != nil {
		t.Fatalf("graceful shutdown failed: %v", err)
	}
}

func TestShutdownDeadlineClosesActiveRequest(t *testing.T) {
	entered, requestCanceled := make(chan struct{}), make(chan struct{})
	server := newServer(testLogger())
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(requestCanceled)
	})
	cancel, url, done := startTestServer(t, server, 20*time.Millisecond)
	result := getAsync(url)
	waitSignal(t, entered)

	cancel()
	if err := waitResult(t, done); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown error = %v, want deadline exceeded", err)
	}
	waitSignal(t, requestCanceled)
	if err := waitResult(t, result); err == nil {
		t.Fatal("expected the unfinished HTTP request to fail on forced close")
	}
}

func TestServeWithCanceledContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, newServer(testLogger()), listener, time.Second) }()
	if err := waitResult(t, done); err != nil {
		t.Fatalf("already-canceled startup failed: %v", err)
	}
}

func TestServeReportsListenerFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- serve(context.Background(), newServer(testLogger()), listener, time.Second) }()
	if err := waitResult(t, done); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("serve error = %v, want closed-listener failure", err)
	}
}
