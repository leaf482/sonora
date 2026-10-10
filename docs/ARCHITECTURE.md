# Sonora architecture

## Runtime and module boundaries

Browser -> same-origin Next.js UI and Go API -> PostgreSQL metadata/state plus a private filesystem audio directory. Next.js owns presentation; Go owns domain rules. Next.js does not access the database or duplicate domain services. Same-origin routing supports cookies and native audio requests; the routing layer must not buffer whole audio bodies.

Go is one modular monolith/process: identity (users/sessions), catalog (artists/albums/tracks/eligibility), media (validation/byte serving), library, playlists, and history. HTTP handlers validate transport input, application services enforce rules/transactions, and repositories use parameterized SQL. Cross-module operations use explicit service functions. No broker, Redis, microservices, HLS, CDN or speculative infrastructure.

PostgreSQL stores metadata, provenance fields, constraints, sessions and user state; never audio blobs. Immutable audio files live outside public/static roots under generated storage keys. Operator import/withdrawal commands reuse catalog/media services; no public upload/admin API. Task 1 selects a licensed original format compatible with browsers; no transcoding pipeline.

## Implemented backend foundation (Task 2)

The Go module lives at `backend/` with executable sources/tests in `backend/cmd/server/` (module `github.com/leaf482/sonora/backend`, Go 1.25+). This is the only implemented service. Most domain behavior and the other runtime components above remain planned; internal/catalog now provides the Artist/Album and Track/AudioAsset reads described below.

- `PORT` defaults to 8080 only when unset. If present, it must contain decimal digits and resolve to 1–65535; empty, signed, whitespace-padded, out-of-range and overflowing values fail startup. The listener binds all local interfaces on that port.
- `GET /health` returns 200, `Content-Type: application/json`, and `{"status":"ok"}`. It is a non-versioned process-liveness endpoint, not database/storage readiness. Standard Go routing also accepts HEAD without a body, rejects other methods with 405, and leaves unknown paths at 404.
- JSON structured logs go to stdout for startup, normal shutdown and fatal failures; standard HTTP server errors use the same logger. Request-ID/access logging remains future work.
- HTTP timeouts: read headers 5s, total request read 10s, response write 10s, idle keep-alive 60s. These cover health and metadata reads; reassess response deadlines before implementing long audio streams.
- Ctrl+C (and SIGTERM on Unix) stops new accepts and allows active requests up to 10s to finish using a separate shutdown context. On expiry, active connections are force-closed and the process reports an error/nonzero exit. Successful draining exits normally. Request contexts are not prematurely canceled by the signal itself.

Run/test commands are in [DEVELOPMENT_WORKFLOW.md](DEVELOPMENT_WORKFLOW.md). Task 2 remains the HTTP foundation; Task 3 adds only the local PostgreSQL environment and connection lifecycle described below. Authentication, audio serving, frontend and domain services remain unimplemented.

## PostgreSQL development connectivity (Task 3)

[compose.yaml](../compose.yaml) provides PostgreSQL 17.11, a persistent named volume and a TCP healthcheck, bound to host loopback only. The backend accepts optional DATABASE_URL plus DB_CONNECT_TIMEOUT (default 5s, maximum 30s). If configured, an authenticated startup ping must succeed before HTTP listens; a failed or malformed configuration exits clearly without logging credentials. Unset DATABASE_URL preserves database-free startup; Task 9 catalog routes return JSON 503. GET /health remains liveness, not dependency readiness.

Connection setup is shared in backend/internal/postgres/database.go and uses database/sql with the pinned pgx driver, a bounded pool (five open/two idle), and close after HTTP shutdown. Tasks 5–6 add artists/albums/tracks/audio_assets tables; Tasks 8 and 10 add Artist/Album and Track/AudioAsset repository reads; other runtime application queries remain unimplemented. The local Compose initialization role is for development only. See [POSTGRESQL.md](POSTGRESQL.md) for configuration, volume persistence/reset, and a disposable live smoke procedure.

## Explicit SQL migrations (Task 4)

The separate backend/cmd/migrate command uses pinned Goose v3.26.0 with the existing pgx driver and shared DATABASE_URL/DB_CONNECT_TIMEOUT helpers. It exposes up/status from backend/, reads versioned SQL in backend/migrations/, bounds execution to one minute and uses Goose's PostgreSQL session advisory lock for apply. SQL migrations are transactional by default. The baseline only executes SELECT 1; goose_db_version is infrastructure bookkeeping, not a domain table. Status may initialize that bookkeeping table on a fresh database. Server startup does not run migrations, and schema readiness checks remain planned. See [MIGRATIONS.md](MIGRATIONS.md) for conventions and disposable verification.

## Artist and Album schema (Task 5)

Migration 00002 adds artists/albums with explicit UUID keys, required names/titles and timestamps, optional biography/release_date, and a required restricted album-to-artist FK. Only primary-key indexes and the album artist-reference index are included. [DOMAIN_MODEL.md](DOMAIN_MODEL.md) records nullability, repeated names/titles, timestamp maintenance and future Track relationships. Task 6 extends these references as described below; runtime catalog behavior remains unimplemented.

## Track and AudioAsset schema (Task 6)

Migration 00003 adds tracks and audio_assets: optional album relationships with unique disc/track positions, one asset per track, private filesystem keys and required file integrity metadata. Separate file availability and license-decision states retain metadata on failure/withdrawal. Verified provenance requires explicit asset-specific evidence, attribution, permitted uses and scope; unknown permissions default to NULL. No media bytes enter PostgreSQL.

[DOMAIN_MODEL.md](DOMAIN_MODEL.md) defines the implemented columns and enforcement boundary. Local CHECK/FK/unique constraints do not prove licensing or file existence. Activation, expiry/scope checks and atomic track/asset withdrawal remain future service responsibilities; no triggers, streaming, seeding or ingestion are introduced. [MIGRATIONS.md](MIGRATIONS.md) records disposable constraint/rollback/reapply verification.

## Explicit development fixture seed (Task 7)

The separate backend/cmd/seed development command populates only the three pinned, rights-reviewed fixtures. It reuses DB configuration, validates all audio/evidence/license hashes, creates two credited artists and three standalone Track/AudioAsset pairs, and copies exact bytes into ignored private backend/.local/audio under stable generated keys. No albums are inferred.

Records start draft/staged with the existing fixture-scope license decision, estimated frame durations and retained attribution/provenance. Transactional insert-only behavior preserves existing states; withdrawn/unavailable records never restore files or reactivate on rerun. Changed metadata or corrupt/missing files fail conservatively. This is not the future production ingestion/activation service. See [SEEDING.md](SEEDING.md) for exact commands, limits and disposable tests.

## Artist and Album read repository (Task 8)

backend/internal/catalog has a concrete Repository borrowing the existing *sql.DB, explicit Artist/Album read models, and four parameterized lookup/list queries. Missing rows are distinct from database failures. Lists use a default limit of 20, maximum 100, nonnegative offset, and name/title ordering with C collation plus UUID tie-breakers. Caller contexts propagate through query/scan/iteration errors and rows are closed.

These internal reads return persisted rows without Track/AudioAsset queries; future public visibility filtering remains a separate requirement. Task 9 wires these reads to HTTP; no writes or repository interface is introduced. [CATALOG_READS.md](CATALOG_READS.md) documents the exact contract and live checks.

## Artist and Album HTTP reads (Task 9)

GET /api/artists, /api/artists/{id}, /api/albums and /api/albums/{id} use the same startup pool and concrete repository. Explicit JSON models preserve nullable biography/release_date and UTC timestamps. Handlers validate UUIDs and pagination, use a five-second child request context for SQL, and return sanitized JSON errors. /health and shutdown behavior remain unchanged. See [CATALOG_API.md](CATALOG_API.md) for the implemented contract and disposable tests.

These development metadata routes return persisted Artist/Album rows, without Track/AudioAsset queries or active-track filtering. They are not the completed public discovery/eligibility implementation. There are no /api/v1 aliases for these four routes.

## Track and AudioAsset read repository (Task 10)

The same concrete repository now reads Track metadata, with bounded title/UUID ordering and optional artist/album filters, and internal AudioAsset metadata by track. Track models contain no storage/checksum/private provenance fields. The separate playback lookup enforces active Track, matching duration, ready file metadata, verified hosting/playback permission and unexpired license in a single database statement snapshot. Missing assets, unavailable assets and withdrawn assets are distinct from missing Tracks and query errors.

These reads perform no file I/O or activation. A database-eligible asset is not proof of file existence, license suitability for a particular audience/territory/use, or browser compatibility. The future streaming service must check those conditions at use time. No new HTTP routes are wired. See [TRACK_READS.md](TRACK_READS.md) for contract and disposable tests.

## Planned API boundaries

The remaining endpoints below are planned under /api/v1; Artist/Album reads use the implemented /api routes above. Responses use JSON except the binary audio stream. Public reads expose active catalog/attribution only. All /me endpoints require a session. Public playback is planned only if Task 1 verifies the intended audience/use; otherwise revise the contract before release.

- POST /auth/register, /auth/login, /auth/logout; GET /me. Opaque expiring database-backed sessions; logout revokes the session.
- GET /tracks, /tracks/{id}; GET /search?q=... for grouped track/artist/album matches. Return safe credits/license links and stream URLs, not disk paths or private evidence.
- GET|HEAD /tracks/{id}/audio; check track and asset eligibility on every request, including ranges.
- GET /me/saved-tracks; idempotent PUT|DELETE /me/saved-tracks/{trackId}.
- GET|POST /me/playlists; GET|PATCH|DELETE /me/playlists/{id}; POST /me/playlists/{id}/items; DELETE /me/playlists/{id}/items/{itemId}; PUT /me/playlists/{id}/order with every current item ID in order.
- POST /me/listening-events with client playback ID, track ID and listened milliseconds; GET /me/listening-events; DELETE /me/listening-events to clear.

Lists use limit/offset (default 20, maximum 100) and stable ordering with an ID tie-breaker. Search uses bounded, case-insensitive PostgreSQL matching over names/titles; no external search engine. Search applies pagination independently to each result group and reports totals. Saved tracks/history sort newest first; playlist items sort by position.

Playlist responses include a version. Every rename/delete/item/reorder mutation supplies the expected version. A transaction locks the parent, checks ownership/version, validates and mutates items, and increments the version; stale edits return 409. Adding items accepts a request ID unique within the playlist. Check an existing request ID after ownership but before version validation; an identical retry returns the existing item without another mutation. Explicit new requests may repeat a track. Reusing a key with different content returns 409.

Use 201 for creation, 200 for reads/updates/identical retry results, 204 for deletion. Planned errors have {error: {code, message, requestId}} (Task 9 omits requestId until request tracing exists): 400 validation, 401 unauthenticated, 404 absent/inaccessible (including another owner's private ID), 409 conflict, 429 throttling and generic 500 unexpected failure. Do not expose SQL, secrets or paths.

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
