# AGENTS

## Mission

### Project Identity

| Item | Value |
| --- | --- |
| Project Name | Real-time Sports Dashboard Backend (`sports-dashboard`) |
| Short Description | A Go backend that exposes REST endpoints for match state management and a WebSocket endpoint for live sports updates and commentary. |
| Primary Mission | Keep match state authoritative in PostgreSQL, expose consistent HTTP APIs, and broadcast real-time match/commentary updates safely over WebSocket. |
| Primary Language | Go 1.22 |
| Architecture Style | Feature-based architecture / DDD-lite / modular monolith |

### Success Criteria

- REST endpoints remain the source for match creation and commentary writes.
- PostgreSQL remains the source of truth for match and commentary state.
- WebSocket broadcasts happen only after successful state persistence.
- Layer boundaries stay explicit across `routers`, `handlers`, `services`, `repositories`, `models`, `schemas`, `core`, and `realtime/hub`.

## Critical Rules

- Do not bypass the established flow: `middleware -> router -> handler -> service -> repository -> database`.
- Do not place business rules in routers or handlers.
- Do not access GORM directly from handlers.
- Do not write ad-hoc JSON response shapes. Use `internal/shared/schemas.Success`, `SuccessWithMeta`, or `Error`.
- Do not return raw internal errors to clients. Convert them into `exceptions.AppError` and let middleware format the response.
- Do not broadcast match or commentary events before the database transaction commits successfully.
- Do not introduce cross-layer shortcuts unless the change is explicitly documented in `architecture.md` and reflected in layer instructions.
- Keep request sanitization and validation at the edge of the system, then re-check domain invariants in services when required.
- Preserve safe WebSocket concurrency patterns. Hub state mutations must remain channel-driven.
- Any missing product, auth, AI, cache, queue, or deployment requirement not present in the codebase must stay marked as `[PERLU DIISI]`.
- Always accept and propagate `context.Context` as the first parameter for request-scoped service and repository operations.
- Do not replace request context with `context.Background()` in the synchronous request path.
- Any new goroutine must have a clear owner, exit condition, and cancellation/shutdown strategy.
- Prefer communication by channels over shared mutable state. If shared mutable state is required, protect it explicitly.
- Wrap errors with context for internal diagnosis, but keep client-facing responses sanitized through `exceptions.AppError`.
- Favor production-safe defaults: graceful shutdown, bounded concurrency, connection pooling, structured logging, and explicit validation.

## Documentation Update Policy

### Update Docs Only When A Contract Changes

Documentation should stay stable. Do not edit `AGENTS.md` or `.ai-context/` for every small refactor or bug fix.

### Docs Must Be Updated When

- API request or response contracts change.
- Validation rules or domain invariants change.
- Route topology or layer ownership changes.
- WebSocket event semantics or room behavior changes.
- Runtime commands, environment variables, middleware, or deployment behavior changes.
- A previously unknown `[PERLU DIISI]` item becomes a decided standard.

### Docs Usually Do Not Need Updates When

- Internal implementation changes but behavior stays the same.
- Code is cleaned up without changing contracts or boundaries.
- Tests are added without changing runtime behavior.
- Local refactors preserve the same architecture and operational model.

## Go Backend Best Practices

### Context And Cancellation

- Pass `context.Context` as the first argument in request-scoped service and repository functions.
- Propagate request context from handlers into services and repositories without dropping cancellation or deadlines.
- Use `context.WithTimeout` or `context.WithDeadline` for long-running operations when the timeout policy is explicit.
- Only use `context.Background()` for truly detached background work, and document the reason.
- Check `ctx.Done()` in long-running loops, fan-out work, streaming, or worker-like flows.

### Goroutines And Channels

- Do not start a goroutine unless its lifecycle is obvious and bounded.
- Every goroutine must be cancelable, naturally terminating, or owned by a long-lived component such as the WebSocket hub.
- Avoid goroutine leaks caused by blocking sends, blocking receives, or abandoned workers.
- Close channels from the sender side only.
- Prefer buffered channels when they prevent avoidable blocking and leak risk.
- Use `select` for cancellation-aware channel operations in long-lived concurrent code.
- Do not mutate shared maps from multiple goroutines without synchronization.

### HTTP And Middleware

- Keep handlers thin and deterministic.
- Validate and sanitize input at the boundary before domain orchestration.
- Return consistent response envelopes through shared helpers.
- Sanitize panic and internal error output before returning anything to clients.
- Middleware must stay cross-cutting and must not absorb feature-level domain rules.

### Database And Transactions

- Keep database access behind repositories by default.
- Use transactions for multi-step state changes that must commit atomically.
- Use row locking when concurrent requests can mutate the same authoritative record.
- Respect connection pooling and avoid creating uncontrolled parallel DB load.
- `[PERLU DIISI]` Define repository timeout standards if the team wants hard query timeout rules beyond request-context cancellation.

### Errors And Logging

- Prefer returning errors instead of panicking.
- Preserve causal chains with wrapped errors for internal observability.
- Use typed or categorized errors when callers need programmatic branching.
- Keep logs structured and include stable fields such as request ID, path, status, and duration where relevant.

### Testing And Verification

- Business logic should default to table-driven tests.
- HTTP behavior should be covered with `httptest`-style handler tests where practical.
- Concurrency-sensitive changes should be validated with `go test -race ./...`.
- Performance-sensitive changes should justify benchmarks before optimization claims are accepted.
- Definition of done is not satisfied when concurrency, cancellation, or transaction behavior changed but no verification plan was considered.

## Layer Boundaries

### Layer Map

| Layer | Primary Paths | Responsibilities |
| --- | --- | --- |
| Core | `internal/core/**` | Config loading, DB bootstrap, middleware, security helpers, global error handling |
| Routers | `internal/router.go`, `internal/features/*/routers/**` | Route registration, route grouping, dependency wiring at composition root |
| Handlers | `internal/features/*/handlers/**` | HTTP/WebSocket request parsing, validation handoff, service calls, response emission |
| Services | `internal/features/*/services/**` | Business orchestration, transactions, status derivation, post-commit event triggering |
| Repositories | `internal/features/*/repositories/**` | GORM queries and persistence helpers |
| Models | `internal/features/*/models/**` | Database table mappings |
| Schemas | `internal/features/*/schemas/**`, `internal/shared/schemas/**` | Request/response DTOs and shared response contracts |
| Realtime Hub | `internal/features/realtime/hub/**` | WebSocket client lifecycle, room subscription, channel-safe broadcasting |

### Core

- Allowed:
  - Load environment-driven configuration.
  - Initialize database connections and pool settings.
  - Implement cross-cutting middleware, validation, sanitization, and error handling.
  - Enforce body limits, CORS, origin checks, panic recovery, and rate limiting.
- Forbidden:
  - Feature-specific business decisions for matches or commentary.
  - Feature route registration outside composition concerns.
  - Direct mutation of match/commentary domain rules.

### Routers

- Allowed:
  - Register routes and route groups.
  - Attach handlers to paths.
  - Wire dependencies in `internal/router.go`.
  - Apply middleware at the engine/group level through composition root changes.
- Forbidden:
  - Run database queries.
  - Implement business logic.
  - Build response payloads directly.
  - Broadcast WebSocket events.

### Handlers

- Allowed:
  - Parse params, query strings, and JSON bodies.
  - Trigger Gin binding validation.
  - Convert parsing/binding failures into `exceptions.AppError`.
  - Call exactly the needed service method.
  - Return responses through shared response helpers.
- Forbidden:
  - GORM queries or transactions.
  - Business rule branching such as score update policy or match status decisions.
  - Direct WebSocket hub mutations.
  - Ad-hoc JSON response formatting.

### Services

- Allowed:
  - Enforce business invariants.
  - Sanitize domain input before persistence.
  - Orchestrate transactions and row locks when atomic writes are required.
  - Derive match status from time windows.
  - Trigger WebSocket broadcasts only after successful persistence.
  - Coordinate bounded asynchronous work only when ownership, cancellation, and consistency rules are explicit.
- Forbidden:
  - Emit HTTP responses directly.
  - Parse Gin route params or bind request bodies.
  - Register routes.
  - Hide persistence logic inside unrelated helpers.
  - Spawn detached goroutines in the request path without a documented lifecycle and failure strategy.

### Repositories

- Allowed:
  - Perform GORM `Create`, `Find`, `Where`, `Order`, and transaction-scoped persistence operations.
  - Encapsulate repeated query patterns.
  - Return `nil, nil` for not-found when that is the established contract.
- Forbidden:
  - Business rules such as status derivation, score semantics, or event broadcasting.
  - HTTP concerns.
  - Response formatting.

### Models

- Allowed:
  - Declare table mappings, GORM tags, and `TableName()` overrides.
- Forbidden:
  - Request validation.
  - Business orchestration.
  - HTTP or WebSocket concerns.

### Schemas

- Allowed:
  - Define request DTOs, response DTOs, and binding tags.
  - Define shared response wrappers.
- Forbidden:
  - GORM access.
  - Business rules.
  - Route registration.

### Realtime Hub

- Allowed:
  - Own room membership, subscription state, client registration, and broadcast fan-out.
  - Enforce channel-based concurrency.
  - Remove slow or disconnected clients safely.
  - Maintain long-lived goroutines only as part of explicit component ownership.
- Forbidden:
  - Database access.
  - Match/commentary business validation.
  - HTTP request parsing.
  - Unsynchronized mutable state access outside the hub event loop.

## Core Domain Rules

### Main Entities

| Entity | Purpose | Notes |
| --- | --- | --- |
| `users` | Auth identity and token-version authority | Stores email, password hash, display name, and access-token invalidation version |
| `refresh_sessions` | Device-scoped refresh token authority | Stores hashed refresh tokens, rotation family, revocation timestamps, and device metadata |
| `matches` | Authoritative match state | Stores teams, scores, status, timing, and JSON metadata |
| `commentary` | Time-ordered commentary events per match | May carry score updates inside JSON payload |

### Status Flow

`scheduled -> live -> finished`

- Status is derived from `start_time` and `end_time`.
- `scheduled` when `now < start_time`.
- `finished` when `now > end_time`.
- `live` otherwise.

### Critical Business Rules

- A match must exist before commentary can be listed or created for it.
- `home_score` and `away_score` must never be negative.
- Commentary creation is transactional.
- Commentary writes may update match scores when payload includes non-negative `homeScore` and/or `awayScore`.
- Match row updates during commentary creation must be protected with `SELECT ... FOR UPDATE` semantics via GORM locking.
- WebSocket events `commentary.created` and `match.updated` must be emitted only after the transaction completes successfully.
- Access tokens are short-lived and currently default to `15` minutes.
- Refresh tokens are long-lived and currently default to `30` days.
- Access tokens are returned in JSON responses.
- Refresh tokens are issued only through HTTP-only cookies and persisted only as hashes.
- Refresh-token rotation must revoke the previous session and create a replacement session in the same `familyId`.
- Reuse of a revoked refresh token is treated as token theft, requiring family revocation and `tokenVersion` increment.
- `sport` input must resolve to a safe slug.
- User-facing strings are currently hardcoded in English. `[PERLU DIISI]` Add an i18n/localization policy if multilingual output is required.

### Naming Conventions

- Go packages use lowercase directory names.
- Exported Go identifiers use PascalCase.
- Unexported identifiers use camelCase.
- JSON fields use camelCase.
- SQL table names are lowercase snake_case plural or singular according to the current schema:
  - `users`
  - `refresh_sessions`
  - `matches`
  - `commentary`

## Security & Operations

### Security Posture

| Concern | Current State |
| --- | --- |
| Authentication | JWT access tokens plus refresh-token rotation backed by hashed refresh-session records |
| Input Validation | Gin binding + `go-playground/validator/v10` with custom validators |
| Sanitization | Trimmed strings and sanitized slugs via `internal/core/security` |
| CORS | Allowlist-based, driven by `ALLOWED_ORIGINS` |
| Rate Limiting | In-memory per-IP token bucket middleware |
| Request Body Limit | 5 MB middleware cap |
| WebSocket Origin Check | Enforced in the Gorilla upgrader |

### Operational Rules

- Configure runtime through environment variables loaded by `internal/core/config`.
- Keep secrets out of docs and repo-tracked `.ai-context` files.
- PostgreSQL is the only persistent storage currently implemented.
- `[PERLU DIISI]` No file/object storage strategy is implemented.
- Logging uses `log/slog` plus GORM logger output.
- `[PERLU DIISI]` No audit snapshot trail or mutation history store is implemented.
- Local infrastructure is defined in `docker-compose.yml`.
- Graceful shutdown must remain intact in `cmd/server/main.go`.
- Concurrency controls must remain explicit for any long-lived goroutine, worker loop, or channel topology introduced later.
- Production behavior should remain observable through structured logs, health checks, and deterministic shutdown.

## AI Context Map

| File | Purpose |
| --- | --- |
| `.ai-context/agent-rules/agent-instructions.md` | Working protocol for agents in this repository |
| `.ai-context/agent-rules/instructions/core.instructions.md` | Rules for config, middleware, security, errors, and database bootstrap |
| `.ai-context/agent-rules/instructions/routers.instructions.md` | Rules for route registration and composition root work |
| `.ai-context/agent-rules/instructions/handlers.instructions.md` | Rules for HTTP/WebSocket edge handling |
| `.ai-context/agent-rules/instructions/services.instructions.md` | Rules for business orchestration and transactions |
| `.ai-context/agent-rules/instructions/repositories.instructions.md` | Rules for persistence code |
| `.ai-context/agent-rules/instructions/models.instructions.md` | Rules for GORM models |
| `.ai-context/agent-rules/instructions/schemas.instructions.md` | Rules for DTOs and shared response schemas |
| `.ai-context/agent-rules/instructions/realtime-hub.instructions.md` | Rules for channel-safe WebSocket hub code |
| `.ai-context/docs/architecture.md` | Tech stack, structure, architecture, coding standards |
| `.ai-context/docs/domain-rules.md` | Entities, status flow, domain invariants, API contracts |
| `.ai-context/docs/operations.md` | Security, env vars, logging, storage, commands, deployment notes |
| `.ai-context/docs/agent-alignment-sprint.md` | Remediation backlog and execution plan for bringing code into agent-rule alignment |
| `.ai-context/docs/feature-test-sprint.md` | End-to-end testing backlog and per-feature verification sprint plan |

`ai-architecture.md` is intentionally omitted because no AI/ML pipeline or background worker exists in the current codebase. `[PERLU DIISI]` Add it when AI or async job infrastructure is introduced.

## Maintenance Commands

```bash
make review
make docs-check
make docs-update
make agent-check
```

- `make review`: checks Go formatting, runs `go vet`, runs tests, and validates agent/doc consistency.
- `make docs-check`: verifies required agent-context files, required `AGENTS.md` sections, instruction `applyTo` frontmatter, and unresolved template placeholders.
- `make docs-update`: inspects changed files and suggests which docs should be reviewed, so docs are updated only when truly needed.
- `make agent-check`: runs the full review plus the targeted docs-update suggestion flow.

## Reference Docs

- `README.md`
- `go.mod`
- `Makefile`
- `docker-compose.yml`
- `cmd/server/main.go`
- `internal/router.go`
- `internal/core/config/config.go`
- `internal/core/database/postgres.go`
- `internal/core/security/validator.go`
- `internal/core/security/sanitizer.go`
- `internal/core/exceptions/*.go`
- `internal/shared/schemas/response.go`
- `migrations/*.sql`

## Definition of Done

- The changed code respects the existing layer boundaries documented above.
- Handlers keep using shared response wrappers and `c.Error(...)` for failures.
- Services own business rules, transactions, and post-commit event triggering.
- Repositories remain the default location for reusable query logic.
- Match and commentary invariants remain enforced:
  - no negative scores
  - commentary cannot target a missing match
  - broadcasts happen only after persistence succeeds
- Any new WebSocket behavior preserves channel-safe hub ownership.
- Any new request-scoped logic preserves context propagation through the call chain.
- Any introduced goroutine has an exit strategy and does not create an obvious leak risk.
- Any concurrency-sensitive change considers race detection as part of verification.
- Any new env var, middleware, or architectural change is reflected in `.ai-context/docs/operations.md` or `.ai-context/docs/architecture.md`.
- Any new domain invariant is reflected in `.ai-context/docs/domain-rules.md`.
- Tests are added or updated when domain logic, validation, or concurrency behavior changes.
- No unresolved { { ... } } placeholders are introduced.
- Any missing requirement that still needs product input is marked `[PERLU DIISI]` rather than guessed.
