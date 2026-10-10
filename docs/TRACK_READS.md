# Track and AudioAsset repository reads (Task 10)

[tracks.go](../backend/internal/catalog/tracks.go) and [audio_assets.go](../backend/internal/catalog/audio_assets.go) extend the existing concrete catalog.Repository and borrowed *sql.DB pool. No new configuration, dependencies, schema, HTTP wiring or file operations are introduced.

## Track metadata

- Track(ctx, id) returns one Track or ErrNotFound.
- Tracks(ctx, Page, TrackFilter) returns a non-nil slice, including [] for no results.
- Track includes ID, PrimaryArtistID, nullable AlbumID, Title, nullable DisplayCredits/DurationMS/DiscNumber/TrackNumber, State and CreatedAt/UpdatedAt.

Pointers preserve SQL NULL; empty display credits remain distinct. Duration uses int64 milliseconds. Timestamps are time.Time instants; callers choose UTC formatting. Track contains no AudioAsset, storage key, checksum, source/download reference, evidence or reviewer data. Future public HTTP responses must use an explicit response model and enforce public visibility.

These metadata reads include draft, active and unavailable tracks, consistent with existing internal Artist/Album reads. They do not imply playability. Nil ArtistID/AlbumID filters mean no filter; supplied UUID strings are ANDed against tracks.primary_artist_id and tracks.album_id. The artist filter never infers an artist from the album: a track and its album can have different primary artists. An absent artist/album ID matches no rows. A supplied empty or malformed UUID is a query error, not an omitted filter. There is no separate standalone-only filter.

Lists reuse Page: zero limit defaults to 20, maximum 100, nonnegative offset. Invalid pages fail before SQL. Ordering is title COLLATE "C", then UUID ascending, including when filtering by album; this is metadata browsing, not an album playback sequence. Offset pagination is stable for unchanged data but is not a snapshot across separate calls.

## Internal assets and playback candidates

AudioAssetByTrack(ctx, trackID) returns the persisted internal asset, including staged, unavailable and withdrawn records. The explicit AudioAsset model includes all persisted technical, lifecycle and provenance fields. Nullable license permissions/metadata remain pointers: NULL never becomes permission. This model must not be serialized into a public response.

PlaybackAssetByTrack(ctx, trackID) returns an asset only if the single query's database snapshot satisfies all of:

- Track is active and its duration matches the asset's duration.
- Asset file_state is ready and license_decision is verified.
- HostingPermitted and PlaybackPermitted are explicitly true.
- LicenseExpiresAt is NULL or strictly later than PostgreSQL statement_timestamp(). Equality is expired.

The SQL computes eligibility together with the asset row. The method discards the entire asset on rejection; no partial storage metadata is returned on error. It performs no second eligibility query, writes or lifecycle changes. No client-supplied clock can bypass expiry.

Error contract, checked with errors.Is:

- Track lookup: ErrNotFound means no Track.
- Either asset lookup: ErrAudioAssetNotFound means no asset row for that track ID, including when the Track itself does not exist. It is deliberately distinct from ErrNotFound. Use Track lookup if the caller needs to establish that the parent exists.
- Playback lookup: ErrAudioAssetWithdrawn identifies a withdrawn asset and also matches ErrAudioAssetUnavailable.
- Playback lookup: ErrAudioAssetUnavailable covers all other failed eligibility conditions, including staged, rejected, unavailable or expired assets and inactive/mismatched Tracks.
- Database, invalid-UUID, scan and iteration errors remain errors, not empty successes or availability results. All failures return zero models/nil lists. Context cancellation/deadlines retain errors.Is semantics through the existing readError helper.

Both asset reads retain the Track relationship through a join. Every query is fixed parameterized SQL; no caller SQL fragments, ORM, repository interfaces or new indexes are added. List rows are closed and iteration errors checked. Callers provide deadlines, as with the existing repository.

## Eligibility limits

This is a database playback-candidate check, not authorization for every use. The returned audience/territory/use conditions, attribution, redistribution/commercial permissions and license term must be evaluated by the future streaming service for its actual audience and use. These free-text conditions cannot be safely inferred from a provider or license name. A verified fixture-only decision does not authorize public playback. Existing seeded fixtures remain draft/staged and fail the playback lookup; this task does not seed, activate, download or change them.

Ready metadata does not guarantee that a file exists, is readable, matches the checksum or is browser-compatible. Future delivery must safely resolve StorageKey under private storage, verify the actual file and recheck eligibility at use time. This read does not lock against subsequent withdrawal or promise cross-request consistency. No file is accessed by these methods or their tests.

## Verification

Run from backend/:

```powershell
go test ./...
go vet ./...
go build ./...
```

TestTrackRepositoryLive is opt-in. It refuses a nonempty public schema, applies current migrations and creates synthetic test records. Never run it against the local development catalog or concurrently with another fresh-database suite on the same database. Coverage includes empty/missing results; repeated titles/case and UUID ordering; default/maximum/out-of-range pagination; individual/combined/absent artist and album filters; nullable/empty metadata; staged/pending, ready, expired, rejected, withdrawn and unavailable assets; duration mismatch/inactive tracks; false/unknown permissions; SQL errors and closed pools; cancellation/deadlines under exclusive table locks and recovery. Synthetic license text grants no real media permissions.

Run this PowerShell procedure in a separate session from the repository root (Docker Desktop must be running). It generates temporary credentials and removes only the isolated Compose project:

```powershell
$smokeProject = 'sonora-tracks-' + [guid]::NewGuid().ToString('N').Substring(0,8)
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
        go test -count=1 -run '^TestTrackRepositoryLive$' -v ./internal/catalog
        if ($LASTEXITCODE -ne 0) { throw 'Track repository integration failed' }
    } finally { Pop-Location }
} finally {
    docker compose -p $smokeProject down --volumes --remove-orphans
    $cleanupExit = $LASTEXITCODE
    Remove-Item Env:SONORA_TEST_DATABASE_URL, Env:POSTGRES_PASSWORD
    if ($cleanupExit -ne 0) { throw 'Disposable cleanup failed' }
}
```

Expected result: PASS for TestTrackRepositoryLive, followed by removal of the disposable container, volume and network. The test deliberately changes synthetic rows and hides the disposable tracks table to exercise failure handling. It creates no audio files and requires no real licensed-media changes.
