package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/leaf482/sonora/backend/internal/postgres"
	"github.com/pressly/goose/v3"
)

func TestPagination(t *testing.T) {
	for _, tc := range []struct {
		page  Page
		limit int
		bad   bool
	}{
		{Page{}, 20, false}, {Page{1, 0}, 1, false}, {Page{100, 105}, 100, false},
		{Page{-1, 0}, 0, true}, {Page{101, 0}, 0, true}, {Page{20, -1}, 0, true},
	} {
		limit, offset, err := tc.page.bounds()
		if tc.bad {
			if !errors.Is(err, ErrInvalidPagination) {
				t.Fatalf("%+v: %v", tc.page, err)
			}
		} else if err != nil || limit != tc.limit || offset != tc.page.Offset {
			t.Fatalf("%+v: %d %d %v", tc.page, limit, offset, err)
		}
	}
	// Invalid pagination is rejected before attempting any database access.
	r := NewRepository(nil)
	if _, err := r.Artists(context.Background(), Page{101, 0}); !errors.Is(err, ErrInvalidPagination) {
		t.Fatal(err)
	}
	if _, err := r.Albums(context.Background(), Page{20, -1}); !errors.Is(err, ErrInvalidPagination) {
		t.Fatal(err)
	}
}

func TestRepositoryLive(t *testing.T) {
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
		t.Fatal("repository live test requires empty public schema; no reset performed")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db)
	artists, err := r.Artists(ctx, Page{})
	if err != nil || artists == nil || len(artists) != 0 {
		t.Fatalf("empty artists: %v %v", artists, err)
	}
	albums, err := r.Albums(ctx, Page{})
	if err != nil || albums == nil || len(albums) != 0 {
		t.Fatalf("empty albums: %v %v", albums, err)
	}
	const missing = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	if a, err := r.Artist(ctx, missing); !errors.Is(err, ErrNotFound) || a.ID != "" {
		t.Fatalf("missing artist: %+v %v", a, err)
	}
	if a, err := r.Album(ctx, missing); !errors.Is(err, ErrNotFound) || a.ID != "" {
		t.Fatalf("missing album: %+v %v", a, err)
	}
	// Test-only writes: reverse insertion order, repeated names/titles and distinct case.
	for _, q := range []string{
		`INSERT INTO artists (id,name,biography,created_at,updated_at)
   SELECT ('10000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,
   CASE WHEN i<3 THEN 'A' WHEN i<104 THEN 'B' WHEN i=104 THEN 'a' ELSE 'z' END,
   CASE WHEN i=1 THEN 'Known biography' WHEN i=2 THEN '' ELSE NULL END,
   '2024-01-02T03:04:05Z','2024-02-03T04:05:06Z'
   FROM generate_series(1,105) i ORDER BY i DESC`,
		`INSERT INTO albums (id,primary_artist_id,title,release_date,created_at,updated_at)
   SELECT ('20000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,
   ('10000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,
   CASE WHEN i<3 THEN 'A' WHEN i<104 THEN 'B' WHEN i=104 THEN 'a' ELSE 'z' END,
   CASE WHEN i=1 THEN DATE '2024-02-29' ELSE NULL END,
   '2024-01-02T03:04:05Z','2024-02-03T04:05:06Z'
   FROM generate_series(1,105) i ORDER BY i DESC`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	artistID := func(i int) string { return fmt.Sprintf("10000000-0000-4000-8000-%012d", i) }
	albumID := func(i int) string { return fmt.Sprintf("20000000-0000-4000-8000-%012d", i) }
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	a, err := r.Artist(ctx, artistID(1))
	if err != nil || a.Name != "A" || a.Biography == nil || *a.Biography != "Known biography" || !a.CreatedAt.Equal(created) || !a.UpdatedAt.Equal(updated) {
		t.Fatalf("artist fields: %+v %v", a, err)
	}
	b, err := r.Album(ctx, albumID(1))
	if err != nil || b.Title != "A" || b.PrimaryArtistID != artistID(1) || b.ReleaseDate == nil || b.ReleaseDate.Format("2006-01-02") != "2024-02-29" ||
		!b.CreatedAt.Equal(created) || !b.UpdatedAt.Equal(updated) {
		t.Fatalf("album fields: %+v %v", b, err)
	}
	a, err = r.Artist(ctx, artistID(2))
	if err != nil || a.Biography == nil || *a.Biography != "" {
		t.Fatal("empty biography lost")
	}
	a, err = r.Artist(ctx, artistID(3))
	if err != nil || a.Biography != nil {
		t.Fatal("NULL biography lost")
	}
	b, err = r.Album(ctx, albumID(2))
	if err != nil || b.ReleaseDate != nil {
		t.Fatal("NULL release date lost")
	}
	for _, tc := range []struct {
		p Page
		n int
	}{
		{Page{}, 20}, {Page{1, 0}, 1}, {Page{2, 1}, 2}, {Page{100, 0}, 100},
		{Page{100, 100}, 5}, {Page{20, 104}, 1}, {Page{20, 105}, 0}, {Page{20, 1 << 30}, 0},
	} {
		t.Run(fmt.Sprintf("page_%d_%d", tc.p.Limit, tc.p.Offset), func(t *testing.T) {
			aa, err := r.Artists(ctx, tc.p)
			if err != nil || aa == nil || len(aa) != tc.n {
				t.Fatalf("artists len=%d err=%v", len(aa), err)
			}
			bb, err := r.Albums(ctx, tc.p)
			if err != nil || bb == nil || len(bb) != tc.n {
				t.Fatalf("albums len=%d err=%v", len(bb), err)
			}
			for i := range aa {
				if aa[i].ID != artistID(tc.p.Offset+i+1) || bb[i].ID != albumID(tc.p.Offset+i+1) {
					t.Fatal("unstable name/title/UUID ordering")
				}
			}
			again, err := r.Artists(ctx, tc.p)
			if err != nil || !reflect.DeepEqual(aa, again) {
				t.Fatal("artist page changed")
			}
			againAlbums, err := r.Albums(ctx, tc.p)
			if err != nil || !reflect.DeepEqual(bb, againAlbums) {
				t.Fatal("album page changed")
			}
		})
	}
	for _, bad := range []string{"not-a-uuid", "' OR true --"} {
		if _, err := r.Artist(ctx, bad); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatal("bad artist ID confused with not-found")
		}
		if _, err := r.Album(ctx, bad); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatal("bad album ID confused with not-found")
		}
	}
	operations := []struct {
		name string
		call func(context.Context) error
	}{
		{"artist lookup", func(c context.Context) error { _, err := r.Artist(c, artistID(1)); return err }},
		{"artist list", func(c context.Context) error { _, err := r.Artists(c, Page{}); return err }},
		{"album lookup", func(c context.Context) error { _, err := r.Album(c, albumID(1)); return err }},
		{"album list", func(c context.Context) error { _, err := r.Albums(c, Page{}); return err }},
	}
	for _, op := range operations {
		c, stop := context.WithCancel(ctx)
		stop()
		if err := op.call(c); !errors.Is(err, context.Canceled) {
			t.Fatalf("%s cancellation: %v", op.name, err)
		}
	}
	// Force real queries to wait, then expire their contexts. No sleeps or timing races.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "LOCK TABLE artists,albums IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	for _, op := range operations {
		c, stop := context.WithTimeout(ctx, 100*time.Millisecond)
		err := op.call(c)
		stop()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("%s blocked deadline: %v", op.name, err)
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Artists(ctx, Page{}); err != nil {
		t.Fatalf("pool unusable after cancellation: %v", err)
	}
	// Server-side query errors must not turn into missing records or successful empty lists.
	if _, err := db.ExecContext(ctx, "ALTER TABLE artists RENAME TO artists_test_hidden"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE albums RENAME TO albums_test_hidden"); err != nil {
		t.Fatal(err)
	}
	for _, op := range operations {
		if err := op.call(ctx); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("%s swallowed SQL error: %v", op.name, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, op := range operations {
		if err := op.call(ctx); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("%s swallowed closed-pool error: %v", op.name, err)
		}
	}
	t.Log("PASS: empty/missing, all fields/NULLs, stable pagination, invalid IDs, cancellation/deadlines and SQL/pool errors")
}
