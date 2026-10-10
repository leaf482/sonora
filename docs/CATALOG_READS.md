# Artist and Album reads

Task 8 adds [internal/catalog](../backend/internal/catalog/repository.go), a concrete PostgreSQL read repository. Pass the existing non-nil *sql.DB pool to catalog.NewRepository; its caller owns and closes the pool. No new connection settings, dependencies, interfaces, ORM, migrations or HTTP wiring are introduced.

## Contract

- Artist(ctx, id) returns one Artist: ID, Name, nullable Biography, CreatedAt and UpdatedAt.
- Artists(ctx, Page) returns a bounded artist slice.
- Album(ctx, id) returns one Album: ID, PrimaryArtistID, Title, nullable ReleaseDate, CreatedAt and UpdatedAt.
- Albums(ctx, Page) returns a bounded album slice.

UUIDs are strings. Optional fields use pointers so NULL is distinct from an empty biography or known date. Timestamps use time.Time. ReleaseDate represents the PostgreSQL calendar date; format it as YYYY-MM-DD without timezone conversion. Scan failures are errors, never partially populated successful records.

Page{Limit: 0, Offset: 0} defaults to 20 results. Limits 1–100 are accepted; negative limits, limits above 100 and negative offsets return ErrInvalidPagination before SQL executes. Large nonnegative offsets are permitted; only the returned page size is bounded. Successful empty lists return a non-nil empty slice.

Artists order by name COLLATE "C", then UUID ascending. Albums order by title COLLATE "C", then UUID ascending. This explicitly fixes case-sensitive byte ordering across database locale settings, with IDs breaking equal names/titles. Offset pagination is deterministic for unchanged data, not a snapshot across separate calls: concurrent inserts/renames/deletes can shift pages. No speculative sort indexes are added.

Missing valid UUIDs return ErrNotFound, detected with errors.Is. Invalid UUID syntax is a database error, not not-found. All SQL is fixed and parameters bind IDs, limits and offsets. No caller-provided SQL fragments are accepted.

QueryRowContext/QueryContext propagate the caller's context. The repository does not choose a new deadline; callers should provide one. Cancellation/deadline failures support errors.Is(err, context.Canceled/DeadlineExceeded), including driver cancellation errors. Other query, scan and iteration failures are wrapped with operation context, preserving their causes. Lists close rows and check rows.Err; errors discard any partial result. Errors are internal diagnostics and are sanitized by the Task 9 HTTP handlers rather than serialized directly.

These reads expose persisted artists/albums irrespective of playable tracks. Public visibility rules still require a future service/query change. No Track/AudioAsset queries, writes or public API are part of this repository.

Task 9 exposes this contract through the [Artist/Album HTTP API](CATALOG_API.md), borrowing the existing startup pool.

## Verify

Run from backend/:

```powershell
go test ./...
go vet ./...
go build ./...
```

Ordinary pagination tests require no database. TestRepositoryLive is opt-in using SONORA_TEST_DATABASE_URL, consistent with the existing integration convention. It requires an empty public schema and refuses to reset a used database. It applies existing migrations, inserts only synthetic Artist/Album test records, tests the repository, and exercises SQL errors by renaming tables in that disposable database. Do not run it alongside the other fresh-database integration tests against the same database.

Run from the repository root in a fresh PowerShell session:

```powershell
$smokeProject = 'sonora-reads-' + [guid]::NewGuid().ToString('N').Substring(0, 8)
$probe = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
$probe.Start()
$env:POSTGRES_PORT = [string]$probe.LocalEndpoint.Port
$probe.Stop()
$env:POSTGRES_USER = 'sonora'
$env:POSTGRES_DB = 'sonora'
$env:POSTGRES_PASSWORD = [guid]::NewGuid().ToString('N')
$env:SONORA_TEST_DATABASE_URL = "postgres://sonora:$($env:POSTGRES_PASSWORD)@127.0.0.1:$($env:POSTGRES_PORT)/sonora?sslmode=disable"
try {
    docker compose -p $smokeProject config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Compose config failed' }
    docker compose -p $smokeProject up -d --wait --wait-timeout 90
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL startup failed' }
    Push-Location backend
    try {
        go test -count=1 -run '^TestRepositoryLive$' -v ./internal/catalog
        if ($LASTEXITCODE -ne 0) { throw 'Repository live checks failed' }
    } finally { Pop-Location }
} finally {
    docker compose -p $smokeProject down --volumes --remove-orphans
    if ($LASTEXITCODE -ne 0) { throw "Cleanup failed: $smokeProject" }
    Remove-Item Env:SONORA_TEST_DATABASE_URL, Env:POSTGRES_PASSWORD
}
```

A host-port race may require retrying with a new port. Cleanup removes only this unique disposable project.

Verified on 2026-10-09 with PostgreSQL 17.11: empty lists, missing IDs, all persisted fields, NULL versus empty biography, optional release dates, default/min/max/last/past-end pagination, name/title ordering with UUID ties, malformed/injection-like IDs, canceled contexts, deadlines during lock-blocked queries, pool recovery, SQL errors and closed-pool errors. All passed; disposable container/network/volume were removed. Existing seed files, fixtures, migrations and HTTP behavior are unchanged.
