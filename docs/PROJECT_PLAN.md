# Sonora project plan

## Purpose and baseline

Sonora is an original music streaming and discovery web app, with consumer-quality UX and a small, understandable backend. Discovery means browsing and searching a curated licensed catalog, not personalized recommendations or a Spotify replica.

Task 0 found an empty repository containing only Git metadata, with no commits or implementation. These documents describe planned behavior. See [architecture](ARCHITECTURE.md), [domain](DOMAIN_MODEL.md), [workflow](DEVELOPMENT_WORKFLOW.md), and [roadmap](TASKS.md).

## MVP

- A small operator-curated, licensed catalog with artist/album/track details and visible credits.
- Browser audio play/pause/seek, time/duration, loading/error states, and a persistent player across client-side navigation. One track at a time; explicit user action starts playback.
- Search track titles, artist names and album titles, with bounded pagination.
- Registration, sign-in/out, private saved-track library, and persistent save/unsave.
- Private playlists: create, rename, delete, add/remove tracks, reorder items and play an item.
- Private listening history for qualifying plays, with a clear-history action.
- Responsive mobile/desktop UI, keyboard operation, visible focus, and useful empty/loading/failure/unavailable states.

## Non-goals

Spotify/YouTube ripping; unlicensed audio; ML recommendations; Kafka; Redis; Kubernetes (K8s); microservices; HLS; CDN delivery; audio blobs in PostgreSQL. Also deferred: public/collaborative playlists, social features, payments, offline downloads, user uploads, transcoding pipelines, gapless playback and cross-device playback synchronization. Streaming still delivers audio bytes and needs applicable permission even without a download button.

## Audio policy and early feasibility gate

Only assets with verified applicable licenses and documented attribution may enter the playable catalog. Jamendo and Free Music Archive (FMA) are candidates, **not preapproved ingestion sources**. A free download or provider-wide label does not establish Sonora's permissions. Task 0 approved no source, individual track, or legal interpretation. Task 1 records a narrowly scoped fixture decision in [LICENSED_AUDIO.md](LICENSED_AUDIO.md); it does not approve a provider catalog or complete the public-release gate.

Task 1 must record evidence for the exact assets and intended deployment:

- Source provider, original page/download reference, creator and relevant rights-holder credits, acquisition time, and checksum of the exact file.
- License name/version/URL, retained permission evidence, verification date, reviewer and rationale.
- Permitted usage and conditions for local hosting, browser streaming, redistribution, commercial use, audience/territory, expiry, and modifications/transcoding. Unknown permissions remain unknown.
- Required attribution text, links and placement, redistribution obligations, and separate artwork rights. Omit unverified artwork.

Gate PASS requires at least three verified tracks, a browser-compatible original format, documented attribution placement, and a withdrawal procedure for the intended deployment. If evidence remains unresolved, mark the gate BLOCKED; proceed only with independent work using silence or self-created audio with documented rights. Test fixtures do not replace a licensed MVP catalog. Do not ingest candidate audio into playable storage or launch streaming while blocked.

Open decisions: eligible individual assets/sources; public/private deployment audience and territories; portfolio/commercial-use permissions; scope of underlying music and recording rights; provider access terms; attribution and redistribution obligations; expiry; and whether fixtures/media may be redistributed in Git. Obtain qualified rights clarification where evidence is ambiguous. Do not commit third-party media by default. This document sets a verification policy, not a claim about any provider's current legal terms.

## Main flows

1. Visitor browses/searches active catalog entries, opens details/credits and starts audio. Playback persists across navigation; seek uses byte ranges.
2. Listener signs in to save tracks or manage private playlists. Mutations persist across refresh. Sign-out clears private frontend state and stops playback so later listening is not attributed to the former user.
3. Listener adds playlist items and reorders them. Version conflicts require refresh rather than silently overwriting concurrent edits.
4. Actual listening time reaches the threshold defined in DOMAIN_MODEL; one deduplicated history event is submitted. Anonymous/short plays create no history.
5. Operator verifies rights, stages and validates a file, then activates its metadata through a local import command. Withdrawal stops new streams and leaves unavailable placeholders in private collections.

## MVP acceptance criteria

- Task 1 is PASS for the actual audience/use; at least three approved tracks play with retained provenance and visible required attribution. Unapproved/withdrawn media cannot start a new stream.
- Fresh setup, migrations, approved import, and Go/Next.js/PostgreSQL startup are reproducible from instructions.
- Play/pause and forward/backward seek work in current stable Chrome, Firefox and Safari where verification is available; explicitly report untested browsers. Range semantics have integration coverage. Network/media errors are recoverable.
- Search returns matching tracks/artists/albums with bounded pagination and useful empty states; public discovery excludes unavailable tracks.
- Two accounts demonstrate persistent library/playlists, correct item ordering, conflict handling and isolation of all private reads/writes.
- Exactly one event per qualifying client playback ID despite retries; short/anonymous plays record none; history clears.
- Core flows work at 360px and 1280px widths without horizontal page overflow. Playback, seek, library and playlist ordering work by keyboard with named controls and visible focus.
- Domain/API/integration and critical browser journeys pass, including withdrawal, missing-file, authorization and restart cases. No known high-severity security findings remain.
- Metadata/media/evidence backup and restore are exercised. Portfolio documentation includes setup, demonstration, credits, limitations and acceptance evidence.

Provisional performance targets: on a documented local reference environment with 1,000 synthetic metadata tracks and 10 concurrent clients, non-audio API p95 below 300ms and local audio start below 2 seconds. Record hardware, dataset and results before claiming targets. Audio delivery uses bounded memory, not whole-file buffering.
