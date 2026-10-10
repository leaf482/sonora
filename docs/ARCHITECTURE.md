# Sonora architecture

## Runtime and module boundaries

Browser -> same-origin Next.js UI and /api/v1 Go API -> PostgreSQL metadata/state plus a private filesystem audio directory. Next.js owns presentation; Go owns domain rules. Next.js does not access the database or duplicate domain services. Same-origin routing supports cookies and native audio requests; the routing layer must not buffer whole audio bodies.

Go is one modular monolith/process: identity (users/sessions), catalog (artists/albums/tracks/eligibility), media (validation/byte serving), library, playlists, and history. HTTP handlers validate transport input, application services enforce rules/transactions, and repositories use parameterized SQL. Cross-module operations use explicit service functions. No broker, Redis, microservices, HLS, CDN or speculative infrastructure.

PostgreSQL stores metadata, provenance fields, constraints, sessions and user state; never audio blobs. Immutable audio files live outside public/static roots under generated storage keys. Operator import/withdrawal commands reuse catalog/media services; no public upload/admin API. Task 1 selects a licensed original format compatible with browsers; no transcoding pipeline.

## Implemented backend foundation (Task 2)

The Go module lives at `backend/` with executable sources/tests in `backend/cmd/server/` (module `github.com/leaf482/sonora/backend`, Go 1.25+). This is the only implemented service. The domain modules and other runtime components above remain planned; no empty domain packages are scaffolded.

- `PORT` defaults to 8080 only when unset. If present, it must contain decimal digits and resolve to 1–65535; empty, signed, whitespace-padded, out-of-range and overflowing values fail startup. The listener binds all local interfaces on that port.
- `GET /health` returns 200, `Content-Type: application/json`, and `{"status":"ok"}`. It is a non-versioned process-liveness endpoint, not database/storage readiness. Standard Go routing also accepts HEAD without a body, rejects other methods with 405, and leaves unknown paths at 404.
- JSON structured logs go to stdout for startup, normal shutdown and fatal failures; standard HTTP server errors use the same logger. Request-ID/access logging remains future work.
- HTTP timeouts: read headers 5s, total request read 10s, response write 10s, idle keep-alive 60s. These are for the health-only server; reassess response deadlines before implementing long audio streams.
- Ctrl+C (and SIGTERM on Unix) stops new accepts and allows active requests up to 10s to finish using a separate shutdown context. On expiry, active connections are force-closed and the process reports an error/nonzero exit. Successful draining exits normally. Request contexts are not prematurely canceled by the signal itself.

Run/test commands are in [DEVELOPMENT_WORKFLOW.md](DEVELOPMENT_WORKFLOW.md). Task 2 remains the HTTP foundation; Task 3 adds only the local PostgreSQL environment and connection lifecycle described below. Authentication, audio serving, frontend and domain services remain unimplemented.

## PostgreSQL development connectivity (Task 3)

[compose.yaml](../compose.yaml) provides PostgreSQL 17.11, a persistent named volume and a TCP healthcheck, bound to host loopback only. The backend accepts optional DATABASE_URL plus DB_CONNECT_TIMEOUT (default 5s, maximum 30s). If configured, an authenticated startup ping must succeed before HTTP listens; a failed or malformed configuration exits clearly without logging credentials. Unset DATABASE_URL preserves health-only startup. GET /health remains liveness, not dependency readiness.

Connection setup is shared in backend/internal/postgres/database.go and uses database/sql with the pinned pgx driver, a bounded pool (five open/two idle), and close after HTTP shutdown. Task 5 adds only artists/albums tables; repositories and runtime application queries remain unimplemented. The local Compose initialization role is for development only. See [POSTGRESQL.md](POSTGRESQL.md) for configuration, volume persistence/reset, and a disposable live smoke procedure.

## Explicit SQL migrations (Task 4)

The separate backend/cmd/migrate command uses pinned Goose v3.26.0 with the existing pgx driver and shared DATABASE_URL/DB_CONNECT_TIMEOUT helpers. It exposes up/status from backend/, reads versioned SQL in backend/migrations/, bounds execution to one minute and uses Goose's PostgreSQL session advisory lock for apply. SQL migrations are transactional by default. The baseline only executes SELECT 1; goose_db_version is infrastructure bookkeeping, not a domain table. Status may initialize that bookkeeping table on a fresh database. Server startup does not run migrations, and schema readiness checks remain planned. See [MIGRATIONS.md](MIGRATIONS.md) for conventions and disposable verification.

## Artist and Album schema (Task 5)

Migration 00002 adds artists/albums with explicit UUID keys, required names/titles and timestamps, optional biography/release_date, and a required restricted album-to-artist FK. Only primary-key indexes and the album artist-reference index are included. [DOMAIN_MODEL.md](DOMAIN_MODEL.md) records nullability, repeated names/titles, timestamp maintenance and future Track relationships. No other domain tables or runtime catalog behavior are implemented.

## API boundaries

All endpoints below are prefixed /api/v1. Responses use JSON except the binary audio stream. Public reads expose active catalog/attribution only. All /me endpoints require a session. Public playback is planned only if Task 1 verifies the intended audience/use; otherwise revise the contract before release.

- POST /auth/register, /auth/login, /auth/logout; GET /me. Opaque expiring database-backed sessions; logout revokes the session.
- GET /artists, /artists/{id}, /albums, /albums/{id}, /tracks, /tracks/{id}; GET /search?q=... for grouped track/artist/album matches. Return safe credits/license links and stream URLs, not disk paths or private evidence.
- GET|HEAD /tracks/{id}/audio; check track and asset eligibility on every request, including ranges.
- GET /me/saved-tracks; idempotent PUT|DELETE /me/saved-tracks/{trackId}.
- GET|POST /me/playlists; GET|PATCH|DELETE /me/playlists/{id}; POST /me/playlists/{id}/items; DELETE /me/playlists/{id}/items/{itemId}; PUT /me/playlists/{id}/order with every current item ID in order.
- POST /me/listening-events with client playback ID, track ID and listened milliseconds; GET /me/listening-events; DELETE /me/listening-events to clear.

Lists use limit/offset (default 20, maximum 100) and stable ordering with an ID tie-breaker. Search uses bounded, case-insensitive PostgreSQL matching over names/titles; no external search engine. Search applies pagination independently to each result group and reports totals. Saved tracks/history sort newest first; playlist items sort by position.

Playlist responses include a version. Every rename/delete/item/reorder mutation supplies the expected version. A transaction locks the parent, checks ownership/version, validates and mutates items, and increments the version; stale edits return 409. Adding items accepts a request ID unique within the playlist. Check an existing request ID after ownership but before version validation; an identical retry returns the existing item without another mutation. Explicit new requests may repeat a track. Reusing a key with different content returns 409.

Use 201 for creation, 200 for reads/updates/identical retry results, 204 for deletion. Errors have {error: {code, message, requestId}}: 400 validation, 401 unauthenticated, 404 absent/inaccessible (including another owner's private ID), 409 conflict, 429 throttling and generic 500 unexpected failure. Do not expose SQL, secrets or paths.

## Playback and HTTP Range

One native HTML audio element lives in the persistent frontend layout. Browser state tracks paused/buffering/playing/ended/failed, ignores stale events on track changes and handles rejected playback promises. Seeking is not listening time. No database playback clock or automatic playback on page load.

Implement and test full GET 200, satisfiable single byte range 206 with exact Content-Range/length, and unsatisfiable range 416 with total size. Include Accept-Ranges: bytes, correct MIME/length, immutable-file validators and HEAD without a body. Honor conditional requests/If-Range consistently. Malformed ranges return 400; unsupported multipart ranges are ignored with full 200. Verify the chosen Go handler against this contract rather than assuming defaults agree.

Use bounded buffers and cancellation-aware descriptor cleanup. Cache-Control: no-store avoids long-lived caching so new requests recheck withdrawal. Bytes already delivered cannot be recalled; withdrawal may not stop an already-open transfer. Include that limit in the rights decision.

## Correctness and reliability

- Foreign keys, uniqueness and checks support service rules in DOMAIN_MODEL. Multi-row mutations are atomic; use versioned SQL migrations and reject incompatible schema on readiness/startup.
- Import verifies rights before publication, stages/validates bytes and atomically renames a complete file before marking the database asset ready and track active. A crash can leave an orphan file, never intentionally published partial bytes. Reconcile missing/orphan files; cleanup uses a grace period and never deletes referenced media blindly.
- Eligible media with a missing file returns recoverable 503, logs an operator diagnostic and is marked unavailable by reconciliation. Never substitute unverified media.
- Serialize activation/withdrawal with catalog mutations; services recheck availability within save/add/history transactions. Stream requests check availability when opening files. In-flight transfers have the stated withdrawal limitation.
- History is best-effort personal data, not royalty/billing accounting. Keep a pending event in memory and retry transient errors with its original ID while signed in. Reload/offline loss is acceptable; do not retry auth/validation failures indefinitely.
- Bound body sizes, list lengths, queries, timeouts and database pools; support graceful shutdown/cancellation. In-process auth throttling suffices for the single-process MVP.
- Structured request-ID/status/latency logs exclude credentials/private listening details. Separate liveness from database/storage readiness; no monitoring platform dependency.
- Back up database, referenced immutable media and license evidence together using a manifest/checksums; exercise restore and reconciliation. Document missing-storage/database-outage recovery.

## Security and privacy

Use a maintained password-hashing library, never plaintext. Sessions use cryptographically random tokens with only hashes in PostgreSQL, expiry, rotation on login and revocation on logout/deletion. Production cookies are HttpOnly, Secure and SameSite=Lax. Enforce same-origin/CSRF protection on mutations including auth; HTTPS outside local development. No tokens in URLs/local storage.

Derive owner identity from the session, not request fields. Authorize every private read/write including nested items/idempotency lookups. Validate inputs, parameterize SQL, escape metadata and never render source HTML. Resolve files only through trusted storage keys, rejecting traversal and symlink escapes. No client file paths or public arbitrary-URL fetch endpoint. Secrets stay outside Git/logs.

History, libraries and playlists are private. Clear deletes the user's history. MVP account deletion is operator-assisted, revokes sessions and cascades private records; self-service deletion UI is deferred. Preserve catalog provenance separately. Decide and document backup retention/delayed deletion before public deployment.

## Verification

Domain unit tests; real PostgreSQL tests for constraints/transactions/ownership; HTTP tests for auth, retries, pagination and ranges; browser tests for core journeys and responsive playback. Exercise failures and races, not just mocks. [PROJECT_PLAN.md](PROJECT_PLAN.md) defines acceptance and licensing; [TASKS.md](TASKS.md) sequences delivery.
