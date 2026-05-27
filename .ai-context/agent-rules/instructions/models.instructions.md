---
applyTo: "internal/**/*model*.go"
---

# Model Layer Instructions

## Intent

Represent database schema mappings and only the minimal model-level metadata needed by GORM.

## Allowed

- Struct fields, tags, indexes, and `TableName()` overrides.
- Simple type-level organization that supports persistence mapping.

## Forbidden

- Handler logic.
- Business rules.
- Validation or sanitization.
- Query orchestration.

## Self-Check

- Did I keep this file as a persistence mapping?
- Should this behavior live in a schema, service, or repository instead?
