# PROJECT_CONTEXT

Lotty AB Platform — an A/B experimentation platform. Currently only the **panel** service (control plane) is implemented; other services from the platform roadmap are not started.

## Stack

| Concern | Choice |
|---|---|
| Language | Go 1.27 |
| HTTP router | `gin-gonic/gin` + `gin-contrib/cors` |
| PostgreSQL | `jackc/pgx/v5` (`pgxpool`) |
| Redis | `redis/go-redis/v9` |
| Migrations | `pressly/goose/v3` as a Go tool (`go tool goose`), plain SQL files |
| Config | `spf13/viper`, JSON file with `{{ ENV "VAR" }}` template rendering |
| Logging | `uber-go/zap` (structured) |
| Auth | `appleboy/gin-jwt/v2` middleware + `golang-jwt/jwt/v4` (HS256), `x/crypto/bcrypt`, Redis-backed session store |
| Request IDs | `google/uuid` (UUIDv7, propagated via `X-Request-ID`) |
| Validation | `go-playground/validator/v10` driven by `binding` struct tags |

## Architecture

```
client
  │
  ▼
[gin engine]  middleware chain:
  gin.Recovery → middleware.RequestID → middleware.Logger → CORS(gin-contrib)
  │
  ▼
/api/v1 group
  ├── auth domain   POST /login ──► auth.Service ──► auth.Repository ─┐
  │                                    │                              │
  │                                    ▼                              ▼
  │                             lib/auth.Tokenizer            pgxpool (PostgreSQL)
  │                            (JWTManager, HS256)
  └── users domain  CRUD /users ─► users.Service ──► users.Repository ┘
```

- **Handlers** parse/validate requests and map domain errors to HTTP codes.
- **Services** hold business logic and depend on repository *interfaces* declared next to them.
- **Repositories** own all SQL.
- **lib/auth** owns JWT issuance/parsing (`Tokenizer` interface), no knowledge of HTTP or DB.

## Data schemas

### users (migration v2)

```sql
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      TEXT        NOT NULL UNIQUE,
    email         TEXT        NOT NULL UNIQUE,          -- added in v2, backfilled
    password_hash TEXT        NOT NULL,                  -- bcrypt
    role          TEXT        NOT NULL DEFAULT 'viewer'
                  CHECK (role IN ('admin','experimenter','approver','viewer')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ                             -- soft delete, v2
);
```

Migrations:

1. `00001_users.sql` — base table with role CHECK
2. `00002_users_email_soft_delete.sql` — unique `email` (backfilled to `<username>@lotty.local`), `deleted_at`

Soft delete semantics: deleted rows stay in the table (unique constraints still apply — a username/email cannot be reused after delete).

## API surface

Base path `/api/v1`. All responses use one envelope (`pkg/api`):

```json
{ "success": true, "data": { ... } }
{ "success": false, "error": { "code": "...", "message": "..." } }
```

Error codes: `BAD_REQUEST`, `NOT_FOUND`, `PAYLOAD_TOO_LARGE`, `CONFLICT`, `UNAUTHORIZED`, `INTERNAL_SERVER_ERROR`.

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/login` | public | `{email, hash_password}` → `{token}` (JWT HS256, `sub` = user id, TTL 24h default) |
| POST | `/users` | Bearer JWT | create user (bcrypt-hashed password, role validated) |
| GET | `/users?limit=&offset=` | Bearer JWT | paginated list (limit clamped 1-100, default 20) |
| GET | `/users/{id}` | Bearer JWT | get by id |
| PATCH | `/users/{id}` | Bearer JWT | partial update (`email`, `role`; nil fields keep old values) |
| DELETE | `/users/{id}` | Bearer JWT | soft delete, returns 204 |

All `/users/*` routes require `Authorization: Bearer <JWT>` (validated by `appleboy/gin-jwt/v2`; the token's session must also exist in Redis — deleting the key revokes it).

Full spec: `docs/openapi/panel.yaml`.

## File structure

```
Makefile                          # dev containers (pg/redis), goose targets
config.json                       # runtime config (supports {{ ENV }} templates)
docs/openapi/panel.yaml           # OpenAPI 3.0.3 spec
migrations/                       # goose SQL migrations (NNNNN_name.sql)
pkg/
  api/response.go                 # response envelope + ValidateRequest (decode+validate)
  config/config.go                # schema, defaults, ENV-template rendering
  database/postgres.go            # NewPostgres(ctx, dsn, maxConns, minConns, log)
  database/redis.go               # NewRedis(ctx, addr, password, db, log); graceful failure allowed
  logger/logger.go                # zap setup per environment (local/dev/prod)
  middleware/
    logger.go                     # entry + completion logs w/ request_id, status, bytes, duration_ms
    request_id.go                 # UUIDv7 X-Request-ID (validates incoming, regenerates garbage)
services/panel/
  cmd/main.go                     # wiring: config → logger → postgres → redis(optional) → router → http.Server
  cmd/router.go                   # gin engine, middleware order, CORS mapping, domain registration
  internal/domain/users/          # handler.go dto.go model.go service.go repository.go
  internal/domain/auth/           # handler.go dto.go service.go repository.go
  internal/lib/auth/auth.go       # Tokenizer interface + JWTManager (HS256)
```

## Config schema

JSON file discovered at `./config.json` or `/labp/config.json`. Values support Go-template actions with an `ENV` function: `"{{ ENV \"POSTGRES_DSN\" }}"`. Missing keys fall back to defaults in `defaultConfig()`.

| Section | Fields (defaults) |
|---|---|
| `environment` | `prod` (also `local`, `dev` affect logging/gin mode) |
| `auth.jwt` | `secret_key` (`change-me-in-production`), `ttl` (24h), `redis_ttl` |
| `database.postgres` | `dsn`, `max_conns` (8), `min_conns` (2) |
| `database.redis` | `address` (`localhost:6379`), `password`, `db`, `pool_size` |
| `panel.http` | `address` (`0.0.0.0:8080`), timeouts, `cors.*` (origins/methods/headers/expose/credentials/max_age) |

Redis connection failure is non-fatal (warn + continue without it); Postgres failure exits.

## Dev workflow

```
make dev-up          # start PostgreSQL (5433) + Redis (6379) containers (podman/docker)
make pg-migrate-up   # apply migrations
make dev-down        # stop both
make redis-cli / make pg-psql
```

Run the service: `go run ./services/panel/cmd`.
