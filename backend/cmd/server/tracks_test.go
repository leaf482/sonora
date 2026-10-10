package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/leaf482/sonora/backend/internal/catalog"
	"github.com/leaf482/sonora/backend/internal/postgres"
	"github.com/pressly/goose/v3"
)

func TestTrackFilters(t *testing.T) {
	h := newServer(testLogger(), nil).Handler
	const id = "ABCDEF00-0000-4000-8000-000000000001"
	for _, key := range []string{"artist_id", "album_id"} {
		for _, value := range []string{"", "bad", id + "&" + key + "=" + id, "%20" + id, "%zz", "' OR true --"} {
			req := httptest.NewRequest("GET", "/api/tracks", nil)
			req.URL.RawQuery = key + "=" + value
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			assertAPIError(t, w, 400, "invalid_request")
		}
		for _, resource := range []string{"artists", "albums"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/api/"+resource+"?"+key+"="+id, nil))
			assertAPIError(t, w, 400, "invalid_request")
		}
	}
	page, filter, err := parseTrackQuery("artist_id=" + id + "&album_id=" + id + "&limit=0&offset=2")
	if err != nil || page.Limit != 20 || page.Offset != 2 || filter.ArtistID == nil || *filter.ArtistID != id || filter.AlbumID == nil || *filter.AlbumID != id {
		t.Fatal(page, filter, err)
	}
	for _, path := range []string{"/api/tracks", "/api/tracks/" + id} {
		for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
			assertAPIError(t, w, 405, "method_not_allowed")
			if w.Header().Get("Allow") != "GET, HEAD" {
				t.Fatal("Allow")
			}
		}
	}
}

func TestTrackAPILive(t *testing.T) {
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
		t.Fatal("track API test requires empty public schema; no reset performed")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
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
		if resp.StatusCode != status || resp.Header.Get("Content-Type") != "application/json" || resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d %s", path, resp.StatusCode, body)
		}
		var v map[string]json.RawMessage
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	const artist = "10000000-0000-4000-8000-000000000001"
	const other = "10000000-0000-4000-8000-000000000002"
	const album = "20000000-0000-4000-8000-000000000001"
	const absent = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	id := func(i int) string { return fmt.Sprintf("30000000-0000-4000-8000-%012d", i) }
	if string(get("/api/tracks", 200)["data"]) != "[]" {
		t.Fatal("empty list")
	}
	if !strings.Contains(string(get("/api/tracks/"+absent, 404)["error"]), "not_found") {
		t.Fatal("404 shape")
	}
	get("/api/tracks?artist_id=bad", 400)
	exec("INSERT INTO artists(id,name) VALUES ($1,'A'),($2,'B')", artist, other)
	exec("INSERT INTO albums(id,primary_artist_id,title) VALUES ($1,$2,'Album')", album, other)
	exec(`INSERT INTO tracks(id,primary_artist_id,album_id,title,display_credits,duration_ms,disc_number,track_number,state,created_at,updated_at)
 SELECT ('30000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,
 CASE WHEN i%2=1 THEN $1::uuid ELSE $2::uuid END,CASE WHEN i<=4 THEN $3::uuid ELSE NULL END,
 CASE WHEN i<104 THEN 'A' ELSE 'a' END,CASE WHEN i=1 THEN 'Credit' WHEN i=2 THEN '' ELSE NULL END,
 CASE WHEN i=1 THEN 1234 ELSE NULL END,CASE WHEN i<=4 THEN 1 ELSE NULL END,CASE WHEN i<=4 THEN i ELSE NULL END,
 CASE WHEN i=1 THEN 'active' WHEN i=2 THEN 'unavailable' ELSE 'draft' END,
 '2024-01-02T03:04:05Z','2024-02-03T04:05:06Z' FROM generate_series(1,105) i ORDER BY i DESC`, artist, other, album)
	// Deliberately unavailable/withdrawn media; catalog metadata must remain visible.
	exec(`INSERT INTO audio_assets(id,track_id,storage_key,sha256,mime_type,codec,byte_length,duration_ms,acquired_at,
 file_state,license_decision,reviewed_by,reviewed_at,review_rationale,unavailable_at,unavailable_reason,withdrawn_at,withdrawal_reason,evidence_reference)
 VALUES ('40000000-0000-4000-8000-000000000001',$1,'private.mp3',repeat('a',64),'audio/mpeg','mp3',42,1234,now(),
 'unavailable','withdrawn','private reviewer',now(),'synthetic only',now(),'missing',now(),'test withdrawal','private/evidence')`, id(2))
	for _, tc := range []struct {
		q             string
		limit, offset int
		ids           []int
	}{
		{"?limit=2", 2, 0, []int{1, 2}}, {"?limit=2&offset=1", 2, 1, []int{2, 3}},
		{"?limit=100&offset=100", 100, 100, []int{101, 102, 103, 104, 105}}, {"?offset=105", 20, 105, []int{}},
		{"?artist_id=" + artist + "&limit=2&offset=1", 2, 1, []int{3, 5}},
		{"?album_id=" + album, 20, 0, []int{1, 2, 3, 4}},
		{"?artist_id=" + artist + "&album_id=" + album, 20, 0, []int{1, 3}},
		{"?artist_id=" + other + "&album_id=" + album, 20, 0, []int{2, 4}},
		{"?artist_id=" + absent, 20, 0, []int{}}, {"?album_id=" + absent, 20, 0, []int{}},
	} {
		v := get("/api/tracks"+tc.q, 200)
		var rows []trackJSON
		var page paginationJSON
		if err := json.Unmarshal(v["data"], &rows); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(v["pagination"], &page); err != nil {
			t.Fatal(err)
		}
		if rows == nil || len(rows) != len(tc.ids) || page.Limit != tc.limit || page.Offset != tc.offset {
			t.Fatalf("page %s: %s", tc.q, v)
		}
		for i, row := range rows {
			if row.ID != id(tc.ids[i]) {
				t.Fatal("ordering/filter")
			}
		}
	}
	for _, q := range []string{"", "?limit=0", "?limit=100"} {
		v := get("/api/tracks"+q, 200)
		var rows []trackJSON
		json.Unmarshal(v["data"], &rows)
		want := 20
		if q == "?limit=100" {
			want = 100
		}
		if len(rows) != want {
			t.Fatal("limit", len(rows))
		}
	}
	for _, i := range []int{1, 2, 5} {
		v := get("/api/tracks/"+id(i), 200)
		if len(v) != 1 {
			t.Fatal("detail envelope")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(v["data"], &fields); err != nil {
			t.Fatal(err)
		}
		allowed := strings.Fields("id primary_artist_id album_id title display_credits duration_ms disc_number track_number state created_at updated_at")
		if len(fields) != len(allowed) {
			t.Fatal("unexpected public fields", fields)
		}
		for _, key := range allowed {
			if _, ok := fields[key]; !ok {
				t.Fatal("missing field", key)
			}
		}
		if string(fields["created_at"]) != `"2024-01-02T03:04:05Z"` || string(fields["updated_at"]) != `"2024-02-03T04:05:06Z"` {
			t.Fatal("timestamps")
		}
		if i == 1 && (string(fields["display_credits"]) != `"Credit"` || string(fields["duration_ms"]) != "1234" || string(fields["disc_number"]) != "1" || string(fields["track_number"]) != "1") {
			t.Fatal("known metadata")
		}
		if i == 2 && (string(fields["display_credits"]) != `""` || string(fields["state"]) != `"unavailable"`) {
			t.Fatal("unavailable track/empty credits")
		}
		if i == 5 {
			for _, key := range []string{"album_id", "display_credits", "duration_ms", "disc_number", "track_number"} {
				if string(fields[key]) != "null" {
					t.Fatal("NULL", key)
				}
			}
		}
		list := get(fmt.Sprintf("/api/tracks?limit=1&offset=%d", i-1), 200)
		var rows []json.RawMessage
		json.Unmarshal(list["data"], &rows)
		var listFields map[string]json.RawMessage
		json.Unmarshal(rows[0], &listFields)
		if !reflect.DeepEqual(fields, listFields) {
			t.Fatal("list/detail differ")
		}
	}
	for _, path := range []string{"/api/tracks", "/api/tracks/" + id(1)} {
		resp, err := client.Head(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || len(body) != 0 {
			t.Fatal("HEAD")
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "LOCK TABLE tracks IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/tracks?artist_id=" + artist, "/api/tracks/" + id(1)} {
		c, stop := context.WithTimeout(ctx, 100*time.Millisecond)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil).WithContext(c))
		stop()
		assertAPIError(t, w, 504, "catalog_timeout")
		c, stop = context.WithCancel(ctx)
		stop()
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil).WithContext(c))
		if w.Body.Len() != 0 {
			t.Fatal("canceled response")
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	get("/api/tracks?limit=1", 200)
	exec("ALTER TABLE audio_assets RENAME TO assets_hidden")
	get("/api/tracks/"+id(2), 200) // Proves no dependency on an AudioAsset query.
	exec("ALTER TABLE tracks RENAME TO tracks_hidden")
	for _, path := range []string{"/api/tracks", "/api/tracks/" + id(1)} {
		v := get(path, 500)
		if string(v["error"]) != `{"code":"internal_error","message":"Catalog request failed."}` {
			t.Fatal("unsanitized error", v)
		}
	}
	t.Log("PASS: live Track HTTP filters/pages/order/NULLs/withdrawal visibility/public field allowlist/HEAD/cancellation/SQL errors")
}
