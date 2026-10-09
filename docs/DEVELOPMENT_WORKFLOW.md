# Sonora development workflow

## Delivery loop

**PM -> Codex Dev -> independent read-only Reviewer -> user commits.** One scoped task at a time. [TASKS.md](TASKS.md) is the status source. Only the user-selected task is actively in progress. Delivered work with no recorded reviewer verdict is awaiting review, not an inferred PASS; planned features are not completed work.

1. **PM:** select a task with satisfied dependencies and provide objective, scope, non-goals, acceptance criteria, documents and checks. Split work exceeding a few hours before expanding implementation. Identify unresolved licensing/product decisions.
2. **Codex Dev:** inspect instructions, status and existing work; preserve unrelated edits; implement only selected scope and update relevant docs. Run proportionate checks and report changed files, results, limitations, status and suggested commit message. Do not stage or commit.
3. **Independent read-only Reviewer:** use a separate person/context from the implementing developer. Inspect the brief, full diff (including untracked files), surrounding code/docs and evidence. Do not edit, fix, stage, commit or change task status. Read-only checks are allowed; report unavailable validation. Developer self-checks are not independent review.
4. **Verdict PASS/FIX:** PASS means the scoped criteria are met without blocking findings, not that future tasks are complete. FIX lists actionable findings with file/line, impact and expected correction; separate optional suggestions. Missing required validation blocks PASS unless PM explicitly revises the criteria.
5. **Correction and user commit:** on FIX, Dev corrects the same scope, reruns affected checks and returns to review. On PASS, user inspects and commits. Agents do not stage/commit or automatically start the next task. Mark DONE only after independent PASS and user acceptance; record review/commit evidence when available.

## Brief and handoff

Task brief: ID/title, objective, dependencies, allowed scope, exclusions, acceptance criteria, validation commands/manual scenarios and open decisions.

Dev handoff: changed files; behavior/design summary; exact checks with pass/fail/not-run reasons; remaining risks; git status --short; suggested commit message. A documentation task does not imply implementation/tests exist.

Reviewer receives the brief, complete change including untracked content, baseline status and check output. Review correctness, isolation/authorization, concurrency/idempotency, license implications, failure behavior, accessibility when relevant and foundation consistency. Do not require unrelated refactors/infrastructure to pass a scoped task.

## Verification conventions

- Capture git status --short before/after; inspect tracked and untracked changes. Never discard unrelated work.
- Always run git diff --check and git status --short. Diff excludes untracked documents; inspect and whitespace-check them separately without staging.
- For docs: confirm required files exist, relative links resolve, terminology/API/domain rules agree and statuses are accurate. Task 0 needs no application build.
- Bootstrap establishes exact formatting/lint/typecheck/test commands and records them here. Until then, do not claim commands/tests exist. Later tasks run appropriate Go/frontend/PostgreSQL/browser checks.
- Use disposable databases/storage and rights-documented fixtures; no destructive verification against user/production data. Keep secrets, private data and unapproved media out of reports/commits.
- A blocked audio gate prevents dependent ingestion/public playback; only explicitly independent work may continue. Record missing evidence; never invent permission.

## Status meanings

NOT STARTED: no work begun. IN PROGRESS: the single selected task, including development/review. AWAITING REVIEW: work is delivered, but the independent verdict is not recorded. BLOCKED: a named dependency/decision prevents acceptance. DONE: independent PASS and user acceptance. Update deliberately.
