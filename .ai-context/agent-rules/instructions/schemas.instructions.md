---
applyTo: "internal/**/schemas/**/*.go"
---

# Schema Layer Instructions

## Intent

Define DTOs and shared response contracts for input/output boundaries.

## Allowed

- Request structs with binding tags.
- Response structs and meta payloads.
- Shared API wrapper shapes.
- WebSocket message contract structs when they are data-only.

## Forbidden

- GORM access.
- Business rules.
- Route registration.
- Nontrivial runtime behavior unrelated to serialization or validation tags.

## Self-Check

- Is this type about data shape rather than behavior?
- Did I keep API field names consistent with existing JSON contracts?
- If the response shape changed, did I update `.ai-context/docs/domain-rules.md`?
