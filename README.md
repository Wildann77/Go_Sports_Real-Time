# Real-time Sports Dashboard Backend

A comprehensive Golang backend for a real-time sports dashboard. It provides a REST API to manage initial state and issue command operations, alongside a WebSocket server to stream live game updates and commentary efficiently to subscribed clients.

## System Architecture

### Feature-Based Architecture / DDD-Lite
The project strictly implements a Feature-based architecture (DDD-lite) ensuring modularity and clean boundaries.
Code is categorized into:
- **`internal/core`**: For centralized configurations, middleware, global error handling, and database setups.
- **`internal/features`**: Grouped strictly by domain (e.g., `matches`, `commentary`, `realtime`).
  - **`routers`**: Registers endpoint routes.
  - **`handlers`**: Parses requests, calls services, sends formatted responses. No business logic resides here!
  - **`services`**: Contains the core business orchestration and transaction logic.
  - **`repositories`**: Interacts exclusively with the PostgreSQL database.
  - **`schemas`**: Request and Response DTOs.
  - **`models`**: Standardized database mappings.
- **`internal/shared`**: Reusable schemas, enums, and constants.

### Technology Stack & Decisions
- **Golang & Gin**: Chosen for performance, robust typing, and high concurrency capability. Gin allows very structured router middleware flow.
- **PostgreSQL (Source of Truth)**: Used for ACID compliance to prevent race conditions during rapid live score updates.
- **gorilla/websocket**: Fast and battle-tested WS driver in Go.
- **GORM**: The fantastic ORM library for Golang, using standard PostgreSQL drivers for declarative schemas and type safety.

## Setup Instructions

1. **Configuration**:
   Copy the example environment variables:
   ```bash
   cp .env.example .env
   ```
2. **Infrastructure**:
   Start the local PostgreSQL container:
   ```bash
   make docker-up
   ```
3. **Database Migration**:
   Run schema migrations to setup tables and indexes:
   ```bash
   make migrate-up
   ```
4. **Run Server**:
   Start the Gin application:
   ```bash
   make run
   ```

## WebSocket Deep Dive

The WebSocket approach uses a Pub/Sub model via a central `Hub`.

- **Pub/Sub Rooms**: Clients subscribe to a specific `matchId`. The Hub safely isolates these `rooms` and broadcasts only to clients existing within that specific room.
- **Heartbeat Management**: Standard ping/pong loops are active. The client must reply to server `ping` requests to prevent the `ReadLimit` deadline from expiring, averting stalled connections.
- **Thread Safety**: All Hub mutations (subscribing, unsubscribing, broadcasting) are routed through channels to a single dedicated goroutine (`Run()` inside Hub). This prevents catastrophic concurrent map writes.
- **Cleanup**: If a connection breaks or is too slow (causing buffer overflow), the client is instantly closed and actively removed from all subscribed rooms strictly avoiding memory leaks.

## Input Validation & Sanitization

Security is enforced at the earliest routing layers:
- **Sanitization**: All strings are trimmed. `strings.TrimSpace` ensures no padded spaces.
- **Custom Validators**: We employ `go-playground/validator` integrated with Gin to apply custom rules (`non_empty_trimmed`, `match_status`, `safe_slug`, `json_object`).
- **Body Limits & Rates**: Custom middlewares cap HTTP Body size and mitigate flooding attacks via IP-based bucket Rate limiters.
- **Validation Refusal**: Any illegal state or unknown WebSocket `type` yields a clear error payload to prevent unsafe casting.

## Structured Response & Error Handling 

**Controller format consistency**: Handlers use centralized `schemas.Success()` and `schemas.Error()` functions maintaining exact JSON schema:
```json
{
  "success": true,
  "message": "Match created successfully",
  "data": {},
  "meta": null,
  "error": null
}
```

**Global AppError Model**: Any internal or validation error utilizes the custom `exceptions.AppError` struct allowing explicit codes (`VALIDATION_ERROR`, `NOT_FOUND`). Database panic traces are intercepted by recover middlewares converting them into generalized 500 exceptions, keeping backend topology entirely concealed from external users. 

## Transactional Integrity (ACID)

The core endpoint: `POST /api/v1/matches/{id}/commentary`
Because an incoming commentary payload may mutate the actual main `matches` core score, it is wrapped entirely within a `pgx.Tx` transaction context constraint.

1. **Locking**: Row is selected with `SELECT ... FOR UPDATE` protecting the current score state from other incoming parallel requests.
2. **Atomic Writes**: Both `INSERT` to commentary and `UPDATE` on the match table must complete simultaneously. If failure occurs anywhere, the scope performs a Rollback.
3. **Post-Commit Hook**: *Only* upon a fully successful PG transaction commit does the `commentary_service` delegate the payload to the WS Hub to be broadcasted, guaranteeing clients never receive ghost data.

## Testing Endpoints

### REST Example - Create a Commentary
```bash
curl -X POST http://localhost:8000/api/v1/matches/1/commentary \
     -H "Content-Type: application/json" \
     -d '{"minute": 15, "eventType": "goal", "message": "First Goal!", "payload": {"homeScore": 1}}'
```

### WebSocket Example
Install `websocat` or use any WS client.
```bash
websocat ws://localhost:8000/ws
```
**Send Subscription Payload**:
```json
{
  "type": "subscribe",
  "matchId": 1
}
```

You will immediately receive a live broadcast on the channel the moment the aforementioned REST curl request processes.

## License

This project is open-source and available under the MIT License.

# Go_Sports_Real-Time
