---
applyTo: "internal/**/*router*.go"
---

# Router Layer Instructions

## Intent

Own route registration and composition-root wiring only.

## Allowed

- Register paths, route groups, and handler bindings.
- Instantiate dependencies in `internal/router.go`.
- Attach middleware through the Gin engine or route groups.
- Keep versioning and path layout clear and predictable.

## Forbidden

- Database queries or transactions.
- Business rules such as status derivation or score update policy.
- Manual JSON success/error payload construction.
- Direct WebSocket broadcasts.

## Self-Check

- Did I only wire or register behavior here?
- Could this decision live in a handler or service instead?
- If I changed route shape, did I update `.ai-context/docs/architecture.md`?
