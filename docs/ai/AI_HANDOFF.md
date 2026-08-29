# AI Handoff — Lotty AB Platform

## Session Summary

This session added the feature flags domain with a human-readable `name` field, replaced the `owner` audit field with separate `created_by`/`updated_by` foreign keys, and refined the list/detail endpoint response shapes: `GET /flags` omits user objects (serializes `created_by`/`updated_by` as `null`), while `GET /flags/:id` returns full user objects. Additionally, the metrics domain was implemented as a configuration entity for future A/B-test analytics.

## What Was Completed

### Flags Domain (`services/panel/internal/domain/flags/`)
- **Model** (`model.go`): `Flag` struct has `CreatedBy`/`UpdatedBy` (bigint, NOT NULL); `FlagWithCreatorAndUpdater` wraps flag + full user objects
- **Repository** (`repository.go`): 
  - `GetByID`/`Update` join `users` twice (creator + updater) -> return `FlagWithCreatorAndUpdater`
  - `List` queries **only** `flags` table (no joins) -> returns `[]Flag` for performance
- **DTO** (`dto.go`): Single `FlagResponse` type; `ToResponse` populates `CreatedBy`/`UpdatedBy` from `FlagWithCreatorAndUpdater`; list uses `FlagWithCreatorAndUpdater{Flag: f}` with nil users so JSON omits them
- **Service** (`service.go`): `List` returns `PaginatedFlagResponse` with `FlagResponse` items where `CreatedBy`/`UpdatedBy` are `null`
- **Handler** (`handler.go`): Standard CRUD + list with pagination; 409 Conflict on duplicate key/name

### Metrics Domain (`services/panel/internal/domain/metrics/`)
- **Model** (`model.go`): `MetricConfig` (byte-slice type with JSON marshal/unmarshal/Scan/Value); `MetricType` (count/sum/unique_count/ratio/percentile/average); `MetricStatus` (active/archived); `Metric` struct; `MetricWithCreatorAndUpdater`
- **Repository** (`repository.go`):
  - `GetByID`/`Update` join `users` twice (creator + updater) -> return `MetricWithCreatorAndUpdater`
  - `List` queries **only** `metrics` table (no joins) -> returns `[]Metric` for performance
  - All queries filter `status = 'active'` (no `deleted_at` column)
  - DELETE sets `status = 'archived'` (soft delete via status)
- **DTO** (`dto.go`): `CreateMetricRequest`, `UpdateMetricRequest`, `MetricResponse`, `PaginatedMetricResponse`, `ToResponse`
- **Service** (`service.go`): Validates metric type, status, config JSON objects; enforces built-in protections; immutable `metric_type` after creation
- **Handler** (`handler.go`): Standard CRUD + list with pagination; 409 Conflict on duplicate key/name; 403 Forbidden for built-in metric violations

### Database Migrations
- `00001_users.sql`: Combined users table (username, email unique, role enum, avatar_url nullable, soft delete, `update_updated_at` trigger)
- `00002_flags.sql`: `flags` table with `key` (unique), `name` (unique, max 256), `type` (string/number/bool), `default_value` (jsonb), `description` (nullable), `created_by`/`updated_by` FK -> `users(id)`, soft delete, updated_at trigger
- `00003_metrics.sql`: `metrics` table with `key` (unique), `name` (unique, max 256), `metric_type` (enum), `aggregation`/`attribution` (jsonb), `is_builtin`, `status` (active/archived), `created_by`/`updated_by` FK -> `users(id)`, updated_at trigger. No `deleted_at` column -- soft delete uses `status = 'archived'`.

### E2E Tests (`tests/e2e/panel/`)
- `flags_test.go`: All create/get/list/update/delete tests; list tests assert `CreatedBy`/`UpdatedBy` are `null`
- `metrics_test.go`: Full E2E coverage for metrics: create (all types), validation, get, list/pagination, update, delete, auth/RBAC, audit field verification, secrets check, repeated delete, negative IDs
- `response_types_test.go`: `metricResponseData` and `paginatedMetricData` types; `decodeMetricResponse` and `decodePaginatedMetricsResponse` helpers
- `suite_test.go`: `cleanupTestState` truncates `metrics, flags, users`

### OpenAPI Spec (`docs/openapi/panel.yaml`)
- `Flag` schema: `created_by`/`updated_by` reference `User` (required, present on detail)
- **New `FlagListItem` schema**: identical to `Flag` but **omits** `created_by`/`updated_by`/`description`
- `FlagListResponse` references `FlagListItem` (array)
- **New `Metric` schema**: full detail with `created_by`/`updated_by` as `User` refs
- **New `MetricListItem` schema**: omits `created_by`/`updated_by` for list performance
- `MetricListResponse` references `MetricListItem` (array)
- `CreateMetricRequest`/`UpdateMetricRequest` with `metric_type` immutability documented
- `MetricType` enum, `MetricStatus` enum

## Design Decisions

### Metrics Domain

| Decision | Rationale |
|----------|-----------|
| Soft delete via `status = 'archived'` (no `deleted_at`) | Simpler schema; `status` field already exists for business lifecycle; archive = delete |
| `metric_type` immutable after creation | Aggregation/attribution semantics depend on type; changing type breaks historical meaning |
| `key`/`name` permanently unique (even after archive) | May be referenced by experiments/events/reporting; prevents confusion |
| Built-in metrics cannot be modified or deleted | Protects system-defined metrics from accidental changes |
| `aggregation`/`attribution` validated as non-empty JSON objects | Type-specific schema validation deferred to future work |
| List omits user objects (same as flags) | Performance: avoids 2 JOINs per row |
| Authorization matches flags (any authenticated user) | Consistent with existing RBAC policy |
| `MetricConfig` custom byte-slice type | Prevents base64 encoding in JSON; implements `json.Marshaler`/`json.Unmarshaler`/`sql.Scanner`/`driver.Valuer` |

## Files Changed

| File | Action | Why |
|------|--------|-----|
| `migrations/00003_metrics.sql` | Modified | Normalized: added `updated_by` FK, removed untracked status |
| `services/panel/internal/domain/metrics/model.go` | Created | MetricConfig, MetricType, MetricStatus, Metric, MetricWithCreatorAndUpdater |
| `services/panel/internal/domain/metrics/dto.go` | Modified | CreateMetricRequest, UpdateMetricRequest, MetricResponse, PaginatedMetricResponse, ToResponse |
| `services/panel/internal/domain/metrics/repository.go` | Created | CRUD, soft delete via status, pagination, audit joins |
| `services/panel/internal/domain/metrics/service.go` | Created | Validation, business logic, built-in protections |
| `services/panel/internal/domain/metrics/handler.go` | Created | HTTP handlers, route registration, error mapping |
| `services/panel/cmd/router.go` | Modified | Registered metrics domain on `anyAuthGroup` |
| `tests/e2e/panel/metrics_test.go` | Created | Full E2E test coverage |
| `tests/e2e/panel/response_types_test.go` | Modified | Added metric response types and decode helpers |
| `tests/e2e/panel/suite_test.go` | Modified | Added `metrics` to TRUNCATE statement |
| `docs/openapi/panel.yaml` | Modified | Added metrics tag, paths, schemas |
| `docs/ai/AI_HANDOFF.md` | Modified | Updated session summary and decisions |

## Unimplemented / Future Work

- Event ingestion
- Metric calculation and aggregation workers
- Experiment linkage and assignments
- Type-specific metric configuration schemas (e.g., `ratio` numerator/denominator)
- Metric unarchiving endpoint
- Dashboards and reporting
- Asynchronous jobs
- Metrics collection SDKs

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

# Apply migrations (e2e DB)
go tool goose -dir migrations postgres "postgres://lotty:lottypassword@localhost:5433/labp_e2e?sslmode=disable" up
```

## What to Read Before Making Changes

1. **`AGENTS.md`** — project rules, constraints, code style
2. **`docs/openapi/panel.yaml`** — current API contract (must stay aligned)
3. **`pkg/config/config.go`** — all config structs and defaults
4. **`services/panel/cmd/router.go`** — how domains are wired together
5. **`services/panel/cmd/main.go`** — bootstrap order and dependency initialization
6. **`services/panel/internal/domain/flags/`** — flags domain (model, repo, dto, service, handler)
7. **`services/panel/internal/domain/metrics/`** — metrics domain (model, repo, dto, service, handler)
8. **`pkg/api/response.go`** — response helpers and `ValidateRequest` pattern
9. **`tests/e2e/panel/flags_test.go`** — expected API behavior for flags
10. **`tests/e2e/panel/metrics_test.go`** — expected API behavior for metrics
