package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/leaf482/sonora/backend/internal/catalog"
)

type artistJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Biography *string   `json:"biography"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type albumJSON struct {
	ID              string    `json:"id"`
	PrimaryArtistID string    `json:"primary_artist_id"`
	Title           string    `json:"title"`
	ReleaseDate     *string   `json:"release_date"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type paginationJSON struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
type catalogResponse struct {
	Data       any             `json:"data"`
	Pagination *paginationJSON `json:"pagination,omitempty"`
}

func artistResponse(a catalog.Artist) artistJSON {
	return artistJSON{a.ID, a.Name, a.Biography, a.CreatedAt.UTC(), a.UpdatedAt.UTC()}
}
func albumResponse(a catalog.Album) albumJSON {
	var date *string
	if a.ReleaseDate != nil {
		value := a.ReleaseDate.Format("2006-01-02")
		date = &value
	}
	return albumJSON{a.ID, a.PrimaryArtistID, a.Title, date, a.CreatedAt.UTC(), a.UpdatedAt.UTC()}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_ = json.NewEncoder(w).Encode(value)
	}
}

type errorJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func catalogError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, r, status, struct {
		Error errorJSON `json:"error"`
	}{errorJSON{code, message}})
}

// Canonical UUID syntax, without restricting the version of existing database IDs.
func validUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func parsePage(raw string) (catalog.Page, error) {
	page := catalog.Page{Limit: 20}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return page, err
	}
	for key, entries := range values {
		if (key != "limit" && key != "offset") || len(entries) != 1 || entries[0] == "" {
			return page, catalog.ErrInvalidPagination
		}
		for _, c := range entries[0] {
			if c < '0' || c > '9' {
				return page, catalog.ErrInvalidPagination
			}
		}
		n, err := strconv.Atoi(entries[0])
		if err != nil {
			return page, catalog.ErrInvalidPagination
		}
		if key == "limit" {
			if n > 100 {
				return page, catalog.ErrInvalidPagination
			}
			if n != 0 {
				page.Limit = n
			}
		} else {
			page.Offset = n
		}
	}
	return page, nil
}

func registerCatalog(mux *http.ServeMux, repository *catalog.Repository, logger *slog.Logger) {
	for _, resource := range []string{"artists", "albums"} {
		for _, detail := range []bool{false, true} {
			path := "/api/" + resource
			if detail {
				path += "/{id}"
			}
			mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet && r.Method != http.MethodHead {
					w.Header().Set("Allow", "GET, HEAD")
					catalogError(w, r, 405, "method_not_allowed", "Use GET or HEAD.")
					return
				}
				var page catalog.Page
				if detail {
					if !validUUID(r.PathValue("id")) || r.URL.RawQuery != "" {
						catalogError(w, r, 400, "invalid_request", "Use a canonical UUID and no query parameters.")
						return
					}
				} else {
					var err error
					page, err = parsePage(r.URL.RawQuery)
					if err != nil {
						catalogError(w, r, 400, "invalid_request", "Use limit 0..100 and a nonnegative offset, each at most once.")
						return
					}
				}
				if repository == nil {
					catalogError(w, r, 503, "catalog_unavailable", "Catalog database is not configured.")
					return
				}
				// WriteTimeout bounds network writes, not SQL execution. Bound query work too.
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				var data any
				var err error
				if resource == "artists" {
					if detail {
						var a catalog.Artist
						a, err = repository.Artist(ctx, r.PathValue("id"))
						data = artistResponse(a)
					} else {
						var rows []catalog.Artist
						rows, err = repository.Artists(ctx, page)
						items := make([]artistJSON, 0, len(rows))
						for _, a := range rows {
							items = append(items, artistResponse(a))
						}
						data = items
					}
				} else {
					if detail {
						var a catalog.Album
						a, err = repository.Album(ctx, r.PathValue("id"))
						data = albumResponse(a)
					} else {
						var rows []catalog.Album
						rows, err = repository.Albums(ctx, page)
						items := make([]albumJSON, 0, len(rows))
						for _, a := range rows {
							items = append(items, albumResponse(a))
						}
						data = items
					}
				}
				if err != nil {
					switch {
					case errors.Is(err, context.Canceled):
						return // The client has gone away.
					case errors.Is(err, context.DeadlineExceeded):
						catalogError(w, r, 504, "catalog_timeout", "Catalog request timed out.")
					case errors.Is(err, catalog.ErrNotFound):
						catalogError(w, r, 404, "not_found", "Catalog record not found.")
					default:
						logger.Error("catalog read failed", "resource", resource, "detail", detail)
						catalogError(w, r, 500, "internal_error", "Catalog request failed.")
					}
					return
				}
				response := catalogResponse{Data: data}
				if !detail {
					response.Pagination = &paginationJSON{page.Limit, page.Offset}
				}
				writeJSON(w, r, 200, response)
			})
		}
	}
}
