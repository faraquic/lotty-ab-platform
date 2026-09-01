# AI Handoff — Lotty AB Platform

## Session Summary

Major refactoring and infrastructure session. Key changes: restructured panel service (removed `internal/` and `cmd/` layers), created dual-framework middleware (gin + fiber), added 52 structured logging field constants, created runtime and analytics service skeletons (Fiber v3), set up Docker Compose with nginx reverse proxy, rewrote Makefile for developer workflow, centralized bootstrap password as pre-hashed bcrypt in config.

## What Was Completed

### Project Restructure
- `services/panel/internal/` → `services/panel/` (flat domain structure)
- `services/panel/cmd/main.go` + `cmd/router.go` → `services/panel/main.go` + `router.go`
- Removed `internal/lib/` (unused utility packages)
- Removed `pkg/api/response_test.go`, `pkg/auth/auth_test.go`, `pkg/database/*_test.go` (stale tests)

### Dual-Framework Middleware (`pkg/middleware/`)

| File | Gin | Fiber |
|------|-----|-------|
| `request_id.go` | `RequestIDGin()`, `GetRequestIDGin(c)` | `RequestIDFiber()`, `GetRequestIDFiber(c)` |
| `recovery.go` | `RecoveryGin(log)` | `RecoveryFiber(log)` |
| `logger.go` | `LoggerGin(log)` | `LoggerFiber(log)` |
| `context.go` | `CallerIDGin(c)` | `CallerIDFiber(c)` |

### Logger Fields (`pkg/logger/fields.go`)
- 52 constants across 10 domain groups (service, HTTP, error, DB, cache, storage, auth, user, flag, metric)
- All string literals in zap calls replaced with `logger.Field*` constants across 18 files
- Fixed `http.status_code` → `http.response.status_code` inconsistency

### Config Changes (`pkg/config/config.go`)
- Added `LogLevel string` field (defaults: local/dev=debug, prod=info)
- Added `AnalyticsConfig` struct (port :8083)
- `ValidateSecurity()` moved from panel to config package
- `BootstrapConfig`: `password` field replaced by `password_hash` (bcrypt pre-hashed)

### Services

| Service | Framework | Port | Status |
|---------|-----------|------|--------|
| panel | Gin | :8081 | Built (auth, users, flags, metrics, health) |
| runtime | Fiber v3 | :8082 | Skeleton (health only) |
| analytics | Fiber v3 | :8083 | Skeleton (health only) |

### Infrastructure
- `Dockerfile` — multi-stage build, `ARG SERVICE`, `ARG PORT`
- `docker-compose.yml` — 8 services: postgres, redis, s3, panel, runtime, analytics, nginx
- `deploy/nginx/nginx.conf` — reverse proxy `/api/v1/{panel,runtime,analytics}/`
- `config.docker.json` — Docker config (hosts = container names)

### Makefile Rewrite

| Target | Description |
|--------|-------------|
| `dev-up` | Start infra containers (pg, redis, s3) |
| `dev-status` | Check which infra containers are running |
| `run-local` | Build + restart all services (fast, no infra restart) |
| `stop-local` | Stop all binary processes |
| `restart-local` | Stop + run-local |
| `run-panel/runtime/analytics` | Build + run single service |
| `build-all` | Compile all three binaries |
| `check` | build-all + vet + gofmt |
| `up` / `down` / `rebuild` | Docker Compose lifecycle |

Key design: `run-local` checks infra exists via `_ensure-infra` but does NOT restart it. Optimized for fast iteration.

### Auth Middleware
- Security logging: revoked token, role denied, auth failed with `client.address`
- Removed dead code: `auth.CallerID`, `Recovery()`, `RequestLogger()`, `RequestFields()`

## Files Changed

| File | Action | Why |
|------|--------|-----|
| `services/panel/main.go` | Created | Entrypoint (was `cmd/main.go`) |
| `services/panel/router.go` | Created | Routing (was `cmd/router.go`) |
| `services/panel/domain/` | Created | Domains moved from `internal/domain/` |
| `services/runtime/` | Created | Fiber v3 skeleton |
| `services/analytics/` | Created | Fiber v3 skeleton |
| `pkg/middleware/request_id.go` | Rewritten | Dual framework (gin + fiber) |
| `pkg/middleware/recovery.go` | Rewritten | Dual framework + shared PanicData |
| `pkg/middleware/logger.go` | Rewritten | Dual framework (gin + fiber) |
| `pkg/middleware/context.go` | Rewritten | CallerIDGin + CallerIDFiber |
| `pkg/logger/fields.go` | Created | 52 field constants |
| `pkg/logger/error_type.go` | Modified | Added SetErrorTypeFiber |
| `pkg/config/config.go` | Modified | AnalyticsConfig, LogLevel, password_hash |
| `Dockerfile` | Created | Multi-stage build |
| `docker-compose.yml` | Created | 8 services |
| `deploy/nginx/nginx.conf` | Created | Reverse proxy |
| `Makefile` | Rewritten | Developer workflow |
| `config.docker.json` | Created | Docker environment config |
| `AGENTS.md` | Rewritten | Updated architecture, commands |

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Dual middleware (gin + fiber) | Panel uses gin (mature ecosystem), runtime/analytics use fiber (performance) |
| `password_hash` replaces `password` | Security: no plaintext passwords in config files |
| `_ensure-infra` in run-local | Fast iteration: don't restart containers on every code change |
| nginx reverse proxy | Single entry point for all three services |
| `ARG SERVICE/PORT` in Dockerfile | Single Dockerfile for all services |

## Commands to Verify the Project

```bash
# Build all
make build-all

# Vet and format
make check

# Run tests
go test ./pkg/... -short -count=1

# Full stack (Docker Compose)
make up
make logs
make down

# Local development
make dev-up
make run-local
make stop-local
```

## What to Read Before Making Changes

1. **`AGENTS.md`** — project rules, constraints, code style
2. **`docs/openapi/panel.yaml`** — current API contract
3. **`pkg/config/config.go`** — all config structs and defaults
4. **`services/panel/router.go`** — how domains are wired together
5. **`services/panel/main.go`** — bootstrap order and dependency initialization
6. **`pkg/middleware/`** — dual-framework middleware (gin + fiber)
7. **`pkg/logger/fields.go`** — structured logging field constants
8. **`pkg/api/response.go`** — response helpers and `ValidateRequest` pattern
9. **`Makefile`** — developer workflow commands
