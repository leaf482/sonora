package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leaf482/sonora/backend/internal/catalog"
	"github.com/leaf482/sonora/backend/internal/postgres"
	"github.com/pressly/goose/v3"
)

func TestCatalogValidation(t *testing.T) {
	h := newServer(testLogger(), nil).Handler
	for _, resource := range []string{"artists", "albums", "tracks"} {
		base := "/api/" + resource
		for _, suffix := range []string{"?limit=-1", "?limit=101", "?limit=", "?limit=1&limit=2", "?offset=-1", "?offset=", "?offset=0&offset=1", "?limit=+1", "?limit=1.0", "?offset=99999999999999999999999", "?unknown=1", "?limit=%zz", "?limit=1;offset=2", "/bad", "/00000000000040008000000000000001", "/00000000-0000-4000-8000-00000000000g", "/00000000-0000-4000-8000-000000000001?limit=1"} {
			t.Run(resource+suffix, func(t *testing.T) {
				r := httptest.NewRequest("GET", base+suffix, nil)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				assertAPIError(t, w, 400, "invalid_request")
			})
		}
		for _, suffix := range []string{"", "?limit=0", "?limit=100&offset=2147483647", "/ABCDEF00-0000-4000-8000-000000000001"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", base+suffix, nil))
			assertAPIError(t, w, 503, "catalog_unavailable")
		}
		for _, suffix := range []string{"", "/00000000-0000-4000-8000-000000000001"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("POST", base+suffix, nil))
			assertAPIError(t, w, 405, "method_not_allowed")
			if w.Header().Get("Allow") != "GET, HEAD" {
				t.Fatal("Allow header")
			}
		}
	}
}
func assertAPIError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var value struct {
		Error struct{ Code, Message string }
	}
	if w.Code != status || w.Header().Get("Content-Type") != "application/json" || json.Unmarshal(w.Body.Bytes(), &value) != nil || value.Error.Code != code || value.Error.Message == "" {
		t.Fatalf("error response: %d %s", w.Code, w.Body.String())
	}
}
func TestCatalogDatabaseFailure(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://localhost/unused")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	h := newServer(testLogger(), catalog.NewRepository(db)).Handler
	for _, path := range []string{"/api/tracks", "/api/tracks/00000000-0000-4000-8000-000000000001", "/api/artists", "/api/albums", "/api/artists/00000000-0000-4000-8000-000000000001", "/api/albums/00000000-0000-4000-8000-000000000001"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		assertAPIError(t, w, 500, "internal_error")
		if strings.Contains(w.Body.String(), "sql") {
			t.Fatal("database error leaked")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != 200 {
		t.Fatal("health changed")
	}
}

// This test writes synthetic records only to a caller-provided empty disposable DB.
func TestCatalogAPILive(t *testing.T) {
	raw, set := os.LookupEnv("SONORA_TEST_DATABASE_URL")
	if !set {
		t.Skip("set SONORA_TEST_DATABASE_URL to an empty disposable PostgreSQL database")
	}
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
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&count); err != nil || count != 0 {
		t.Fatal("API live test requires empty public schema; no reset performed")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	handler := newServer(testLogger(), catalog.NewRepository(db)).Handler
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	get := func(path string, status int) map[string]json.RawMessage {
		t.Helper()
		resp, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != status || resp.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("%s: %d %s", path, resp.StatusCode, body)
		}
		var value map[string]json.RawMessage
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, resource := range []string{"artists", "albums"} {
		if string(get("/api/"+resource, 200)["data"]) != "[]" {
			t.Fatal("empty list is not []")
		}
		v := get("/api/"+resource+"/ffffffff-ffff-4fff-8fff-ffffffffffff", 404)
		if !strings.Contains(string(v["error"]), "not_found") {
			t.Fatal("404 shape")
		}
	}
	for _, q := range []string{
		`INSERT INTO artists(id,name,biography) SELECT ('10000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,CASE WHEN i<104 THEN 'A' ELSE 'a' END,CASE WHEN i=1 THEN 'Credit' WHEN i=2 THEN '' ELSE NULL END FROM generate_series(1,105) i ORDER BY i DESC`,
		`INSERT INTO albums(id,primary_artist_id,title,release_date) SELECT ('20000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,('10000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,CASE WHEN i<104 THEN 'A' ELSE 'a' END,CASE WHEN i=1 THEN DATE '2024-02-29' ELSE NULL END FROM generate_series(1,105) i ORDER BY i DESC`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	for ri, resource := range []string{"artists", "albums"} {
		for _, p := range []struct {
			query            string
			limit, offset, n int
		}{{"", 20, 0, 20}, {"?limit=0", 20, 0, 20}, {"?limit=1&offset=1", 1, 1, 1}, {"?limit=100", 100, 0, 100}, {"?limit=100&offset=100", 100, 100, 5}, {"?offset=105", 20, 105, 0}} {
			v := get("/api/"+resource+p.query, 200)
			var rows []struct{ ID string }
			if err := json.Unmarshal(v["data"], &rows); err != nil {
				t.Fatal(err)
			}
			var page paginationJSON
			if err := json.Unmarshal(v["pagination"], &page); err != nil {
				t.Fatal(err)
			}
			if len(rows) != p.n || page.Limit != p.limit || page.Offset != p.offset {
				t.Fatalf("page %s: %s", resource, v)
			}
			for i, row := range rows {
				if row.ID != fmt.Sprintf("%d0000000-0000-4000-8000-%012d", ri+1, p.offset+i+1) {
					t.Fatal("ordering")
				}
			}
		}
		for i := 1; i <= 3; i++ {
			id := fmt.Sprintf("%d0000000-0000-4000-8000-%012d", ri+1, i)
			v := get("/api/"+resource+"/"+id, 200)
			if _, ok := v["pagination"]; ok {
				t.Fatal("detail pagination")
			}
			var row map[string]json.RawMessage
			json.Unmarshal(v["data"], &row)
			if string(row["id"]) != strconvQuote(id) || row["created_at"] == nil || row["updated_at"] == nil {
				t.Fatal("missing persisted fields")
			}
			if resource == "artists" {
				want := []string{`"Credit"`, `""`, `null`}[i-1]
				if string(row["biography"]) != want || string(row["name"]) != `"A"` {
					t.Fatal("artist metadata")
				}
			} else {
				want := "null"
				if i == 1 {
					want = `"2024-02-29"`
				}
				if string(row["release_date"]) != want || string(row["title"]) != `"A"` || string(row["primary_artist_id"]) != strconvQuote(fmt.Sprintf("10000000-0000-4000-8000-%012d", i)) {
					t.Fatal("album metadata")
				}
			}
		}
		resp, err := client.Head(server.URL + "/api/" + resource)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || len(body) != 0 {
			t.Fatal("HEAD")
		}
	}
	// Exclusive locks force every read to wait, proving request deadlines reach SQL.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "LOCK TABLE artists, albums IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/artists", "/api/albums", "/api/artists/10000000-0000-4000-8000-000000000001", "/api/albums/20000000-0000-4000-8000-000000000001"} {
		requestCtx, stop := context.WithTimeout(ctx, 100*time.Millisecond)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil).WithContext(requestCtx))
		stop()
		assertAPIError(t, w, 504, "catalog_timeout")
		canceled, stop := context.WithCancel(ctx)
		stop()
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil).WithContext(canceled))
		if w.Body.Len() != 0 {
			t.Fatal("wrote response to canceled request")
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	get("/api/artists?limit=1", 200)
	t.Log("fresh migrations; real HTTP empty/404/details/nulls/date/pagination/order/HEAD; all four query deadlines/cancellation; recovery passed")
}
func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }
