# Database migrations

Task 4 introduces explicit operator-run SQL migrations using Goose v3.26.0 and the existing pgx driver. This version supports the project's Go 1.25 toolchain; Goose v3.28.0 requires Go 1.26. The small [command](../backend/cmd/migrate/main.go) delegates version tracking, SQL parsing, transactions and locking to Goose. It imports the library rather than the multi-database CLI. There is no custom migration framework.

## Configuration and commands

Use the same DATABASE_URL already configured for the server in [POSTGRESQL.md](POSTGRESQL.md). The migration command requires it, including explicit user, host and database. It never accepts credentials as command-line arguments, reads a separate secrets file, or loads .env. DB_CONNECT_TIMEOUT has the same validation and default (5s, positive and at most 30s) as the server. Both commands reuse [connection helpers](../backend/internal/postgres/database.go).

From backend/:

```powershell
go run ./cmd/migrate status
go run ./cmd/migrate up
go run ./cmd/migrate status
```

The SQL directory is relative to backend/. Run there, including when running a compiled migration executable. A missing directory or invalid command fails nonzero. The operation context is bounded to one minute, including connecting, waiting for the PostgreSQL advisory lock and executing SQL. Goose lock release and connection cleanup may finish afterward. Ctrl+C cancels it. Pool cleanup runs on return. Reassess this explicit limit in a future task if migrations need longer.

The up command applies pending migrations in order and reports the number applied. Repeating it reports zero. The status command reports each version as pending/applied. Goose can initialize its goose_db_version bookkeeping table even for status on a fresh database: status does not apply migration SQL, but is not strictly read-only. Do not manually edit this table. The up command uses Goose's PostgreSQL session advisory lock. Run one operator workflow at a time; status is a snapshot and does not acquire that lock.

The HTTP server never imports Goose or runs migrations. Its optional DB startup ping and GET /health semantics are unchanged. Schema compatibility/readiness checks remain future work.

## SQL conventions

Files live in [backend/migrations/](../backend/migrations/). Use unique increasing numeric versions with five-digit filenames, starting at 00001_baseline.sql, followed by 00002_descriptive_name.sql. Reserve the next number against the current branch before merging. Do not renumber, remove or edit an applied migration; add a new correction migration. Goose records versions, not file checksums, so immutability depends on review and Git history.

Use standard -- +goose Up and -- +goose Down sections. Goose wraps each SQL migration in a transaction by default; keep that default. Review reversibility, data loss and deployment ordering before future schema changes. Do not add NO TRANSACTION or Go migrations without a concrete scoped need.

The baseline executes SELECT 1 in each direction to verify the workflow. It creates no application objects, extensions or seed data. The only persisted table is Goose bookkeeping. DOMAIN_MODEL.md remains a design, not implemented schema.

Only up/status are exposed intentionally for this task. There is no rollback/reset command. For failed transactional SQL, correct an unapplied migration, inspect status and retry; never modify a version already applied anywhere shared. Earlier successful migrations stay applied if a later one fails. For future deployed schema fixes, prefer a reviewed forward migration; destructive recovery requires a separately scoped plan and backup. Errors omit raw driver/server text and connection strings; check the file/permissions and inspect PostgreSQL logs locally when diagnosing, without publishing credentials.

## Fresh local database

Follow the password/environment setup in [POSTGRESQL.md](POSTGRESQL.md), using its same DATABASE_URL. From the repository root:

```powershell
docker compose config --quiet
docker compose up -d --wait --wait-timeout 90
Set-Location backend
go run ./cmd/migrate status
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/server
```

Expect version 00001 pending, one applied migration, then version 00001 applied. Stop the backend with Ctrl+C. From the repository root, docker compose down stops PostgreSQL while preserving the volume. Starting it again and applying migrations preserves existing versions. Do not erase an existing development volume merely to run migrations. For a fresh disposable test, use the isolated procedure below.

## Verification

From backend/:

```powershell
go test ./...
go vet ./...
go build ./...
```

Ordinary tests check invalid commands, missing/malformed URLs and timeout configuration without PostgreSQL. Existing server configuration, connection and health/shutdown tests continue to run. The migration live test is opt-in through SONORA_TEST_DATABASE_URL; it requires an empty public schema, refuses a used database without resetting it, and leaves its migration bookkeeping for the caller to clean up.

Run this in a fresh PowerShell session from the repository root. It reuses Compose with a unique project/volume and generated temporary password, never the normal development volume:

```powershell
$smokeProject = 'sonora-migrations-' + [guid]::NewGuid().ToString('N').Substring(0, 8)
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
        go test -count=1 -run '^TestMigrationsLive$' -v ./cmd/migrate
        if ($LASTEXITCODE -ne 0) { throw 'Migration test failed' }
    } finally { Pop-Location }
} finally {
    docker compose -p $smokeProject down --volumes --remove-orphans
    if ($LASTEXITCODE -ne 0) { throw "Cleanup failed: $smokeProject" }
    Remove-Item Env:SONORA_TEST_DATABASE_URL, Env:POSTGRES_PASSWORD
}
```

The free-port selection has a small bind race; rerun on a new port if another process claims it. The test checks initial pending status, first apply, applied status, repeated apply with no new bookkeeping rows, and absence of application tables. Only the uniquely named disposable project is removed. If Docker is unavailable, report this live test as blocked.

Verified on 2026-10-09 with PostgreSQL 17.11: pending -> applied (one migration) -> repeated apply (zero migrations); bookkeeping unchanged; no domain tables. Actual go run status/up commands also passed after the test, and the disposable container/network/volume were removed.

Reference: [Goose provider documentation](https://pressly.github.io/goose/documentation/provider/). Dependency versions are pinned in [go.mod](../backend/go.mod) and [go.sum](../backend/go.sum).
