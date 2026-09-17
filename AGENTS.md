# AGENTS.md — Lotty AB Platform

> **Живой документ.** Обновляется при изменениях в архитектуре, доменных правилах или статусе реализации.
>
> **Последнее обновление:** 2026-09-17

---

## 1. Проект

**Lotty AB Platform** — self-hosted A/B testing platform для feature flags, controlled experiments, review workflow, runtime variant delivery, event collection, attribution, analytics и guardrails.

**Формат:** Production-ready pet project для портфолио (solo dev + AI). Запускается локально через Docker Compose или бинарники.

**Frontend:** React SPA (`frontend/`, Mantine + React Query) — auth shell (login, `RequireAuth`) и public Status Page (`/status`) готовы, доменные страницы впереди. Детали — в `frontend/AGENTS.md`.

**Аудитория жюри:** 30-минутная проверка по критериям B1-B10.

---

## 2. Tech Stack

| Компонент                 | Технология                                                                               |
| ------------------------- | ---------------------------------------------------------------------------------------- |
| Language                  | Go 1.27                                                                                  |
| HTTP (panel)              | Gin (`github.com/gin-gonic/gin`)                                                         |
| HTTP (runtime, analytics) | Fiber v3 (`github.com/gofiber/fiber/v3`)                                                 |
| JSON                      | `goccy/go-json` (не `encoding/json`)                                                     |
| Database                  | PostgreSQL via pgx v5 (`pgxpool`)                                                        |
| Cache                     | Redis via rueidis (client-side caching, cluster-aware)                                   |
| Object Storage            | S3 via `aws-sdk-go-v2` (MinIO для local dev)                                             |
| Auth                      | JWT (HS256) via `golang-jwt/jwt/v5`, Redis-backed sessions                               |
| Migrations                | goose (`github.com/pressly/goose/v3`)                                                    |
| Config                    | viper с `{{ ENV "..." }}` template rendering                                             |
| Logging                   | zap (structured JSON)                                                                    |
| Validation                | `go-playground/validator/v10`                                                            |
| Hashing                   | xxhash64 (deterministic variant assignment)                                              |
| Password                  | Argon2id (bcrypt cost=13 для legacy), bootstrap: pre-hashed bcrypt                       |
| Frontend                  | React 19 + TypeScript strict + Vite 8, Mantine v8, React Query v5, i18next (en/ru), pnpm |
| Frontend serving          | Собственный nginx (SPA fallback `index.html`, `/health`), за compose-nginx на `/`        |

---

## 3. Архитектура

```
services/
├── panel/              — control plane (Gin, port :8081)
│   ├── main.go         — entrypoint, DI wiring
│   ├── router.go       — route registration, middleware chain
│   ├── snapshot/       — runtime snapshot composer (writer/reader, map lookup)
│   └── domain/         — domain packages
│       ├── auth/       — JWT, Redis sessions, RBAC middleware
│       ├── users/      — CRUD, avatar storage (S3)
│       ├── flags/      — CRUD, typed default values
│       ├── metrics/    — CRUD, aggregation config
│       ├── experiments/— CRUD + lifecycle (Phase 6+9)
│       ├── reviews/    — approver groups, thresholds, comment threads (Phase 8)
│       └── health/     — liveness/readiness probes
├── runtime/            — decide API (Fiber v3, port :8082)
│   ├── main.go         — entrypoint
│   ├── app.go          — DI, middleware, routing
│   └── domain/
│       ├── decide/     — xxhash64 buckets, allocation, targeting stub
│       └── health/     — liveness/readiness probes
└── analytics/          — event ingestion, attribution (Fiber v3, port :8083) [skeleton]
    ├── main.go         — entrypoint
    ├── app.go          — DI, middleware, routing
    └── domain/
        └── health/     — liveness/readiness probes

pkg/
├── api/                — response helpers (OK, Error, InternalError, ValidateRequest)
├── auth/               — JWT manager (generate, parse, keyFunc)
├── config/             — viper config loading, typed structs, env templates
├── database/           — NewPostgres, NewRedis, NewS3, IsUniqueViolation
├── dto/                — shared DTOs (ReadyResponse, ComponentStatus)
├── logger/             — zap logger setup, field constants, error type constants
└── middleware/          — dual-framework middleware (gin + fiber):
    ├── request_id.go   — RequestIDGin, RequestIDFiber, GetRequestIDGin, GetRequestIDFiber
    ├── recovery.go     — RecoveryGin, RecoveryFiber
    ├── logger.go       — LoggerGin, LoggerFiber
    └── context.go      — CallerIDGin, CallerIDFiber

migrations/             — goose SQL migrations (00001_users … 00006_review_widths)
deploy/nginx/           — compose-nginx reverse proxy config
docs/openapi/           — OpenAPI 3.0.3 specs: panel.yaml, runtime.yaml, analytics.yaml
tests/e2e/panel/        — panel end-to-end tests
tests/e2e/runtime/      — runtime decide end-to-end tests
tools/s3-provision/     — MinIO bucket provisioning utility

frontend/               — React SPA (см. frontend/AGENTS.md)
├── src/app/           — providers (Mantine+Query), router (/login, /status public, / + /users + /flags под RequireAuth), layout (sidebar Home/Users/Flags, CurrentUserChip)
├── src/pages/         — LoginPage, StatusPage, UsersPage, FlagsPage, NotFoundPage
├── src/features/auth/ — LoginForm, RequireAuth, api/login + useLoginMutation, types, lib/*
├── src/features/users/ — Users CRUD: types (role enum, shape guards), api/users + useUsers (Bearer, limit/offset), components (Table, CreateModal, DetailsModal), lib/*
├── src/features/flags/ — Flags CRUD: types (FlagType enum, per-type default guards), api/flags + useFlags (Bearer, limit/offset), components (Table, CreateModal, DetailsModal, DefaultInput), lib/*
├── src/features/status/ — StatusPage data: api/probes + useServiceStatus, lib/aggregate, components/*, types
├── src/components/ui/ — BottomBar, MetricIcon
├── src/shared/        — lib/queryClient, types, config/api (VITE_*_API_BASE_URL)
├── src/i18n/          — en/ru locales, typed resources
├── nginx.conf         — собственный nginx: SPA fallback, /health
└── Dockerfile         — build SPA → nginx:alpine
```

### Схема потоков

```
                    nginx (:8080)
                   /     |     \      \
        /api/v1/panel  /api/v1/runtime  /api/v1/analytics  / → frontend:80 (SPA)
              |              |                |
         panel:8081    runtime:8082     analytics:8083
              |
         PostgreSQL (source of truth)
              |
              ├──→ Outbox ──→ Kafka ──→ Runtime (snapshot reload)
              │
              └──→ Notifications (goroutines, buffered channel)

Runtime ──→ In-memory snapshot ←── Redis ←── Kafka (config.changed)
       │
       └──→ Kafka (decisions, async)

Analytics ──→ Kafka ──→ ClickHouse (facts)
```

---

## 4. Текущий статус реализации

### Готово

| Домен                                                                                    | Файлы                                                                                              | Статус |
| ---------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | ------ |
| Auth (JWT, Redis sessions)                                                               | `services/panel/domain/auth/`                                                                      | ✅     |
| Users CRUD (avatar, self-protection)                                                     | `services/panel/domain/users/`                                                                     | ✅     |
| Flags CRUD (typed values)                                                                | `services/panel/domain/flags/`                                                                     | ✅     |
| Metrics CRUD (6 типов)                                                                   | `services/panel/domain/metrics/`                                                                   | ✅     |
| Experiments CRUD + lifecycle (Phase 6+9)                                                 | `services/panel/domain/experiments/`                                                               | ✅     |
| Approver groups + review workflow (Phase 8: thresholds, comment threads, resolve)        | `services/panel/domain/reviews/`                                                                   | ✅     |
| Health probes (panel, runtime, analytics)                                                | `services/*/domain/health/`                                                                        | ✅     |
| Shared packages                                                                          | `pkg/api`, `pkg/auth`, `pkg/config`, `pkg/database`, `pkg/logger`, `pkg/middleware`, `pkg/dto`     | ✅     |
| Dual-framework middleware                                                                | `pkg/middleware/` (gin + fiber)                                                                    | ✅     |
| Response helpers (net/http + Gin + Fiber variants)                                       | `pkg/api/` (`response.go`, `gin.go`, `fiber.go`)                                                   | ✅     |
| E2E тесты                                                                                | `tests/e2e/panel/`, `tests/e2e/runtime/`                                                           | ✅     |
| OpenAPI specs                                                                            | `docs/openapi/` (panel, runtime, analytics)                                                        | ✅     |
| Migrations                                                                               | 00001_users … 00006_review_widths                                                                  | ✅     |
| Docker Compose                                                                           | `docker-compose.yml` (8 сервисов: postgres, redis, s3, panel, runtime, analytics, frontend, nginx) | ✅     |
| Multi-service Makefile                                                                   | `Makefile` (build, dev, run, docker)                                                               | ✅     |
| Runtime skeleton                                                                         | `services/runtime/` (Fiber v3, health only)                                                        | ✅     |
| Runtime decide API (xxhash64, allocation, snapshot v2)                                   | `services/runtime/domain/decide/`                                                                  | ✅     |
| Analytics skeleton                                                                       | `services/analytics/` (Fiber v3, health only)                                                      | ✅     |
| Frontend auth shell (login, RequireAuth, in-memory session, bottom-right notifications)  | `frontend/src/features/auth/`                                                                      | ✅     |
| Frontend shell (AppLayout, LoginPage, NotFoundPage, BottomBar, i18n en/ru)               | `frontend/src/`                                                                                    | ✅     |
| Frontend Status Page (public /status: panel/runtime/analytics health+ready, polling 20s) | `frontend/src/features/status/`, `frontend/src/pages/StatusPage.tsx`                               | ✅     |
| Component criticality (required/optional) in ready responses                             | `pkg/dto`, `services/*/domain/health/`, OpenAPI ×3                                                 | ✅     |

### Статус по майлстоунам (детализация к таблице выше)

| Milestone | Описание                                                                  | Статус                                                                                                                                                                                                                     |
| --------- | ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 3         | Runtime decide API, targeting engine, xxhash64, snapshot                  | ⚠️ частично (decide + xxhash + snapshot v2 готовы; targeting — заглушка до Phase 7, participation policy отложена)                                                                                                         |
| 4         | Experiments domain, versions, variants, allocation                        | ⚠️ частично (CRUD + lifecycle готовы, нет conflicts/priority; review — см. Milestone 5)                                                                                                                                    |
| 5         | Approver groups, review workflow                                          | ✅ (группы + пороги + треды комментариев; `services/panel/domain/reviews/`, миграции 00005/00006)                                                                                                                          |
| 6         | Kafka, outbox, config propagation                                         | ❌                                                                                                                                                                                                                         |
| 7         | Events API, Kafka ingestion, dedup                                        | ❌                                                                                                                                                                                                                         |
| 8         | Exposures, attribution, ClickHouse                                        | ❌                                                                                                                                                                                                                         |
| 9         | Report queries, metrics catalog                                           | ❌                                                                                                                                                                                                                         |
| 10        | Guardrail evaluator                                                       | ❌                                                                                                                                                                                                                         |
| 11        | Notifications (goroutines) + 3 channels                                   | ❌                                                                                                                                                                                                                         |
| 12        | Prometheus, Loki, Grafana                                                 | ❌                                                                                                                                                                                                                         |
| 13        | Failure/load tests                                                        | ❌                                                                                                                                                                                                                         |
| 14        | Docs, ADR                                                                 | ❌                                                                                                                                                                                                                         |
| UI-1      | Frontend: auth shell (login + guard + notifications)                      | ✅                                                                                                                                                                                                                         |
| UI-2      | Frontend: domain pages (flags, experiments, reviews, metrics, users)      | ⚠️ частично (Users CRUD и Flags CRUD готовы: `/users` + `/flags` за RequireAuth, limit/offset-пагинация, typed defaults у флагов, admin-only writes с read-only UI, i18n en/ru, vitest; experiments/reviews/metrics — нет) |
| UI-3      | Frontend: public Status Page (aggregate panel/runtime/analytics, polling) | ✅                                                                                                                                                                                                                         |

### Актуальные проблемы и долг

1. ~~Миграция 00004 сломана~~ — **решено 2026-09-14**: убран FK на несуществующий `reviews`, добавлены `owner_id`, `version`, `guardrail_paused`, `completion_decision/reason`; function обёрнут в `StatementBegin/End`. Up/down проверены.
2. ~~Нет Approver Groups~~ — **решено 2026-09-14** (Phase 8): `services/panel/domain/reviews/`, миграции 00005/00006.
3. **Нет Audit Records** — сущность описана в task.md, миграция удалена из scope (см. git history `b980807`), кода нет. §6 ниже описывает контракт, не реализацию.

---

## 5. Доменные правила (task.md §4)

| Правило | Описание                                                   |
| ------- | ---------------------------------------------------------- |
| D-01    | Single-tenant, no `environment_id`                         |
| D-02    | Global participation policy (config.json)                  |
| D-03    | Basis points (10000 = 100%)                                |
| D-04    | UTC RFC 3339 timestamps with milliseconds                  |
| D-05    | xxhash64 deterministic hashing                             |
| D-06    | Immutable published versions                               |
| D-07    | Flag type immutability                                     |
| D-08    | One active experiment per flag                             |
| D-09    | Pause ≠ rollback                                           |
| D-10    | Resume constraints                                         |
| D-11    | Completion decisions (rollout_winner, rollback, no_effect) |
| D-12    | Stickiness (subject + salt → variant)                      |
| D-13    | Targeting: missing field → false                           |
| D-14    | Idempotency (Idempotency-Key header)                       |
| D-15    | Optimistic locking (version field)                         |
| D-16    | Review invalidation on new version                         |
| D-17    | Running version freeze                                     |
| D-18    | Metric immutability when in use                            |
| D-19    | Ownership check (owner_id == actor.id)                     |

---

## 6. Сущности и их инварианты

> Легенда: ✅ реализовано (backend + миграции), ❌ описано в task.md, но не реализовано.
> Реализованы: User, Feature Flag, Experiment (+Version, +Variant), Metric, Review.
> Не реализованы: Event Type, Guardrail, Audit Record (audit удалён из scope, см. §4 п.3).

### User

- UUIDv7, unique email (case-insensitive)
- Soft deletion via `deleted_at` (не hard delete, не восстанавливается)
- 4 роли: `admin`, `experimenter`, `approver`, `viewer`

### Feature Flag

- Unique key (case-sensitive)
- Typed default: `string` (quoted), `number` (valid JSON number), `bool` (`true`/`false`)
- `value_type` immutable после создания
- Soft deletion via `deleted_at`

### Experiment

- Unique key, attached to exactly one flag
- Lifecycle: `draft → review → approved → running → paused → completed → archived`
- Only one `running`/`paused` experiment per flag (partial unique index)
- `guardrail_paused` boolean: requires admin override to resume

### Experiment Version

- Immutable configuration snapshot
- Contains: variants, weights, targeting, assignment salt
- Any distribution-affecting change → new version

### Variant

- Named value within experiment version
- Exactly one control, at least 2 variants
- Weights in basis points; sum = allocation_bps

### Event Type

- Named event with versioned schema (JSONB)
- Attribution policy: `require_exposure` or `none`
- CRUD через admin API
- Validation rules per type (required fields, type constraints, value ranges)

### Metric

- Named aggregation: `count`, `sum`, `unique_count`, `ratio`, `average`, `percentile`
- Immutable if used by running/completed experiment
- Calculation rules: which events and how to aggregate

### Guardrail

- Bound to one experiment + one metric
- Threshold + operator + min_sample_size + consecutive breaches
- Action: pause or rollback to control
- Cooldown: default 10 minutes

### Review

- Bound to one experiment version
- State machine: `OPEN → APPROVED | CHANGES_REQUESTED | REJECTED`
- One approval per reviewer per review

### Audit Record

- Append-only, never mutated
- Records: actor, action, resource, before/after JSONB

---

## 7. Code Style & Conventions

```go
// Каждый домен следует паттерну:
handler.go    → HTTP handlers, route registration
service.go    → Business logic, validation
repository.go → SQL queries, database operations
model.go      → Domain types, constants
dto.go        → Request/response DTOs

// Паттерн ответа (panel, net/http-стиль — напрямую через Writer):
pkg/api.OK(c.Writer, data)           // 200
pkg.api.Error(c.Writer, code, msg)   // 4xx
pkg.api.InternalError(c.Writer)      // 500
pkg.api.ValidateRequest(w, r, &req)  // binding errors → 400
// Gin/Fiber-варианты (OKGin, ErrorFiber, …) существуют в pkg/api/gin.go и fiber.go,
// но panel-хендлеры используют Writer-версии; Fiber-версии — для runtime/analytics.

// Паттерн ответа (Fiber):
c.Status(code).JSON(obj)

// Паттерн ошибок домена:
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

// Паттерн валидации (Gin):
binding:"required,email"
binding:"min=3,max=64"
binding:"omitempty,oneof=string number bool"
```

### Правила

1. **JSON codec:** `goccy/go-json`, не `encoding/json`
2. **ID path params (Gin):** `gin.Param("id")` + `strconv.ParseUint`, не `c.ShouldBindUri`
3. **ID path params (Fiber):** `c.Params("id")`
4. **Rueidis:** `rueidis.Client` — interface. Дереверенс через `(*client).Do(...)`
5. **S3:** `*s3.Client` — concrete type. Используй `s3Client.Method(...)` напрямую
6. **Config:** S3 использует config structs, не raw strings
7. **Environment:** `"local"`, `"dev"`, `"prod"`
8. **No comments** unless explicitly requested
9. **No emojis** unless explicitly requested
10. **Middleware:** Dual-framework — каждая функция имеет `*Gin` и `*Fiber` версии
11. **Zap fields:** Все строковые литералы заменяются на `logger.Field*` константы из `pkg/logger/fields.go`
12. **Bootstrap password:** Хранится в конфиге как `password_hash` (bcrypt), не как plaintext
13. **Frontend:** TypeScript strict — no `any`, no `@ts-ignore`, Mantine-only UI; детали — в `frontend/AGENTS.md`

---

## 8. Ключевые ограничения

1. **Health routes** под `/api/v1/{service}/`, не на root
2. **`ServiceVersion`** — `var` (не const), переопределяется через `-ldflags`
3. **`ValidateRequest`** использует `io.LimitReader` (goccy swallows MaxBytesError)
4. **Postgres pool:** `MaxConns`/`MinConns` на `cfg` ДО `pgxpool.NewWithConfig`
5. **Redis и S3 optional** — unavailability = warning, не fatal
6. **Self-management:** Users не могут удалить себя (403), сменить роль (403), последний admin не demotable (409)
7. **Bootstrap admin** на каждом старте если `auth.bootstrap.password_hash` задан и users пуст
8. **OpenAPI spec** должен быть aligned с backend
9. **Flags list** возвращает `created_by`/`updated_by` как `null` (без join)
10. **Flag names unique** — duplicate → 409
11. **Panel** слушает на `:8081`, **runtime** на `:8082`, **analytics** на `:8083`
12. **nginx** проксирует все три сервиса через `:8080`, SPA отдаётся с `frontend:80` на `/` (fallback `index.html`)
13. **Docker Compose** требует `PORT` arg при build каждого сервиса
14. **Frontend API base:** `VITE_PANEL_API_BASE_URL`, fallback `/api/v1/panel`; dev-proxy vite → `localhost:8081`
15. **Frontend auth:** JWT только in-memory (`useSyncExternalStore`), никакого `localStorage`; `RequireAuth` шлёт на `/login`

---

## 9. Команды

```bash
# Infrastructure
make dev-up          # start Postgres + Redis + S3/MinIO
make dev-down        # stop all containers
make dev-clean       # stop and remove volumes
make dev-status      # check which infra containers are running

# Local development (fast restart)
make run-local       # build + restart all services (checks infra, no restart)
make stop-local      # stop all service binaries
make restart-local   # stop + run-local
make run-panel       # build + run only panel
make run-runtime     # build + run only runtime
make run-analytics   # build + run only analytics

# Build
make build           # compile panel    -> bin/panel
make build-runtime   # compile runtime  -> bin/runtime
make build-analytics # compile analytics -> bin/analytics
make build-all       # compile all three
make vet             # go vet ./...
make check           # build-all + vet + gofmt

# Docker Compose (full stack)
make up              # build + start all services in docker compose
make down            # stop docker compose
make logs            # follow docker compose logs
make rebuild         # down + up (full rebuild)

# Database
make pg-migrate-up       # apply all pending migrations
make pg-migrate-down     # rollback last migration
make pg-migrate-status   # show migration status
make psql                # open psql shell

# Tests
go test ./pkg/... -short -count=1
make test-e2e

# Tools
make redis-cli       # open redis-cli
make s3-provision    # create S3 bucket
```

### Required checks before completing a task

```bash
make check           # build-all + vet + gofmt
go test ./pkg/... -short -count=1
```

---

## 10. Ссылки на документацию

| Документ                           | Путь                                                       |
| ---------------------------------- | ---------------------------------------------------------- |
| Техническое задание                | `docs/task/task.md`                                        |
| TODO (по фазам)                    | `docs/task/todo.md`                                        |
| Оригинальное ТЗ                    | `original_task.md`                                         |
| OpenAPI spec                       | `docs/openapi/panel.yaml`                                  |
| OpenAPI specs (runtime, analytics) | `docs/openapi/runtime.yaml`, `docs/openapi/analytics.yaml` |
| Frontend brief                     | `frontend/AGENTS.md`                                       |
| AI Handoff                         | `docs/ai/AI_HANDOFF.md`                                    |
| Logging standard                   | `docs/logging.md`                                          |
| Миграции                           | `migrations/`                                              |

---

## 11. Changelog

> Обновляйте этот раздел при изменениях в архитектуре или статусе.

| Дата       | Изменение                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 2026-08-31 | Инициализация AGENTS.md. Статус: Milestone 1-2 готовы, Milestone 3-14 не начаты. Миграция 00006 сломана.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 2026-08-31 | Notifications: заменены notification-worker на goroutines в panel                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 2026-08-31 | Conflict domains: удалены из scope                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| 2026-08-31 | Web UI: удалён из scope                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| 2026-08-31 | Email notifications: удалены из scope                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 2026-08-31 | Flags CRUD: write операции (POST/PATCH/DELETE) ограничены ролью admin                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 2026-09-01 | Restructure: `services/panel/internal/` → `services/panel/`, `cmd/main.go` + `cmd/router.go` → `main.go` + `router.go`, `internal/lib/` removed                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| 2026-09-01 | Logging: health handler gains probe failure logging, auth middleware logs security events, dead code removed (`Recovery`, `CallerID`, `RequestLogger`, `RequestFields`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| 2026-09-01 | Middleware: dual-framework support (gin + fiber) — `RequestIDGin/Fiber`, `RecoveryGin/Fiber`, `LoggerGin/Fiber`, `CallerIDGin/Fiber`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| 2026-09-01 | Logger fields: created `pkg/logger/fields.go` with 52 constants, replaced all string literals across 18 files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 2026-09-01 | Config: `LogLevel` field, `ValidateSecurity()` moved to `pkg/config`, bootstrap `password_hash` replaces plaintext `password`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 2026-09-01 | Services: runtime skeleton (Fiber v3, port :8082), analytics skeleton (Fiber v3, port :8083)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 2026-09-01 | Docker: `Dockerfile` (multi-stage, ARG SERVICE/PORT), `docker-compose.yml` (8 services), nginx reverse proxy                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 2026-09-01 | Makefile: rewritten for developer workflow — `run-local` (fast restart), `dev-up` (infra only), `up` (docker compose), `check`, `dev-status`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 2026-09-14 | Experiments: CRUD + lifecycle (Phase 6+9) — `services/panel/domain/experiments/`, миграция 00004 починена (owner/version/guardrail_paused/completion, StatementBegin/End, без FK reviews), review — заглушка, internal pause/rollback за admin-группой                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| 2026-09-14 | Runtime decide: snapshot v2 (experiments), xxhash64 buckets, allocation, decision_id, degraded/stale; композер `services/panel/snapshot/`; `tests/e2e/runtime/`; фикс `findConfigFile` для абсолютного CONFIG_NAME; targeting — заглушка, participation policy отложена                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| 2026-09-14 | Review fixes: approver может ревьюить чужие эксперименты (§6.1); unknown flag → 400 (§7.5); `test-e2e -p 1`; `CreateVersion` FOR UPDATE; `subject_id` max 128; D-05 зафиксирован на `experiment_id`; пустой targeting `{}` = match                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| 2026-09-14 | Review workflow (Phase 8): группы апруверов + пороги + треды комментариев с resolve — `services/panel/domain/reviews/`, миграции 00005/00006, статус `rejected` у эксперимента, D-10 через APPROVED-ревью; стабы `/approve` удалены                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| 2026-09-16 | Docker build: `.dockerignore` (whitelist: go.mod/go.sum/pkg/services, без bin/frontend/node_modules), BuildKit-кеш `go mod download` + `go build` (`labp-go-mod`, `labp-go-build`), точечные COPY вместо `COPY . .`, `-trimpath -buildvcs=false -s -w`. Повторная сборка образа ~10s                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| 2026-09-16 | MinIO healthcheck: `mc ready local` → `curl /minio/health/ready` (в compose и Makefile), `dev-up` с таймаутом вместо бесконечного ожидания. Compose: `PGDATA=/var/lib/postgresql/data/pgdata` для postgres:18 (иначе контейнер не поднимается с существующим томом)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| 2026-09-17 | Frontend auth shell: login (`POST /api/v1/panel/login`, fetch + React Query, `LoginApiError` http/network/unexpected), in-memory JWT-сессия (`useSyncExternalStore`), `RequireAuth` (редирект на `/login`), уведомления bottom-right со стабильными id, детализация ошибок 401/400/403/404/422/429/5xx (статус/код/сообщение бэкенда, en/ru); dev-proxy vite → `:8081`, `VITE_PANEL_API_BASE_URL`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 2026-09-17 | Frontend light theme: темнее и контрастнее (body `#e8edf2`, border `#c3ccd5`, dimmed `#475569`; поверхности в `interactions.css` синхронизированы), тёмная тема не тронута                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 2026-09-17 | Docs sync: §3 (+experiments/reviews/snapshot, frontend/, 3 OpenAPI-спеки, e2e runtime, миграции 00001–00006, `/` → frontend в схеме), §4 (таблица UI-1/UI-2, Approver Groups — решено, Audit — вне scope), §6 (легенда реализовано/запланировано), §7 (Writer-паттерн `pkg/api` вместо неверного `OK(c, …)`), §8/§10/§12 (frontend-правила и ссылки); `frontend/AGENTS.md`: цель, Implementation status, фактическая структура, правило «OpenAPI-фрагмент → types → api → hook → UI»                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| 2026-09-17 | Frontend logout: `LogoutButton` в шапке `AppLayout` (`clearAuthSession` + уведомление `signed-out` + редирект `/login`); backend `/logout` нет — сессия только in-memory, Redis-записи истекают по TTL; i18n `auth.*` (en/ru)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 2026-09-17 | Frontend dev-префилл логина: `lib/devLoginDefaults.ts` подставляет `root@labp.net` / `root!@#$` (bootstrap из e2e) только при `import.meta.env.DEV`, override через `VITE_DEV_LOGIN_EMAIL`/`VITE_DEV_LOGIN_PASSWORD`; в prod-бандле пароль отсутствует (проверено grep по `dist`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 2026-09-17 | Public Status Page: backend `ComponentStatus.criticality` (`required`/`optional`, typed enum `dto.Criticality` + `Valid()`), criticality во всех 3 health-хендлерах (panel: storage=optional; остальное required), санитизация `Message` (raw `err.Error()` только в серверные логи, наружу — безопасные строки), OpenAPI ×3 synced, e2e-ассерты criticality; frontend `/status` (public, вне `RequireAuth`): health+ready всех сервисов через nginx, агрегат HEALTHY/DEGRADED/DOWN + overall, polling 20s (React Query), timeout 5s, latency, Mantine Accordion, i18n en/ru, vitest (21 тест: aggregate + probes со стабнутым fetch); `make check` + e2e `TestHealth_*` зелёные                                                                                                                                                                                                                                                                                                                                                                                               |
| 2026-09-17 | Public Status Page redesign (только frontend): широкий layout (Container md) вместо узкого dashboard — доминантный global status header (ThemeIcon + Last checked + Refresh через query `refetch`), service rows (индикатор, имя, `service · version · environment`, latency, badge), expanded meta/компоненты/футер; сообщения об ошибках только для не-ok компонентов; новый `lib/format.ts`; i18n `status.refresh`; aggregate/polling/Backend untouched                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 2026-09-17 | Status Page visual refinement (только frontend): убраны почти все горизонтальные Divider'ы — Accordion `separated` + `classNames` (`status-service-item`, surfaces из theme-токенов, spacing вместо линий, expanded-поверхность через `[data-active]`, subtle hover); component status — compact `Badge` с иконкой (`size sm`, `variant light`) вместо plain text; standalone-иконка у компонентов убрана (сигнал остался в badge); aggregate/polling/i18n/Backend untouched                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 2026-09-17 | Status Page surface polish (только frontend): Refresh `subtle` → `default` (видимая secondary-кнопка); единая surface-система — collapsed/hover/expanded выводятся из `--mantine-color-default` через color-mix + приглушённый border (`default-border` 45/65/75%), собственный hover Control'а Mantine нейтрализован (давал конфликтный серый прямоугольник); expanded+hover без скачка цвета; status-цвета/aggregate/polling/i18n/Backend untouched                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 2026-09-17 | Status Page expanded-surface tuning (только frontend): expanded `color-mix 84%` → `90%`, hover `90%` → `93%` — expanded едва светлее базы, иерархия collapsed < hover < expanded без скачков                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 2026-09-17 | Status Page multi-expand (только frontend): Accordion переведён в контролируемый `multiple`-режим, expanded state — `useState<string[]>` в `StatusPage` (открытие/закрытие cards независимо); polling/refetch state не трогают; UI/styling/logic untouched                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 2026-09-17 | BottomBar focus-glow fix (только frontend): `onDropdownClose` снимает фокус с Select (`blurActiveControl`) — зелёный `:focus`-halo больше не залипает после выбора опции, включая повторный выбор уже выбранного значения                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 2026-09-17 | StatusPage Refresh в стиле `panel-illuminate` (только frontend): кнопка получила frosted-поверхность и бордер как у BottomBar (`className="panel-illuminate"`, unlayered CSS перекрывает фон/бордер Mantine-кнопки); hover — зелёный оттенок бордера как у панели                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 2026-09-17 | Status↔App навигация (только frontend): кнопка Back (`/status` → `/`, subtle + `IconArrowLeft`, `status.back`); в шапке `AppLayout` — кнопка Status (`/status`, subtle + `IconActivity`, `status.openStatus`) и минимальный `OverallStatusPill` (точка + короткий overall-статус, `status.overallShort.*`, те же probes/aggregate, без auth); i18n en/ru                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 2026-09-17 | StatusPage custom SVG loader (только frontend): новый shared `components/ui/SegmentedSpinner.tsx` — 6 SVG-дуг (`stroke=currentColor`, `linecap=round`, opacity-градиент 1→0.3, viewBox 64), вращение только группы (`segmented-spinner__group`, 1.2s linear infinite через `--spinner-duration`, `prefers-reduced-motion` отключает); Refresh-кнопка показывает спиннер 14px вместо Mantine `loading` (размер стабилен, без дубля aria — у кнопки есть текстовый лейбл); aggregate/polling/i18n/Backend untouched                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 2026-09-17 | StatusPage spinner relocation (только frontend): `SegmentedSpinner size={56}` перенесён из Refresh-кнопки в global status header ВМЕСТО большой иконки (`ThemeIcon` удалён) — persistent status-индикатор в цвете aggregate (`green`/`yellow`/`red` через `Text c=`, декоративный `aria-hidden`); Refresh вернулся на стандартный Mantine `loading`; API компонента, aggregate/polling/i18n/Backend untouched                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 2026-09-17 | StatusPage indicator ring+icon (только frontend): убран opacity-градиент сегментов (все 6 дуг однородные, `opacity: 1`); global status — двухслойный `.status-indicator` (grid, одна ячейка): кольцо 56px + центральная иконка 22px из существующего маппинга `overallDisplay`, оба в цвете aggregate, контейнер `aria-hidden`; геометрия/API/анимация спиннера, Refresh/polling/i18n/Backend untouched                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| 2026-09-17 | Frontend Users CRUD (только frontend, backend/OpenAPI untouched): `/users` за `RequireAuth` + кнопка Users в шапке `AppLayout`; `src/features/users/` (types с guards + `UsersApiError`, `api/users.ts` с Bearer через `getAuthToken()` и 401→clear session, `useUsers.ts` с ключом `['users',{limit,offset}]` и инвалидацией, `UsersTable`/`CreateUserModal`/`UserDetailsModal`, `lib/` — error mapping, self-rules, format); create (full_name/email/password/role), edit только email/role (changed-fields PATCH, своя роль заблокирована), delete через `openConfirmModal` (204), avatar upload/remove multipart (admin-only в UI, backend authoritative), skeletons/empty/error+Retry, пагинация Mantine, i18n `users.*` en/ru, vitest (17 тестов); UI-2 → частично                                                                                                                                                                                                                                                                                                       |
| 2026-09-17 | Users shell fixes (только frontend): сброс React Query-кэша при login/logout (`queryClient.clear()` — чинит «перепутанные» delete-кнопки после перелогина другим аккаунтом: `useMe`/`users` отдавали данные прошлой сессии); левый sidebar в `AppLayout` (Navbar 250px, Home + Users `NavLink`, Burger ниже `sm`), кнопка Users убрана из шапки; чип текущего пользователя в шапке (аватар + имя + бейдж роли из `/me`, `CurrentUserChip`, скелетон при загрузке); общий `lib/roleBadge.ts`; i18n `layout.home/navigation/menu`, `users.currentUser`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| 2026-09-17 | Frontend palette doc (только docs): `frontend/docs/palette.md` — полное описание палитры light/dark (токены, фоны, frosted-поверхности, инпуты, primary green + свечения, статусы, бейджи ролей, поверхности StatusPage, спиннер, правила); ссылка из `frontend/AGENTS.md`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 2026-09-17 | Frontend neutral restyle (только frontend): строгая монохромная шкала Neutral/Zinc вместо slate — токены (`body #fafafa/#121212`, surface `#ffffff/#171717`, border `#e5e5e5/#333333`, dimmed `#737373/#a3a3a3`), solid-фоны, непрозрачные инпуты (`#ffffff/#0a0a0a`, focus-кольцо 1px без blur), frosted шапка/панель (`rgba(255,255,255,0.85)` / `rgba(23,23,23,0.85)`), кнопка (hover темнеет: `#16a34a→#15803d` / `#22c55e→#16a34a`, направленная тень `0 4px 12px`), навигация `.app-nav-link` (dimmed → hover neutral-wash → active green-alpha); slate-точка loading `#64748b` → `#a3a3a3/#525252`; `docs/palette.md` и `frontend/AGENTS.md` synced                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 2026-09-17 | Frontend Flags CRUD (только frontend, backend/OpenAPI untouched): `/flags` за `RequireAuth` + пункт Flags в sidebar; `src/features/flags/` (types: `FlagType` enum + per-type `default_value` guards, nullable creators через users `parseUser`, `FlagsApiError`; `api/flags.ts` + `useFlags.ts` — тот же fetch/auth/401-паттерн, ключ `['flags',{limit,offset}]`, DELETE 204; `FlagDefaultInput` — typed редактор default (text/number/bool-select), number валидируется через `JSON.parse` как backend; `FlagsTable`/`CreateFlagModal`/`FlagDetailsModal`; `lib/flagValue.ts` — changed-fields PATCH, пустые key/name/description опускаются т.к. backend трактует их как «не менять», description очистить нельзя); type immutable в UI (D-07), writes admin-only с read-only UI для остальных ролей (backend authoritative, live-проверено: viewer list 200 / create 403); пагинация через общий `shared/lib/pagination.ts` (users `rules.ts` — re-export); i18n `flags.*` en/ru, vitest (17 тестов: `flags.test.ts` + `flagValue.test.ts`); UI-2 → частично (users+flags) |
| 2026-09-17 | CI/CD: добавлен `.github/workflows/ci-cd.yml` (Go/frontend tests, `make check`, Compose config/image build, SSH deploy на dev после успешного push в `master`) и `docs/deploy-dev.md` с требованиями dev-сервера и GitHub Secrets                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |

---

## 12. Правила обновления этого файла

1. **При добавлении нового домена** — обновить §4 (статус) и §6 (сущности)
2. **При изменении доменных правил** — обновить §5
3. **При изменении архитектуры** — обновить §3
4. **При добавлении зависимостей** — обновить §2
5. **При изменении code style** — обновить §7
6. **Всегда** — добавить запись в §11 Changelog
7. **При изменении frontend** — обновить раздел Implementation status в `frontend/AGENTS.md`
