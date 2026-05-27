# Agent Instructions

## Working Goal

Keep this repository aligned with its current Go backend architecture: PostgreSQL-backed REST endpoints for authoritative state changes and WebSocket broadcasts for live updates.

## How To Identify The Layer You Are Editing

| Path Pattern | Layer | What It Usually Means |
| --- | --- | --- |
| `internal/core/**` | Core | Cross-cutting runtime concerns |
| `internal/router.go` and `internal/features/*/routers/**` | Routers | Route registration and dependency composition |
| `internal/features/*/handlers/**` | Handlers | HTTP or WebSocket edge adapters |
| `internal/features/*/services/**` | Services | Business orchestration and transactions |
| `internal/features/*/repositories/**` | Repositories | GORM persistence logic |
| `internal/features/*/models/**` | Models | Database mappings |
| `internal/features/*/schemas/**`, `internal/shared/schemas/**` | Schemas | DTOs and shared API response contracts |
| `internal/features/realtime/hub/**` | Realtime Hub | Channel-driven WebSocket state |

If a file touches more than one concern, default to the narrowest layer that already owns the behavior. If ownership is unclear, prefer preserving the current boundary instead of inventing a new one.

## Preferred Working Style

- Read the nearby feature files before editing to confirm the local pattern.
- Preserve the existing flow: parse in handlers, decide in services, persist in repositories, map tables in models.
- Prefer small, composable changes over broad refactors.
- Reuse shared helpers and response wrappers before adding new abstractions.
- Keep code and docs synchronized when behavior changes.
- Treat docs as contract files, not changelogs.
- Prefer Go standard-library patterns unless the repo already standardized on a stronger abstraction.
- Design concurrency deliberately: ownership, cancellation, and cleanup must be obvious.
- Mark unknown product decisions as `[PERLU DIISI]` in docs instead of making them up.

## Mandatory Self-Check Before Finishing

- Did I edit the correct layer for this behavior?
- Did I avoid pushing business logic into routers, handlers, or models?
- Did I preserve `exceptions.AppError`-based error handling?
- Did I keep response payloads consistent with `internal/shared/schemas/response.go`?
- Did I keep commentary write behavior transactional if match state can change?
- Did I avoid pre-commit WebSocket broadcasts?
- Did I preserve `context.Context` propagation through request-scoped code?
- If I introduced concurrency, does each goroutine have a lifecycle, cancellation path, and cleanup strategy?
- If I touched concurrency-sensitive code, should `go test -race ./...` be part of verification?
- Did I update `.ai-context/docs/*.md` if architecture, domain rules, operations, or invariants changed?
- Did I leave `[PERLU DIISI]` on any requirement that is still unknown?

## Repo-Specific Guardrails

- `matches` and `commentary` are the core persisted entities.
- Match status is time-derived, not free-form.
- WebSocket hub state must remain channel-driven to avoid concurrent map writes.
- Request-scoped code should reuse the incoming request context instead of creating detached background contexts.
- Detached background work is discouraged until the project has a documented worker/outbox strategy. `[PERLU DIISI]`
- Current storage scope is PostgreSQL only. `[PERLU DIISI]` Add storage rules here if file/object storage is introduced.
- Current auth scope is undefined. `[PERLU DIISI]` Add auth and authorization rules before protected routes are introduced.

## Go-Specific Expectations

- `context.Context` is first parameter for request-scoped methods outside handlers.
- Errors should be wrapped with operational context before conversion into transport-safe errors.
- Table-driven tests are preferred for service and validation logic.
- `httptest` is preferred for handler-level tests.
- `go test -race ./...` is expected for changes touching goroutines, channels, shared state, or the WebSocket hub.
- Benchmarks are recommended only for genuinely hot paths or optimization work, not every change.

## When To Escalate In Documentation

Update the docs in `.ai-context/docs/` when you change:

- route topology
- layer ownership
- environment variables
- domain invariants
- response contracts
- WebSocket event semantics
- deployment or runtime behavior

Do not update docs for implementation-only changes that preserve the same contract.

## Review Commands

```bash
make review
make docs-check
make docs-update
make agent-check
```

- Run `make review` after code updates.
- Run `make docs-update` to see which docs deserve review based on changed files.
- Run `make docs-check` after any doc edits.
- Run `make agent-check` before handing off larger changes.
