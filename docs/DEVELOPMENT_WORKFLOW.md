# Sonora development workflow

## Delivery loop

**PM -> Codex Dev -> independent read-only Reviewer -> user commits.** One scoped task at a time. [TASKS.md](TASKS.md) is the status source. Only the user-selected task is actively in progress. Delivered work with no recorded reviewer verdict is awaiting review, not an inferred PASS; planned features are not completed work.

1. **PM:** select a task with satisfied dependencies and provide objective, scope, non-goals, acceptance criteria, documents and checks. Split work exceeding a few hours before expanding implementation. Identify unresolved licensing/product decisions.
2. **Codex Dev:** inspect instructions, status and existing work; preserve unrelated edits; implement only selected scope and update relevant docs. Run proportionate checks and report changed files, results, limitations, status and suggested commit message. Do not stage or commit.
3. **Independent read-only Reviewer:** use a separate person/context from the implementing developer. Inspect the brief, full diff (including untracked files), surrounding code/docs and evidence. Do not edit, fix, stage, commit or change task status. Read-only checks are allowed; report unavailable validation. Developer self-checks are not independent review.
4. **Verdict PASS/FIX:** PASS means the scoped criteria are met without blocking findings, not that future tasks are complete. FIX lists actionable findings with file/line, impact and expected correction; separate optional suggestions. Missing required validation blocks PASS unless PM explicitly revises the criteria.
5. **Correction and user commit:** on FIX, Dev corrects the same scope, reruns affected checks and returns to review. On PASS, user inspects and commits. Agents do not stage/commit or automatically start the next task. Unless the user explicitly directs a completion status, mark DONE only after independent PASS and user acceptance; record review/commit evidence when available.

## Brief and handoff

Task brief: ID/title, objective, dependencies, allowed scope, exclusions, acceptance criteria, validation commands/manual scenarios and open decisions.

Dev handoff: changed files; behavior/design summary; exact checks with pass/fail/not-run reasons; remaining risks; git status --short; suggested commit message. A documentation task does not imply implementation/tests exist.

Reviewer receives the brief, complete change including untracked content, baseline status and check output. Review correctness, isolation/authorization, concurrency/idempotency, license implications, failure behavior, accessibility when relevant and foundation consistency. Do not require unrelated refactors/infrastructure to pass a scoped task.

## Verification conventions

- Capture git status --short before/after; inspect tracked and untracked changes. Never discard unrelated work.
- Always run git diff --check and git status --short. Diff excludes untracked documents; inspect and whitespace-check them separately without staging.
- For docs: confirm required files exist, relative links resolve, terminology/API/domain rules agree and statuses are accurate. Task 0 needs no application build.
- The Go bootstrap commands are recorded below. PostgreSQL setup and the live smoke procedure are in [POSTGRESQL.md](POSTGRESQL.md). Explicit SQL migration commands and disposable apply/status verification are in [MIGRATIONS.md](MIGRATIONS.md). Frontend/browser commands will be added when those components exist; do not claim checks for unimplemented components.
- Use disposable databases/storage and rights-documented fixtures; no destructive verification against user/production data. Keep secrets, private data and unapproved media out of reports/commits.
- A blocked audio gate prevents dependent ingestion/public playback; only explicitly independent work may continue. Record missing evidence; never invent permission.

## Status meanings

NOT STARTED: no work begun. IN PROGRESS: the single selected task, including development/review. AWAITING REVIEW: work is delivered, but the independent verdict is not recorded. BLOCKED: a named dependency/decision prevents acceptance. DONE: independent PASS and user acceptance, or an explicit user instruction to record completion. Record a user-directed status change without inventing a reviewer verdict. Update deliberately.

## Local Go backend commands

Prerequisite: Go 1.25 or newer (bootstrap verified with Go 1.25.4). The pgx PostgreSQL driver is pinned in go.mod/go.sum. No running database is required when DATABASE_URL is unset; configured database startup is covered in [POSTGRESQL.md](POSTGRESQL.md). The module is [backend/go.mod](../backend/go.mod). All Go commands below run from `backend/`, not the repository root.

PowerShell, starting at the repository root:

```powershell
Set-Location backend
go run ./cmd/server
```

Default URL: `http://localhost:8080/health`. In a second terminal:

```powershell
Invoke-RestMethod http://localhost:8080/health
```

Expect status `ok` (HTTP 200). To choose a different port, stop the process, then run from `backend/`:

```powershell
$env:PORT = '9090'
go run ./cmd/server
```

Use `Remove-Item Env:PORT` to return to the default; setting an empty value is not the documented default mechanism. No .env loader is installed. POSIX shells can use `PORT=9090 go run ./cmd/server`.

Use Ctrl+C to stop. To run the executable directly (useful for observing signal handling without the `go run` wrapper), run from `backend/`:

```powershell
go build -o bin/sonora.exe ./cmd/server
./bin/sonora.exe
```

On Unix, build to `bin/sonora` and execute `./bin/sonora`; SIGTERM also requests graceful shutdown. `backend/bin/` and the default `backend/server` / `backend/server.exe` outputs from `go build ./...` are ignored build output. Logs are JSON on stdout. Invalid configuration, bind/serve failures or shutdown deadline expiry produce a nonzero exit. Normal signal-driven shutdown allows up to 10s for in-flight requests and exits successfully.

Required checks, from `backend/`:

```powershell
gofmt -l cmd internal
go test ./...
go vet ./...
go build ./...
```

`gofmt -l` should print nothing; apply formatting with `gofmt -w cmd internal` when needed. Tests cover health JSON/method/path behavior, PORT defaults/errors, real HTTP request draining, forced close on shutdown deadline, cancellation before serving, and listener failure. Lifecycle tests use loopback ephemeral ports and synchronized events; ordinary tests need no database and do not touch media. The explicit SONORA_TEST_DATABASE_URL opt-in enables the separate disposable PostgreSQL test; see [POSTGRESQL.md](POSTGRESQL.md).

Then inspect tracked and untracked changes and run `git diff --check` and `git status --short` from the repository root. Build/test caches are toolchain output; do not stage generated executables.
