# AGENTS.md — Lotty AB Platform

> **Живой документ.** Обновляется при изменениях в архитектуре, доменных правилах или статусе реализации.
>
> **Последнее обновление:** 2026-09-01

---

## 1. Проект

**Lotty AB Platform** — self-hosted A/B testing platform для feature flags, controlled experiments, review workflow, runtime variant delivery, event collection, attribution, analytics и guardrails.

**Формат:** Production-ready pet project для портфолио (solo dev + AI). Запускается локально через Docker Compose или бинарники.

**Аудитория жюри:** 30-минутная проверка по критериям B1-B10.

---

## 2. Tech Stack

| Компонент | Технология |
|-----------|------------|
| Language | Go 1.27 |
| HTTP (panel) | Gin (`github.com/gin-gonic/gin`) |
| HTTP (runtime, analytics) | Fiber v3 (`github.com/gofiber/fiber/v3`) |
| JSON | `goccy/go-json` (не `encoding/json`) |
| Database | PostgreSQL via pgx v5 (`pgxpool`) |
| Cache | Redis via rueidis (client-side caching, cluster-aware) |
| Object Storage | S3 via `aws-sdk-go-v2` (MinIO для local dev) |
| Auth | JWT (HS256) via `golang-jwt/jwt/v5`, Redis-backed sessions |
| Migrations | goose (`github.com/pressly/goose/v3`) |
| Config | viper с `{{ ENV "..." }}` template rendering |
| Logging | zap (structured JSON) |
| Validation | `go-playground/validator/v10` |
| Hashing | xxhash64 (deterministic variant assignment) |
| Password | Argon2id (bcrypt cost=13 для legacy), bootstrap: pre-hashed bcrypt |

---

## 3. Архитектура

```
services/
├── panel/              — control plane (Gin, port :8081)
│   ├── main.go         — entrypoint, DI wiring
│   ├── router.go       — route registration, middleware chain
│   └── domain/         — domain packages
│       ├── auth/       — JWT, Redis sessions, RBAC middleware
│       ├── users/      — CRUD, avatar storage (S3)
│       ├── flags/      — CRUD, typed default values
│       ├── metrics/    — CRUD, aggregation config
│       └── health/     — liveness/readiness probes
├── runtime/            — decide API (Fiber v3, port :8082) [skeleton]
│   ├── main.go         — entrypoint
│   ├── app.go          — DI, middleware, routing
│   └── domain/
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

migrations/             — goose SQL migrations (numbered)
deploy/nginx/           — nginx reverse proxy config
docs/openapi/           — OpenAPI 3.0.3 spec
tests/e2e/panel/        — end-to-end tests
tools/s3-provision/     — MinIO bucket provisioning utility
```

### Схема потоков

```
                    nginx (:8080)
                   /     |     \
        /api/v1/panel  /api/v1/runtime  /api/v1/analytics
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

### Готово (Milestone 1-2)

| Домен | Файлы | Статус |
|-------|-------|--------|
| Auth (JWT, Redis sessions) | `services/panel/domain/auth/` | ✅ |
| Users CRUD (avatar, self-protection) | `services/panel/domain/users/` | ✅ |
| Flags CRUD (typed values) | `services/panel/domain/flags/` | ✅ |
| Metrics CRUD (6 типов) | `services/panel/domain/metrics/` | ✅ |
| Health probes (panel, runtime, analytics) | `services/*/domain/health/` | ✅ |
| Shared packages | `pkg/api`, `pkg/auth`, `pkg/config`, `pkg/database`, `pkg/logger`, `pkg/middleware`, `pkg/dto` | ✅ |
| Dual-framework middleware | `pkg/middleware/` (gin + fiber) | ✅ |
| E2E тесты | `tests/e2e/panel/` (8 файлов) | ✅ |
| OpenAPI spec | `docs/openapi/panel.yaml` | ✅ |
| Migrations | 00001_users, 00002_flags, 00003_metrics | ✅ |
| Docker Compose | `docker-compose.yml` (8 сервисов) | ✅ |
| Multi-service Makefile | `Makefile` (build, dev, run, docker) | ✅ |
| Runtime skeleton | `services/runtime/` (Fiber v3, health only) | ✅ |
| Analytics skeleton | `services/analytics/` (Fiber v3, health only) | ✅ |

### Не построено (Milestone 3-14)

| Milestone | Описание | Статус |
|-----------|----------|--------|
| 3 | Runtime decide API, targeting engine, xxhash64, snapshot | ❌ |
| 4 | Experiments domain, versions, variants, allocation | ❌ (миграция 00006 сломана) |
| 5 | Approver groups, review workflow | ❌ |
| 6 | Kafka, outbox, config propagation | ❌ |
| 7 | Events API, Kafka ingestion, dedup | ❌ |
| 8 | Exposures, attribution, ClickHouse | ❌ |
| 9 | Report queries, metrics catalog | ❌ |
| 10 | Guardrail evaluator | ❌ |
| 11 | Notifications (goroutines) + 3 channels | ❌ |
| 12 | Prometheus, Loki, Grafana | ❌ |
| 13 | Failure/load tests | ❌ |
| 14 | Docs, ADR | ❌ |

### Критические проблемы

1. **Миграция 00006 сломана** — синтаксические ошибки: `text(64)`, stray `)`, неправильное имя FK, `NULLABLE` не является SQL
2. **Нет миграций 00004, 00005** — пропуск в нумерации
3. **Нет Approver Groups** — сущность описана в task.md, нет миграции и кода
4. **Нет Audit Records** — сущность описана в task.md, нет миграции и кода

---

## 5. Доменные правила (task.md §4)

| Правило | Описание |
|---------|----------|
| D-01 | Single-tenant, no `environment_id` |
| D-02 | Global participation policy (config.json) |
| D-03 | Basis points (10000 = 100%) |
| D-04 | UTC RFC 3339 timestamps with milliseconds |
| D-05 | xxhash64 deterministic hashing |
| D-06 | Immutable published versions |
| D-07 | Flag type immutability |
| D-08 | One active experiment per flag |
| D-09 | Pause ≠ rollback |
| D-10 | Resume constraints |
| D-11 | Completion decisions (rollout_winner, rollback, no_effect) |
| D-12 | Stickiness (subject + salt → variant) |
| D-13 | Targeting: missing field → false |
| D-14 | Idempotency (Idempotency-Key header) |
| D-15 | Optimistic locking (version field) |
| D-16 | Review invalidation on new version |
| D-17 | Running version freeze |
| D-18 | Metric immutability when in use |
| D-19 | Ownership check (owner_id == actor.id) |

---

## 6. Сущности и их инварианты

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

// Паттерн ответа (Gin):
pkg/api.OK(c, data)           // 200
pkg.api.Error(c, code, msg)   // 4xx
pkg.api.InternalError(c, err) // 500
pkg.api.ValidateRequest(c)    // binding errors → 422

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
12. **nginx** проксирует все три сервиса через `:8080`
13. **Docker Compose** требует `PORT` arg при build каждого сервиса

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

| Документ | Путь |
|----------|------|
| Техническое задание | `docs/task/task.md` |
| TODO (по фазам) | `docs/task/todo.md` |
| Оригинальное ТЗ | `original_task.md` |
| OpenAPI spec | `docs/openapi/panel.yaml` |
| AI Handoff | `docs/ai/AI_HANDOFF.md` |
| Logging standard | `docs/logging.md` |
| Миграции | `migrations/` |

---

## 11. Changelog

> Обновляйте этот раздел при изменениях в архитектуре или статусе.

| Дата | Изменение |
|------|-----------|
| 2026-08-31 | Инициализация AGENTS.md. Статус: Milestone 1-2 готовы, Milestone 3-14 не начаты. Миграция 00006 сломана. |
| 2026-08-31 | Notifications: заменены notification-worker на goroutines в panel |
| 2026-08-31 | Conflict domains: удалены из scope |
| 2026-08-31 | Web UI: удалён из scope |
| 2026-08-31 | Email notifications: удалены из scope |
| 2026-08-31 | Flags CRUD: write операции (POST/PATCH/DELETE) ограничены ролью admin |
| 2026-09-01 | Restructure: `services/panel/internal/` → `services/panel/`, `cmd/main.go` + `cmd/router.go` → `main.go` + `router.go`, `internal/lib/` removed |
| 2026-09-01 | Logging: health handler gains probe failure logging, auth middleware logs security events, dead code removed (`Recovery`, `CallerID`, `RequestLogger`, `RequestFields`) |
| 2026-09-01 | Middleware: dual-framework support (gin + fiber) — `RequestIDGin/Fiber`, `RecoveryGin/Fiber`, `LoggerGin/Fiber`, `CallerIDGin/Fiber` |
| 2026-09-01 | Logger fields: created `pkg/logger/fields.go` with 52 constants, replaced all string literals across 18 files |
| 2026-09-01 | Config: `LogLevel` field, `ValidateSecurity()` moved to `pkg/config`, bootstrap `password_hash` replaces plaintext `password` |
| 2026-09-01 | Services: runtime skeleton (Fiber v3, port :8082), analytics skeleton (Fiber v3, port :8083) |
| 2026-09-01 | Docker: `Dockerfile` (multi-stage, ARG SERVICE/PORT), `docker-compose.yml` (8 services), nginx reverse proxy |
| 2026-09-01 | Makefile: rewritten for developer workflow — `run-local` (fast restart), `dev-up` (infra only), `up` (docker compose), `check`, `dev-status` |

---

## 12. Правила обновления этого файла

1. **При добавлении нового домена** — обновить §4 (статус) и §6 (сущности)
2. **При изменении доменных правил** — обновить §5
3. **При изменении архитектуры** — обновить §3
4. **При добавлении зависимостей** — обновить §2
5. **При изменении code style** — обновить §7
6. **Всегда** — добавить запись в §11 Changelog
