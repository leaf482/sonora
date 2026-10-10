// Package catalog provides internal catalog reads, not public visibility filtering.
package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound          = errors.New("catalog record not found")
	ErrInvalidPagination = errors.New("limit must be 0..100 and offset must be nonnegative")
)

type Artist struct {
	ID        string
	Name      string
	Biography *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Album struct {
	ID              string
	PrimaryArtistID string
	Title           string
	ReleaseDate     *time.Time // PostgreSQL date; use the calendar date, not a local-time conversion.
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Page uses zero Limit for the default (20); results are capped at 100.
type Page struct{ Limit, Offset int }

func (p Page) bounds() (int, int, error) {
	if p.Limit < 0 || p.Limit > 100 || p.Offset < 0 {
		return 0, 0, ErrInvalidPagination
	}
	if p.Limit == 0 {
		p.Limit = 20
	}
	return p.Limit, p.Offset, nil
}

// Repository borrows the existing pool. The caller owns its lifecycle.
type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Artist(ctx context.Context, id string) (Artist, error) {
	var a Artist
	err := r.db.QueryRowContext(ctx, `SELECT id,name,biography,created_at,updated_at FROM artists WHERE id=$1`, id).
		Scan(&a.ID, &a.Name, &a.Biography, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Artist{}, ErrNotFound
	}
	if err != nil {
		return Artist{}, readError(ctx, "artist lookup", err)
	}
	return a, nil
}

func (r *Repository) Artists(ctx context.Context, page Page) ([]Artist, error) {
	limit, offset, err := page.bounds()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,biography,created_at,updated_at FROM artists
 ORDER BY name COLLATE "C",id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, readError(ctx, "artist list", err)
	}
	defer rows.Close()
	result := make([]Artist, 0)
	for rows.Next() {
		var a Artist
		if err := rows.Scan(&a.ID, &a.Name, &a.Biography, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, readError(ctx, "artist scan", err)
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, readError(ctx, "artist iteration", err)
	}
	return result, nil
}

func (r *Repository) Album(ctx context.Context, id string) (Album, error) {
	var a Album
	err := r.db.QueryRowContext(ctx, `SELECT id,primary_artist_id,title,release_date,created_at,updated_at FROM albums WHERE id=$1`, id).
		Scan(&a.ID, &a.PrimaryArtistID, &a.Title, &a.ReleaseDate, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Album{}, ErrNotFound
	}
	if err != nil {
		return Album{}, readError(ctx, "album lookup", err)
	}
	return a, nil
}

func (r *Repository) Albums(ctx context.Context, page Page) ([]Album, error) {
	limit, offset, err := page.bounds()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,primary_artist_id,title,release_date,created_at,updated_at FROM albums
 ORDER BY title COLLATE "C",id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, readError(ctx, "album list", err)
	}
	defer rows.Close()
	result := make([]Album, 0)
	for rows.Next() {
		var a Album
		if err := rows.Scan(&a.ID, &a.PrimaryArtistID, &a.Title, &a.ReleaseDate, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, readError(ctx, "album scan", err)
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, readError(ctx, "album iteration", err)
	}
	return result, nil
}

func readError(ctx context.Context, op string, err error) error {
	// pgx may surface cancellation as a driver error; retain errors.Is semantics.
	if ctx.Err() != nil {
		return fmt.Errorf("%s: %w", op, ctx.Err())
	}
	return fmt.Errorf("%s: %w", op, err)
}
