# AI Handoff — Lotty AB Platform

## Session Summary

This session added S3 object storage support to the panel service, completing the storage layer alongside the existing Postgres and Redis integrations.

## What Was Completed

### S3 Client (`pkg/database/s3.go`)
- `S3Config` struct with `Bucket`, `Region`, `Endpoint`, `AccessKey`, `SecretKey` fields
- `NewS3(ctx, S3Config, logger)` constructor that:
  - Loads AWS config via `aws-sdk-go-v2/config`
  - Supports custom endpoints (MinIO) with path-style addressing
  - Verifies bucket connectivity with `HeadBucket` using `pingTimeout` (3s)
  - Does **not** create buckets or modify bucket policies
  - Returns `*s3.Client` or error

### S3 Tests (`pkg/database/s3_test.go`)
- 6 test cases:
  - Empty bucket → `"s3 bucket is empty"` error
  - Unreachable endpoint (192.0.2.1) → connection timeout
  - Connection refused (localhost:1) → connection error
  - Non-existent bucket on MinIO (integration, skipped in `-short`)
  - Valid bucket on MinIO (integration, skipped in `-short`)
  - Context cancelled → timeout error
- Helper `isS3Reachable(addr)` for integration test gating

### Config (`pkg/config/config.go`)
- Added `S3Config` struct: `Bucket`, `Region`, `Endpoint`
- Added `S3` field to `DatabaseConfig`
- Defaults: `bucket=labp`, `region=us-east-1`, `endpoint=http://localhost:9000`

### Main Bootstrap (`services/panel/cmd/main.go`)
- S3 initialized after Redis (lines 60–73)
- Optional: warn and continue on failure (like Redis, unlike Postgres)
- S3 client passed to router

### Router (`services/panel/cmd/router.go`)
- `newRouter` signature updated to accept `*s3.Client`
- S3 client passed to `healthdomain.NewHandler`

### Health Probes (`services/panel/internal/domain/health/handler.go`)
- `Handler` struct updated with `s3 *s3.Client` field
- `NewHandler` accepts S3 client as 3rd parameter
- `checkS3(ctx)` method added: uses `ListBuckets` as a connectivity ping
- `/ready` response now includes `storage` component
- S3 unavailable reports status but **does not fail readiness** (like Redis)

### Makefile
- Added S3/MinIO variables: `S3_IMAGE`, `S3_NAME`, `S3_PORT`, `S3_CONSOLE_PORT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`
- Added targets: `s3-up`, `s3-down`, `s3-logs`, `s3-clean`
- `dev-up`, `dev-down`, `dev-clean` aggregates updated to include S3

### OpenAPI (`docs/openapi/panel.yaml`)
- `ReadyResponse.components` updated with `storage` property

## Files Changed

| File | Action | Why |
|------|--------|-----|
| `pkg/database/s3.go` | Created | S3 client constructor |
| `pkg/database/s3_test.go` | Created | S3 client tests |
| `pkg/config/config.go` | Modified | Added S3Config to DatabaseConfig |
| `services/panel/cmd/main.go` | Modified | Initialize S3, pass to router |
| `services/panel/cmd/router.go` | Modified | Accept *s3.Client, pass to health handler |
| `services/panel/internal/domain/health/handler.go` | Modified | Add S3 check to /ready |
| `Makefile` | Modified | Add S3/MinIO dev container targets |
| `docs/openapi/panel.yaml` | Modified | Add storage to ReadyResponse |
| `go.mod` / `go.sum` | Modified | AWS SDK v2 dependencies |

## Git Commits (this session)

```
3c7d0c4 feat(health): add S3 storage check to /ready probe
aed1285 feat(s3): add S3/MinIO client, config, tests, and dev container
5445abc test(database): add tests for redis.go — empty/invalid/unreachable/ping failures
1820d10 feat(api): add GET /me for authenticated users, rename login hash_password to password
aeecaee fix(openapi): align spec with backend — server URL, 403/409 responses, query defaults
ff954bb feat(panel): bootstrap admin, self-management protections, health probes, tests
6a03b50 docs: agent guidance, project context, conventions, task status
6545ce8 feat: users domain CRUD, auth with JWT middleware and redis sessions
6d0d44a feat: panel service skeleton — users domain, postgres/redis, migrations, CORS, openapi spec
```

## Current Unfinished Tasks

- S3 is not yet used by any domain logic — it's initialized and available but no endpoints upload/download objects
- No integration tests that exercise S3 operations (put/get/delete objects)
- No tests for the health handler with S3 scenarios

## Known Issues & Technical Debt

1. **MinIO health check** uses `mc ready local` — requires `mc` inside the container. Verify this works with the official MinIO image.
2. **S3 bucket provisioning is explicit** — `make dev-up` provisions the `labp` bucket via `tools/s3-provision`; `make test-e2e` provisions `labp-e2e`. Buckets are created idempotently with public-read avatar policies.
3. **No S3 CORS config** — MinIO defaults have no CORS; browser-based uploads will fail without it.
4. **`config.ServiceVersion`** is a `var`, not `const` — this is intentional (overridden via ldflags) but could confuse new contributors.
5. **JWT secret defaults to `"change-me"`** — fine for local dev but must be overridden in production.

## Important Decisions & Rationale

| Decision | Rationale |
|----------|-----------|
| S3 uses `*s3.Client` (concrete), not interface | AWS SDK v2 returns concrete types; wrapping in interface adds indirection with no benefit |
| S3 failure is warning, not fatal | Keeps service operational for auth/users even if storage is misconfigured |
| `HeadBucket` for connectivity check | Lightweight, requires only `s3:GetBucketLocation` permission; does not mutate state |
| `NewS3` does not create buckets | Buckets are infrastructure; provisioning belongs in `make dev-up` / `make test-e2e`, not application startup |
| Path-style addressing for MinIO | MinIO doesn't support virtual-hosted-style by default |
| Health check uses `ListBuckets` not `HeadBucket` | `ListBuckets` is a lighter operation and doesn't require bucket-specific permissions |
| `/ready` fails only on database | Consistent with design: DB is the critical dependency; cache and storage are optional |
| Explicit bucket provisioning via `tools/s3-provision` | Uses AWS SDK v2 (existing dependency); idempotent; works on any environment without external tooling |

## Commands to Verify the Project

```bash
# Build and vet
go build ./... && go vet ./...

# Run all tests (short mode, skips integration)
go test ./pkg/... -short -count=1

# Run S3 tests only
go test ./pkg/database/ -v -short -run TestNewS3

# Format check
gofmt -l pkg/ services/ | grep -v '^$' || echo 'clean'

# Start dev containers
make dev-up

# Apply migrations
make pg-migrate-up

# Build with version
make build

# Run the service
./bin/panel

# Health check
curl http://localhost:8080/api/panel/v1/health
curl http://localhost:8080/api/panel/v1/ready
```

## What to Read Before Making Changes

1. **`AGENTS.md`** — project rules, constraints, code style
2. **`docs/openapi/panel.yaml`** — current API contract (must stay aligned)
3. **`pkg/config/config.go`** — all config structs and defaults
4. **`services/panel/cmd/router.go`** — how domains are wired together
5. **`services/panel/cmd/main.go`** — bootstrap order and dependency initialization
6. **Relevant domain handler** — e.g., `services/panel/internal/domain/users/handler.go` for user endpoints
7. **`pkg/api/response.go`** — response helpers and `ValidateRequest` pattern
