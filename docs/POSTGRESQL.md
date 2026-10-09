# Local PostgreSQL development

Task 3 adds only a database process and backend connectivity. No migrations, application tables, repositories or application SQL are included. `GET /health` remains process liveness: 200 with `{"status":"ok"}`; it does not ping PostgreSQL on each request or become a readiness endpoint.

## Configuration

[compose.yaml](../compose.yaml) runs the official `postgres:17.11-alpine3.23` image. It binds only `127.0.0.1` on the host and persists data in the Compose-managed `postgres_data` named volume. The default project is `sonora`; an explicit `-p` isolates smoke resources. The TCP `pg_isready` healthcheck waits for normal server startup, but is not proof that the backend's password works. The backend's authenticated startup ping supplies that check.

Compose variables:

- `POSTGRES_USER` and `POSTGRES_DB`: default `sonora`.
- `POSTGRES_PORT`: host port, default 5432 (container port stays 5432).
- `POSTGRES_PASSWORD`: required and nonempty; there is no tracked working password.

Backend variables:

- `DATABASE_URL`: optional. Unset preserves health-only operation without a database. If set, including an empty value, configuration must be valid; failures never silently fall back to database-free operation. Use a `postgres://` or `postgresql://` URL with explicit host, user and database. URL-encode special characters in credentials.
- `DB_CONNECT_TIMEOUT`: default `5s`; positive Go duration up to `30s`. Bounds initial connect/authentication/ping using the signal context. Invalid values fail configuration even when DATABASE_URL is unset.
- `PORT`: HTTP port; existing semantics/default 8080 unchanged.

The backend uses `database/sql` with the pgx v5 driver, pinned in [go.mod](../backend/go.mod) / [go.sum](../backend/go.sum). Five indirect dependencies come from that driver; no ORM, retry framework or general database abstraction is introduced. The pool allows at most five open/two idle connections. A configured database must pass its startup ping before HTTP binds. Failed setup closes the pool and exits nonzero with a credential-free error; successful setup closes it after the existing HTTP shutdown completes. No URL/password or raw driver/server error is logged. An outage after startup does not change liveness; readiness and runtime database behavior are separate future work.

For this loopback-only development service, use `sslmode=disable`. For another environment, select and configure its actual TLS policy (for example `verify-full`); do not reuse the local setting as a production default. The image's initialization user is a local development superuser, not a production least-privilege application role.

## Start locally

Requirements: Go 1.25+, Docker with Linux containers, and Docker Compose. Start Docker Desktop if needed. Run Compose commands from the repository root and Go commands from `backend/`.

PowerShell, from the repository root (pick a local password; nothing is written to tracked files):

```powershell
$env:POSTGRES_USER = 'sonora'
$env:POSTGRES_DB = 'sonora'
$env:POSTGRES_PORT = '5432'
$localPassword = Read-Host 'Local PostgreSQL password' -AsSecureString
$env:POSTGRES_PASSWORD = [Net.NetworkCredential]::new('', $localPassword).Password
$encodedPassword = [Uri]::EscapeDataString($env:POSTGRES_PASSWORD)
$env:DATABASE_URL = "postgres://sonora:$encodedPassword@127.0.0.1:$($env:POSTGRES_PORT)/sonora?sslmode=disable"

docker compose config --quiet
docker compose up -d --wait --wait-timeout 90
docker compose ps
Set-Location backend
go run ./cmd/server
```

In another terminal: `Invoke-RestMethod http://localhost:8080/health`. Startup should log `database connected` before `server listening`. Stop the backend with Ctrl+C.

Alternatively, copy [.env.example](../.env.example) to a root `.env`, supply your own password, and let Compose load it. Shell variables override the file. The Go backend does **not** load .env: export its DATABASE_URL separately using the same credentials/host port. Do not print expanded `docker compose config` into reports; use `--quiet` because expanded configuration contains the password. Local .env files are ignored by Git.

Initialization variables only apply to an empty volume. Changing POSTGRES_PASSWORD/USER/DB later does not change an existing cluster. Reuse the original local credentials, or explicitly reset disposable development data as described below. A healthy container plus an authentication failure often indicates this mismatch. A host-port conflict can be resolved by changing POSTGRES_PORT and the URL together.

## Stop, restart and deliberate reset

From the repository root:

```powershell
docker compose stop
docker compose up -d --wait --wait-timeout 90
docker compose down
```

Stop/down preserve the named data volume; a later up reuses it. Keep the same local password. Only when you deliberately want to erase this project's local PostgreSQL data, use `docker compose down --volumes`. Never run that command against a project containing data you need. No broad Docker prune command is required. PostgreSQL major-version upgrades need a separate data migration plan; do not simply replace the image's major version against an existing volume.

To return the backend to health-only startup, remove DATABASE_URL from its environment; an empty value is an error. Environment variables persist for the current shell and children; open a clean shell or remove the local credential variables when finished.

## Automated checks

From `backend/`:

```powershell
go test ./...
go vet ./...
go build ./...
```

Ordinary tests need no PostgreSQL and use no real credentials: they cover URL/timeout validation, canceled startup, a stalled TCP handshake, credential-free failure messages, and the existing health/shutdown behaviors. `TestPostgresLive` skips unless `SONORA_TEST_DATABASE_URL` is explicitly set, so normal tests never silently use a developer's DATABASE_URL. An explicitly empty/invalid test URL fails.

## Disposable live smoke procedure

Run in a **fresh PowerShell session** from the repository root. This creates a unique project/volume, generates a temporary password, and chooses a free host port. It does not use the default development volume or create application schema. The password-authenticated test verifies connect/ping/close and rejection of an incorrect password. The selected port has a small bind race; if another process claims it before Compose, rerun with a newly selected port.

```powershell
$smokeProject = 'sonora-smoke-' + [guid]::NewGuid().ToString('N').Substring(0, 8)
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
        go test -run '^TestPostgresLive$' -v ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw 'Live connectivity test failed' }
    } finally { Pop-Location }
} finally {
    docker compose -p $smokeProject down --volumes --remove-orphans
    if ($LASTEXITCODE -ne 0) { throw "Cleanup failed: $smokeProject" }
    Remove-Item Env:SONORA_TEST_DATABASE_URL, Env:POSTGRES_PASSWORD
}
```

The final down command is scoped to the newly generated disposable project, not `sonora`. If Docker is unavailable, still run Go checks and Compose config validation if the CLI exists; report the live test as blocked, never as passed. Do not substitute mocks for this live result.

## Task 3 validation record

On 2026-10-09, Go 1.25.4 / Docker Compose v5.5.1 / Docker Engine 29.8.1:

- Compose configuration validated and PostgreSQL 17.11 container became healthy.
- Live Go connect/ping/close and wrong-password test passed.
- Compiled backend started after the authenticated ping; GET /health remained 200 with the original JSON.
- Wrong-password backend startup exited 1 before HTTP listening; credentials were absent from logs.
- The isolated container/network/volume were removed afterward; no application schema was created.

References: [official PostgreSQL image](https://hub.docker.com/_/postgres) (initialization and volume layout), [pgx database/sql driver](https://pkg.go.dev/github.com/jackc/pgx/v5/stdlib). Exact dependency versions are tracked; review updates deliberately.
