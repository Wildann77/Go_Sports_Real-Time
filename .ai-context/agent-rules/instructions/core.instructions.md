---
applyTo: "internal/core/**/*.go"
---

# Core Layer Instructions

## Intent

Own runtime-wide concerns only: configuration, database bootstrap, middleware, security helpers, and global error handling.

## Allowed

- Add or update environment-backed config fields.
- Tune database pool settings and bootstrap behavior.
- Implement cross-cutting middleware such as logging, recovery, rate limiting, CORS, body limits, and validators.
- Add sanitization or validation helpers that are not feature-specific.
- Normalize shared error behavior through `exceptions.AppError`.
- Define application-wide timeout, shutdown, logging, and health-check concerns.

## Forbidden

- Feature-specific match or commentary business rules.
- Route registration for specific features.
- GORM queries for feature use cases.
- Direct WebSocket room or broadcast decisions.
- Package-level mutable state without clear synchronization and ownership.

## Self-Check

- Is this logic truly cross-cutting?
- Would a feature package own this better?
- Did I avoid leaking domain-specific policies into core?
- If I introduced shared state, is synchronization explicit and justified?
