package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// All metadata is synthetic; no real file or license permission is asserted.
func checkTrackAssetSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	exec := func(q string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	// Artist 2 and album 1 were created by checkCatalogSchema; artist 3 is distinct.
	exec(`INSERT INTO tracks (id, primary_artist_id, title) VALUES
 ('30000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000003', 'Repeated title'),
 ('30000000-0000-4000-8000-000000000003', '10000000-0000-4000-8000-000000000003', 'Repeated title')`)
	exec(`INSERT INTO tracks (id, primary_artist_id, album_id, title, disc_number, track_number, duration_ms) VALUES
 ('30000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000003',
 '20000000-0000-4000-8000-000000000001', 'Repeated title', 1, 1, 15000)`)
	exec(`INSERT INTO audio_assets
 (id, track_id, storage_key, sha256, mime_type, codec, byte_length, duration_ms, acquired_at) VALUES
 ('40000000-0000-4000-8000-000000000001', '30000000-0000-4000-8000-000000000001', 'test-a.mp3', repeat('a',64), 'audio/mpeg', 'mp3', 1024, 15000, CURRENT_TIMESTAMP),
 ('40000000-0000-4000-8000-000000000002', '30000000-0000-4000-8000-000000000002', 'test-b.mp3', repeat('a',64), 'audio/mpeg', 'mp3', 1024, 15000, CURRENT_TIMESTAMP)`)
	var valid bool
	if err := db.QueryRowContext(ctx, `SELECT
 (SELECT album_id IS NULL AND disc_number IS NULL AND track_number IS NULL AND display_credits IS NULL AND duration_ms IS NULL AND state = 'draft' FROM tracks WHERE id = '30000000-0000-4000-8000-000000000001')
 AND (SELECT file_state = 'staged' AND license_decision = 'pending' AND hosting_permitted IS NULL AND redistribution_permitted IS NULL AND evidence_reference IS NULL FROM audio_assets WHERE storage_key = 'test-a.mp3')
 AND (SELECT t.primary_artist_id <> a.primary_artist_id FROM tracks t JOIN albums a ON t.album_id = a.id)`).Scan(&valid); err != nil || !valid {
		t.Fatalf("single, differing artist or pending unknown defaults failed: %v", err)
	}
	bad := func(name, query, state string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			_, err := db.ExecContext(ctx, query)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != state {
				t.Fatalf("want SQLSTATE %s, got %v", state, err)
			}
		})
	}
	const one = " WHERE id = '30000000-0000-4000-8000-000000000001'"
	const albumTrack = " WHERE id = '30000000-0000-4000-8000-000000000002'"
	const asset = " WHERE storage_key = 'test-a.mp3'"
	for _, tc := range []struct{ name, q, code string }{
		{"track duplicate id", "UPDATE tracks SET id = '30000000-0000-4000-8000-000000000001' WHERE id = '30000000-0000-4000-8000-000000000003'", "23505"},
		{"missing artist", "UPDATE tracks SET primary_artist_id = '10000000-0000-4000-8000-000000000099'" + one, "23503"},
		{"missing album", "UPDATE tracks SET album_id = '20000000-0000-4000-8000-000000000099'" + albumTrack, "23503"},
		{"delete album", "DELETE FROM albums WHERE id = '20000000-0000-4000-8000-000000000001'", "23503"},
		{"delete track artist", "DELETE FROM artists WHERE id = '10000000-0000-4000-8000-000000000003'", "23503"},
		{"duplicate position", "UPDATE tracks SET album_id = '20000000-0000-4000-8000-000000000001', disc_number = 1, track_number = 1" + one, "23505"},
		{"single with position", "UPDATE tracks SET track_number = 1" + one, "23514"},
		{"album missing disc", "UPDATE tracks SET disc_number = NULL" + albumTrack, "23514"},
		{"album missing position", "UPDATE tracks SET track_number = NULL" + albumTrack, "23514"},
		{"zero disc", "UPDATE tracks SET disc_number = 0" + albumTrack, "23514"},
		{"negative position", "UPDATE tracks SET track_number = -1" + albumTrack, "23514"},
		{"blank title", "UPDATE tracks SET title = '  '" + one, "23514"},
		{"bad track state", "UPDATE tracks SET state = 'deleted'" + one, "23514"},
		{"unknown active duration", "UPDATE tracks SET state = 'active'" + one, "23514"},
		{"zero duration", "UPDATE tracks SET duration_ms = 0" + one, "23514"},
		{"asset duplicate id", "UPDATE audio_assets SET id = '40000000-0000-4000-8000-000000000002'" + asset, "23505"},
		{"duplicate track asset", "UPDATE audio_assets SET track_id = '30000000-0000-4000-8000-000000000002'" + asset, "23505"},
		{"duplicate storage", "UPDATE audio_assets SET storage_key = 'test-b.mp3'" + asset, "23505"},
		{"missing asset track", "UPDATE audio_assets SET track_id = '30000000-0000-4000-8000-000000000099'" + asset, "23503"},
		{"delete asset track", "DELETE FROM tracks" + one, "23503"},
		{"path traversal", "UPDATE audio_assets SET storage_key = '../test.mp3'" + asset, "23514"},
		{"absolute storage", "UPDATE audio_assets SET storage_key = '/test.mp3'" + asset, "23514"},
		{"bad checksum", "UPDATE audio_assets SET sha256 = 'not-a-sha256'" + asset, "23514"},
		{"bad mime", "UPDATE audio_assets SET mime_type = 'image/png'" + asset, "23514"},
		{"empty codec", "UPDATE audio_assets SET codec = ''" + asset, "23514"},
		{"zero bytes", "UPDATE audio_assets SET byte_length = 0" + asset, "23514"},
		{"negative asset duration", "UPDATE audio_assets SET duration_ms = -1" + asset, "23514"},
		{"bad file state", "UPDATE audio_assets SET file_state = 'deleted'" + asset, "23514"},
		{"bad license decision", "UPDATE audio_assets SET license_decision = 'approved'" + asset, "23514"},
		{"unverified ready", "UPDATE audio_assets SET file_state = 'ready'" + asset, "23514"},
		{"verified without evidence", "UPDATE audio_assets SET license_decision = 'verified'" + asset, "23514"},
		{"unavailable without reason", "UPDATE audio_assets SET file_state = 'unavailable'" + asset, "23514"},
	} {
		bad(tc.name, tc.q, tc.code)
	}
	for table, columns := range map[string][]string{
		"tracks":       {"id", "primary_artist_id", "title", "state", "created_at", "updated_at"},
		"audio_assets": {"id", "track_id", "storage_key", "sha256", "mime_type", "codec", "byte_length", "duration_ms", "file_state", "license_decision", "acquired_at", "created_at", "updated_at"},
	} {
		for _, column := range columns {
			bad(table+" null "+column, "UPDATE "+table+" SET "+column+" = NULL", "23502")
		}
	}
	// Explicitly recorded synthetic scope: redistribution/commercial use are denied.
	exec(`UPDATE audio_assets SET source_provider = 'Synthetic test', source_url = 'https://example.invalid/asset',
 download_reference = 'https://example.invalid/test.mp3', creator_credits = 'Test creator', rights_holder_credits = 'Test owner',
 license_name = 'Synthetic permission', license_version = '1', license_url = 'https://example.invalid/license',
 evidence_url = 'https://example.invalid/evidence', evidence_reference = 'synthetic-evidence.json',
 reviewed_by = 'test', reviewed_at = CURRENT_TIMESTAMP, review_rationale = 'Synthetic constraint test only',
 hosting_permitted = true, playback_permitted = true, redistribution_permitted = false, commercial_use_permitted = false,
 audience_scope = 'local tests only', territory_scope = 'test environment', license_term = 'test only, no fixed expiry',
 use_conditions = 'Synthetic metadata; not a rights grant', attribution = 'Test creator; synthetic permission link above',
 modifications = 'None', license_decision = 'verified', file_state = 'ready'
 WHERE storage_key = 'test-a.mp3'`)
	for _, col := range []string{"source_provider", "source_url", "download_reference", "creator_credits", "rights_holder_credits", "license_name", "license_version", "license_url", "evidence_url", "evidence_reference", "reviewed_by", "review_rationale", "audience_scope", "territory_scope", "license_term", "use_conditions", "attribution", "modifications"} {
		bad("verified missing "+col, "UPDATE audio_assets SET "+col+" = NULL"+asset, "23514")
		bad("verified blank "+col, "UPDATE audio_assets SET "+col+" = '  '"+asset, "23514")
	}
	for _, col := range []string{"reviewed_at", "hosting_permitted", "playback_permitted", "redistribution_permitted", "commercial_use_permitted"} {
		bad("verified unknown "+col, "UPDATE audio_assets SET "+col+" = NULL"+asset, "23514")
	}
	bad("ready hosting denied", "UPDATE audio_assets SET hosting_permitted = false"+asset, "23514")
	bad("ready playback denied", "UPDATE audio_assets SET playback_permitted = false"+asset, "23514")
	bad("withdrawal without details", "UPDATE audio_assets SET license_decision = 'withdrawn'"+asset, "23514")
	// A missing file does not revoke known rights, or remove the storage/provenance record.
	exec(`UPDATE audio_assets SET file_state = 'unavailable', unavailable_at = CURRENT_TIMESTAMP,
 unavailable_reason = 'Synthetic missing file' WHERE storage_key = 'test-a.mp3'`)
	exec(`UPDATE audio_assets SET file_state = 'ready', unavailable_at = NULL, unavailable_reason = NULL
 WHERE storage_key = 'test-a.mp3'`)
	exec(`UPDATE tracks SET duration_ms = 15000, state = 'active' WHERE id = '30000000-0000-4000-8000-000000000001'`)
	// Future services must perform both updates atomically; no trigger is introduced.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE audio_assets SET license_decision = 'withdrawn', file_state = 'unavailable',
 unavailable_at = CURRENT_TIMESTAMP, unavailable_reason = 'Permission withdrawn',
 withdrawn_at = CURRENT_TIMESTAMP, withdrawal_reason = 'Synthetic withdrawal', updated_at = CURRENT_TIMESTAMP
 WHERE storage_key = 'test-a.mp3'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE tracks SET state = 'unavailable', updated_at = CURRENT_TIMESTAMP"+one); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT a.license_decision = 'withdrawn' AND a.file_state = 'unavailable'
 AND a.evidence_reference = 'synthetic-evidence.json' AND a.sha256 = repeat('a',64)
 AND a.redistribution_permitted = false AND a.withdrawn_at IS NOT NULL AND t.state = 'unavailable'
 FROM audio_assets a JOIN tracks t ON t.id = a.track_id WHERE a.storage_key = 'test-a.mp3'`).Scan(&valid); err != nil || !valid {
		t.Fatalf("withdrawal lost metadata/provenance: %v", err)
	}
	bad("withdrawn ready", "UPDATE audio_assets SET file_state = 'ready', unavailable_at = NULL, unavailable_reason = NULL"+asset, "23514")
	exec(`UPDATE audio_assets SET license_decision = 'rejected', file_state = 'unavailable',
 unavailable_at = CURRENT_TIMESTAMP, unavailable_reason = 'No applicable permission',
 reviewed_by = 'test', reviewed_at = CURRENT_TIMESTAMP, review_rationale = 'Unknown permission not accepted'
 WHERE storage_key = 'test-b.mp3'`)
	t.Log("track/asset constraints and atomic withdrawal with retained provenance passed")
}
