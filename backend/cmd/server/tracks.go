package main

import (
	"net/url"
	"time"

	"github.com/leaf482/sonora/backend/internal/catalog"
)

// Explicit public metadata allowlist: never embed an AudioAsset or repository model.
type trackJSON struct {
	ID              string    `json:"id"`
	PrimaryArtistID string    `json:"primary_artist_id"`
	AlbumID         *string   `json:"album_id"`
	Title           string    `json:"title"`
	DisplayCredits  *string   `json:"display_credits"`
	DurationMS      *int64    `json:"duration_ms"`
	DiscNumber      *int      `json:"disc_number"`
	TrackNumber     *int      `json:"track_number"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func trackResponse(t catalog.Track) trackJSON {
	return trackJSON{t.ID, t.PrimaryArtistID, t.AlbumID, t.Title, t.DisplayCredits, t.DurationMS, t.DiscNumber, t.TrackNumber, t.State, t.CreatedAt.UTC(), t.UpdatedAt.UTC()}
}

func parseTrackQuery(raw string) (catalog.Page, catalog.TrackFilter, error) {
	var filter catalog.TrackFilter
	values, err := url.ParseQuery(raw)
	if err != nil {
		return catalog.Page{}, filter, err
	}
	for _, key := range []string{"artist_id", "album_id"} {
		if entries, ok := values[key]; ok {
			if len(entries) != 1 || !validUUID(entries[0]) {
				return catalog.Page{}, filter, catalog.ErrInvalidPagination
			}
			value := entries[0]
			if key == "artist_id" {
				filter.ArtistID = &value
			} else {
				filter.AlbumID = &value
			}
			values.Del(key)
		}
	}
	// The existing parser still rejects unknown parameters and malformed pagination.
	page, err := parsePage(values.Encode())
	return page, filter, err
}
