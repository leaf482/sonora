# Catalog HTTP read API (Tasks 9 and 11)

The existing server exposes GET /api/artists, GET /api/artists/{id}, GET /api/albums, GET /api/albums/{id}, GET /api/tracks and GET /api/tracks/{id}. HEAD performs the same read and returns headers/status without a body. Other methods on these routes return JSON 405 with Allow: GET, HEAD. Unknown paths retain standard Go 404 behavior. There are no /api/v1 aliases.

This development metadata API returns all persisted Artist/Album/Track records through the [existing repository](CATALOG_READS.md). It does not apply the planned active-track public visibility rule. Track reads use the [Track repository](TRACK_READS.md) without playback eligibility filtering. It exposes no AudioAssets, storage paths, checksums or private license evidence. Production discovery/eligibility remains separate work.

## Input and response contract

IDs require 36-character UUID syntax (8-4-4-4-12 hexadecimal digits; either case), without restricting UUID version. Malformed IDs return 400; well-formed absent IDs return 404. Detail routes accept no query parameters.

All lists accept limit and offset, each at most once; Track lists additionally accept the UUID filters below. Values must be nonempty unsigned decimal digits fitting a Go int; signs, whitespace, fractions, overflow, duplicate/unknown parameters and malformed query encoding return 400. Omitted limit or limit=0 means 20, maximum 100; offset defaults to 0 and has no additional cap. Leading zeros are accepted. Empty query separators are ignored by Go URL parsing. Pagination matches the repository: names/titles use C collation ascending with UUID tie-breakers. Offset pagination is stable for unchanged data, not a snapshot across concurrent changes.

Successful responses are HTTP 200, Content-Type: application/json and Cache-Control: no-store. Lists have data as an array (including [] when empty) and effective pagination; details have only data. There is no total-count query.

```json
{"data":[],"pagination":{"limit":20,"offset":0}}
```

Artist fields: id, name, biography, created_at, updated_at. Album fields: id, primary_artist_id, title, release_date, created_at, updated_at. Unknown biography/release_date is explicit JSON null; an empty biography stays an empty string. Dates use YYYY-MM-DD without timezone conversion; timestamps use UTC RFC3339 with fractional precision when present. No additional metadata is inferred.

## Track contract

GET /api/tracks accepts optional artist_id and album_id. Each must occur at most once and contain a nonempty canonical UUID using the same syntax as path IDs. Both filters are ANDed against the Track's own primary_artist_id and album_id; album artist is not substituted for track artist. Omitted filters mean no restriction; a valid but absent relationship yields an empty list, not 404. Malformed/empty/duplicate filters return 400. Artist/Album list routes still reject these parameters. Track details accept no query parameters.

Track data contains exactly id, primary_artist_id, album_id, title, display_credits, duration_ms, disc_number, track_number, state, created_at and updated_at. Optional album/credits/duration/positions are explicit null; empty credits remain an empty string. Duration is integer milliseconds and timestamps use UTC RFC3339. Lists use title C-collation order then UUID, even with an album filter; this is not album-position ordering.

Tracks in draft, active and unavailable states remain visible. A missing, staged, unavailable or withdrawn AudioAsset never hides catalog Track metadata. State is metadata, not a playback guarantee. No audio asset lookup occurs, and no stream URL or eligibility promise is returned. Playback and public-release decisions remain separate.

```json
{"data":{"id":"30000000-0000-4000-8000-000000000005","primary_artist_id":"10000000-0000-4000-8000-000000000001","album_id":null,"title":"Example","display_credits":null,"duration_ms":null,"disc_number":null,"track_number":null,"state":"draft","created_at":"2024-01-02T03:04:05Z","updated_at":"2024-01-02T03:04:05Z"}}
```

## Errors and lifecycle

Errors on matched routes have one shape (HEAD omits the body):

```json
{"error":{"code":"not_found","message":"Catalog record not found."}}
```

- 400 invalid_request: invalid UUID or query parameters.
- 404 not_found: valid ID with no record.
- 405 method_not_allowed: unsupported method.
- 503 catalog_unavailable: DATABASE_URL was unset at startup.
- 504 catalog_timeout: SQL exceeds the five-second handler deadline or an earlier request deadline.
- 500 internal_error: other query/scan/database failures, including missing migrations or a runtime DB outage.

Errors do not serialize SQL, credentials or driver diagnostics. Structured error logs identify the resource and list/detail operation without raw driver errors. Request IDs/access logging remain planned. A canceled request propagates to SQL and the handler stops without attempting an error body. The handler's query context inherits the request context; server write timeouts alone do not cancel SQL. Pool ownership, startup ping and graceful shutdown are unchanged. /health remains liveness, including when the DB is absent or fails later.

## Run locally

Follow [POSTGRESQL.md](POSTGRESQL.md) to start Compose and set DATABASE_URL. Run Go commands from backend/:

```powershell
go run ./cmd/migrate up
go run ./cmd/server
```

In a second terminal:

```powershell
curl.exe -i 'http://localhost:8080/api/artists?limit=2&offset=0'
curl.exe -i 'http://localhost:8080/api/albums'
curl.exe -i 'http://localhost:8080/api/artists/ffffffff-ffff-4fff-8fff-ffffffffffff'
curl.exe -i 'http://localhost:8080/api/albums?limit=101'
curl.exe -i 'http://localhost:8080/api/tracks?limit=2&offset=0'
curl.exe -i 'http://localhost:8080/api/tracks?artist_id=10000000-0000-4000-8000-000000000001&album_id=20000000-0000-4000-8000-000000000001'
curl.exe -i 'http://localhost:8080/api/tracks/ffffffff-ffff-4fff-8fff-ffffffffffff'
```

An empty database returns empty lists; the absent UUID returns 404; limit=101 returns 400. The optional [existing fixture seed](SEEDING.md) provides two artists, no albums and three draft Tracks (not playable). Seeding is not required for startup or tests. Migrations never run automatically at server startup.

## Verification

From backend/, run go test ./..., go vet ./..., and go build ./.... Unit tests cover invalid/duplicate/overflowing inputs, method errors, database-free 503, closed-pool 500 and health preservation. TestCatalogAPILive applies real migrations to an empty disposable database, serves real HTTP requests, and verifies empty lists, missing IDs, all persisted metadata/nulls/date-only values, pagination boundaries and stable ordering with 105 synthetic records, plus HEAD. Table locks exercise request deadlines/cancellation on all four query paths and recovery after releasing locks.

TestTrackAPILive verifies real HTTP Track empty/missing results, filter combinations, bounded pagination/order, nulls/UTC metadata, the exact public field allowlist, withdrawn-asset visibility, HEAD, query cancellation/deadlines, recovery and sanitized SQL errors. It also verifies reads still work when the asset table is unavailable.

Use only an empty disposable database: the live test refuses a nonempty public schema and does not reset existing data. Do not run other fresh-database test suites against the same database. This PowerShell procedure starts at the repository root, generates temporary credentials, and removes only its isolated Compose project:

```powershell
$liveTest = 'TestTrackAPILive' # Use TestCatalogAPILive in a separate fresh run for Artist/Album regression.
$smokeProject = 'sonora-api-' + [guid]::NewGuid().ToString('N').Substring(0,8)
$probe = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
$probe.Start()
$env:POSTGRES_PORT = $probe.LocalEndpoint.Port.ToString()
$probe.Stop()
$env:POSTGRES_USER = 'sonora'
$env:POSTGRES_DB = 'sonora'
$env:POSTGRES_PASSWORD = [guid]::NewGuid().ToString('N')
$env:SONORA_TEST_DATABASE_URL = "postgres://sonora:$($env:POSTGRES_PASSWORD)@127.0.0.1:$($env:POSTGRES_PORT)/sonora?sslmode=disable"
try {
    docker compose -p $smokeProject config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Compose validation failed' }
    docker compose -p $smokeProject up -d --wait --wait-timeout 90
    if ($LASTEXITCODE -ne 0) { throw 'Disposable DB startup failed' }
    Push-Location backend
    try {
        go test -count=1 -run "^$liveTest$" -v ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw 'API integration failed' }
    } finally { Pop-Location }
} finally {
    docker compose -p $smokeProject down --volumes --remove-orphans
    $cleanupExit = $LASTEXITCODE
    Remove-Item Env:SONORA_TEST_DATABASE_URL, Env:POSTGRES_PASSWORD
    if ($cleanupExit -ne 0) { throw 'Disposable cleanup failed' }
}
```

Run in a separate PowerShell session so temporary Compose settings do not replace your normal local settings. Do not print connection URLs/passwords. Successful output includes PASS for the selected live test; all disposable containers, volumes and networks should then be removed.
