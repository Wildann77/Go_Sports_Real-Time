# Domain Rules

## Main Entities

| Entity | Fields Of Note | Rules |
| --- | --- | --- |
| `users` | `email`, `name`, `passwordHash`, `tokenVersion` | `email` must be unique. Passwords are verified against stored hashes. Increment `tokenVersion` to invalidate previously issued access tokens. |
| `refresh_sessions` | `userId`, `tokenHash`, `jti`, `familyId`, `expiresAt`, `revokedAt`, `replacedBy`, `userAgent`, `ipAddress` | Never store raw refresh tokens. Rotation must revoke old session and create a new one in the same family. Revoked-token reuse is treated as token theft. |
| `api_keys` | `userId`, `name`, `keyPrefix`, `keyHash`, `keyLastFour`, `scopes`, `lastUsedAt`, `expiresAt`, `revokedAt` | Never store raw API keys. Raw key is shown only once at creation. Active keys are user-owned and scope-gated. |
| `matches` | `sport`, `homeTeam`, `awayTeam`, `homeScore`, `awayScore`, `status`, `startTime`, `endTime`, `metadata` | Scores must be non-negative. Status must be one of `scheduled`, `live`, `finished`. |
| `commentary` | `matchId`, `minute`, `eventType`, `message`, `payload`, `createdAt` | Commentary belongs to a valid match. `minute` must be non-negative. |

## Status Flow

```text
scheduled -> live -> finished
```

Status is derived from the current time against `startTime` and `endTime`.

- `scheduled`: current time is before `startTime`
- `live`: current time is between `startTime` and `endTime`
- `finished`: current time is after `endTime`

## Critical Business Rules

- Match creation sanitizes `sport`, `homeTeam`, and `awayTeam`.
- `sport` must conform to the safe slug validator.
- `startTime` must be before `endTime`.
- Listing or creating commentary requires the target match to exist.
- Commentary creation is atomic:
  - lock the match row
  - create commentary
  - update score if payload contains valid score values
  - commit
  - broadcast
- Commentary payload may include:

```json
{
  "homeScore": 1,
  "awayScore": 0
}
```

- If commentary payload does not include score changes, the match score remains unchanged.
- WebSocket welcome and room events are transport-level notifications; authoritative state still comes from persisted match/commentary data.
- `POST /api/v1/login` validates credentials, returns a JSON `accessToken`, and sets the refresh token in an HTTP-only cookie.
- `GET /api/v1/me` requires a valid Bearer access token and rejects token-version mismatches.
- `POST /api/v1/refresh-token` must verify the refresh JWT, match the stored token hash, reject revoked or expired sessions, rotate the session, and return a new JSON `accessToken`.
- `POST /api/v1/api-keys` requires a valid Bearer access token, stores only a hashed API key, and returns the raw API key exactly once.
- `GET /api/v1/api-keys` lists only the authenticated user's API key metadata and never returns raw API keys.
- `DELETE /api/v1/api-keys/:id` revokes only the authenticated user's API key record.
- `POST /api/v1/matches` accepts either a valid access token or a valid API key with scope `matches:write`.
- `POST /api/v1/matches/:id/commentary` accepts either a valid access token or a valid API key with scope `commentary:write`.
- Reusing a revoked refresh token is treated as token theft:
  - revoke every session in the same `familyId`
  - increment the user `tokenVersion`
  - return an unauthorized security error
- `POST /api/v1/logout` revokes only the current refresh session when present and clears the refresh cookie.
- `POST /api/v1/logout-all` requires a valid access token, revokes every refresh session for the user, increments `tokenVersion`, and clears the refresh cookie.

## Concurrency And Consistency Rules

- PostgreSQL remains the authoritative state store; WebSocket delivery is downstream of persistence success.
- Any future concurrent mutation of match state must preserve atomicity and race safety.
- Multi-step writes that affect match state and commentary must remain transactional.
- Refresh-token rotation and family revocation must remain transactional.
- Hub-managed room state must stay single-owner through its channel/event-loop model.
- `[PERLU DIISI]` If the project later adds worker pools, async jobs, or an outbox, define ownership and retry semantics here.

## API Response Convention

All HTTP responses use the shared wrapper:

```json
{
  "success": true,
  "message": "Match created successfully",
  "data": {},
  "meta": null,
  "error": null
}
```

Error responses use the same wrapper with:

```json
{
  "success": false,
  "message": "Validation failed",
  "data": null,
  "meta": null,
  "error": {
    "code": "VALIDATION_ERROR",
    "details": []
  }
}
```

## Error Handling Convention

- Convert validation issues into `exceptions.AppError`.
- Use `c.Error(...)` inside handlers.
- Let `exceptions.ErrorHandlerMiddleware()` convert errors into the shared response format.
- Use `Recover()` middleware for panics and conceal stack details from clients.
- Infrastructure and database failures should be logged with internal cause context but returned to clients as sanitized `AppError` responses.

## Auth Contracts

- Access tokens are short-lived and currently default to `15` minutes.
- Refresh tokens are long-lived and currently default to `30` days.
- Access tokens travel in JSON responses.
- Refresh tokens travel only through HTTP-only cookies.
- Refresh sessions persist only hashed refresh tokens.
- `tokenVersion` mismatch invalidates previously issued access tokens and refresh flows tied to the older version.
- API keys travel through the Bearer authorization header and are distinguished by `sk_live_` or `sk_test_` prefixes.
- API keys persist only SHA-256 hashes plus safe metadata such as prefix and last four characters.
- API key scopes currently include:
  - `matches:write`
  - `commentary:write`
- Expired or revoked API keys must be rejected.

## Naming Conventions

- Go packages: lowercase
- Exported identifiers: PascalCase
- Internal identifiers: camelCase
- JSON fields: camelCase
- SQL entities:
  - `matches`
  - `commentary`

## Localization

- Current user-facing messages are hardcoded English strings.
- `[PERLU DIISI]` Define a localization or message-key strategy if multilingual support is required.

## Known Gaps Requiring Product Or Architecture Input

- `[PERLU DIISI]` Cache invalidation rules if a cache layer is added.
- `[PERLU DIISI]` Background processing rules if async jobs are introduced.
