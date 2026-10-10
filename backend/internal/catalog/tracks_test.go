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

func TestTrackPagination(t *testing.T) {
	r := NewRepository(nil)
	for _, p := range []Page{{-1, 0}, {101, 0}, {20, -1}} {
		if rows, err := r.Tracks(context.Background(), p, TrackFilter{}); !errors.Is(err, ErrInvalidPagination) || rows != nil {
			t.Fatalf("%+v: %v %v", p, rows, err)
		}
	}
}

func TestTrackRepositoryLive(t *testing.T) {
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
		t.Fatal("track live test requires empty public schema; no reset performed")
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
	r := NewRepository(db)
	id := func(i int) string { return fmt.Sprintf("30000000-0000-4000-8000-%012d", i) }
	artist := "10000000-0000-4000-8000-000000000001"
	otherArtist := "10000000-0000-4000-8000-000000000002"
	album := "20000000-0000-4000-8000-000000000001"
	absent := "ffffffff-ffff-4fff-8fff-ffffffffffff"
	if rows, err := r.Tracks(ctx, Page{}, TrackFilter{}); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("empty tracks", rows, err)
	}
	if row, err := r.Track(ctx, absent); !errors.Is(err, ErrNotFound) || row.ID != "" {
		t.Fatal("missing track", err)
	}
	for _, lookup := range []func(context.Context, string) (AudioAsset, error){r.AudioAssetByTrack, r.PlaybackAssetByTrack} {
		if row, err := lookup(ctx, absent); !errors.Is(err, ErrAudioAssetNotFound) || errors.Is(err, ErrNotFound) || row.ID != "" {
			t.Fatal("missing asset", err)
		}
	}
	exec("INSERT INTO artists(id,name) VALUES ($1,'First'),($2,'Second')", artist, otherArtist)
	exec("INSERT INTO albums(id,primary_artist_id,title) VALUES ($1,$2,'Album')", album, otherArtist)
	// Reverse insertion order; same titles, uppercase/lowercase and UUID tie breakers.
	exec(`INSERT INTO tracks(id,primary_artist_id,album_id,title,display_credits,duration_ms,disc_number,track_number,state,created_at,updated_at)
 SELECT ('30000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,
 CASE WHEN i%2=1 THEN $1::uuid ELSE $2::uuid END,
 CASE WHEN i<=4 THEN $3::uuid ELSE NULL END,
 CASE WHEN i<104 THEN 'A' ELSE 'a' END,
 CASE WHEN i=1 THEN 'Credit' WHEN i=2 THEN '' ELSE NULL END,
 CASE WHEN i<=2 THEN 1234 ELSE NULL END,
 CASE WHEN i<=4 THEN 1 ELSE NULL END,CASE WHEN i<=4 THEN i ELSE NULL END,
 CASE WHEN i=1 THEN 'active' ELSE 'draft' END,'2024-01-02T03:04:05Z','2024-02-03T04:05:06Z'
 FROM generate_series(1,105) i ORDER BY i DESC`, artist, otherArtist, album)
	for _, tc := range []struct {
		p      Page
		filter TrackFilter
		want   []int
	}{
		{Page{2, 0}, TrackFilter{}, []int{1, 2}},
		{Page{2, 1}, TrackFilter{}, []int{2, 3}},
		{Page{100, 100}, TrackFilter{}, []int{101, 102, 103, 104, 105}},
		{Page{20, 105}, TrackFilter{}, []int{}},
		{Page{20, 1 << 30}, TrackFilter{}, []int{}},
		{Page{3, 1}, TrackFilter{ArtistID: &artist}, []int{3, 5, 7}},
		{Page{}, TrackFilter{AlbumID: &album}, []int{1, 2, 3, 4}},
		{Page{}, TrackFilter{ArtistID: &artist, AlbumID: &album}, []int{1, 3}},
		{Page{}, TrackFilter{ArtistID: &otherArtist, AlbumID: &album}, []int{2, 4}},
		{Page{}, TrackFilter{ArtistID: &absent}, []int{}},
		{Page{}, TrackFilter{AlbumID: &absent}, []int{}},
	} {
		rows, err := r.Tracks(ctx, tc.p, tc.filter)
		if err != nil || rows == nil || len(rows) != len(tc.want) {
			t.Fatalf("page %+v %+v: %d %v", tc.p, tc.filter, len(rows), err)
		}
		for i, row := range rows {
			if row.ID != id(tc.want[i]) {
				t.Fatal("filter/order", row.ID, tc.want)
			}
		}
	}
	for _, tc := range []struct {
		p Page
		n int
	}{{Page{}, 20}, {Page{100, 0}, 100}} {
		rows, err := r.Tracks(ctx, tc.p, TrackFilter{})
		if err != nil || len(rows) != tc.n {
			t.Fatal("limit", len(rows), err)
		}
		for i, row := range rows {
			if row.ID != id(i+1) {
				t.Fatal("stable order")
			}
		}
	}
	row, err := r.Track(ctx, id(1))
	if err != nil {
		t.Fatal(err)
	}
	if row.PrimaryArtistID != artist || row.AlbumID == nil || *row.AlbumID != album || row.Title != "A" || row.DisplayCredits == nil || *row.DisplayCredits != "Credit" || row.DurationMS == nil || *row.DurationMS != 1234 || row.DiscNumber == nil || *row.DiscNumber != 1 || row.TrackNumber == nil || *row.TrackNumber != 1 || row.State != "active" || row.CreatedAt.UTC().Format(time.RFC3339) != "2024-01-02T03:04:05Z" || row.UpdatedAt.UTC().Format(time.RFC3339) != "2024-02-03T04:05:06Z" {
		t.Fatalf("metadata %+v", row)
	}
	rows, err := r.Tracks(ctx, Page{1, 0}, TrackFilter{})
	if err != nil || !reflect.DeepEqual(row, rows[0]) {
		t.Fatal("list and lookup mismatch")
	}
	row, err = r.Track(ctx, id(2))
	if err != nil || row.DisplayCredits == nil || *row.DisplayCredits != "" {
		t.Fatal("empty credits lost")
	}
	row, err = r.Track(ctx, id(5))
	if err != nil || row.AlbumID != nil || row.DisplayCredits != nil || row.DurationMS != nil || row.DiscNumber != nil || row.TrackNumber != nil || row.State != "draft" {
		t.Fatal("NULL standalone metadata lost")
	}
	if _, err := r.AudioAssetByTrack(ctx, id(1)); !errors.Is(err, ErrAudioAssetNotFound) {
		t.Fatal("existing track without asset", err)
	}
	exec(`INSERT INTO audio_assets(id,track_id,storage_key,sha256,mime_type,codec,byte_length,duration_ms,acquired_at)
 VALUES ('40000000-0000-4000-8000-000000000001',$1,'synthetic.mp3',repeat('a',64),'audio/mpeg','mp3',42,1234,'2024-01-02T03:04:05Z')`, id(1))
	a, err := r.AudioAssetByTrack(ctx, id(1))
	if err != nil {
		t.Fatal(err)
	}
	if a.StorageKey != "synthetic.mp3" || a.ByteLength != 42 || a.DurationMS != 1234 || a.HostingPermitted != nil || a.LicenseExpiresAt != nil || a.EvidenceReference != nil || a.ReviewedAt != nil || a.WithdrawnAt != nil || a.LicenseDecision != "pending" || a.FileState != "staged" {
		t.Fatal("asset metadata/NULLs", a)
	}
	unavailable := func(want error) {
		t.Helper()
		a, err := r.PlaybackAssetByTrack(ctx, id(1))
		if !errors.Is(err, want) || !reflect.DeepEqual(a, AudioAsset{}) {
			t.Fatalf("eligibility: %+v %v; want %v", a, err, want)
		}
	}
	unavailable(ErrAudioAssetUnavailable)
	// Synthetic evidence is solely for testing constraints, never a real permission claim.
	exec(`UPDATE audio_assets SET file_state='ready',license_decision='verified',
 source_provider='test',source_url='https://example.invalid/source',download_reference='synthetic',
 creator_credits='test creator',rights_holder_credits='test owner',license_name='test grant',license_version='test version',
 license_url='https://example.invalid/license',evidence_url='https://example.invalid/evidence',evidence_reference='private/test',
 reviewed_by='test reviewer',reviewed_at='2024-01-02T03:04:05Z',review_rationale='synthetic only',
 hosting_permitted=true,playback_permitted=true,redistribution_permitted=false,commercial_use_permitted=false,
 audience_scope='local test',territory_scope='test territory',license_term='no fixed expiry in synthetic test',
 use_conditions='not a real grant',attribution='test attribution',modifications='none' WHERE track_id=$1`, id(1))
	a, err = r.PlaybackAssetByTrack(ctx, id(1))
	if err != nil {
		t.Fatal(err)
	}
	rawAsset, err := r.AudioAssetByTrack(ctx, id(1))
	if err != nil || !reflect.DeepEqual(a, rawAsset) {
		t.Fatal("asset lookup consistency")
	}
	if a.TrackID != id(1) || a.MIMEType != "audio/mpeg" || a.Codec != "mp3" || len(a.SHA256) != 64 || a.Attribution == nil || *a.Attribution != "test attribution" || a.EvidenceReference == nil || *a.EvidenceReference != "private/test" || a.AudienceScope == nil || *a.AudienceScope != "local test" || a.RedistributionPermitted == nil || *a.RedistributionPermitted || a.CommercialUsePermitted == nil || *a.CommercialUsePermitted {
		t.Fatal("scope/false permission lost")
	}
	for _, state := range []string{"draft", "unavailable"} {
		exec("UPDATE tracks SET state=$1 WHERE id=$2", state, id(1))
		unavailable(ErrAudioAssetUnavailable)
	}
	exec("UPDATE tracks SET state='active',duration_ms=1235 WHERE id=$1", id(1))
	unavailable(ErrAudioAssetUnavailable)
	exec("UPDATE tracks SET duration_ms=1234 WHERE id=$1", id(1))
	exec("UPDATE audio_assets SET license_expires_at=statement_timestamp() WHERE track_id=$1", id(1))
	unavailable(ErrAudioAssetUnavailable)
	exec("UPDATE audio_assets SET license_expires_at=statement_timestamp()+interval '1 hour' WHERE track_id=$1", id(1))
	if _, err := r.PlaybackAssetByTrack(ctx, id(1)); err != nil {
		t.Fatal("future expiry", err)
	}
	exec("UPDATE audio_assets SET license_expires_at=NULL,file_state='staged',playback_permitted=false WHERE track_id=$1", id(1))
	unavailable(ErrAudioAssetUnavailable)
	exec("UPDATE audio_assets SET file_state='unavailable',unavailable_at=now(),unavailable_reason='missing file',playback_permitted=true WHERE track_id=$1", id(1))
	unavailable(ErrAudioAssetUnavailable)
	exec("UPDATE audio_assets SET license_decision='rejected' WHERE track_id=$1", id(1))
	unavailable(ErrAudioAssetUnavailable)
	exec("UPDATE audio_assets SET license_decision='withdrawn',withdrawn_at=now(),withdrawal_reason='test withdrawal' WHERE track_id=$1", id(1))
	unavailable(ErrAudioAssetWithdrawn)
	unavailable(ErrAudioAssetUnavailable)
	a, err = r.AudioAssetByTrack(ctx, id(1))
	if err != nil || a.LicenseDecision != "withdrawn" || a.WithdrawnAt == nil || a.UnavailableAt == nil || a.WithdrawalReason == nil || *a.WithdrawalReason != "test withdrawal" {
		t.Fatal("withdrawal metadata missing", err)
	}
	if _, err := r.Track(ctx, id(1)); err != nil {
		t.Fatal("withdrawal deleted metadata")
	}
	for _, bad := range []string{"", "not-a-uuid", "' OR true --"} {
		if _, err := r.Track(ctx, bad); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatal("invalid track ID", err)
		}
		for _, f := range []TrackFilter{{ArtistID: &bad}, {AlbumID: &bad}} {
			if _, err := r.Tracks(ctx, Page{}, f); err == nil {
				t.Fatal("invalid filter")
			}
		}
		if _, err := r.AudioAssetByTrack(ctx, bad); err == nil || errors.Is(err, ErrAudioAssetNotFound) {
			t.Fatal("invalid asset ID", err)
		}
	}
	operations := []func(context.Context) error{
		func(c context.Context) error { _, e := r.Track(c, id(1)); return e },
		func(c context.Context) error { _, e := r.Tracks(c, Page{}, TrackFilter{ArtistID: &artist}); return e },
		func(c context.Context) error { _, e := r.AudioAssetByTrack(c, id(1)); return e },
		func(c context.Context) error { _, e := r.PlaybackAssetByTrack(c, id(1)); return e },
	}
	for _, op := range operations {
		c, stop := context.WithCancel(ctx)
		stop()
		if err := op(c); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation", err)
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "LOCK TABLE tracks,audio_assets IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	for _, op := range operations {
		c, stop := context.WithTimeout(ctx, 100*time.Millisecond)
		err := op(c)
		stop()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("deadline", err)
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Track(ctx, id(1)); err != nil {
		t.Fatal("pool recovery", err)
	}
	exec("ALTER TABLE tracks RENAME TO tracks_test_hidden")
	for _, op := range operations {
		if err := op(ctx); err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrAudioAssetNotFound) || errors.Is(err, ErrAudioAssetUnavailable) {
			t.Fatal("SQL error misclassified", err)
		}
	}
	db.Close()
	for _, op := range operations {
		if err := op(ctx); err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrAudioAssetNotFound) {
			t.Fatal("closed pool error", err)
		}
	}
	t.Log("PASS: track filters/pagination/order/NULLs; asset missing/staged/ready/expired/rejected/withdrawn; context and SQL/pool errors; no files accessed")
}
