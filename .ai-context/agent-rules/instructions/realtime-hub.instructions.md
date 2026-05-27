---
applyTo: "internal/features/realtime/hub/**/*.go"
---

# Realtime Hub Instructions

## Intent

Own WebSocket client lifecycle, room membership, and channel-safe broadcast fan-out.

## Allowed

- Register and unregister clients.
- Manage room subscriptions.
- Broadcast to room members through channels.
- Clean up disconnected or slow consumers safely.
- Use cancellation-aware or bounded blocking patterns in long-lived concurrent flows where applicable.

## Forbidden

- Database access.
- HTTP parsing or route wiring.
- Match/commentary business validation beyond transport-level message handling.
- Shared mutable state updates outside the hub event loop.
- Goroutine creation that has no ownership model or cleanup path.

## Self-Check

- Does hub state mutate only through the established channels?
- Did I avoid introducing concurrent map write risk?
- If I changed event semantics, did I update `.ai-context/docs/domain-rules.md`?
- If I changed concurrency behavior, would `go test -race ./...` be an expected verification step?
