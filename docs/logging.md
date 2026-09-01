# Logging Standard — Lotty AB Platform

## Goals

1. Make log levels meaningful for production debugging and operations.
2. Emit exactly one structured log per HTTP request completion.
3. Never log secrets, credentials, tokens, or PII unless explicitly safe.
4. Use consistent structured field names across all packages.
5. Ensure every log entry answers: what operation, which component, outcome, duration, correlation IDs.

## Log Level Policy

| Level | When to use |
|-------|-------------|
| DEBUG | Diagnostic events: token parsed, session validated, DB query completed, config validated, cache hit/miss, state transitions, internal decisions. Must contain info not available from the HTTP completion log. |
| INFO | Important normal events: service startup, dependency connected, bootstrap admin created, HTTP request completed (2xx/3xx), domain lifecycle events (user/flag/metric created/deleted), session revoked. |
| WARN | Degraded but recoverable: Redis/S3 unavailable, session check failed, auth rejected (401/403), old avatar deletion failed, insecure JWT secret, CORS wildcard. |
| ERROR | Failed operations requiring investigation: Postgres unavailable, unexpected internal failure, panic recovery, migration failure, bootstrap admin creation failed, server listen/shutdown failed. |

## HTTP Request Logging

### Single Completion Record

Every HTTP request emits exactly **one** log entry on completion:

```
INFO http request completed
{
  "request_id": "0192a1b2-c3d4-7e8f-9a0b-1c2d3e4f5a6b",
  "http.request.method": "GET",
  "http.route": "/api/panel/v1/users/:id",
  "http.response.status_code": 200,
  "duration_ms": 1.03,
  "http.response.body.size": 4914,
  "client.address": "127.0.0.1",
  "user_id": 42
}
```

### Failed Request Record

```
WARN http request completed
{
  "request_id": "0192a1b2-c3d4-7e8f-9a0b-1c2d3e4f5a6b",
  "http.request.method": "GET",
  "http.route": "/api/panel/v1/users",
  "http.response.status_code": 403,
  "duration_ms": 0.89,
  "http.response.body.size": 62,
  "client.address": "127.0.0.1",
  "user_id": 42,
  "error.type": "authorization_denied"
}
```

### Level Assignment

| HTTP Status | Level |
|---|---|
| 2xx | INFO |
| 3xx | INFO |
| 401, 403 | WARN |
| Other 4xx | INFO |
| 5xx | ERROR |
| Panic recovery | ERROR |

### Removed: `request received` log

The old `DEBUG request received` log was removed because:
- It duplicated almost all fields from the completion record.
- It provided no diagnostic information beyond "a request started."
- It increased log volume without operational value.

## Structured Field Conventions

### Common Fields

| Field | Type | Description |
|---|---|---|
| `request_id` | string | UUIDv7 correlation ID for the request |
| `user_id` | int64 | Authenticated user ID (after auth succeeds) |
| `actor_id` | int64 | User performing a state-changing operation |
| `error.type` | string | Error classification (see below) |
| `duration_ms` | float64 | Operation duration in milliseconds |

### HTTP Fields

| Field | Type |
|---|---|
| `http.request.method` | string |
| `http.route` | string |
| `http.response.status_code` | int |
| `http.response.body.size` | int |
| `client.address` | string |
| `url.path` | string |

### Database Fields

| Field | Type |
|---|---|
| `db.system` | string |
| `db.name` | string |
| `db.operation` | string |
| `db.table` | string |
| `db.rows_affected` | int64 |

### Cache (Redis) Fields

| Field | Type |
|---|---|
| `cache.system` | string |
| `cache.operation` | string |
| `cache.hit` | bool |
| `cache.key_namespace` | string |

### Storage (S3) Fields

| Field | Type |
|---|---|
| `storage.system` | string |
| `storage.operation` | string |
| `storage.region` | string |
| `storage.object_prefix` | string |

### Pagination Fields

| Field | Type |
|---|---|
| `pagination.limit` | int |
| `pagination.offset` | int |
| `result.count` | int |
| `result.total` | int64 |
| `result.has_next` | bool |

### Domain Event Fields

| Field | Type |
|---|---|
| `user.role` | string |
| `user.old_role` | string |
| `user.new_role` | string |
| `flag.key` | string |
| `flag.type` | string |
| `metric.key` | string |
| `metric.type` | string |
| `auth.failure_reason` | string |
| `auth.token_source` | string |
| `auth.session_count` | int |
| `auth.session_state` | string |

## Error Classification

| error.type | Description |
|---|---|
| `validation_error` | Request body or query parameter validation failed |
| `authentication_failed` | Invalid credentials, expired/revoked token |
| `authorization_denied` | Authenticated but insufficient permissions |
| `not_found` | Resource does not exist |
| `conflict` | Duplicate key/name violation |
| `database_unavailable` | PostgreSQL connection failed |
| `database_query_failed` | SQL query execution failed |
| `redis_unavailable` | Redis connection failed |
| `redis_session_missing` | Session not found in Redis |
| `storage_unavailable` | S3 connection or operation failed |
| `storage_operation_failed` | S3 put/delete failed |
| `internal_error` | Unexpected internal failure |
| `panic` | Runtime panic recovered |

## Sensitive Data — Never Log

The following values must never appear in log output at any level:

- Plaintext passwords or password hashes
- JWT tokens or signing secrets
- Authorization header values
- Cookie values
- Redis passwords, session keys, or session values
- S3 access keys or secret keys
- PostgreSQL DSNs containing credentials
- Raw database passwords
- Raw request or response bodies
- Uploaded file bytes
- Full query parameters that may contain secrets
- Full email addresses (use `user_id` instead)
- Full IP addresses with ports (use `client.address` without port)
- Stack traces in non-panic API response logs

## Request ID Correlation

Request IDs flow through the system via:

1. `middleware.RequestIDGin` / `middleware.RequestIDFiber` — generates or accepts UUIDv7 from `X-Request-ID` header, stores in context.
2. `middleware.LoggerGin` / `middleware.LoggerFiber` — includes `request_id` in the HTTP completion log.
3. `middleware.CallerIDGin(c)` / `middleware.CallerIDFiber(c)` — extracts `user_id` from context for handler logic.

Domain handlers use `h.log.Error(...)` with explicit `zap.String("request_id", ...)` when they need request-correlated error logs. The HTTP completion log (middleware.LoggerGin/LoggerFiber) always includes the request ID.

## Log Configuration

| Environment | Format | Default Level | Output |
|---|---|---|---|
| `local` | Human-readable colorized console | DEBUG | stdout |
| `dev` | Structured JSON | DEBUG | stdout |
| `prod` | Structured JSON | INFO | stdout |

Configuration is driven by the `environment` and `log_level` fields in config. `log_level` overrides the environment default when set.

## Domain Event Logging

### Auth
- INFO: login successful (user_id only, no email/token), session revoked
- DEBUG: token generated, session stored/checked with cache hit/miss
- WARN: invalid credentials, session check failed, session store failed, revoked token used, role denied, authentication failed

### Users
- INFO: user created, user deleted, role changed, bootstrap admin created
- WARN: old avatar deletion failed, session revocation failed
- DEBUG: avatar uploaded/deleted

### Flags
- INFO: flag created, flag deleted
- DEBUG: flag updated

### Metrics
- INFO: metric created
- DEBUG: metric updated

### Health
- WARN: readiness probe: database/cache/storage unavailable (with status and message)
- ERROR: health check: database ping failed

## Examples

### Good DEBUG

```json
{
  "level": "DEBUG",
  "message": "auth token generated",
  "request_id": "...",
  "user_id": 42,
  "auth.token_source": "login"
}
```

```json
{
  "level": "DEBUG",
  "message": "session checked",
  "request_id": "...",
  "user_id": 42,
  "cache.operation": "exists",
  "cache.hit": true,
  "duration_ms": 0.41
}
```

### Good INFO

```json
{
  "level": "INFO",
  "message": "user created",
  "user_id": 5,
  "user.role": "viewer"
}
```

### Good WARN

```json
{
  "level": "WARN",
  "message": "redis unavailable; continuing without cache",
  "cache.system": "redis",
  "error": "connection refused"
}
```

### Good ERROR

```json
{
  "level": "ERROR",
  "message": "postgres unavailable; startup aborted",
  "db.system": "postgresql",
  "error": "ping postgres: connection refused"
}
```

### Bad — Never Write These

```go
// BAD: format string instead of structured fields
logger.Info(fmt.Sprintf("user %d created", id))

// BAD: logging secrets
logger.Info("login", zap.String("token", token))

// BAD: redundant start/finish pairs
logger.Debug("handler started")
logger.Debug("handler completed")

// BAD: logging request bodies
logger.Debug("request", zap.String("body", string(body)))
```
