# Sonora incremental roadmap

Only **Task 0 is IN PROGRESS**. All future tasks are **NOT STARTED**. Estimates are focused effort, roughly 30 minutes to a few hours, excluding external rights decisions/user commit time. Follow [DEVELOPMENT_WORKFLOW.md](DEVELOPMENT_WORKFLOW.md): one scoped task, independent read-only PASS/FIX review, user commits. Split oversized work before expanding scope.

## Foundation and feasibility

- **Task 0 — Foundation — IN PROGRESS — 1–2h.** Inspect repo and create the five foundation docs. Exit: scope/license/API/domain/workflow/roadmap agree, only requested docs changed, whitespace/status checks reported. No implementation or commits; independent review/user acceptance remains.
- **Task 1 — Technical bootstrap — NOT STARTED — 2–3h.** Depends on 0. Go module, Next.js/TypeScript, environment examples, local PostgreSQL, health endpoint and setup/check commands. Exit: fresh startup and format/build checks pass; no external audio.
- **Task 2 — Licensed-audio feasibility gate — NOT STARTED — 1–3h per research pass.** Depends on 0–1. Establish audience/use and evidence for at least three exact assets, hosting/streaming/redistribution permission, attribution placement, original format/browser feasibility, withdrawal and fixture rights. Jamendo/FMA are candidates only. Exit: evidence-backed PASS for specified assets/use or explicit BLOCKED findings. No candidate becomes playable before approval; external clarification may require another scoped research pass.
- **Task 3 — Schema and migrations — NOT STARTED — 2–3h.** Depends on 1; independent of gate outcome. Entities, supporting sessions, keys/checks/indexes and migration runner. Exit: fresh migration plus duplicate/reference constraint tests and reproducible integration setup. Synthetic metadata only.

## Backend slices

- **Task 4 — Identity/sessions — NOT STARTED — 2–4h.** Depends on 3. Register/login/logout/me, password hashing, expiring sessions, cookies/CSRF and auth throttling. Exit: positive/negative auth and revocation checks pass.
- **Task 5 — Verified catalog import/withdrawal — NOT STARTED — 2–4h.** Depends on 2 PASS and 3. Local operator commands, provenance, staged immutable media, activation and reconciliation. Exit: partial/unapproved assets cannot activate; interruption, missing files and withdrawal tested; no unauthorized media in Git.
- **Task 6 — Catalog/search API — NOT STARTED — 1–3h.** Depends on 5. Lists/details/search, attribution and bounded stable pagination. Exit: matching/filtering/validation tests pass; no private evidence/disk paths exposed.
- **Task 7 — Audio/Range delivery — NOT STARTED — 2–3h.** Depends on 5–6. GET/HEAD, ranges/validators, cancellation and eligibility. Exit: full/single/malformed/unsatisfiable/multipart behavior, missing-file errors and bounded memory verified.
- **Task 8 — Saved-track API — NOT STARTED — 1–2h.** Depends on 4 and 6. Private list/save/unsave. Exit: idempotency, two-user isolation and unavailable references tested.
- **Task 9 — Playlist CRUD/items — NOT STARTED — 2–3h.** Depends on 4 and 6. Private lifecycle, append/remove, versions, request deduplication. Exit: ownership, repeat-track semantics, retry safety and cascades verified.
- **Task 10 — Playlist reorder — NOT STARTED — 1–2h.** Depends on 9. Atomic complete item permutation under lock/version check. Exit: concurrent edits conflict safely; bad/foreign IDs leave state unchanged.
- **Task 11 — History API — NOT STARTED — 1–2h.** Depends on 4 and 6. Threshold validation, event deduplication, recent list and clear. Exit: duplicate/conflicting retries, isolation, withdrawal and clear behavior tested.

## Consumer experience

- **Task 12 — Responsive shell/account UI — NOT STARTED — 2–3h.** Depends on 4. Navigation, auth flows, shared loading/error/focus patterns. Exit: keyboard/360px/1280px checks pass; sign-out clears private state.
- **Task 13 — Catalog/search/credits UI — NOT STARTED — 2–3h.** Depends on 6 and 12. Browse/details/search with attribution and empty/error states. Exit: discover a track and inspect credits on mobile/desktop.
- **Task 14 — Persistent player — NOT STARTED — 2–4h.** Depends on 7 and 13. Single audio element, play/pause/seek/time, switch handling and failures. Exit: navigation preserves playback, logout stops it, stale events/seek work and controls are accessible.
- **Task 15 — Saved library UI — NOT STARTED — 1–2h.** Depends on 8 and 14. Save/unsave, library and unavailable placeholders. Exit: refresh persistence and mutation failure recovery verified with two users.
- **Task 16 — Playlist UI — NOT STARTED — 2–3h.** Depends on 10 and 14. Lifecycle/items, accessible move controls, play item and conflict recovery. Exit: persisted order, repeated occurrences and stale refresh behavior verified.
- **Task 17 — History integration — NOT STARTED — 1–3h.** Depends on 11 and 14. Actual listened-time tracking, retry-safe threshold submission, screen and clear. Exit: seek/pause/buffering excluded, one event per qualifying play, no short/anonymous events.

## Validation and delivery

- **Task 18 — MVP integration testing — NOT STARTED — 2–4h.** Depends on 15–17. Critical browser journeys, PostgreSQL/media failure and restart tests. Exit: evidence against all MVP criteria including browsers/two-user isolation; failures become scoped fixes before proceeding.
- **Task 19 — Focused refactor — NOT STARTED — 1–3h.** Depends on 18. Address concrete duplication/coupling exposed by flows without behavior changes. Exit: regression checks pass; if none justified, record review and avoid speculative abstractions.
- **Task 20 — UX/accessibility polish — NOT STARTED — 2–3h.** Depends on 19. Hierarchy, copy, states, mobile player, focus/keyboard flow. Exit: mobile/desktop and manual accessibility checklist passes; no new features.
- **Task 21 — Security audit/fixes — NOT STARTED — 2–4h.** Depends on 20. Auth/CSRF, ownership, paths/inputs, secrets/dependencies, privacy and withdrawal. Exit: no known high-severity issues, relevant regression checks and recorded residual risks; split large fixes.
- **Task 22 — Performance/recovery audit — NOT STARTED — 2–3h.** Depends on 21. Measure API/audio targets, query plans, limits, stream memory/cancellation, failures and backup/restore including evidence. Exit: reproducible results/recovery instructions; optimize measured problems without speculative infrastructure.
- **Task 23 — Portfolio delivery — NOT STARTED — 1–3h.** Depends on 22 and rights approval for delivery audience. Setup/demo, architecture explanation, screenshots, credits, limitations, acceptance evidence and single-process/filesystem runbook. Exit: reproducible demo, approved catalog, independent PASS and user acceptance. Public deployment needs applicable rights and explicit deployment scope; media upload is not an assumed portfolio entitlement.

## Dependency rule

Roadmap order is default. If Task 2 is blocked, PM may select explicitly independent work (for example 3–4 and 12 with synthetic fixtures); do not begin dependent ingestion/playback or mark the gate passed. Scope inserted fixes/research separately and update dependencies. Future tasks remain NOT STARTED until selected.
