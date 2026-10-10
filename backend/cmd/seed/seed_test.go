package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leaf482/sonora/backend/internal/postgres"
	"github.com/pressly/goose/v3"
)

func TestSeedLive(t *testing.T) {
	raw, set := os.LookupEnv("SONORA_TEST_DATABASE_URL")
	if !set {
		t.Skip("set SONORA_TEST_DATABASE_URL to an empty disposable database")
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
	var n int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&n); err != nil || n != 0 {
		t.Fatal("seed live test requires empty public schema; no reset performed")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	assets, err := loadFixtures(fixtureDir())
	if err != nil {
		t.Fatal(err)
	}
	seedWorkspace := t.TempDir()
	copyFixtures(t, filepath.Join(seedWorkspace, "fixtures", "audio"))
	base := filepath.Join(seedWorkspace, "backend")
	if err := os.Mkdir(base, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", raw)
	t.Setenv("DB_CONNECT_TIMEOUT", "5s")
	command := func(want string) {
		t.Helper()
		original, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(base)
		var output bytes.Buffer
		err = run(ctx, []string{"development"}, &output)
		t.Chdir(original)
		if err != nil || !strings.Contains(output.String(), want) {
			t.Fatalf("seed command output=%q err=%v", output.String(), err)
		}
		t.Log(strings.TrimSpace(output.String()))
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	const unrelated = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	exec("INSERT INTO artists (id,name) VALUES ($1,'yesme')", unrelated)
	// A late collision must roll back the earlier inserts, leaving unrelated data alone.
	exec("INSERT INTO tracks (id,primary_artist_id,title) VALUES ($1,$2,'Unrelated collision')", assets[2].Track["id"], unrelated)
	if _, err := seed(ctx, db, assets, base); err == nil {
		t.Fatal("expected identity conflict")
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM audio_assets").Scan(&n); err != nil || n != 0 {
		t.Fatal("partial assets committed")
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM artists").Scan(&n); err != nil || n != 1 {
		t.Fatal("partial artists committed")
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM tracks").Scan(&n); err != nil || n != 1 {
		t.Fatal("partial tracks committed")
	}
	exec("DELETE FROM tracks WHERE id=$1", assets[2].Track["id"])
	command("3 new assets, 0 preserved")
	for table, want := range map[string]int{"artists": 3, "albums": 0, "tracks": 3, "audio_assets": 3} {
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != want {
			t.Fatalf("%s count=%d err=%v", table, n, err)
		}
	}
	root, err := os.OpenRoot(filepath.Join(base, ".local", "audio"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, a := range assets {
		if _, err := verifyFile(root, fileRecord{Path: a.Key, Bytes: int64(len(a.Data)), SHA256: a.Asset["sha256"].(string)}); err != nil {
			t.Fatal(err)
		}
		var title, credits, attribution, evidence, hash, license, state, fileState, decision string
		var duration, byteLength int64
		var album sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT t.title,t.display_credits,t.album_id,t.duration_ms,t.state,a.file_state,a.license_decision,
   a.attribution,a.evidence_reference,a.sha256,a.license_name,a.byte_length FROM tracks t JOIN audio_assets a ON a.track_id=t.id WHERE t.id=$1`,
			a.Track["id"]).Scan(&title, &credits, &album, &duration, &state, &fileState, &decision, &attribution, &evidence, &hash, &license, &byteLength); err != nil {
			t.Fatal(err)
		}
		if title != a.Track["title"] || credits != a.Asset["attribution"] || attribution != a.Asset["attribution"] || album.Valid || duration != a.Track["duration_ms"] ||
			state != "draft" || fileState != "staged" || decision != "verified" || evidence != a.Asset["evidence_reference"] ||
			hash != a.Asset["sha256"] || license != a.Asset["license_name"] || byteLength != int64(len(a.Data)) {
			t.Fatal("catalog/provenance mismatch")
		}
	}
	before := snapshot(t, ctx, db)
	command("0 new assets, 3 preserved")
	if before != snapshot(t, ctx, db) {
		t.Fatal("repeat changed rows or timestamps")
	}
	// Preserve both rights withdrawal and ordinary file unavailability.
	for i := 0; i < 2; i++ {
		a := assets[i]
		exec("UPDATE tracks SET state='unavailable' WHERE id=$1", a.Track["id"])
		exec("UPDATE audio_assets SET file_state='unavailable',unavailable_at=CURRENT_TIMESTAMP,unavailable_reason='test missing file' WHERE id=$1", a.Asset["id"])
		if i == 0 {
			exec("UPDATE audio_assets SET license_decision='withdrawn',withdrawn_at=CURRENT_TIMESTAMP,withdrawal_reason='test withdrawal' WHERE id=$1", a.Asset["id"])
		}
		if err := root.Remove(a.Key); err != nil {
			t.Fatal(err)
		}
	}
	before = snapshot(t, ctx, db)
	if n, err := seed(ctx, db, assets, base); err != nil || n != 0 {
		t.Fatalf("disabled repeat count=%d err=%v", n, err)
	}
	if before != snapshot(t, ctx, db) {
		t.Fatal("disabled state/provenance overwritten")
	}
	for i := 0; i < 2; i++ {
		if _, err := root.Stat(assets[i].Key); !os.IsNotExist(err) {
			t.Fatal("disabled audio restored")
		}
	}
	// Never repair/overwrite corrupted existing storage automatically.
	if err := root.WriteFile(assets[2].Key, []byte("corrupted test copy"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := seed(ctx, db, assets, base); err == nil {
		t.Fatal("corrupted destination accepted")
	}
	if before != snapshot(t, ctx, db) {
		t.Fatal("failed destination verification changed DB")
	}
	// Execute the actual CLI path with copied missing/corrupted sources, never the original files.
	sandbox := t.TempDir()
	copyFixtures(t, filepath.Join(sandbox, "fixtures", "audio"))
	backend := filepath.Join(sandbox, "backend")
	if err := os.Mkdir(backend, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", raw)
	t.Chdir(backend)
	p := filepath.Join(sandbox, "fixtures", "audio", "disco-sting.mp3")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, []string{"development"}, &bytes.Buffer{}); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := os.WriteFile(p, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, []string{"development"}, &bytes.Buffer{}); err == nil {
		t.Fatal("corrupt source accepted")
	}
	if before != snapshot(t, ctx, db) {
		t.Fatal("source validation failure changed DB")
	}
	t.Log("PASS: migrations, seed/repeat, provenance/hashes, late-conflict rollback, unrelated data, disabled states and missing/corrupt files")
}
func snapshot(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	var s string
	if err := db.QueryRowContext(ctx, `SELECT jsonb_build_object(
 'artists',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM artists a),
 'albums',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM albums a),
 'tracks',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM tracks a),
 'assets',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM audio_assets a))::text`).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}
