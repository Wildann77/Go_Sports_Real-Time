---
applyTo: "internal/**/*repository*.go"
---

# Repository Layer Instructions

## Intent

Own database access and reusable GORM query patterns.

## Allowed

- Build GORM queries.
- Persist models.
- Provide transaction-scoped persistence helpers when services need atomic workflows.
- Return domain data in the shapes already expected by services.
- Respect caller-provided context for every request-scoped DB operation.

## Forbidden

- Business rules or status decisions.
- HTTP-aware logic.
- Response formatting.
- WebSocket event emission.
- Creating detached contexts that ignore upstream cancellation for normal query execution.

## Self-Check

- Is this purely persistence logic?
- Did I keep domain branching out of the repository?
- If I changed query behavior, do services still get the same contract?
- Did I preserve context-aware DB access semantics?
