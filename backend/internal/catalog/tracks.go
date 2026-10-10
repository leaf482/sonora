package catalog

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Track contains catalog metadata only, never storage or private license evidence.
type Track struct {
	ID              string
	PrimaryArtistID string
	AlbumID         *string
	Title           string
	DisplayCredits  *string
	DurationMS      *int64
	DiscNumber      *int
	TrackNumber     *int
	State           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Nil filters are omitted; supplied filters are ANDed against the Track's own relationships.
type TrackFilter struct{ ArtistID, AlbumID *string }

func (r *Repository) Track(ctx context.Context, id string) (Track, error) {
	var t Track
	err := r.db.QueryRowContext(ctx, `SELECT id,primary_artist_id,album_id,title,display_credits,duration_ms,
 disc_number,track_number,state,created_at,updated_at FROM tracks WHERE id=$1`, id).
		Scan(&t.ID, &t.PrimaryArtistID, &t.AlbumID, &t.Title, &t.DisplayCredits, &t.DurationMS, &t.DiscNumber, &t.TrackNumber, &t.State, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Track{}, ErrNotFound
	}
	if err != nil {
		return Track{}, readError(ctx, "track lookup", err)
	}
	return t, nil
}

func (r *Repository) Tracks(ctx context.Context, page Page, filter TrackFilter) ([]Track, error) {
	limit, offset, err := page.bounds()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,primary_artist_id,album_id,title,display_credits,duration_ms,
 disc_number,track_number,state,created_at,updated_at FROM tracks
 WHERE ($1::uuid IS NULL OR primary_artist_id=$1) AND ($2::uuid IS NULL OR album_id=$2)
 ORDER BY title COLLATE "C",id LIMIT $3 OFFSET $4`, filter.ArtistID, filter.AlbumID, limit, offset)
	if err != nil {
		return nil, readError(ctx, "track list", err)
	}
	defer rows.Close()
	result := make([]Track, 0)
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.ID, &t.PrimaryArtistID, &t.AlbumID, &t.Title, &t.DisplayCredits, &t.DurationMS, &t.DiscNumber, &t.TrackNumber, &t.State, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, readError(ctx, "track scan", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, readError(ctx, "track iteration", err)
	}
	return result, nil
}
