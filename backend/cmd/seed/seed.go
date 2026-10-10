package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Explicit SQL for this one recipe. No update/delete paths: existing states and
// operator changes are never overwritten. Mismatched identities abort the transaction.
func seed(ctx context.Context, db *sql.DB, assets []seedAsset, workingDir string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, errors.New("seed transaction failed")
	}
	defer tx.Rollback()
	fail := func() (int, error) {
		return 0, errors.New("seed database conflict/failure; verify migrations and existing fixture metadata; no rows committed")
	}
	// Serialize this recipe's concurrent runs; PostgreSQL releases the lock at transaction end.
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(736667001)"); err != nil {
		return fail()
	}
	base, err := os.OpenRoot(workingDir)
	if err != nil {
		return 0, errors.New("cannot open backend storage base")
	}
	defer base.Close()
	if err := base.MkdirAll(".local/audio", 0700); err != nil {
		return 0, errors.New("cannot create private development audio directory")
	}
	storage, err := base.OpenRoot(".local/audio")
	if err != nil {
		return 0, errors.New("cannot open private development audio directory")
	}
	defer storage.Close()
	inserted := 0
	for _, a := range assets {
		artistJSON, _ := json.Marshal(a.Artist)
		if _, err := tx.ExecContext(ctx, `INSERT INTO artists (id,name)
   SELECT id,name FROM jsonb_populate_record(NULL::artists,$1::jsonb)
   ON CONFLICT (id) DO NOTHING`, string(artistJSON)); err != nil {
			return fail()
		}
		var matches bool
		if err := tx.QueryRowContext(ctx, `SELECT to_jsonb(a) @> $2::jsonb FROM artists a WHERE id=$1 FOR UPDATE`,
			a.Artist["id"], string(artistJSON)).Scan(&matches); err != nil || !matches {
			return fail()
		}
		trackJSON, _ := json.Marshal(a.Track)
		if _, err := tx.ExecContext(ctx, `INSERT INTO tracks
   (id,primary_artist_id,album_id,title,display_credits,duration_ms,disc_number,track_number)
   SELECT id,primary_artist_id,album_id,title,display_credits,duration_ms,disc_number,track_number
   FROM jsonb_populate_record(NULL::tracks,$1::jsonb) ON CONFLICT (id) DO NOTHING`, string(trackJSON)); err != nil {
			return fail()
		}
		var trackState string
		if err := tx.QueryRowContext(ctx, `SELECT to_jsonb(t) @> $2::jsonb,state FROM tracks t WHERE id=$1 FOR UPDATE`,
			a.Track["id"], string(trackJSON)).Scan(&matches, &trackState); err != nil || !matches {
			return fail()
		}
		assetJSON, _ := json.Marshal(a.Asset)
		var fileState, decision string
		err := tx.QueryRowContext(ctx, `SELECT
   (to_jsonb(a)-'acquired_at'-'reviewed_at') @> ($2::jsonb-'acquired_at'-'reviewed_at')
   AND acquired_at=($2::jsonb->>'acquired_at')::timestamptz
   AND reviewed_at=($2::jsonb->>'reviewed_at')::timestamptz,
   file_state,license_decision FROM audio_assets a WHERE id=$1 FOR UPDATE`,
			a.Asset["id"], string(assetJSON)).Scan(&matches, &fileState, &decision)
		if err == nil {
			if !matches {
				return fail()
			}
			// Never restore storage for unavailable/withdrawn/rejected/review-pending rows.
			if trackState != "unavailable" && fileState != "unavailable" && decision == "verified" {
				if _, err := verifyFile(storage, fileRecord{Path: a.Key, Bytes: int64(len(a.Data)), SHA256: a.Asset["sha256"].(string)}); err != nil {
					return 0, fmt.Errorf("existing asset storage invalid; seed will not repair it: %w", err)
				}
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) || trackState != "draft" {
			return fail()
		}
		if err := installFile(storage, a); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO audio_assets (
   id,track_id,storage_key,sha256,mime_type,codec,byte_length,duration_ms,acquired_at,
   source_provider,source_url,download_reference,creator_credits,rights_holder_credits,
   license_name,license_version,license_url,evidence_url,evidence_reference,reviewed_by,reviewed_at,review_rationale,
   hosting_permitted,playback_permitted,redistribution_permitted,commercial_use_permitted,
   audience_scope,territory_scope,license_term,license_expires_at,use_conditions,attribution,modifications,license_decision)
   SELECT id,track_id,storage_key,sha256,mime_type,codec,byte_length,duration_ms,acquired_at,
   source_provider,source_url,download_reference,creator_credits,rights_holder_credits,
   license_name,license_version,license_url,evidence_url,evidence_reference,reviewed_by,reviewed_at,review_rationale,
   hosting_permitted,playback_permitted,redistribution_permitted,commercial_use_permitted,
   audience_scope,territory_scope,license_term,license_expires_at,use_conditions,attribution,modifications,'verified'
   FROM jsonb_populate_record(NULL::audio_assets,$1::jsonb)`, string(assetJSON)); err != nil {
			return fail()
		}
		inserted++
	}
	if err := tx.Commit(); err != nil {
		return 0, errors.New("seed commit not confirmed; inspect database before retrying")
	}
	return inserted, nil
}

// Publish complete bytes without overwriting any existing storage entry. A prior
// rolled-back attempt may leave a verified orphan, which is safe to reuse.
func installFile(root *os.Root, a seedAsset) error {
	if _, err := root.Lstat(a.Key); err == nil {
		_, err = verifyFile(root, fileRecord{Path: a.Key, Bytes: int64(len(a.Data)), SHA256: a.Asset["sha256"].(string)})
		return err
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("cannot inspect destination storage")
	}
	temp := ".seed-" + rand.Text()
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("cannot create temporary audio copy")
	}
	defer root.Remove(temp)
	_, writeErr := f.Write(a.Data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return errors.New("cannot write complete audio copy")
	}
	if err := root.Link(temp, a.Key); err != nil {
		if errors.Is(err, os.ErrExist) {
			_, err = verifyFile(root, fileRecord{Path: a.Key, Bytes: int64(len(a.Data)), SHA256: a.Asset["sha256"].(string)})
			return err
		}
		return errors.New("cannot publish audio copy without overwriting; local filesystem must support hard links")
	}
	return nil
}
