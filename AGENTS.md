# AGENTS.md — Lotty AB Platform

## Purpose

Control-plane backend for the Lotty A/B testing platform. Provides user management, authentication, RBAC, and health/readiness probes for the panel service. Designed as a Go monorepo with shared libraries under `pkg/` and one service under `services/`.

## Tech Stack

- **Language:** Go 1.27
- **HTTP:** Gin (`github.com/gin-gonic/gin`)
- **JSON:** `goccy/go-json` (high-performance JSON codec)
- **Database:** PostgreSQL via pgx v5 (`pgxpool`)
- **Cache:** Redis via rueidis (client-side caching, cluster-aware)
- **Object Storage:** S3 via `aws-sdk-go-v2` (MinIO for local dev)
- **Auth:** JWT (HS256) via `golang-jwt/jwt/v5`, Redis-backed sessions
- **Migrations:** goose (`github.com/pressly/goose/v3`)
- **Config:** viper with `{{ ENV "..." }}` template rendering
- **Logging:** zap
- **Validation:** `go-playground/validator/v10`

## Directory Structure

```
├── cmd/router.go          — route wiring, middleware, domain registration
├── cmd/main.go            — bootstrap, signal handling, graceful shutdown
├── migrations/            — goose SQL migrations (numbered)
├── pkg/
│   ├── api/               — response helpers (OK, Error, InternalError, ValidateRequest)
│   ├── auth/              — JWT manager (generate, parse, keyFunc)
│   ├── config/            — viper config loading, typed structs, env templates
│   ├── database/          — NewPostgres, NewRedis, NewS3 constructors
│   ├── logger/            — zap logger setup
│   └── middleware/        — RequestID, Logger middleware
├── services/
│   └── panel/             — the single service (control plane)
│       ├── cmd/           — main.go, router.go
│       └── internal/
│           └── domain/
│               ├── auth/  — login, JWT token creation, middleware, Redis sessions
│               ├── flags/ — feature flags CRUD, list/detail endpoints
│               ├── health/ — /health, /ready probes
│               └── users/ — CRUD, self-management protections, RBAC
└── docs/openapi/panel.yaml — OpenAPI 3.0.3 spec (must stay aligned with backend)
```

## Commands

```bash
# Dev containers
make dev-up          # start Postgres + Redis + S3/MinIO
make dev-down        # stop all containers
make dev-clean       # stop and remove volumes

# Build
make build           # compile to bin/panel with version ldflags

# Database
make pg-migrate-up   # apply all pending migrations
make pg-migrate-down # rollback last migration
make pg-migrate-status

# Tests
go test ./pkg/... -short -count=1
go test ./pkg/database/ -short -run TestNewS3 -v

# Lint / Vet
go vet ./...
gofmt -l pkg/ services/

# Run (requires dev containers up)
./bin/panel           # or: go run ./services/panel/cmd
```

## Code Style & Conventions

- **JSON codec:** Use `goccy/go-json`, not `encoding/json`. The router and all DTOs depend on it.
- **Errors:** Return domain errors; use `pkg/api.OK/Error/InternalError` for responses. Call `ValidateRequest` to parse binding errors.
- **Validation tags:** `binding:"required,email"`, `binding:"min=3,max=64"`, `binding:"omitempty,email"`.
- **ID path params:** Use `gin.Param("id")` and `strconv.ParseUint` — never `c.ShouldBindUri`.
- **Rueidis client:** `rueidis.Client` is an interface. `NewRedis` returns `*rueidis.Client`. Callers dereference with `(*client).Do(...)` and `(*client).Close()`.
- **S3 client:** `*s3.Client` is a concrete type (not interface). Callers use `s3Client.Method(...)` directly.
- **Config versions:** S3 uses config structs (`database.S3.*`), not raw strings like Redis.
- **Environment:** Acceptable values: `"local"`, `"dev"`, `"prod"`.
- **No comments in code** unless explicitly requested.
- **No emojis** in code, commit messages, or docs unless explicitly requested.

## Rules & Constraints

1. **Health routes live under `/api/panel/v1`** — never at root (`/health`, `/ready`). This was a deliberate design decision.
2. **`ServiceVersion` is a `var`** (not const) in `pkg/config/config.go` — overridden via `-ldflags "-X ..."`. Never make it a const.
3. **`ValidateRequest` uses `io.LimitReader`** because `goccy/go-json` swallows `*http.MaxBytesError`. Body size is checked by byte length, not by `MaxBytesReader`.
4. **Postgres pool creation:** `MaxConns`/`MinConns` must be set on `cfg` **before** calling `pgxpool.NewWithConfig`, not after.
5. **Redis and S3 are optional** — their unavailability produces a warning, not a fatal error. Database failure is the only fatal case.
6. **Self-management protections:** Users cannot delete themselves (403), cannot change their own role (403), and the last admin cannot be demoted or deleted (409). These are enforced at the service layer.
7. **Bootstrap admin** runs on every startup if `auth.bootstrap.password` is set and the users table is empty. Password must be ≥ 8 characters; empty password produces a warning, not a failure.
8. **OpenAPI spec** at `docs/openapi/panel.yaml` must stay aligned with the backend. Any new endpoint or changed response schema must update both code and spec.
9. **Flags list endpoint** (`GET /api/panel/v1/flags`) returns `created_by`/`updated_by` as `null` — no user object joins. The detail endpoint (`GET /flags/:id`) returns full user objects.
10. **Flag names are unique** — duplicate names return 409 Conflict. The `name` field is required and max 256 chars.

## Required Checks Before Completing a Task

```bash
go build ./... && go vet ./... && go test ./pkg/... -short -count=1
gofmt -l pkg/ services/ | grep -v '^$' || echo 'clean'
```

All must pass. No output from `gofmt -l` means files are formatted.

## Where to Find Detailed Context

- **Session recap:** `docs/ai/AI_HANDOFF.md`
- **OpenAPI spec:** `docs/openapi/panel.yaml`
- **Migration history:** `migrations/`
