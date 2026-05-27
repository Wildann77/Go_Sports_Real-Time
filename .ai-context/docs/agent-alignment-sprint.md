# Agent Alignment Sprint

## Goal

Bring current codebase into alignment with `AGENTS.md` and `.ai-context/` rules before broader feature work continues.

## Current Assessment

### Strong Areas

- Feature-based structure is already clear.
- Handlers are mostly thin and pass request context downstream.
- Shared API response envelope is consistent.
- Commentary write flow already commits before WebSocket broadcast.
- Commentary transaction now keeps tx-aware match persistence inside repository methods.
- WebSocket hub mostly uses channel ownership instead of ad-hoc shared mutation.
- Graceful shutdown exists at HTTP server level.
- Runtime ownership is now explicit for rate limiter cleanup and WebSocket hub startup/shutdown.
- Request-scoped DB timeout policy is now implemented.
- Infrastructure errors now preserve internal cause context while returning sanitized API errors.
- Health endpoints now distinguish liveness from readiness, and readiness checks PostgreSQL reachability.

### Main Gaps

- Runtime verification commands still need execution in a real environment.

## Findings To Fix

### 1. Background cleanup goroutine has no owner or shutdown path

Status:

- Implemented

Evidence:

- [rate_limit.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/core/middleware/rate_limit.go:19)
- [rate_limit.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/core/middleware/rate_limit.go:37)
- [rate_limit.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/core/middleware/rate_limit.go:50)

Problem:

- Package-level state plus `init()`-spawned infinite goroutine violates explicit ownership and shutdown expectations.
- Hard to test, hard to stop, easy to forget in future runtime lifecycle changes.

Target state:

- Rate limiter cleanup lifecycle owned by application bootstrap.
- Cleanup loop cancelable through context or stop channel.

### 2. WebSocket hub run loop has no shutdown contract

Status:

- Implemented

Evidence:

- [router.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/router.go:53)
- [router.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/router.go:55)

Problem:

- Hub goroutine starts from composition root but never receives stop signal.
- HTTP server shuts down, but hub loop stays conceptually immortal.

Target state:

- Hub has explicit `Run(ctx)` or `Start/Stop` lifecycle.
- Shutdown path documented and tested.

### 3. Transactional service bypasses repository boundary

Status:

- Implemented

Evidence:

- [commentary_service.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/features/commentary/services/commentary_service.go:83)
- [commentary_service.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/features/commentary/services/commentary_service.go:85)
- [commentary_service.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/features/commentary/services/commentary_service.go:121)

Problem:

- Service directly performs locked fetch and `tx.Save(&match)` instead of using tx-aware repository methods.
- Makes service own persistence details, not only orchestration.

Target state:

- Repository layer exposes tx-aware methods for lock/read/update steps.
- Service keeps orchestration, invariants, transaction sequencing.

### 4. DB bootstrap does not verify live connectivity, and `sqlDB` error is ignored

Status:

- Implemented

Evidence:

- [postgres.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/core/database/postgres.go:30)
- [postgres.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/core/database/postgres.go:36)
- [main.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/cmd/server/main.go:31)

Problem:

- App can start without explicit `Ping()` verification.
- `db.DB()` error is ignored in `main.go`.

Target state:

- Startup fails fast if DB not reachable.
- Resource acquisition errors fully handled.

### 5. Health endpoint is liveness-only but presented like full service health

Status:

- Implemented

Evidence:

- [health_handler.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/features/health/handlers/health_handler.go:17)

Problem:

- Endpoint always returns healthy even if PostgreSQL is down.
- Misleading for production readiness, orchestration, and monitoring.

Target state:

- Distinguish liveness and readiness, or make `/health` check critical dependencies.

### 6. Timeout config exists but runtime does not enforce it

Status:

- Implemented

Evidence:

- [config.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/core/config/config.go:19)
- [config.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/core/config/config.go:20)

Problem:

- `DB_QUERY_TIMEOUT_SECONDS` and `DB_TX_TIMEOUT_SECONDS` exist in config only.
- Current code relies only on incoming request context with no standardized timeout application.

Target state:

- Repo and transaction entry points have explicit timeout policy or documented decision to rely solely on request context.

### 7. Critical transactional test coverage is placeholder-only

Status:

- Implemented

Evidence:

- [commentary_service_test.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/features/commentary/services/commentary_service_test.go:10)
- [commentary_service_test.go](/mnt/windows/Users/boyblanco/Documents/code/web/sport_socket_go/server/internal/features/commentary/services/commentary_service_test.go:14)

Problem:

- Most important transaction in project has no real integration or repository-backed test.
- Current test only checks dummy request struct values.

Target state:

- Real test proves:
  - row lock path
  - commentary insert
  - score update
  - rollback safety
  - post-commit broadcast behavior

## Sprint Plan

## Sprint 1: Runtime Ownership And Safety

### Task S1-T1

Refactor rate limiter state into owned component.

Status:

- Implemented

Acceptance:

- No `init()` goroutine for cleanup.
- Cleanup loop stoppable.
- Ownership wired from app bootstrap.

### Task S1-T2

Add WebSocket hub lifecycle control.

Status:

- Implemented

Acceptance:

- Hub start and stop explicit.
- Shutdown path tied to app lifecycle.
- No immortal background loop without owner.

### Task S1-T3

Define timeout policy for request-scoped DB work.

Status:

- Implemented

Acceptance:

- Query and tx timeout behavior documented and implemented.
- No dead config fields.

## Sprint 2: Layer Boundary Cleanup

### Task S2-T1

Move tx-aware match persistence operations into repository methods.

Status:

- Implemented

Acceptance:

- Locked match fetch exposed via repository.
- Match score/status save inside tx exposed via repository.
- Service no longer issues direct `tx.First` or `tx.Save` for match persistence.

### Task S2-T2

Tighten error flow for internal diagnosis.

Status:

- Implemented

Acceptance:

- Internal errors wrapped with operation context where useful.
- Transport layer still returns sanitized `AppError` responses.

## Sprint 3: Readiness And Operations

### Task S3-T1

Harden DB bootstrap.

Status:

- Implemented

Acceptance:

- `Ping()` or equivalent startup verification added.
- `db.DB()` error handled.

### Task S3-T2

Split or improve health checks.

Status:

- Implemented

Acceptance:

- `/health` semantics documented.
- Readiness checks DB reachability if endpoint claims service readiness.

## Sprint 4: Test And Verification Upgrade

### Task S4-T1

Replace placeholder commentary transaction test with real test.

Status:

- Implemented

Acceptance:

- Test validates commit path.
- Test validates rollback path.
- Test validates broadcast-after-commit behavior.

### Task S4-T2

Add concurrency verification for realtime and middleware.

Status:

- Implemented

Acceptance:

- Race-sensitive areas have verification plan.
- `go test -race ./...` included in sprint verification.

### Task S4-T3

Expand handler and error-path coverage.

Status:

- Implemented

Acceptance:

- Match/commentary handlers cover validation and not-found cases.
- Health behavior tests reflect final readiness policy.

## Deferred Until Product Decision

- Authentication and authorization design.
- Object storage/upload policy.
- Cache strategy.
- Worker/outbox/background job strategy.

These remain `[PERLU DIISI]` by design until product/architecture decision exists.

## Recommended Execution Order

1. Runtime ownership and shutdown
2. Repository boundary cleanup
3. DB bootstrap and health readiness
4. Timeout enforcement
5. Transaction and race tests
6. Smaller consistency cleanups

## Verification Checklist

```bash
make review
make docs-update
make docs-check
go test -race ./...
```

Race-sensitive focus areas:

- `internal/features/realtime/hub/**`
- `internal/core/middleware/rate_limit.go`

Optional when optimizing hot paths:

```bash
go test -bench . ./...
```

## Definition Of Sprint Done

- No long-lived goroutine without explicit owner and shutdown strategy.
- Request-scoped DB work follows documented timeout/context policy.
- Transaction-heavy commentary flow respects repository boundaries.
- Health/readiness signal matches real dependency state.
- Critical transactional path has real tests.
- Concurrency-sensitive paths have race-verification coverage.
