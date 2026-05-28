# Operations

## Runtime Configuration

| Variable | Purpose | Default |
| --- | --- | --- |
| `PORT` | HTTP server port | `8000` |
| `DATABASE_URL` | PostgreSQL connection string | empty |
| `APP_ENV` | Environment mode | `development` |
| `ALLOWED_ORIGINS` | Comma-separated CORS and WS origin allowlist | `http://localhost:3000` |
| `RATE_LIMIT_RPS` | Requests per second per IP | `5.0` |
| `RATE_LIMIT_BURST` | Burst bucket size per IP | `10` |
| `WS_MAX_PAYLOAD_BYTES` | Max accepted WebSocket message payload size | `1048576` |
| `DB_MAX_CONNS` | Max open DB connections | `10` |
| `DB_MIN_CONNS` | Max idle DB connections | `2` |
| `DB_QUERY_TIMEOUT_SECONDS` | Default timeout for request-scoped repository queries and non-transactional DB operations | `5` |
| `DB_TX_TIMEOUT_SECONDS` | Default timeout for request-scoped transactional DB work | `10` |
| `JWT_ACCESS_SECRET` | HMAC secret for access-token signing | empty |
| `JWT_REFRESH_SECRET` | HMAC secret for refresh-token signing | empty |
| `ACCESS_TOKEN_TTL_MINUTES` | Access-token TTL in minutes | `15` |
| `REFRESH_TOKEN_TTL_DAYS` | Refresh-token TTL in days | `30` |
| `REFRESH_COOKIE_NAME` | HTTP-only refresh cookie name | `refresh_token` |
| `REFRESH_COOKIE_SECURE` | Whether refresh cookie requires HTTPS | `false` |
| `REFRESH_COOKIE_DOMAIN` | Optional refresh cookie domain override | empty |
| `REFRESH_COOKIE_PATH` | Refresh cookie path | `/` |
| `REFRESH_COOKIE_SAME_SITE` | Refresh cookie same-site mode (`lax`, `strict`, `none`) | `lax` |

## Security Controls

| Control | Implementation |
| --- | --- |
| Validation | Gin binding plus custom validator rules |
| Sanitization | Trimmed strings and sanitized slugs |
| CORS | Allowlist middleware |
| Rate Limiting | In-memory per-IP token bucket |
| Request Size Limit | 5 MB |
| Panic Recovery | `Recover()` middleware |
| WebSocket Origin Check | Gorilla upgrader `CheckOrigin` |
| Authentication | JWT access tokens plus refresh-token rotation backed by hashed refresh-session records |
| Machine Access | User-owned API keys over `Authorization: Bearer <token>` with `sk_live_` / `sk_test_` prefixes and scope validation on protected write routes |
| Authorization | `[PERLU DIISI]` No role or permission model is implemented beyond authenticated route protection |

## Runtime And Shutdown Expectations

- Request-scoped work should honor request cancellation through propagated contexts.
- Long-running operations should use explicit timeout policy when the project defines one.
- Graceful shutdown must stop accepting traffic, allow in-flight work to drain, and close shared resources cleanly.
- Any future background component must document its startup, shutdown, and cancellation behavior.
- Database bootstrap fails fast on connectivity check during startup.
- Rate limiter cleanup is an owned runtime component started from app bootstrap, not from `init()`.
- WebSocket hub startup and shutdown are explicit runtime lifecycle concerns.
- Refresh-token rotation, logout, logout-all, and family revocation flows must execute inside bounded transactions.
- API key verification is synchronous request-path work; successful verification updates `last_used_at` before the write request proceeds.

## Timeout Policy

- Repository query methods derive a child context using `DB_QUERY_TIMEOUT_SECONDS`.
- Transactional service flows derive a child context using `DB_TX_TIMEOUT_SECONDS`.
- Request cancellation still propagates downward; timeout policy adds an upper bound, not a detached context.
- Health readiness checks reuse `DB_QUERY_TIMEOUT_SECONDS` as the upper bound for database reachability checks.

## Health Endpoints

| Endpoint | Semantics | Dependency Check |
| --- | --- | --- |
| `GET /health` | Liveness endpoint for process-alive checks and backward compatibility | None |
| `GET /health/live` | Explicit liveness alias | None |
| `GET /health/ready` | Readiness endpoint for traffic gating and orchestration | PostgreSQL `PingContext` |

- `/health` must not claim downstream dependency readiness.
- `/health/ready` returns `200` only when the service can acquire a SQL DB handle and ping PostgreSQL within the configured query timeout.
- `/health/ready` returns `503` with a sanitized `AppError` response when the database is unreachable or not ready.

## Storage

- Primary storage is PostgreSQL.
- `users` stores identity plus `token_version`.
- `refresh_sessions` stores hashed refresh-token sessions for device-scoped persistence and rotation.
- `api_keys` stores hashed machine-access keys plus safe metadata, scopes, usage timestamp, and revocation state.
- `matches.metadata` uses `JSONB`.
- `commentary.payload` uses `JSONB`.
- `[PERLU DIISI]` No object storage, upload path, or retention policy is defined.

## Logging And Audit

- HTTP requests are logged with `log/slog` and request IDs.
- Server lifecycle events are logged with `log/slog`.
- GORM query logging is enabled with environment-sensitive verbosity.
- Structured logs should prefer stable key-value fields over free-form strings where practical.
- `[PERLU DIISI]` No dedicated audit trail exists for data mutations.
- `[PERLU DIISI]` No log shipping, aggregation, or retention policy is documented.

## Development Commands

```bash
make run
make dev
make build
make test
make vet
make fmt
make review
make docs-check
make docs-update
make agent-check
make migrate-up
make migrate-down
make docker-up
make docker-down
make tidy
```

## Review Workflow

Use this lightweight sequence after changes:

1. Run `make review` to validate formatting, vetting, tests, and agent/doc consistency.
2. Run `make docs-update` to see whether the change actually requires updates to `AGENTS.md` or `.ai-context/docs/`.
3. If docs were edited, run `make docs-check`.
4. For larger handoff-ready changes, run `make agent-check`.

Concurrency-sensitive changes should also consider:

```bash
go test -race ./...
```

Performance claims or hot-path optimizations should consider:

```bash
go test -bench . ./...
```

## Local Deployment Notes

- `docker-compose.yml` starts:
  - PostgreSQL 15
  - the API service on port `8000`
- Graceful shutdown is implemented in `cmd/server/main.go`.
- `[PERLU DIISI]` Redis-backed per-key rate limiting is not implemented in current API key v1.
- `[PERLU DIISI]` Production deployment topology is not documented in the current codebase.

## Documentation Safety

- Keep secrets out of `.ai-context/`.
- Current `.ai-context/` files are documentation-only and contain no secret material.
- No `.gitignore` change is required unless future AI context files include sensitive environment or credential details.
