# Architecture

## Project Snapshot

| Item | Value |
| --- | --- |
| Project Name | Real-time Sports Dashboard Backend (`sports-dashboard`) |
| Description | A Go backend for authoritative match management over REST plus live commentary and score updates over WebSocket. |
| Primary Language | Go 1.22 |
| Architecture Pattern | Feature-based architecture / DDD-lite / modular monolith |
| Main Flow | `middleware -> router -> handler -> service -> repository -> database`, then post-commit `service -> realtime hub` when needed |

## Tech Stack

| Layer | Technology |
| --- | --- |
| Framework | Gin |
| Database | PostgreSQL 15 |
| ORM / Query Builder | GORM |
| Validation / Schema | Gin binding, `go-playground/validator/v10`, custom validators, Go DTO structs |
| Auth | JWT access token plus refresh-token rotation with hashed refresh-session persistence and token-version invalidation |
| Machine Auth | User-owned API keys with hashed storage and scope-gated write access |
| Cache | `[PERLU DIISI]` No cache layer found in the current codebase. |
| Queue / Worker | `[PERLU DIISI]` No queue or background worker found in the current codebase. |
| Storage | PostgreSQL tables with JSONB fields for metadata and commentary payloads |
| Logging | `log/slog` plus GORM logger |
| AI / ML | `[PERLU DIISI]` No AI/ML stack found in the current codebase. |
| Realtime | Gorilla WebSocket with an in-process channel-driven hub |
| Local Ops | Docker Compose, `golang-migrate`, Makefile |
| Config | Environment variables via `joho/godotenv/autoload` |

## Architecture Principles

- Organize by feature under `internal/features`.
- Keep cross-cutting concerns in `internal/core`.
- Use handlers as thin adapters.
- Use services for orchestration and invariants.
- Use repositories for persistence boundaries.
- Treat PostgreSQL as the source of truth.
- Treat the WebSocket hub as a delivery mechanism, not a source of truth.
- Propagate request context through every request-scoped layer.
- Treat goroutines and channels as explicit design choices, not incidental implementation details.
- Prefer bounded concurrency and deterministic shutdown over ad-hoc background work.
- Start long-lived runtime components from app bootstrap with explicit lifecycle ownership.

## Folder Structure

```text
.
├── cmd/server/main.go
├── internal
│   ├── core
│   │   ├── config
│   │   ├── database
│   │   ├── exceptions
│   │   ├── middleware
│   │   └── security
│   ├── features
│   │   ├── auth
│   │   │   ├── handlers
│   │   │   ├── models
│   │   │   ├── repositories
│   │   │   ├── routers
│   │   │   ├── schemas
│   │   │   ├── services
│   │   │   └── utils
│   │   ├── apikeys
│   │   │   ├── handlers
│   │   │   ├── models
│   │   │   ├── repositories
│   │   │   ├── routers
│   │   │   ├── schemas
│   │   │   └── services
│   │   ├── commentary
│   │   │   ├── handlers
│   │   │   ├── models
│   │   │   ├── repositories
│   │   │   ├── routers
│   │   │   ├── schemas
│   │   │   └── services
│   │   ├── health
│   │   │   ├── handlers
│   │   │   ├── services
│   │   │   └── routers
│   │   ├── matches
│   │   │   ├── handlers
│   │   │   ├── models
│   │   │   ├── repositories
│   │   │   ├── routers
│   │   │   ├── schemas
│   │   │   ├── services
│   │   │   └── utils
│   │   └── realtime
│   │       ├── handlers
│   │       ├── hub
│   │       ├── routers
│   │       └── schemas
│   ├── router.go
│   └── shared
│       ├── constants
│       ├── enums
│       ├── helpers
│       └── schemas
├── migrations
├── docker-compose.yml
├── Makefile
└── README.md
```

## Layer Ownership

| Layer | Owns | Must Not Own |
| --- | --- | --- |
| Core | Config, DB setup, middleware, security, global errors | Feature business rules |
| Routers | Route registration and composition root wiring | Queries, business logic |
| Handlers | Request parsing and response emission | Persistence, orchestration |
| Services | Business rules, transactions, broadcast timing | HTTP formatting |
| Repositories | GORM access | Domain decisions |
| Models | Table mappings | API contracts, orchestration |
| Schemas | DTOs and response wrappers | Queries, business logic |
| Realtime Hub | Client rooms and broadcast fan-out | Source-of-truth state changes |

## Coding Standards

- Keep Go package names lowercase.
- Keep JSON property names camelCase.
- Prefer explicit error conversion into `exceptions.AppError`.
- Keep handlers thin and deterministic.
- Use services to coordinate multi-step state changes.
- Use repository methods for reusable DB interactions.
- Verify protected routes through auth middleware and token-version checks.
- Verify protected write routes through JWT-or-API-key middleware and scope checks where documented.
- Preserve channel ownership in realtime code.
- Pass `context.Context` as the first parameter of request-scoped service and repository methods.
- Avoid `context.Background()` in the request path unless detachment is intentional and documented.
- Wrap internal errors with enough context for logs and diagnosis.
- Prefer table-driven tests for business rules and validators.
- Use `httptest` for HTTP-layer verification.
- Consider `go test -race ./...` mandatory for concurrency-sensitive changes.
- `[PERLU DIISI]` Add team-specific formatting, linting, or commit-convention rules if the team has them.

## Operational Endpoint Semantics

- `GET /health` is a liveness endpoint for process-alive monitoring and backward compatibility.
- `GET /health/live` is an explicit liveness alias.
- `GET /health/ready` is the readiness endpoint and must verify PostgreSQL reachability before claiming the service is ready.
