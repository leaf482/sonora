package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// Called only by the empty-database live test, before rollback and after reapply.
// These synthetic rows are test data, never seed catalog.
func checkCatalogSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	exec := func(query string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO artists (id, name) VALUES
 ('10000000-0000-4000-8000-000000000001', 'Same name'),
 ('10000000-0000-4000-8000-000000000002', 'Same name')`)
	exec(`INSERT INTO albums (id, primary_artist_id, title) VALUES
 ('20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', 'Same title'),
 ('20000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000001', 'Same title')`)
	exec(`UPDATE artists SET biography = 'Synthetic biography', updated_at = CURRENT_TIMESTAMP
 WHERE id = '10000000-0000-4000-8000-000000000002'`)
	exec(`UPDATE albums SET release_date = '2024-02-29', updated_at = CURRENT_TIMESTAMP
 WHERE id = '20000000-0000-4000-8000-000000000002'`)
	var valid bool
	if err := db.QueryRowContext(ctx, `SELECT
 (SELECT biography IS NULL AND created_at IS NOT NULL AND updated_at IS NOT NULL FROM artists WHERE id = '10000000-0000-4000-8000-000000000001')
 AND (SELECT release_date IS NULL AND created_at IS NOT NULL AND updated_at IS NOT NULL FROM albums WHERE id = '20000000-0000-4000-8000-000000000001')
 AND (SELECT release_date = DATE '2024-02-29' FROM albums WHERE id = '20000000-0000-4000-8000-000000000002')`).Scan(&valid); err != nil || !valid {
		t.Fatalf("optional metadata or timestamp defaults failed: %v", err)
	}
	for _, tc := range []struct{ name, query, code string }{
		{"artist duplicate ID", "INSERT INTO artists (id, name) VALUES ('10000000-0000-4000-8000-000000000001', 'Other')", "23505"},
		{"album duplicate ID", "INSERT INTO albums (id, primary_artist_id, title) VALUES ('20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', 'Other')", "23505"},
		{"artist missing ID", "INSERT INTO artists (name) VALUES ('No ID')", "23502"},
		{"album missing ID", "INSERT INTO albums (primary_artist_id, title) VALUES ('10000000-0000-4000-8000-000000000001', 'No ID')", "23502"},
		{"missing artist", "UPDATE albums SET primary_artist_id = '10000000-0000-4000-8000-000000000099'", "23503"},
		{"delete referenced artist", "DELETE FROM artists WHERE id = '10000000-0000-4000-8000-000000000001'", "23503"},
		{"change referenced artist ID", "UPDATE artists SET id = '10000000-0000-4000-8000-000000000099' WHERE id = '10000000-0000-4000-8000-000000000001'", "23503"},
		{"empty artist name", "UPDATE artists SET name = ''", "23514"},
		{"spaces artist name", "UPDATE artists SET name = '   '", "23514"},
		{"empty album title", "UPDATE albums SET title = ''", "23514"},
		{"spaces album title", "UPDATE albums SET title = '   '", "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.ExecContext(ctx, tc.query)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("want SQLSTATE %s, got %v", tc.code, err)
			}
		})
	}
	for table, columns := range map[string][]string{
		"artists": {"id", "name", "created_at", "updated_at"},
		"albums":  {"id", "primary_artist_id", "title", "created_at", "updated_at"},
	} {
		for _, column := range columns {
			t.Run(table+" null "+column, func(t *testing.T) {
				// Identifiers are fixed test literals, never external input.
				_, err := db.ExecContext(ctx, "UPDATE "+table+" SET "+column+" = NULL")
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "23502" {
					t.Fatalf("want NOT NULL violation, got %v", err)
				}
			})
		}
	}
	// Albums can be corrected explicitly to another primary artist; there is no cascade.
	exec(`UPDATE albums SET primary_artist_id = '10000000-0000-4000-8000-000000000002'
 WHERE primary_artist_id = '10000000-0000-4000-8000-000000000001'`)
	exec(`DELETE FROM artists WHERE id = '10000000-0000-4000-8000-000000000001'`)
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM albums").Scan(&count); err != nil || count != 2 {
		t.Fatal("artist reassignment/deletion lost albums")
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND tablename IN ('artists', 'albums')`).Scan(&count); err != nil || count != 3 {
		t.Fatal("expected two primary key indexes and one artist lookup index")
	}
	t.Log("catalog inserts, nullable metadata, PK/NOT NULL/check/FK constraints and reassignment passed")
}
