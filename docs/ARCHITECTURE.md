# Sonora architecture

## Runtime and module boundaries

Browser -> same-origin Next.js UI and /api/v1 Go API -> PostgreSQL metadata/state plus a private filesystem audio directory. Next.js owns presentation; Go owns domain rules. Next.js does not access the database or duplicate domain services. Same-origin routing supports cookies and native audio requests; the routing layer must not buffer whole audio bodies.

Go is one modular monolith/process: identity (users/sessions), catalog (artists/albums/tracks/eligibility), media (validation/byte serving), library, playlists, and history. HTTP handlers validate transport input, application services enforce rules/transactions, and repositories use parameterized SQL. Cross-module operations use explicit service functions. No broker, Redis, microservices, HLS, CDN or speculative infrastructure.

PostgreSQL stores metadata, provenance fields, constraints, sessions and user state; never audio blobs. Immutable audio files live outside public/static roots under generated storage keys. Operator import/withdrawal commands reuse catalog/media services; no public upload/admin API. Task 2 selects a licensed original format compatible with browsers; no transcoding pipeline.

## API boundaries

All endpoints below are prefixed /api/v1. Responses use JSON except the binary audio stream. Public reads expose active catalog/attribution only. All /me endpoints require a session. Public playback is planned only if Task 2 verifies the intended audience/use; otherwise revise the contract before release.

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
