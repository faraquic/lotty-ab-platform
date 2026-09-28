# AGENTS.md — Lotty AB Platform

> Практичный справочник для агентов и разработчиков. Все утверждения подтверждены файлами репозитория.
> Последнее обновление: 2026-09-28

## 1. Overview

**Lotty AB Platform** — self-hosted A/B testing platform: feature flags, controlled experiments,
review workflow, runtime variant delivery, event attribution и analytics. Solo pet project,
запускается локально через Docker Compose или бинарники.

Три Go-сервиса. PostgreSQL — source of truth для control plane; runtime обслуживает decide API
из in-memory snapshot (Redis). Frontend в текущем рабочем дереве недоступен; его удалённые
файлы не восстанавливать без отдельного запроса.

- **Backend:** Go 1.27, Gin (panel), Fiber v3 (runtime/analytics), pgx/v5, rueidis, aws-sdk-go-v2,
  clickhouse-go/v2, segmentio/kafka-go.
- **Инфраструктура:** PostgreSQL 18, Redis 8, MinIO (S3), Kafka (KRaft), ClickHouse 24.8, nginx.

## 2. Структура репозитория

```
services/
├── panel/       control plane (Gin, :8081) + домены: auth, users, flags, metrics, experiments, reviews, reports
├── runtime/     decide API (Fiber v3, :8082): xxhash64 buckets, allocation, snapshot, decision publishing
└── analytics/   analytics (Fiber v3, :8083): Events API, attribution pipeline, ClickHouse ingest

pkg/             api, audit, auth, config, consumer, clickhouse, database, dto, logger,
                 metrics, middleware, outbox, snapshot, targeting
migrations/      goose SQL: 00001_users … 00011_metrics_engine (проверять git status)
tests/e2e/       панель и runtime end-to-end (запускаются с E2E_TEST=1, -p 1)
tools/           s3-provision
deploy/
├── nginx/       nginx.conf — reverse proxy для compose
└── clickhouse/  tables.sql — DDL для ClickHouse (применяется через make clickhouse-tables)
docs/openapi/    panel.yaml, runtime.yaml, analytics.yaml (source of truth для API)
docs/            logging.md, deploy-dev.md, ai/AI_HANDOFF.md, ai/AI_HANDOFF_M4.md;
                 task.ru.md и todo.md в docs/task (gitignored)
config.*.json    конфиги (см. §8)
```

## 3. Архитектура

- **panel :8081** — control plane: JWT (HS256) + Redis-сессии, RBAC, CRUD доменов; PostgreSQL —
  источник истины. Публичные маршруты под `/api/v1/panel`. Reports API (`GET /reports/{id}`,
  `/data-quality`, CSV export) и Metrics Engine (formula AST, ClickHouse query builder, built-ins).
- **runtime :8082** — evaluate из in-memory snapshot; публичный `POST /api/v1/runtime/decide`.
  Decision publishing: fire-and-forget в Kafka `analytics.decisions.raw` (bounded channel, background worker).
- **analytics :8083** — Events API (`POST /events/batch`, `/exposures/batch`, `GET /event-types`),
  attribution pipeline (validator → dedup → attributor → ClickHouse), decision ingestion.
- **nginx :8080** проксирует `/api/v1/{panel,runtime,analytics}`; web UI не поставляется.

### Kafka / Outbox / ClickHouse

- **Kafka** (KRaft, single-broker в compose): топики `analytics.events.raw`, `analytics.events.validated`,
  `analytics.events.deduped`, `analytics.exposures.raw`, `analytics.decisions.raw`, `analytics.dlq`.
  Consumer groups: `labp-analytics-pipeline` (validator+dedup), `labp-analytics-attribution`,
  `labp-analytics-ingest` (ClickHouse).
- **Outbox**: `pkg/outbox/publisher.go` — relay с SKIP LOCKED, exponential backoff, dead-letter.
  Envelope ID = event_id для дедупликации на стороне консьюмеров.
- **ClickHouse**: 5 таблиц (`events_raw`, `decisions_raw`, `exposures`, `attributed_events`,
  `metrics_cache`), `ReplacingMergeTree`, месячная партиция, TTL 90 дней. Batch writer
  (`pkg/clickhouse/writer.go`): три очереди, flush по размеру/тику, requeue при ошибке.
- **Attribution**: Redis pending store (`attribution:pending:exposure:{decision_id}`,
  `attribution:pending:conversion:{decision_id}`), sweeper для TTL expiry, exposure catalog
  (PG-backed, background refresh). Out-of-order: conversion до exposure → pending; exposure приходит →
  attribute; TTL → expired fact с NULL experiment/variant.
- **Consumer framework** (`pkg/consumer`): MessageHandler interface, exponential backoff retry,
  DLQ routing с headers, `ErrDrop` sentinel, `ErrNonRetryable` для немедленного DLQ.

Проверенный срез требований на 2026-09-28 описан в локальном `docs/task/todo.md`. Существенные
ограничения текущего рабочего дерева: `pkg/targeting` содержит parser, но evaluator не подключён
к runtime; decide DTO принимает attributes, но решение их не оценивает; слишком старый snapshot
может продолжать обслуживать запросы; audit table/helper покрывают лишь часть мутаций. Перед
работой сверять фактический код: этот срез включает локальные, незакоммиченные изменения.

## 4. Команды разработки

Все команды запускаются из корня репозитория (подтверждено `Makefile`).

| Задача                | Команда                                                                                    |
| --------------------- | ------------------------------------------------------------------------------------------ |
| Инфраструктура        | `make dev-up` / `make dev-down` / `make dev-status` / `make dev-clean`                     |
| Локальный запуск      | `make run-local` / `make stop-local` / `make restart-local`                                |
| Точечный запуск       | `make run-panel` / `make run-runtime` / `make run-analytics`                               |
| Сборка Go-сервисов    | `make build-all` (или `build-panel`, `build-runtime`, `build-analytics`; цели `build` нет) |
| Полный стек в Compose | `make up` / `make down` / `make logs` / `make rebuild`                                     |
| Миграции              | `make pg-migrate-up` / `pg-migrate-down` / `pg-migrate-status` / `pg-migrate-new name=…`   |
| БД / кэш              | `make psql` / `make redis-cli`                                                             |
| S3                    | `make s3-provision` / `make s3-provision-e2e`                                              |
| ClickHouse            | `make clickhouse-tables` / `make clickhouse-query q="SELECT 1"` / `make clickhouse-logs`   |
| Kafka                 | `make kafka-topics` / `make kafka-logs`                                                    |

Порты: panel `:8081`, runtime `:8082`, analytics `:8083`, nginx `:8080`, Postgres host `:5433`,
Redis `:6379`, MinIO `:9000`/`:9001`, Kafka `:9092`, ClickHouse HTTP `:8123` / native `:9009`.

Требуют Docker/podman и запущенных контейнеров (`make dev-up`): все `make run-*` (через
`_ensure-infra`), `make psql`, `make redis-cli`, `make test-e2e`, `make up/down/logs/rebuild`,
`make clickhouse-*`, `make kafka-*`.
Деструктивны: `make dev-clean` (удаляет volumes с данными), `make pg-migrate-down` (откатывает
последнюю миграцию), `make rebuild`/`make down` (пересобирают/останавливают compose-стек) — не
запускать без явной необходимости и подтверждения.

## 5. Validation (обязательно перед завершением задачи)

```bash
make check                                # build-all + vet + gofmt -l
go test ./pkg/... -short -count=1
go test ./services/... -short -count=1
```

`make check` включает сборку Go-сервисов и `go vet ./...`.
Важно: `make check` не запускает Go-тесты; `gofmt -l` только печатает список
неформатированных файлов и не роняет цель. Полный локальный прогон: `make check`, затем
`go test ./pkg/... ./services/... -short -count=1`.
Интеграционные e2e требуют поднятой инфраструктуры: `make dev-up && make test-e2e`.

## 6. Conventions

- **JSON-кодек:** `goccy/go-json`, не `encoding/json`.
- **ID path params:** Gin — `c.Param("id")` + `uuid.Parse` (ID — UUIDv7); Fiber — `c.Params("id")`.
  `strconv.ParseUint` не использовать; в runtime параметризованных маршрутов нет.
- **Домен-пакет (обычно):** `handler.go`, `service.go`, `repository.go`, `model.go`, `dto.go`;
  набор отличается по домену (например, `runtime/domain/decide` — без `model.go`).
- **Ответы panel (Gin):** `pkg/api` Writer-варианты (`OK`, `Error`, `InternalError`,
  `ValidateRequest`) с `c.Writer`; рядом есть `*Gin`-варианты. Fiber — `pkg/api` fiber-варианты /
  `c.Status(...).JSON(...)`.
- **Ошибки домена:** sentinel-ошибки (`ErrNotFound`, `ErrConflict`, …), маппинг в handler.
- **Валидация:** `go-playground/validator/v10` (`binding:"..."`).
- **Middleware:** dual-framework — `*Gin` и `*Fiber` версии в `pkg/middleware`.
- **Zap-поля:** использовать константы `pkg/logger.Field*`, не строковые литералы.
- **Комментарии:** не добавлять без явной просьбы. Без эмодзи.

## 7. Testing

- **Unit (Go):** `pkg/{snapshot,consumer,clickhouse,metrics,outbox,targeting}`,
  `services/panel/domain/{experiments,metrics,reviews,reports}`,
  `services/runtime/domain/decide`, `services/analytics/domain/{events,ingest,attribution}`.
  Запуск: `go test ./pkg/... ./services/... -short -count=1`.
- **E2E:** `tests/e2e/{panel,runtime}`; используют общую БД `labp_e2e`, фиксированные порты
  (panel `18080`; runtime-suite — `18081`/`18083`), bucket `labp-e2e`; обязателен `-p 1`.
  Требуют `make dev-up` и `make test-e2e`.
- **E2E analytics pipeline** (добавить): `tests/e2e/analytics/` — Events API → Kafka → ClickHouse,
  attribution flow, dedup, DLQ. Требуют `make dev-up` (включая ClickHouse и Kafka).
- **CI:** `.github/workflows/ci-cd.yml` (в `HEAD`) гоняет `go test ./pkg/...` (без `./services/...`),
  `make check`, `make dev-up` + `make test-e2e`, `docker compose config`,
  `docker compose build`; деплой на dev только при push в `master`. Файл в рабочем дереве сейчас
  удалён — статус неясен, перед правками CI свериться с `HEAD`.

## 8. Configuration

- Загрузка: `CONFIG_NAME` (абсолютный или базовое имя) → `config.local.json` → `config.json`,
  поиск в `.` и `/labp` (`pkg/config/config.go`).
- Трекается только шаблон `config.example.json`; `config.local.json`, `.env*` — gitignored.
- `docker compose` по умолчанию монтирует `config.example.json`; локальные бинарники используют
  `config.local.json` (создать из шаблона и заполнить значениями окружения).
- Шаблон поддерживает `{{ ENV "VAR" }}` для подстановки переменных окружения. Compose получает
  значения из окружения или `.env`; локальную копию `config.local.json` нужно заполнить и
  экспортировать нужные переменные перед `make run-local`.
- `ServiceVersion` инжектится через `-ldflags` (`pkg/config.ServiceVersion`).
- Дефолт `environment` — `prod` (`defaultConfig`); без конфига `MustLoad` завершает процесс.
  Локально в корне уже может лежать gitignored `config.local.json` — он приоритетнее `config.json`.
- **Секреты:** `config.example.json` содержит env-шаблоны, а локальные `.env`/`config.local.json`
  не трекаются. Для prod задать реальные секреты: `ValidateSecurity`
  завершает процесс при `change-me*` в `environment: prod`. Реальные секреты не коммитить и не
  цитировать их значения (в т.ч. из `config.local.json` / `.env*`) в отчётах и диффах.

### Analytics / ClickHouse / Kafka config

```json
{
  "database": {
    "clickhouse": { "dsn": "clickhouse://labp:labppassword@localhost:9009/labp" }
  },
  "analytics": {
    "kafka": {
      "group_id": "labp-analytics",
      "pipeline_group_id": "labp-analytics-pipeline",
      "ingest_group_id": "labp-analytics-ingest",
      "attribution_group_id": "labp-analytics-attribution",
      "events_topic": "analytics.events.raw",
      "exposures_topic": "analytics.exposures.raw",
      "events_validated_topic": "analytics.events.validated",
      "events_deduped_topic": "analytics.events.deduped",
      "dlq_topic": "analytics.dlq"
    },
    "clickhouse": { "batch_size": 1000, "flush_interval": "1s" },
    "attribution": { "window_days": 7, "late_event_grace_hours": 1 },
    "pii": {
      "hash_subject_id": true,
      "salts": { "v1": "..." },
      "current_salt_version": "v1"
    }
  },
  "runtime": {
    "kafka": { "decisions_topic": "analytics.decisions.raw" }
  }
}
```

## 9. Protected / generated files

Не редактировать вручную:

- `bin/` — результаты сборки (gitignored).
- `go.sum` — генерируется Go toolchain.
- Применённые миграции `migrations/*.sql` — только добавление новых через `make pg-migrate-new`.
- `docs/openapi/*.yaml` — контракт; менять синхронно с backend.

Служебные/локальные (gitignored): `.opencode/`, `docs/task/`, `.vscode/`, `.idea/`, `*.log`.

## 10. Change boundaries

- Не менять зависимости, CI, миграции и документацию без явного запроса.
- Не редактировать чужие домены «попутно»: изменения по задаче — минимальны и локальны.
- `.dockerignore` — whitelist (`go.mod`, `go.sum`, `pkg/`, `services/`), плюс исключение
  `**/*_test.go`; в образ не попадают `migrations/`, `tools/`, конфиги, `deploy/`. При добавлении
  новых корневых путей, нужных образу, обновлять `.dockerignore` и `Dockerfile`.
- Публичный API/services/фронтенд-контракты выводить из `docs/openapi/`, не придумывать endpoints.
- Рабочее дерево может содержать пользовательские изменения (правки, untracked-файлы, удалённый
  CI). Не восстанавливать, не удалять, не форматировать и не переписывать их; работать только в
  границах задачи и сверяться с `git status`.

## 11. Definition of Done

- Код собирается: `make check` зелёный.
- Go unit-тесты (`go test ./pkg/... ./services/... -short -count=1`) зелёные; при изменении
  доменов/API — обновлены или добавлены тесты.
- Миграции (если затронуты) применяются вверх/вниз без ошибок; `make pg-migrate-down` откатывает
  последнюю миграцию — выполнять только по явному запросу.
- Нет секретов в диффе, нет сломанных ссылок в документации, нет TODO-заглушек в поставленной задаче.

## 12. Workflow агента

1. Изучить `AGENTS.md` (этот файл); OpenAPI — источник контракта.
2. Определить границы изменения; не трогать лишнее. Свериться с `git status`, не задевать чужие
   изменения.
3. Реализовать минимально достаточное решение в существующем стиле.
4. Прогнать релевантные проверки из §5 и §7; исправить ошибки.
5. Показать краткий отчёт: изменённые файлы, добавленные/убранные зависимости, что сделано,
   результаты проверок (что запускалось, а что нет), известные ограничения.
6. Коммит — только по явному запросу.

## 13. Ссылки

| Документ       | Путь                                                        |
| -------------- | ----------------------------------------------------------- |
| OpenAPI        | `docs/openapi/panel.yaml`, `runtime.yaml`, `analytics.yaml` |
| Логирование    | `docs/logging.md`                                           |
| Dev deployment | `docs/deploy-dev.md`                                        |
| AI handoff     | `docs/ai/AI_HANDOFF.md`, `docs/ai/AI_HANDOFF_M4.md`         |
| ТЗ (локально)  | `docs/task/task.ru.md`, `docs/task/todo.md`                 |
