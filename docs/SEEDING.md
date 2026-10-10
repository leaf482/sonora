# Development fixture catalog seeding

Task 7 provides one explicit local recipe, [cmd/seed](../backend/cmd/seed/), for the three existing reviewed fixtures. It is not production ingestion, a downloader, a migration, or a service. It uses the existing DATABASE_URL and DB_CONNECT_TIMEOUT and never runs automatically with the HTTP server.

## Run

Use a development database and set DATABASE_URL as described in [POSTGRESQL.md](POSTGRESQL.md). Keep the repository's fixture directory, evidence, license texts and credits intact. From the repository root start PostgreSQL, then run Go commands from backend/:

```powershell
docker compose up -d --wait --wait-timeout 90
Set-Location backend
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/seed development
go run ./cmd/seed development
```

Expected first result on a migrated, unseeded database:

```text
development fixtures: 3 new assets, 0 preserved; new tracks draft, files staged; storage .local/audio
```

The repeat reports 0 new assets, 3 preserved. There are two newly seeded artists (yesme and Kevin MacLeod), zero albums, three standalone tracks (Pong, Gustav Sting, Disco Sting), and three AudioAssets. Existing unrelated rows are retained, including an unrelated artist with the same name.

No albums, release dates, biographies or ownership guarantees are invented. Credits, source/download URLs, license name/version/URL, acquisition date, evidence reference, attribution, modifications, size, checksum and format come from the pinned manifest and its retained evidence. rights_holder_credits names the credited creator/publisher with an explicit ownership-warranty limitation, not a separate inferred rights holder.

The existing Task 1 fixture decision is recorded as verified with its original reviewer and date, not as a new independent verdict. Because only a review date is supplied, reviewed_at uses that date at 00:00 UTC and the rationale states day precision. Permission booleans apply only to these exact reviewed bytes and stated scope; license conditions and the pending production gate are retained. No permission is inferred from a provider name or applied to other audio.

Track and asset duration_ms use the manifest's frame-duration estimate rounded to milliseconds: 14040, 18260 and 16431. These are not claimed as decoded playback measurements. Tracks stay draft and files staged; no active/ready records are created because full decode/browser acceptance and actual release-scope review remain pending. This task populates a development catalog; it does not make it publicly playable.

## Storage and validation

The command reads ../fixtures/audio relative to backend/. The manifest's exact SHA-256 is pinned in the recipe; changes to title, permission, included status or file inventory cause a clear failure requiring review. Never refresh that pin merely to bypass a withdrawal or integrity failure. All three audio files plus all five retained evidence/license files must exist and match both size and SHA-256 before connecting/writing database records.

Reads are bounded and confined through Go os.Root; absolute/traversal paths, symlink components and non-regular files are rejected. The pinned original fixture files are read only.

Byte-identical local copies go to backend/.local/audio/<asset-uuid>.mp3, outside public/static directories and ignored by Git. storage_key contains only that generated filename; the private audio root for these records is backend/.local/audio. Stable UUIDs derive from a fixed recipe namespace plus entity kind and fixture ID (or credited artist name). Their algorithm and namespace must not be changed for reruns.

Complete temporary copies are synced, then published using a non-overwriting hard link within the private directory; the temporary name is removed. The local filesystem must support hard links (verified on this Windows workspace). Existing destination entries must match the approved bytes and are never overwritten. Preserve the original fixture README, manifest, evidence and license texts with any distribution: the private audio folder alone is not a complete distributable fixture bundle.

## Repeat and failure behavior

All database inserts run in one transaction with a transaction-scoped advisory lock for this recipe. Parameters are bound, and SQL only names the three explicit tables. There are no database UPDATE/DELETE paths. Existing matching IDs are checked against expected metadata/provenance; their state, timestamps and unrelated optional artist metadata are preserved. Other ID, track, storage-key or provenance conflicts fail without committing earlier inserts.

Unavailable Tracks, unavailable files, or assets with withdrawn/rejected/pending decisions are never reactivated, overwritten, or restored on disk. A changed review/provenance record may instead trigger a conservative metadata-conflict error; it is never replaced. Retain withdrawal metadata/tombstones. Deleting it discards the information the seed uses to recognize a withdrawn asset; do not delete it and then use this recipe to rebuild the record.

For an existing eligible fixture record with missing/corrupted private storage, the command fails rather than repairing it silently. Investigate the failure and follow [withdrawal policy](LICENSED_AUDIO.md); restoration/re-review is a separate explicit operation. A changed manifest likewise blocks seeding but does not itself update existing database states: the withdrawal procedure must disable those rows separately.

File publication and PostgreSQL commit are not a distributed transaction. A failed/aborted transaction can leave complete, verified unreferenced copies (or a temporary .seed-* file after abrupt termination). A retry may reuse an exact matching complete orphan for a new record. Nothing is published as active or ready. No automatic storage cleanup is performed; inspect unreferenced files before removing any. A commit-not-confirmed error requires inspecting the database; rerunning remains insert-only.

Malformed configuration, bad files, unavailable database, migration/schema errors and conflicts exit nonzero. Raw database/server errors and credentials are not printed. Database work has a one-minute context and follows the existing connection timeout. Ctrl+C cancels database work. This procedure makes no external requests.

## Verification

From backend/:

```powershell
go test ./...
go vet ./...
go build ./...
```

Ordinary tests verify fixture metadata, deterministic identities, missing/truncated/corrupt audio, missing evidence, changed metadata/withdrawn manifests, unsafe paths and command arguments. The optional symlink test skips only if the OS denies symlink creation.

The live test requires a fresh disposable PostgreSQL database, creates the existing migrations, and uses temporary storage/copies. It refuses any nonempty public schema. It checks fresh seed/repeat, provenance and file hashes, unrelated same-name data, a late conflict with transaction rollback, disabled-state preservation, and missing/corrupted file failures. It does not modify checked-in audio.

Run in a fresh PowerShell session from the repository root:

```powershell
$smokeProject = 'sonora-seed-' + [guid]::NewGuid().ToString('N').Substring(0, 8)
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
        go test -count=1 -run '^TestSeedLive$' -v ./cmd/seed
        if ($LASTEXITCODE -ne 0) { throw 'Seed live checks failed' }
    } finally { Pop-Location }
} finally {
    docker compose -p $smokeProject down --volumes --remove-orphans
    if ($LASTEXITCODE -ne 0) { throw "Cleanup failed: $smokeProject" }
    Remove-Item Env:SONORA_TEST_DATABASE_URL, Env:POSTGRES_PASSWORD
}
```

A host-port race can require rerunning with a new port. The down command removes only the unique disposable project; ordinary docker compose down preserves the development volume. Do not run the empty-database test against the persistent development catalog.

Verified on 2026-10-09 against PostgreSQL 17.11: all live checks above passed and the disposable container/network/volume were removed. New seed files are local copies; original licensed audio and retained evidence remain unchanged.
