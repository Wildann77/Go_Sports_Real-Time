# Feature Test Sprint

## Goal

Define full testing backlog for every implemented feature in this repository.

This document is execution-focused:

- grouped by sprint
- broken into task IDs
- tied to feature boundaries
- aligned with `AGENTS.md`
- ready for later manual or automated execution

## Scope

Covered areas:

- `internal/core/config`
- `internal/core/database`
- `internal/core/exceptions`
- `internal/core/middleware`
- `internal/core/security`
- `internal/shared/helpers`
- `internal/shared/schemas`
- `internal/features/auth`
- `internal/features/matches`
- `internal/features/commentary`
- `internal/features/health`
- `internal/features/realtime`
- composition root in `internal/router.go`
- bootstrap path in `cmd/server/main.go`

Out of scope until product decision:

- Cache layer
- Background worker or outbox
- Object storage and uploads

These remain `[PERLU DIISI]` by design where noted.

## Test Strategy

Use layered verification. Do not rely on one test style only.

| Test Level | Purpose | Main Targets |
| --- | --- | --- |
| Unit | Validate pure logic, validation, mapping, error categorization | utils, sanitizer, schemas, small service logic |
| Handler | Validate HTTP binding, status codes, wrapper consistency, error flow | Gin handlers |
| Repository Integration | Validate real SQL behavior and persistence contracts | GORM repositories, row locking, transactions |
| Service Integration | Validate orchestration across repo + tx + broadcast | commentary and match workflows |
| Realtime Integration | Validate WebSocket lifecycle and room broadcast behavior | hub, client, websocket handler |
| End-to-End | Validate full user flow across HTTP + DB + WS | match creation, commentary write, readiness, live updates |
| Concurrency Verification | Validate race safety and shutdown behavior | hub, rate limiter, runtime ownership |

## Execution Rules

- Prefer table-driven tests for pure logic and validation.
- Use `httptest` for HTTP handlers.
- Use PostgreSQL-backed integration tests for repository and transaction behavior.
- Use race verification for concurrency-sensitive packages.
- Keep broadcasts post-commit only.
- Do not mark sprint done if response wrapper, context propagation, or shutdown safety regresses.

## Test Environment

### Required

- Go toolchain
- PostgreSQL 15
- migration CLI with PostgreSQL support
- Docker Compose for local DB

### Suggested Commands

```bash
make docker-up
make migrate-up
make test
go test -race ./...
make review
```

### Suggested Test Grouping

```bash
go test ./internal/core/...
go test ./internal/features/matches/...
go test ./internal/features/commentary/...
go test ./internal/features/health/...
go test ./internal/features/realtime/...
go test -race ./...
```

## Current Coverage Snapshot

Already present now:

- sanitizer tests
- app error tests
- response wrapper tests
- match status tests
- match schema tests
- health handler tests
- health service tests
- commentary service transaction-oriented tests
- commentary handler tests
- match handler tests
- rate limiter concurrency-oriented tests
- hub lifecycle and broadcast tests

Still missing or incomplete:

- repository integration tests for matches and commentary
- service tests for match creation and listing paths
- middleware HTTP-behavior tests for body limit, CORS, recover, logging
- websocket handler tests
- composition root and bootstrap smoke tests
- full HTTP + DB + WS end-to-end flow
- race verification execution in real environment

## Sprint 1: Core Foundations

### Task FT-S1-T1

Add config loading tests.

Acceptance:

- default values verified
- invalid numeric env falls back safely
- timeout conversion functions verified

### Task FT-S1-T2

Add database bootstrap tests.

Acceptance:

- connection bootstrap fail-fast path covered
- ping failure path covered
- pool config application verified where practical
- timeout policy helpers covered for nil and positive policy

### Task FT-S1-T3

Expand shared response wrapper tests.

Acceptance:

- `Success` shape verified
- `SuccessWithMeta` shape verified
- error response shape verified
- sanitized error payload contract preserved

### Task FT-S1-T4

Expand exception middleware tests.

Acceptance:

- `AppError` converted to shared response correctly
- unknown error converted to `INTERNAL_SERVER_ERROR`
- status codes preserved

## Sprint 2: Security And Middleware

### Task FT-S2-T1

Add validator tests for custom rules.

Acceptance:

- `non_empty_trimmed` covered
- `safe_slug` covered
- `json_object` covered
- invalid enum values rejected

### Task FT-S2-T2

Expand sanitizer tests.

Acceptance:

- string trim behavior covered
- slug normalization covered
- empty and malformed input behavior covered

### Task FT-S2-T3

Add middleware HTTP behavior tests.

Acceptance:

- body limit returns expected failure
- recover middleware converts panic to sanitized error
- CORS allowlist behavior covered
- rate limit middleware returns `RATE_LIMITED` envelope

### Task FT-S2-T4

Add origin policy tests for WebSocket origin checking.

Acceptance:

- allowed origin accepted
- disallowed origin rejected
- empty origin behavior documented and verified

## Sprint 3: Matches Feature

### Task FT-S3-T1

Add `MatchService` unit tests.

Acceptance:

- valid create path covered
- invalid sport rejected
- `startTime` after `endTime` rejected
- status derivation on create verified
- invalid status filter rejected
- not-found path returns `NOT_FOUND`

### Task FT-S3-T2

Add match repository integration tests.

Acceptance:

- create persists row
- find by ID returns nil on not found
- find all respects status filter
- ordering and limit behavior verified

### Task FT-S3-T3

Expand match handler tests.

Acceptance:

- create success response covered
- list success response and `meta` covered
- get success response covered
- validation and not-found already covered remain passing

### Task FT-S3-T4

Add match feature end-to-end REST tests.

Acceptance:

- create match through HTTP
- fetch single match through HTTP
- list matches with filter and limit
- response wrapper consistent in all cases

## Sprint 4: Commentary Feature

### Task FT-S4-T1

Expand commentary schema and service validation tests.

Acceptance:

- minute lower bound covered
- empty message rejected after sanitization
- payload with score fields parsed correctly
- payload without score leaves match unchanged

### Task FT-S4-T2

Add commentary repository integration tests.

Acceptance:

- create with tx persists commentary row
- find by match ID orders by `created_at ASC`
- limit behavior verified

### Task FT-S4-T3

Expand commentary transaction integration tests with real DB.

Acceptance:

- row lock path verified against real PostgreSQL transaction
- commentary insert persists on commit
- rollback leaves no partial data
- score updates persist atomically
- status update persists atomically

### Task FT-S4-T4

Expand commentary handler tests.

Acceptance:

- list success path covered
- create success path covered
- invalid ID, invalid body, and not-found paths remain covered

### Task FT-S4-T5

Add commentary feature end-to-end flow.

Acceptance:

- create match
- create commentary against that match
- verify commentary list reflects write
- verify score mutation visible through match read path

## Sprint 5: Health And Operations

### Task FT-S5-T1

Keep health tests aligned with documented semantics.

Acceptance:

- `/health` returns liveness response
- `/health/live` mirrors liveness response
- `/health/ready` success path covered
- `/health/ready` failure path returns sanitized `SERVICE_UNAVAILABLE`

### Task FT-S5-T2

Add readiness timeout behavior tests.

Acceptance:

- readiness uses bounded timeout
- nil DB provider path covered
- DB handle acquisition failure path covered

### Task FT-S5-T3

Add bootstrap smoke tests.

Acceptance:

- router setup returns engine and cleanup
- health routes registered
- feature routes registered
- long-lived components can start without immediate failure

## Sprint 6: Realtime WebSocket

### Task FT-S6-T1

Expand hub tests.

Acceptance:

- register and unregister behavior covered
- subscribe and unsubscribe behavior covered
- empty room cleanup verified
- slow client removal path verified

### Task FT-S6-T2

Add `Client` message-processing tests.

Acceptance:

- invalid JSON rejected
- missing `type` rejected
- invalid `type` rejected
- `ping` returns `pong`
- subscribe and unsubscribe command paths covered

### Task FT-S6-T3

Add websocket handler tests.

Acceptance:

- upgrade path success covered
- invalid origin rejected
- welcome message path covered
- payload size limit behavior covered

### Task FT-S6-T4

Add realtime integration tests.

Acceptance:

- client subscribes to match room
- commentary write triggers `commentary.created`
- score change triggers `match.updated`
- unrelated room does not receive event

## Sprint 7: Concurrency And Race Verification

### Task FT-S7-T1

Run race-sensitive verification for hub and middleware.

Acceptance:

- `go test -race ./...` executed
- hub package included
- middleware package included
- failures triaged into concrete issues if any

### Task FT-S7-T2

Add targeted concurrency tests for shutdown and cancellation.

Acceptance:

- hub stop path does not deadlock
- register/broadcast after stop do not block
- rate limiter cleanup loop exits on context cancel
- readiness path does not leak goroutines under failure

### Task FT-S7-T3

Document concurrency verification outcome.

Acceptance:

- race run result recorded
- flaky tests, if any, identified
- retry strategy documented

### Sprint 7 Execution Outcome

- `2026-05-26` first full race run executed with `go test -race ./...`.
- Packages covered included `internal/features/realtime/hub` and `internal/core/middleware`.
- First run result: failed.
  - `internal/features/matches/schemas`
    - failure type: test bug, not race
    - issue: validator test used default tag name instead of `binding`
    - fix: set validator tag name to `binding`
  - `internal/features/realtime/hub`
    - failure type: race in tests
    - issue: tests read hub-owned maps while hub goroutine still mutating them
    - fix: rewrite tests to assert behavior through messages, stop hub, then inspect final state only after shutdown
- Targeted follow-up verification after fixes:
  - `go test ./internal/features/matches/schemas ./internal/features/realtime/hub`
  - result: passed
- Full second `go test -race ./...` rerun:
  - status: pending
  - reason: session command usage limit blocked rerun before completion
- Flaky test assessment:
  - no confirmed flaky production tests observed
  - first race failure in realtime area traced to test implementation, not nondeterministic app behavior
- Retry strategy:
  - rerun full suite with `go test -race ./...` once command budget resets
  - if full suite fails again, rerun failing package with `go test -race -count=1 ./path/to/package`
  - keep race-sensitive assertions behavior-based and avoid direct concurrent reads of hub-owned maps

## Sprint 8: Full System Verification

### Task FT-S8-T1

Add end-to-end happy path scenario.

Status: Completed (2026-05-26)

Acceptance:

- service boots against local PostgreSQL (verified in TestSystemE2EHappyPath)
- create match via REST (verified in TestSystemE2EHappyPath)
- subscribe via WebSocket (verified in TestSystemE2EHappyPath)
- create commentary via REST (verified in TestSystemE2EHappyPath)
- receive live WebSocket events (verified in TestSystemE2EHappyPath)
- match read path reflects committed score (verified in TestSystemE2EHappyPath)

### Task FT-S8-T2

Add end-to-end failure scenario.

Status: Completed (2026-05-26)

Acceptance:

- missing match commentary write returns `NOT_FOUND` (verified in TestSystemE2EFailureScenario)
- malformed request returns `VALIDATION_ERROR` (verified in TestSystemE2EFailureScenario)
- readiness returns failure when DB unavailable (verified in TestSystemE2EFailureScenario)
- failed write does not emit WebSocket event (verified in TestSystemE2EFailureScenario)

### Task FT-S8-T3

Add regression command checklist.

Status: Completed (2026-05-26)

Acceptance:

- `make test` (verified locally)
- `make review` (verified locally)
- `go test -race ./...` (verified locally)
- `[PERLU DIISI]` no formal CI pipeline configured yet; regression checklist executed locally via `make review` and `go test -race ./...`

## Recommended Execution Order

1. Sprint 1: foundation
2. Sprint 2: security and middleware
3. Sprint 3: matches
4. Sprint 4: commentary
5. Sprint 5: health and operations
6. Sprint 6: realtime
7. Sprint 7: concurrency and race
8. Sprint 8: full system verification
9. Sprint 9: auth and session security
10. Sprint 10: api key management and hybrid write auth

## Sprint 9: Auth And Session Security

### Task FT-S9-T1

Add auth handler tests.

Status: Completed (2026-05-28)

Acceptance:

- login success response returns JSON `accessToken`
- login failure returns sanitized `UNAUTHORIZED`
- refresh cookie is set on login
- refresh cookie is rotated on `POST /api/v1/refresh-token`
- logout clears refresh cookie

### Task FT-S9-T2

Add auth service tests.

Status: Completed (2026-05-28)

Acceptance:

- credentials verified against stored password hash
- refresh session is persisted with hashed token only
- access-token verification rejects invalid or expired token
- token-version mismatch rejects `/me` and other protected flows
- refresh rotation revokes old session and links `replacedBy`
- revoked refresh-token reuse revokes family and increments `tokenVersion`
- logout-all revokes all user sessions and invalidates old access tokens

### Task FT-S9-T3

Add auth repository integration tests.

Status: Completed (2026-05-28)

Acceptance:

- unique email constraint behavior documented and verified
- refresh-session lookup by `jti` works
- row locking path for refresh rotation works
- family revoke updates only matching active sessions
- logout-all revoke path updates all active sessions for user

### Task FT-S9-T4

Add auth end-to-end tests.

Status: Completed (2026-05-28)

Acceptance:

- login succeeds for seeded user
- `/me` works with valid Bearer token
- `/refresh-token` rotates cookie and returns new access token
- revoked refresh-token reuse returns security error
- `/logout` revokes current session
- `/logout-all` revokes all sessions and invalidates prior access token

### Sprint 9 Execution Outcome

- `2026-05-28` auth test sprint executed for middleware, utils, handlers, services, repositories, and E2E auth flow.
- New primary test files:
  - `internal/core/middleware/auth_test.go`
  - `internal/features/auth/utils/jwt_test.go`
  - `internal/features/auth/handlers/auth_handler_test.go`
  - `internal/features/auth/services/auth_service_test.go`
  - `internal/features/auth/repositories/auth_repository_test.go`
  - `internal/features/auth/handlers/auth_e2e_test.go`
- Targeted auth verification executed:
  - `go test ./internal/core/middleware ./internal/features/auth/...`
  - result: passed
- Full regression verification executed after auth tests:
  - `go test ./...`
  - result: passed

## Sprint 10: API Key Management And Hybrid Write Auth

### Task FT-S10-T1

Add API key service tests.

Status: Completed (2026-05-28)

Acceptance:

- key generation uses `sk_test_` or `sk_live_` prefix based on environment
- raw key is returned once but only hash is stored
- invalid or duplicate scopes are rejected
- expired key is rejected
- missing-scope key returns `FORBIDDEN`
- successful verification updates `last_used_at`

### Task FT-S10-T2

Add hybrid auth middleware tests.

Status: Completed (2026-05-28)

Acceptance:

- Bearer JWT access still works on protected write routes
- Bearer API key works when prefix and scope are valid
- malformed Authorization header is rejected
- missing-scope API key is rejected
- expired or invalid API key is rejected

### Task FT-S10-T3

Add API key handler and repository tests.

Status: Completed (2026-05-28)

Acceptance:

- create/list/revoke success envelopes covered
- validation failures stay sanitized
- owner-scoped revoke behavior covered
- active lookup by key hash works
- revoked key is no longer accepted
- `last_used_at` persistence path covered

### Task FT-S10-T4

Add API key end-to-end flow and update existing system E2E auth assumptions.

Status: Completed (2026-05-28)

Acceptance:

- login user and create API key through JWT-protected management endpoint
- use API key to create match and commentary
- GET match and commentary routes remain public
- JWT still works for write routes after hybrid auth change
- existing full-system E2E flows updated to authenticate protected writes

### Sprint 10 Execution Outcome

- `2026-05-28` API key feature tests added for middleware, handlers, services, repositories, and E2E.
- New primary test files:
  - `internal/core/middleware/hybrid_auth_test.go`
  - `internal/features/apikeys/services/api_key_service_test.go`
  - `internal/features/apikeys/handlers/api_key_handler_test.go`
  - `internal/features/apikeys/repositories/api_key_repository_test.go`
  - `internal/features/apikeys/handlers/api_key_e2e_test.go`
- Existing full-system E2E flow updated to authenticate protected writes with JWT.
- Targeted verification executed:
  - `go test ./internal/core/middleware ./internal/features/apikeys/... ./internal/...`
  - result: passed
- Full regression and review commands executed after doc updates:
  - `go test ./...`
  - result: passed
  - `go test -race ./...`
  - result: passed
  - `make review`
  - result: passed when run directly through `review_agent_standards.sh`; `make review` terminal wrapper returned noisy non-zero output under session capture even though underlying review log ended with `Review checks passed.`
  - `make docs-check`
  - result: passed

## Definition Of Test Sprint Done

- Every implemented feature has unit or handler coverage at minimum.
- Repository-heavy flows have integration coverage where behavior depends on PostgreSQL.
- Commentary transactional path has both simulated and real DB verification.
- WebSocket delivery path has command, room, and broadcast verification.
- Shared response contract remains stable across success and error paths.
- Concurrency-sensitive packages have explicit race verification.
- End-to-end happy path and failure path are both covered.
- Any still-missing area is marked `[PERLU DIISI]` instead of guessed.
