# CONVENTIONS

Coding conventions and patterns used in this repo. Follow them when adding code.

## Domain layout

Each domain lives in `services/panel/internal/domain/<name>/` with fixed file roles:

| File | Responsibility |
|---|---|
| `model.go` | DB-shaped domain types (e.g. `User`, `Role` enum) |
| `dto.go` | request/response structs + `toResponse` mapper; DTOs never leak DB fields (e.g. password hashes) |
| `repository.go` | SQL only; concrete `Repository` struct over `*pgxpool.Pool` |
| `service.go` | business logic; depends on a repository **interface** declared in the same file |
| `handler.go` | HTTP glue: validate → call service → map errors → respond |

Cross-cutting reusable helpers go to `pkg/` (api, config, database, logger, middleware). Service-specific libs live in `internal/lib/` (e.g. `lib/auth`).

## Dependency rule

Services depend on interfaces, not concrete repos:

```go
type UserRepo interface {
    Create(ctx context.Context, u User) (int64, error)
    // ...
}

type Service struct {
    repo UserRepo
}
```

The pgx-backed `Repository` satisfies the interface implicitly. This keeps services unit-testable with fakes without touching the DB.

## Error handling

- Repositories return **sentinel errors**: `ErrNotFound`, `ErrConflict` (unique violation, pgcode 23505 via `isUniqueViolation`). Services add their own (`ErrInvalidCredentials`, `ErrInvalidRole`) and pass repo sentinels through.
- Handlers map errors **centrally** in one `respondError(w, err)` function:

  | Error | HTTP | Code |
  |---|---|---|
  | `ErrNotFound` | 404 | `NOT_FOUND` |
  | `ErrConflict` | 409 | `CONFLICT` |
  | validation / invalid role | 400 | `BAD_REQUEST` |
  | credentials | 401 | `UNAUTHORIZED` |
  | anything else | 500 | `INTERNAL_SERVER_ERROR`, message always `"internal server error"` (never leak internals) |

- Authentication failures must not reveal *why*: unknown email and wrong password produce the same `invalid email or password`.
- Redis is optional at startup: log a warning and continue. Postgres failure is fatal.

## Validation

- Request bodies are decoded and validated **only** through `pkg/api.ValidateRequest(w, r, &dto)`:
  strict `encoding/json` decode + `validator` using the `binding` tag name.
- Constraints live on DTO fields: `binding:"required,min=3,max=32"`, `binding:"required,email"`, `binding:"required,min=8"`, `binding:"omitempty,oneof=admin experimenter approver viewer"`.
- Unknown JSON fields are ignored (documented behavior).
- Path/query params are parsed defensively; invalid values become 400 (`parseID` pattern).

## HTTP responses

- Always respond through `pkg/api`: `OK(w, data)`, `Error(w, status, code, msg)`, `InternalError(w)`. No ad-hoc `json.Marshal` envelopes.
- Success envelope: `{"success":true,"data":...}`; error envelope adds `error:{code,message}`.
- 204 responses have an empty body.
- Handlers may take `*gin.Context` but pass `c.Writer`/`c.Request` (std `net/http` types) down to services/api helpers.

## Middleware order (contract)

```
gin.Recovery → RequestID → Logger → CORS
```

- `RequestID` must run before `Logger` so both log lines carry `request_id`.
- Incoming `X-Request-ID` is kept only if it is a valid UUIDv7; otherwise a new UUIDv7 is generated. The resolved id is echoed back as a response header.

## Logging

- `zap` structured logging only; no `fmt.Println`/`log.Print`.
- Standard field names: `request_id`, `method`, `path`, `remote_addr`, `user_agent`, `status`, `bytes`, `duration_ms`, `error`, `id`.
- Debug for entry points/handlers, Info for lifecycle/completions, Warn for degraded-but-alive states, Error before returning 500.

## Config

- Defaults live in `defaultConfig()` in `pkg/config/config.go`; config files only override.
- Secrets/DSNs should be injected via ENV templates: `"{{ ENV \"JWT_SECRET\" }}"`. Never commit real secrets.
- New config sections need: struct + mapstructure tags, defaults, and an entry in `docs/openapi` if they affect the API.

## Database

- Schema changes = new migration file `migrations/NNNNN_name.sql` with `-- +goose Up` / `-- +goose Down`. Never edit applied migrations.
- Soft delete pattern: `deleted_at TIMESTAMPTZ`; all reads filter `deleted_at IS NULL`; unique constraints intentionally survive deletion.
- Timestamps are DB-generated (`now()`), verified not assumed.
- Repos scan rows explicitly into models; `pgx.ErrNoRows` maps to `ErrNotFound`.

## Git / Make

- Commit style: `feat:` / `fix:` prefixes, concise scope summary.
- Make targets are grouped by service: `pg-*`, `redis-*`, aggregates `dev-up/dev-down/dev-clean`.
- Before committing: `go build ./... && go vet ./...` must be clean.
