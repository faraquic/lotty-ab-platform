# AI Handoff — Lotty AB Platform

> Updated: 2026-09-27. The current-state notes below supersede the historical session snapshot.

## Current State

The workspace contains three Go services. `panel` is the control plane (Gin, PostgreSQL, Redis-backed
sessions and snapshot refresh); `runtime` serves deterministic decisions from an in-memory snapshot;
`analytics` is still a health-only skeleton. OpenAPI files under `docs/openapi/` are the API source
of truth. There is no usable frontend in the current working tree; frontend files appear deleted,
so do not restore them unless explicitly asked.

The worktree is intentionally treated as user-owned and may contain unrelated edits, deletions, and
untracked files. Check `git status` before every edit. In particular, `AGENTS.md` has local edits,
and `docs/task/` is gitignored; its current audit is useful local context but is not necessarily
tracked or present in another checkout.

### Requirement Audit Snapshot

See `docs/task/todo.md` for the detailed requirement matrix, implementation evidence, test gaps,
priorities, and rough effort estimates. The audit is based on the working tree at 2026-09-27. The
most consequential gaps are:

- Targeting parsing exists in `pkg/targeting`, but runtime does not evaluate request attributes.
- Runtime fallback does not yet enforce the too-old snapshot behavior from the specification.
- Audit support exists in `pkg/audit/` and migration `00007`, but coverage is incomplete and
  append-only enforcement is not established.
- Idempotency keys, participation limits, Kafka/outbox, Events API, ClickHouse attribution/reports,
  automated guardrails, notifications, and Prometheus/Loki/Grafana are not implemented.
- Performance/SLO targets and the corresponding failure, load, and recovery tests are unverified.

The current worktree also has audit-related source files and tests that may be untracked. Do not
assume migration `00007` or any other local change is committed. For detailed status, trust a fresh
inspection of the code, not old checkboxes or this summary.

### Current Architecture Map

| Area             | Current location                                   | State                                                          |
| ---------------- | -------------------------------------------------- | -------------------------------------------------------------- |
| Control plane    | `services/panel/`                                  | Auth, users, flags, metrics, experiments, reviews, health      |
| Runtime          | `services/runtime/domain/decide/`                  | Snapshot-based assignment/hash/defaults; targeting gap remains |
| Analytics        | `services/analytics/`                              | Health endpoint only                                           |
| Shared targeting | `pkg/targeting/`                                   | Lexer/parser/AST; runtime evaluator not wired                  |
| Snapshot         | `pkg/snapshot/`, `services/panel/snapshot/`        | Redis persistence/watch plus local in-memory view              |
| Audit            | `pkg/audit/`, `migrations/00007_audit_records.sql` | Partial mutation coverage in current tree                      |
| API contracts    | `docs/openapi/`                                    | Source of truth; synchronize contract and implementation       |

Runtime's current public route is `/api/v1/runtime/decide`. The task specification also describes
`/runtime/v1/decide`; resolve any route change against OpenAPI and nginx together instead of inventing
or changing an endpoint in isolation.

### Before Changing Code

1. Read `AGENTS.md` and check `git status`; preserve all pre-existing user changes.
2. Read `docs/openapi/*.yaml` for public API behavior.
3. For task requirements, use `docs/task/task.ru.md`; use `docs/task/todo.md` as a local audit/backlog, not as a substitute for code evidence.
4. Keep changes within the owning domain and add focused tests for behavior changes.
5. Never expose secret values from local config or environment files in reports or diffs.

### Validation

From repository root:

```bash
make check
go test ./pkg/... ./services/... -short -count=1
```

`make check` builds and vets but does not run Go tests. E2E tests require infrastructure and run
serially: `make dev-up && make test-e2e`. Do not run destructive targets such as `make dev-clean`,
`make pg-migrate-down`, or `make down` without explicit need and approval.

## Historical Session Snapshot (2026-09-24)

The notes below describe an earlier refactoring session. Treat them as history, not as a verified
description of the current worktree.

## Session Summary

Major refactoring and infrastructure session. Key changes: restructured panel service (removed `internal/` and `cmd/` layers), created dual-framework middleware (gin + fiber), added 52 structured logging field constants, created runtime and analytics service skeletons (Fiber v3), set up Docker Compose with nginx reverse proxy, rewrote Makefile for developer workflow, centralized bootstrap password as pre-hashed bcrypt in config.

## What Was Completed

### Project Restructure

- `services/panel/internal/` → `services/panel/` (flat domain structure)
- `services/panel/cmd/main.go` + `cmd/router.go` → `services/panel/main.go` + `router.go`
- Removed `internal/lib/` (unused utility packages)
- Removed `pkg/api/response_test.go`, `pkg/auth/auth_test.go`, `pkg/database/*_test.go` (stale tests)

### Dual-Framework Middleware (`pkg/middleware/`)

| File            | Gin                                    | Fiber                                      |
| --------------- | -------------------------------------- | ------------------------------------------ |
| `request_id.go` | `RequestIDGin()`, `GetRequestIDGin(c)` | `RequestIDFiber()`, `GetRequestIDFiber(c)` |
| `recovery.go`   | `RecoveryGin(log)`                     | `RecoveryFiber(log)`                       |
| `logger.go`     | `LoggerGin(log)`                       | `LoggerFiber(log)`                         |
| `context.go`    | `CallerIDGin(c)`                       | `CallerIDFiber(c)`                         |

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

| Service   | Framework | Port  | Status                                      |
| --------- | --------- | ----- | ------------------------------------------- |
| panel     | Gin       | :8081 | Built (auth, users, flags, metrics, health) |
| runtime   | Fiber v3  | :8082 | Skeleton (health only)                      |
| analytics | Fiber v3  | :8083 | Skeleton (health only)                      |

### Infrastructure

- `Dockerfile` — multi-stage build, `ARG SERVICE`, `ARG PORT`
- `docker-compose.yml` — 8 services: postgres, redis, s3, panel, runtime, analytics, nginx
- `deploy/nginx/nginx.conf` — reverse proxy `/api/v1/{panel,runtime,analytics}/`
- `config.example.json` — shared environment-templated example; the former separate Docker config was removed

### Makefile Rewrite

| Target                        | Description                                           |
| ----------------------------- | ----------------------------------------------------- |
| `dev-up`                      | Start infra containers (pg, redis, s3)                |
| `dev-status`                  | Check which infra containers are running              |
| `run-local`                   | Build + restart all services (fast, no infra restart) |
| `stop-local`                  | Stop all binary processes                             |
| `restart-local`               | Stop + run-local                                      |
| `run-panel/runtime/analytics` | Build + run single service                            |
| `build-all`                   | Compile all three binaries                            |
| `check`                       | build-all + vet + gofmt                               |
| `up` / `down` / `rebuild`     | Docker Compose lifecycle                              |

Key design: `run-local` checks infra exists via `_ensure-infra` but does NOT restart it. Optimized for fast iteration.

### Auth Middleware

- Security logging: revoked token, role denied, auth failed with `client.address`
- Removed dead code: `auth.CallerID`, `Recovery()`, `RequestLogger()`, `RequestFields()`

## Files Changed

| File                           | Action    | Why                                      |
| ------------------------------ | --------- | ---------------------------------------- |
| `services/panel/main.go`       | Created   | Entrypoint (was `cmd/main.go`)           |
| `services/panel/router.go`     | Created   | Routing (was `cmd/router.go`)            |
| `services/panel/domain/`       | Created   | Domains moved from `internal/domain/`    |
| `services/runtime/`            | Created   | Fiber v3 skeleton                        |
| `services/analytics/`          | Created   | Fiber v3 skeleton                        |
| `pkg/middleware/request_id.go` | Rewritten | Dual framework (gin + fiber)             |
| `pkg/middleware/recovery.go`   | Rewritten | Dual framework + shared PanicData        |
| `pkg/middleware/logger.go`     | Rewritten | Dual framework (gin + fiber)             |
| `pkg/middleware/context.go`    | Rewritten | CallerIDGin + CallerIDFiber              |
| `pkg/logger/fields.go`         | Created   | 52 field constants                       |
| `pkg/logger/error_type.go`     | Modified  | Added SetErrorTypeFiber                  |
| `pkg/config/config.go`         | Modified  | AnalyticsConfig, LogLevel, password_hash |
| `Dockerfile`                   | Created   | Multi-stage build                        |
| `docker-compose.yml`           | Created   | 8 services                               |
| `deploy/nginx/nginx.conf`      | Created   | Reverse proxy                            |
| `Makefile`                     | Rewritten | Developer workflow                       |
| `config.example.json`          | Added     | Replaces the former separate Docker config |
| `AGENTS.md`                    | Rewritten | Updated architecture, commands           |

## Design Decisions

| Decision                            | Rationale                                                                    |
| ----------------------------------- | ---------------------------------------------------------------------------- |
| Dual middleware (gin + fiber)       | Panel uses gin (mature ecosystem), runtime/analytics use fiber (performance) |
| `password_hash` replaces `password` | Security: no plaintext passwords in config files                             |
| `_ensure-infra` in run-local        | Fast iteration: don't restart containers on every code change                |
| nginx reverse proxy                 | Single entry point for all three services                                    |
| `ARG SERVICE/PORT` in Dockerfile    | Single Dockerfile for all services                                           |

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
