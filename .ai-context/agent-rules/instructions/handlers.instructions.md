---
applyTo: "internal/**/*handler*.go"
---

# Handler Layer Instructions

## Intent

Translate HTTP or WebSocket edge input into service calls and standardized responses.

## Allowed

- Parse params, query values, and request bodies.
- Run Gin binding and convert failures into `exceptions.AppError`.
- Call a single service or hub entrypoint as the next step.
- Return output via shared response helpers.
- Pass `c.Request.Context()` into downstream request-scoped calls.

## Forbidden

- GORM access.
- Multi-step business orchestration.
- Score update rules, match status derivation, or transactional decisions.
- Ad-hoc JSON response formats.
- Replacing request context with `context.Background()` for normal request handling.

## Self-Check

- Did I keep this file thin?
- Did I call `c.Error(...)` for failures instead of hand-rolling responses?
- Did I avoid putting business branches here?
- Did I preserve request cancellation by forwarding the request context?
