# AI Handoff: Async Analytics Pipeline (M4+)

> Дата: 2026-09-27. Статус: M1–M3 завершены и проверены. Следующий шаг: M4 (Attribution Engine).

---

## 1. Что сделано (M1–M3)

### M1: Event Catalog + Events API
- **Миграция** `migrations/00009_event_catalog.sql`: таблицы `event_types` (каталог с require_exposure) и `event_idempotency_keys` (24h TTL, индекс по expires_at). Применена.
- **Домен** `services/analytics/domain/events/`: model.go (валидация: UUID, subject ≤128 байт, occurred_at ±5мин/−30д, payload ≤32KB; ResolveSubject hash/passthrough; партишн-ключи), dto.go, repository.go, producer.go (envelope ID=event_id, RequireAll, ensure-topic retry), service.go (claim-before-publish + компенсация), handler.go (`POST /events/batch`, `/exposures/batch`, `GET /event-types`; статусы 202/207/400/503).
- **Config**: `analytics.kafka` (group/topics), `analytics.pii` (hash/salts/current version + prod-guard).
- **OpenAPI**: `docs/openapi/analytics.yaml` — контракт трёх эндпоинтов.

### M2: ClickHouse + Raw Tables
- **DDL** `deploy/clickhouse/tables.sql`: 4 таблицы (`events_raw`, `decisions_raw`, `exposures`, `attributed_events`) — месячная партиция, `ORDER BY (event_id)` / `(decision_id, flag_key)`, TTL 90 дней, `ReplacingMergeTree` для идемпотентной перезаписи.
- **Инфраструктура**: docker-compose (сервис `labp-clickhouse`, volume + initdb-маппинг), Makefile цели `clickhouse-up/down/logs/query/tables/clean` (image 24.8, порты 8123/9009, `--pids-limit=2048` — обязателен: default 128 падал с "Not enough threads").
- **Go**: `pkg/database/clickhouse.go` (NewClickHouse: parse DSN, ping, dial timeout), `pkg/clickhouse/writer.go` (batch writer: три очереди, flush по размеру/тику, requeue при ошибке, WrapConn адаптер).
- **Config**: `database.clickhouse.dsn`, `analytics.clickhouse.{batch_size,flush_interval}`.
- **go.mod**: clickhouse-go/v2 переведён из indirect в direct.

### M3: Kafka Consumers + DLQ
- **Фреймворк** `pkg/consumer/consumer.go`: MessageHandler interface, exponential backoff retry (max 3), DLQ routing с headers (`dlq.origin_topic`, `dlq.attempts`, `dlq.error`), `ErrDrop` sentinel (тихий пропуск), `ErrNonRetryable` (немедленный DLQ).
- **Валидатор** `services/analytics/domain/ingest/validator.go`: envelope parse, event type catalog check (PG-backed, background refresh 1 мин), timestamp window ±24h, payload JSON validation. Все ошибки → non-retryable → сразу DLQ.
- **Дедупликатор** `services/analytics/domain/ingest/dedup.go`: in-memory `Deduper` + PG-таблица `kafka_dedup_keys` для выживания рестарта. Дубликат → `ErrDrop`.
- **Wiring** `services/analytics/domain/ingest/pipeline.go`: raw → validator → validated → dedup → deduped → ClickHouse. DLQ topic, graceful shutdown с финальным flush.
- **Миграция** `migrations/00010_kafka_dedup_keys.sql`: persistent dedup таблица. Применена.
- **Config**: `pipeline_group_id`, `events_validated_topic`, `events_deduped_topic`, `dlq_topic`.
- **main.go**: вызов `ingest.StartPipeline` вместо старого `startIngest`.

---

## 2. Текущее состояние пайплайн

```
Runtime /decide → (M5: publish decision) → analytics.decisions.raw
                                           ↓
Events API → analytics.events.raw → validator → analytics.events.validated → dedup → analytics.events.deduped → ClickHouse events_raw
                                                                                           ↓ (M4: attribution)
                                                                                    analytics.attributed_events
                                           ↓
Exposures API → analytics.exposures.raw → ClickHouse exposures
```

**Топики Kafka** (auto-created, 6 партиций, RF=1):
- `analytics.events.raw` — producer (Events API)
- `analytics.events.validated` — validator consumer
- `analytics.events.deduped` — dedup consumer
- `analytics.exposures.raw` — producer (Exposures API)
- `analytics.dlq` — dead letters (3 партиции)

**Consumer groups**:
- `labp-analytics-pipeline` — validator + dedup
- `labp-analytics-ingest` → ClickHouse ingest (deduped + exposures)

**Статус M3** (live smoke подтверждён):
- Event: raw → validated → deduped → ClickHouse (payload `{"sku":"c"}`)
- Kafka-level дубликат (реплей в validated): отброшен дедупликатором (deduped=1, CH=1)
- Poison message в raw: перемещён в `analytics.dlq` с `non-retryable error`

---

## 3. Следующий шаг: M4 — Attribution Engine

### Цель
Реализовать атрибуцию событий к экспериментам по `decision_id` с поддержкой out-of-order доставки.

### Ключевые требования (из ТЗ §6.11, Phase 15)
- `decision_id` — первичный ключ атрибуции
- Event type может требовать exposure (require_exposure=true)
- Technical events (error, latency) могут bypass exposure requirement
- Attribution window (configurable, default 7 дней)
- Out-of-order: conversion до exposure → pending; exposure приходит → attribute; TTL expiry → expired (retained для data-quality)
- Идемпотентность: дубликат exposure/conversion не дублирует атрибуцию

### Архитектура (из плана)
1. **Pending Exposure Store** (Redis):
   - Key: `attribution:pending:{decision_id}` → JSON `{exposure_event, ttl_at}`
   - TTL: attribution window + grace period
   - Atomic `SET NX` для регистрации exposure
2. **Attribution Consumer**:
   - Читает `analytics.events.deduped`
   - Lookup decision_id в pending exposures
   - Если найден: пишет в `attributed_events`, удаляет pending
   - Если не найден и require_exposure: хранит в `pending_conversions` (отдельный key) с TTL
   - Если technical event: пишет в `attributed_events` с NULL experiment/variant
3. **Out-of-order handling**:
   - Conversion до exposure → `pending_conversions` (TTL = window + 1h)
   - Exposure приходит → match + attribute
   - Нет exposure до TTL → пишет `attributed_events` с NULL experiment/variant (expired)
4. **Идемпотентность**:
   - Consumer использует `event_id` из envelope (уже дедуплицирован)
   - Attribution writes идемпотентны (same `event_id` → same row)

### Файлы для создания/изменения
- `services/analytics/domain/attribution/` (new) — consumer, pending store, attribution logic
- `services/analytics/domain/ingest/pipeline.go` — добавить attribution consumer после dedup
- `pkg/config/config.go` — `analytics.attribution.{window_days, late_event_grace_hours}`
- `config.example.json` — соответствующие ENV-шаблоны
- `deploy/clickhouse/tables.sql` — таблица `attributed_events` уже создана
- Unit tests: attribution consumer, pending store TTL, out-of-order scenarios

### Acceptance criteria (из плана)
- Exposure → conversion: корректная атрибуция
- Conversion → exposure: out-of-order, атрибуция после exposure
- No exposure: expired fact с NULL experiment
- Wrong decision_id: не атрибутируется
- Duplicate exposure/conversion: не дублирует атрибуцию
- Late conversion: вне TTL → expired

### Оценка
XL / 8–12 инженерных дней (включая тесты)

---

## 4. Последующие шаги (после M4)

### M5: Runtime Decision Publishing (0.5 дн.)
- Runtime `Service` получает Kafka writer для `analytics.decisions.raw`
- Fire-and-forget publish на каждый `/decide` (bounded channel + background worker)
- DecisionRecord schema: decision_id, request_id, subject_id, flag_key, experiment_id, experiment_version_id, variant_id, result_source, config_revision, created_at
- Топик `analytics.decisions.raw` → pipeline (validator + dedup) → ClickHouse `decisions_raw`

### M6: Metrics Engine (1.5 дн.)
- Миграция `00011_metrics_engine.sql`: таблица `metrics` (extends panel domain)
- Formula AST parser/validator (no raw SQL; parameterized CH queries)
- Types: count, sum, unique_count, ratio, average, percentile (quantileTDigest)
- Built-ins: exposure_count, conversion_count, conversion_rate, error_count, error_rate, avg_latency, p95_latency
- Safe ClickHouse query builders per metric type

### M7: Reports + Data Quality (1 дн.)
- Report API: `GET /api/v1/panel/reports/{experiment_id}?start=&end=&interval=&metrics=&format=`
- Time series, variant breakdown, sample size, allocation skew
- Segmentation dimensions (country, app_version, platform, screen)
- CSV export + persistent URL
- Data quality dashboard: rejected rate, duplicate rate, pending attribution, attribution lag

---

## 5. Архитектурные решения (подтверждены)

| Решение | Выбор |
|---------|--------|
| Partition keys | `decision_id` для decisions; `experiment_id` для events/exposures |
| Schema registry | Inline JSON в `event_types.schema` |
| PII (subject_id) | Configurable: hash (SHA256+salt) или passthrough; default=hash |
| Salt rotation | Rotating salts с `salt_version` в event payload |
| Guardrail output | Write to Kafka topic `analytics.guardrail.triggered` |
| ClickHouse deployment | Single-node dev (docker-compose) |
| DLQ | Kafka topic `analytics.dlq` с headers для origin/attempts/error |

---

## 6. Известные ограничения текущего состояния

- **Idempotency GC**: `event_idempotency_keys` чистятся лениво (по чтению), фонового GC нет — понадобится к M3 completion или M4
- **JSON schema**: `event_types.schema` пока не enforced (структурная валидация только); полная JSON Schema validation — в consumer M3 или M4
- **Poison messages**: ingest consumer (CH) коммитит poison без DLQ; framework DLQ работает только для pipeline consumers (validator, dedup)
- **Decision publishing**: runtime пока не публикует decisions — нужно для M5
- **attributed_events**: таблица создана, но пока не пишется — M4
- **E2E tests**: не добавлены для pipeline (требуют полного стека); unit tests покрывают основную логику

---

## 7. Команды для проверки

```bash
make check                                    # build-all + vet + gofmt
go test ./pkg/... ./services/... -short -count=1
make dev-up                                   # инфраструктура (pg, redis, s3, kafka, clickhouse)
make clickhouse-tables                        # применить CH DDL
make pg-migrate-up                            # применить PG миграции
make clickhouse-query q="SELECT 1"            # проверить CH
make kafka-topics                             # список топиков
```

**Live smoke** (после запуска analytics):
```bash
# Отправить event
curl -s -X POST http://localhost:8083/api/v1/analytics/events/batch \
  -H 'Content-Type: application/json' \
  -d '{"events":[{"event_id":"<uuid>","event_type":"conversion","subject_id":"u1","occurred_at":"<now>","decision_id":"<uuid>","payload":{"sku":"a"}}]}'

# Проверить ClickHouse
make clickhouse-query q="SELECT event_id, event_type FROM labp.events_raw FINAL LIMIT 5"
```

---

## 8. Структура ключевых файлов

```
services/analytics/
├── main.go                    # вызов ingest.StartPipeline
├── app.go                     # NewApp: events handler + health
└── domain/
    ├── events/                # M1: Event Catalog + API
    │   ├── model.go           # валидация, ResolveSubject, партишн-ключи
    │   ├── dto.go             # EventBatchRequest, ExposureBatchRequest
    │   ├── repository.go      # event_types, idempotency_keys
    │   ├── producer.go        # Kafka envelope publisher
    │   ├── service.go         # claim-before-publish, компенсация
    │   └── handler.go         # POST /events/batch, /exposures/batch
    └── ingest/                # M2+M3: pipeline consumers
        ├── consumer.go        # CH ingest consumer (deduped → events_raw)
        ├── rows.go            # envelope → CH row mapping
        ├── validator.go       # envelope + catalog + timestamp validation
        ├── dedup.go           # in-memory + PG dedup
        └── pipeline.go        # wiring: raw → validated → deduped → CH

pkg/
├── consumer/                  # M3: generic consumer framework
│   └── consumer.go            # retry, DLQ, ErrDrop, ErrNonRetryable
├── clickhouse/                # M2: batch writer
│   └── writer.go              # три очереди, flush, requeue
├── database/
│   ├── clickhouse.go          # NewClickHouse
│   └── kafka.go               # NewWriter, NewReader, EnsureTopic
├── outbox/
│   ├── consumer.go            # Deduper, ParseEnvelope
│   ├── message.go             # Envelope, CreateMessage
│   └── publisher.go           # outbox relay
└── config/
    └── config.go              # все конфиги (analytics.kafka, pii, clickhouse)

migrations/
├── 00009_event_catalog.sql    # event_types + idempotency_keys
└── 00010_kafka_dedup_keys.sql # persistent dedup

deploy/clickhouse/
└── tables.sql                 # events_raw, decisions_raw, exposures, attributed_events
```

---

## 9. Приоритеты и зависимости

| Milestone | Depends On | Est. Effort |
|-----------|------------|-------------|
| M4: Attribution Engine | M3 | 2w |
| M5: Decision Publishing | M2 | 0.5w |
| M6: Metrics Engine | M4 | 1.5w |
| M7: Reports | M6 | 1w |

**Рекомендуемый порядок**: M4 → M5 → M6 → M7 (M5 можно параллельно с M4, т.к. независим)

---

## 10. Контактная информация

- Репозиторий: `/opt/Projects/lotty-ab-platform`
- Ветка: master (все изменения в рабочем дереве, не закоммичены)
- Последний коммит: `d72b2ba fix(panel): coherent 400 vs 422 on complete/rollout validation`
- Рабочее дерево содержит незакоммиченные изменения M1–M3 (не коммитить без явного запроса)
