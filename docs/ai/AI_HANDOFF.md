# AI Handoff — Lotty AB Platform

## Session Summary

This session added the feature flags domain with a human-readable `name` field, replaced the `owner` audit field with separate `created_by`/`updated_by` foreign keys, and refined the list/detail endpoint response shapes: `GET /flags` omits user objects (serializes `created_by`/`updated_by` as `null`), while `GET /flags/:id` returns full user objects. All changes backed by migrations, OpenAPI spec updates, and e2e tests.

## What Was Completed

### Flags Domain (`services/panel/internal/domain/flags/`)
- **Model** (`model.go`): `Flag` struct has `CreatedBy`/`UpdatedBy` (bigint, NOT NULL); `FlagWithCreatorAndUpdater` wraps flag + full user objects
- **Repository** (`repository.go`): 
  - `GetByID`/`Update` join `users` twice (creator + updater) → return `FlagWithCreatorAndUpdater`
  - `List` queries **only** `flags` table (no joins) → returns `[]Flag` for performance
- **DTO** (`dto.go`): Single `FlagResponse` type; `ToResponse` populates `CreatedBy`/`UpdatedBy` from `FlagWithCreatorAndUpdater`; list uses `FlagWithCreatorAndUpdater{Flag: f}` with nil users so JSON omits them
- **Service** (`service.go`): `List` returns `PaginatedFlagResponse` with `FlagResponse` items where `CreatedBy`/`UpdatedBy` are `null`
- **Handler** (`handler.go`): Standard CRUD + list with pagination; 409 Conflict on duplicate key/name

### Database Migrations
- `00001_users.sql`: Combined users table (username, email unique, role enum, avatar_url nullable, soft delete, `update_updated_at` trigger)
- `00002_flags.sql`: `flags` table with `key` (unique), `name` (unique, max 256), `type` (string/number/bool), `default_value` (jsonb), `description` (nullable), `created_by`/`updated_by` FK → `users(id)`, soft delete, updated_at trigger
- `00003_metrics.sql`: (untracked — excluded from commit)

### E2E Tests (`tests/e2e/panel/`)
- `flags_test.go`: All create/get/list/update/delete tests; list tests assert `CreatedBy`/`UpdatedBy` are `null`
- `response_types_test.go`: `flagResponseData` has `CreatedBy`/`UpdatedBy` as `*userResponseData` (nil for list)
- `suite_test.go`: `decodePaginatedFlagsResponse` helper reused for list/detail

### OpenAPI Spec (`docs/openapi/panel.yaml`)
- `Flag` schema: `created_by`/`updated_by` reference `User` (required, present on detail)
- **New `FlagListItem` schema**: identical to `Flag` but **omits** `created_by`/`updated_by`/`description`
- `FlagListResponse` references `FlagListItem` (array)
- `CreateFlagRequest`/`UpdateFlagRequest` include `name` (required, 1–256)

## Files Changed

| File | Action | Why |
|------|--------|-----|
| `migrations/00001_users.sql` | Modified | Combined users migrations, added email UNIQUE, avatar_url nullable |
| `migrations/00002_flags.sql` | Modified | Flags table with name, created_by, updated_by, triggers, FKs |
| `services/panel/internal/domain/flags/model.go` | Modified | Flag, FlagWithCreatorAndUpdater |
| `services/panel/internal/domain/flags/repository.go` | Modified | List without user joins; GetByID/Update with double join |
| `services/panel/internal/domain/flags/dto.go` | Modified | FlagResponse, ToResponse (single type for list + detail) |
| `services/panel/internal/domain/flags/service.go` | Modified | List returns PaginatedFlagResponse with nil users |
| `services/panel/internal/domain/flags/handler.go` | Modified | Error handling for ErrConflictNames |
| `tests/e2e/panel/flags_test.go` | Modified | List tests verify null CreatedBy/UpdatedBy |
| `tests/e2e/panel/response_types_test.go` | Modified | Reused flagResponseData for list |
| `tests/e2e/panel/suite_test.go` | No change | decodePaginatedFlagsResponse reused |
| `docs/openapi/panel.yaml` | Modified | FlagListItem schema; FlagListResponse uses it |

## Git Commits (this session)

```
a65f8a6 feat(flags): list endpoint omits created_by/updated_by user objects
<previous commits from earlier session>
```

## Current Unfinished Tasks

- S3 is initialized and available but no domain logic uses it yet (no upload/download endpoints)
- Metrics/experiments domains not implemented
- No avatar upload endpoint implementation (S3 integration pending)

## Known Issues & Technical Debt

1. **MinIO health check** uses `mc ready local` — requires `mc` inside the container.
2. **S3 bucket provisioning** is explicit via `make dev-up` / `make test-e2e`.
3. **No S3 CORS config** — browser uploads will fail without it.
4. **`config.ServiceVersion`** is a `var`, not `const` — intentional (ldflags override).
5. **JWT secret defaults to `"change-me"`** — must override in production.
6. **Migration `00003_metrics.sql` and `00006_experiments.sql.~`** are untracked (excluded from commit).

## Important Decisions & Rationale

| Decision | Rationale |
|----------|-----------|
| List endpoint omits user objects | Performance: avoids 2 JOINs per row; list often returns many rows |
| Single `FlagResponse` type with nil users | Simpler than separate types; `omitempty` on pointers serializes as absent/null |
| `FlagListItem` in OpenAPI without user refs | Documents actual list response shape; detail endpoint uses `Flag` |
| `name` unique + max 256 | Human-readable identifier; separate from machine `key` |
| `created_by`/`updated_by` NOT NULL + FK | Audit trail integrity; every flag has creator/updater |
| Soft delete (`deleted_at`) on users/flags | Referential integrity for audit fields; no cascade deletes |

## Commands to Verify the Project

```bash
# Build and vet
go build ./... && go vet ./...

# Run all tests (short mode, skips integration)
go test ./pkg/... -short -count=1

# Run e2e tests (requires dev containers: make dev-up)
make test-e2e

# Format check
gofmt -l pkg/ services/ tests/ | grep -v '^$' || echo 'clean'

# Start dev containers
make dev-up

# Apply migrations (dev DB: labp on localhost:5432)
make pg-migrate-up

# Apply migrations (e2e DB: labp_e2e on localhost:5433)
go tool goose -dir migrations postgres "postgres://lotty:lottypassword@localhost:5433/labp_e2e?sslmode=disable" up

# Build with version
make build

# Run the service
./bin/panel

# Health check
curl http://localhost:8080/api/panel/v1/health
curl http://localhost:8080/api/panel/v1/ready

# Flags list (no user objects)
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/panel/v1/flags

# Flag detail (with user objects)
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/panel/v1/flags/1
```

## What to Read Before Making Changes

1. **`AGENTS.md`** — project rules, constraints, code style
2. **`docs/openapi/panel.yaml`** — current API contract (must stay aligned)
3. **`pkg/config/config.go`** — all config structs and defaults
4. **`services/panel/cmd/router.go`** — how domains are wired together
5. **`services/panel/cmd/main.go`** — bootstrap order and dependency initialization
6. **`services/panel/internal/domain/flags/`** — flags domain (model, repo, dto, service, handler)
7. **`pkg/api/response.go`** — response helpers and `ValidateRequest` pattern
8. **`tests/e2e/panel/flags_test.go`** — expected API behavior