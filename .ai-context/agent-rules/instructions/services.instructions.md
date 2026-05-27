---
applyTo: "internal/**/*service*.go"
---

# Service Layer Instructions

## Intent

Own business orchestration, domain validation, transactions, and post-persistence side effects.

## Allowed

- Sanitize domain input before persistence.
- Enforce match and commentary invariants.
- Compose repository calls.
- Open GORM transactions when atomic workflows are required.
- Use row locking for concurrent match mutations.
- Trigger WebSocket broadcasts only after successful persistence.
- Coordinate bounded goroutines only when consistency, cancellation, and ownership are documented.

## Forbidden

- Direct HTTP response writing.
- Gin request parsing or route registration.
- Pure table-mapping concerns that belong in models.
- Hidden persistence shortcuts that bypass repository contracts without a clear transactional reason.
- Starting fire-and-forget goroutines in request flows without an explicit lifecycle policy.
- Swallowing context cancellation or replacing it with detached contexts unless the detachment is intentional and documented.

## Self-Check

- Did I keep domain rules here instead of the handler?
- If I used a transaction, does every side effect happen after success?
- Did I preserve error wrapping into `exceptions.AppError`?
- If I introduced concurrency, can every goroutine stop cleanly?
