# Sonora domain model

## Shared conventions

Opaque server-generated UUIDs and UTC timestamps. Catalog records are operator-managed, not listener-owned. Private records belong to one User. Titles/names are not globally unique. Foreign keys enforce references; no entity stores audio bytes.

This is the full logical model. Tasks 5–6 implement Artist/Album in [00002_artists_albums.sql](../backend/migrations/00002_artists_albums.sql) and Track/AudioAsset in [00003_tracks_audio_assets.sql](../backend/migrations/00003_tracks_audio_assets.sql); the other entities remain planned. Sessions are a supporting identity table (user, token hash, expiry). Required license provenance is structured AudioAsset metadata with references to retained evidence files.

## User

ID, normalized email, password hash, display name, created/updated timestamps. Trimmed/lowercased email is unique for this application's sign-in contract. Owns playlists, saved tracks, listening events and sessions. Operator-assisted account deletion revokes sessions and transactionally deletes owned private data; never catalog records.

## Artist

ID, name, optional biography, timestamps. Has many albums/tracks. MVP uses one primary artist per album/track; other contributor credits can be display text, without a collaboration graph. Names may repeat. Restrict hard deletion while referenced.

## Album

ID, primary artist ID, title, optional release date, timestamps. Has many tracks; titles may repeat. Public albums/artists are visible only when associated with active tracks. Optional artwork requires separately verified rights; omit otherwise. Restrict album deletion while tracks reference it; operator corrections must explicitly reassign tracks first.

## Implemented catalog schema decisions (Task 5)

- artists: explicit uuid id primary key, required text name, nullable text biography, required created_at/updated_at timestamptz.
- albums: explicit uuid id primary key, required primary_artist_id referencing artists.id, required text title, nullable date release_date, required created_at/updated_at timestamptz.
- UUIDs are supplied by the future server/operator code; no sequence, UUID extension or database-generated ID default. Tracks reference these independent keys without requiring track and album primary artists to match.
- Names/titles reject empty or ordinary-space-only strings using length(btrim(...)) > 0. They are not unique, including album titles within one artist. No arbitrary maximum length, slug, artwork, genre or partial-date representation is introduced. Broader Unicode whitespace normalization belongs to future input validation.
- Both timestamps default to CURRENT_TIMESTAMP on insert. timestamptz represents an absolute instant (UTC internally; display follows the session timezone). Writers must explicitly maintain updated_at on mutation; there are no update triggers.
- An album requires one existing primary artist. ON DELETE RESTRICT / ON UPDATE RESTRICT prevents deleting or changing a referenced artist ID; corrections reassign album.primary_artist_id explicitly. Task 6 restricts deletion of albums referenced by tracks.
- Primary keys create their own indexes. The sole additional index, albums_primary_artist_id_idx, supports artist album lookup and FK reference checks. No speculative search/title/date indexes.
- Unknown biography/release_date is NULL. A known release date uses PostgreSQL date; do not manufacture a month/day for a year-only source. Visibility based on active tracks remains future service behavior.

Rollback drops albums before artists, without CASCADE, and destroys their rows. The [disposable test procedure](MIGRATIONS.md) verifies rollback and reapply; it is not a data-preserving production recovery process.

Task 8 exposes these Artist/Album fields through explicit [read models](CATALOG_READS.md), retaining NULL biography/release dates. Internal repository reads and the Task 9 [development metadata HTTP API](CATALOG_API.md) do not implement the future public active-track visibility rule.

## Track

ID, primary artist ID, optional album ID, title, display credits, duration milliseconds, optional disc/track numbers, state (draft/active/unavailable), timestamps. Standalone singles have no album or position; album tracks require positive disc/track numbers unique within (album_id, disc_number, track_number). Track and album primary artists may differ.

At most one AudioAsset per Track in MVP. Activation requires positive measured duration and a ready asset with verified applicable rights. Services enforce this cross-record rule transactionally. Public discovery excludes draft/unavailable tracks. Private collections/history retain unavailable placeholders with playback disabled. Restrict hard deletion of referenced tracks; if metadata removal is required, retain an ID tombstone with only permitted minimal display information.

## AudioAsset

ID, unique track ID, unique generated storage key, checksum, MIME/codec, byte length, measured duration, file state (staged/ready/unavailable), license decision (pending/verified/rejected/withdrawn), timestamps.

Before verified: source provider/page/download reference; creator/rights-holder credits; acquisition time; license name/version/URL; preserved evidence reference; reviewer/date/rationale; exact permitted use/restrictions for hosting, streaming, redistribution, commercial use, audience/territory, expiry and modifications; required attribution text/links. Unknown permission blocks verification. Preserve evidence for the exact served bytes. A replacement file requires renewed validation and retained prior provenance.

Staged files are never served. Ready requires a complete validated file and verified rights. A recorded file failure, rejection or withdrawal makes the asset and track unavailable transactionally. Stream-time eligibility includes applicable expiry/use conditions. Keep provenance when removing media; disable access before purging and evaluate backup/retention obligations. Jamendo/FMA are candidates only; [PROJECT_PLAN.md](PROJECT_PLAN.md) owns the licensing gate.

## Implemented Track/AudioAsset decisions (Task 6)

### Tracks

tracks uses explicit UUID keys, required primary_artist_id/title/state and timestamps. display_credits and duration_ms are nullable for drafts or unavailable records; known duration must be positive, and active requires a known positive duration. State defaults to draft. Standalone tracks have NULL album_id/disc_number/track_number. Album tracks require all three with positive positions; the unique (album_id, disc_number, track_number) constraint allows repeated track titles and multiple singles. No relation requires the album and track artists to match.

Artist/album FKs restrict deletion and ID updates. The only extra track index is primary_artist_id; the album-position unique index already covers album reference checks. IDs/timestamps use the same conventions as Artist/Album; writers maintain updated_at explicitly.

### Audio files and availability

audio_assets has an explicit UUID primary key and unique required track_id (one per track) and storage_key. A key is a lowercase generated flat filename (letters/digits/underscore/hyphen, optional single extension), not a directory, absolute path or URL. Future file access must resolve it beneath private storage and handle symlinks safely. Bytes remain outside PostgreSQL.

Required technical metadata describes acquired bytes even while staged: lowercase 64-hex sha256, audio/* mime_type, nonblank codec, positive bigint byte_length/duration_ms, acquired_at and timestamps. Format strings are descriptive, not a codec decoder or browser-support guarantee. Checksum is not unique: identical bytes can legitimately appear in separate track/provenance records. A draft Track may exist before an AudioAsset is acquired.

file_state defaults to staged; license_decision defaults to pending. ready requires verified rights plus hosting_permitted/playback_permitted true. unavailable requires unavailable_at/reason and retains the same ID, track, storage key, checksum and provenance; other file states require those current-unavailability fields to be NULL. This represents missing/inaccessible bytes without falsely withdrawing the license. rejected decisions also require unavailable. withdrawn additionally requires withdrawn_at/withdrawal_reason and unavailable. Reinstatement is a future explicit re-review operation, not a deletion/reinsert; preserve prior evidence externally before changing decisions or replacing files.

Deleting or changing an asset-referenced Track is restricted. This migration introduces no metadata purge, cascades, soft-delete flag or audio blob. Unavailability is a state change, not SQL DELETE. Row deletion by a privileged operator remains technically possible; retention/purge policy is a future service responsibility.

### Asset-specific license provenance

Provenance columns may be NULL while pending, so unknown rights are never silently granted. A non-pending decision requires reviewed_by, reviewed_at and review_rationale. verified additionally requires nonblank source_provider/source_url/download_reference, creator_credits/rights_holder_credits, license_name/version/url, evidence_url and evidence_reference, plus audience_scope, territory_scope, license_term, use_conditions, attribution and modifications. Use an explicit applicable value such as “unversioned grant” or “no modifications”; do not fabricate a version or permission to fill a field. Evidence references point to retained files for the exact checksum, not provider-wide approval.

hosting_permitted, playback_permitted, redistribution_permitted and commercial_use_permitted are nullable booleans: NULL unknown, false prohibited, true allowed **within the recorded scope and conditions**. All four must be known for verified; redistribution/commercial use may remain false even for a ready asset. No license name or provider automatically sets any permission. license_expires_at may be NULL; license_term must explain no fixed expiry for verified records. Expiry and audience/territory/commercial suitability must be checked at use time, not with time-dependent CHECK constraints.

The fixture manifest's per-asset source/download, creator, license reference, evidence path, attribution, modifications, acquisition, size and hash map to these columns; license-level conditions can inform a recorded asset-specific review, never replace it. The manifest's fixture-only scope and pending public-release gate remain unchanged. No fixture rows are seeded, no files are acquired and no new rights decisions are made by this migration. Task 7 adds a separate [development seed](SEEDING.md), preserving this boundary: two artists, no invented albums, three draft/staged track/assets with the existing fixture-scope decision and explicitly estimated durations.

### Database versus future service responsibilities

CHECK constraints enforce local values/completeness and readiness/withdrawal combinations, not truth of evidence, file existence, legal applicability, immutability or historical retention. Cross-record activation (active Track has ready AudioAsset, matching measured duration and applicable rights) and withdrawing both rows must be performed by a future service in one transaction. There are no cross-table triggers. The live test demonstrates that transaction while retaining the asset's provenance, and deliberately uses synthetic permission text rather than claiming a real license.

The asset track/storage unique indexes cover their lookups/FK checks; no extra checksum/state/license indexes are justified yet. Rollback drops audio_assets before tracks without CASCADE and preserves artists/albums. It destroys new-table data and is exercised only in the disposable migration test.

Task 10 adds [Track and AudioAsset reads](TRACK_READS.md) without changing this schema. Track metadata retains NULLs and lifecycle state but excludes storage and private provenance. Internal asset inspection retains unavailable/withdrawn metadata; a separate playback candidate lookup rejects inactive tracks, non-ready/unverified/expired assets and inconsistent durations. Scope and file checks remain future stream-time responsibilities; no record is activated by reading it.

## Playlist

ID, owner user ID, name, positive monotonically increasing version, timestamps. Private in MVP; names may repeat even for one user. Owner-only reads/writes. Deletion cascades items; user deletion cascades playlists. Mutations require expected version, parent lock and transactional version increment; stale edits return 409.

## PlaylistItem

ID, playlist ID, track ID, nonnegative position, add-request ID, created timestamp. Unique (playlist_id, position) and (playlist_id, add_request_id). A track can intentionally appear multiple times; item IDs identify occurrences. An identical add retry returns the existing item without appending, before stale-version rejection. Conflicting key reuse returns 409. Deduplication lasts while the item exists; clients must not replay removed-item add requests.

Order is contiguous from zero after mutation: append on add, compact on remove, full item-ID permutation on reorder. Reject missing/duplicate/foreign item IDs atomically. Use deferred position uniqueness or a safe transactional rewrite to avoid temporary collisions. Withdrawal retains item position; new additions require active tracks, but unavailable items can be removed.

## SavedTrack

User ID, track ID, saved timestamp; composite primary key (user_id, track_id). Set semantics: repeated save preserves timestamp, repeated unsave succeeds. New saves require active tracks; existing references survive withdrawal and remain removable. Sort newest saved timestamp first, then track ID. User deletion cascades; referenced track deletion is restricted.

## ListeningEvent

ID, user ID, track ID, client playback ID, listened milliseconds, server recorded timestamp. Unique (user_id, client_playback_id). New client playback ID on explicit track start/replay; pause/resume and seek retain it. Switching away and restarting creates a new ID.

Record once when actual played time reaches min(30 seconds, half the track duration). Accumulate only while playing, excluding pause/buffering/seek jumps. Server requires an active track for a new event and validates the submitted duration is at least the threshold and no greater than track duration. Client timing is untrusted; this is personal history, not financial/royalty evidence.

An identical retry returns the existing event even after withdrawal; changed content under the same key returns 409. Retrying clients reuse the original threshold payload. Anonymous/short plays create none. Events are immutable; sort by server recorded timestamp descending, then ID. History represents qualifying plays, not audio HTTP requests.

Clear-history deletes the user's events and cancels that client's pending submissions. Concurrent other-tab submissions can create new events afterward; clearing also removes old deduplication keys. User deletion cascades; withdrawal retains unavailable history placeholders.

## Lifecycle consistency

Database constraints enforce local invariants; services enforce ownership, versions, license eligibility and transitions. Filesystem/SQL are not a distributed transaction; follow [ARCHITECTURE.md](ARCHITECTURE.md) staging/reconciliation. Availability is checked at stream time, so an old catalog response grants no access. Test duplicate requests, concurrent playlist changes and races with withdrawal.
