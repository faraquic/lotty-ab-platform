# Lotty A/B Platform

Self-hosted платформа A/B-тестирования: feature flags, контролируемые эксперименты с
ревью-воркфлоу, детерминированная раздача вариантов в рантайме и асинхронная
аналитика на Kafka + ClickHouse.

Три Go-сервиса, PostgreSQL как источник истины для control plane, in-memory snapshot
для hot path раздачи флагов. Гарантии доставки — at-least-once с идемпотентными
потребителями, SLA на `/decide` — p99 < 10 мс при 256 одновременных соединениях
(проверяется нагрузочным тестом в CI).

---

## Содержание

- [Возможности](#возможности)
- [Архитектура](#архитектура)
- [Стек и почему именно он](#стек-и-почему-именно-он)
- [Быстрый старт](#быстрый-старт)
- [Демо-сценарий end-to-end](#демо-сценарий-end-to-end)
- [Проектные решения](#проектные-решения)
  - [Transactional Outbox](#transactional-outbox)
  - [Идемпотентность API](#идемпотентность-api)
  - [Идемпотентные консьюмеры](#идемпотентные-консьюмеры)
  - [Асинхронный пайплайн аналитики](#асинхронный-пайплайн-аналитики)
  - [Схемы ClickHouse](#схемы-clickhouse)
  - [Атрибуция по decision_id](#атрибуция-по-decision_id)
  - [Наблюдаемость](#наблюдаемость)
  - [Performance SLA](#performance-sla)
  - [Graceful degradation](#graceful-degradation)
- [Тестирование и quality gates](#тестирование-и-quality-gates)
- [Репозиторий](#репозиторий)
- [Конфигурация](#конфигурация)
- [API-контракты](#api-контракты)
- [CI/CD](#cicd)
- [Статус по требованиям](#статус-по-требованиям)
- [Известные ограничения](#известные-ограничения)

---

## Возможности

**Control plane (panel, `:8081`)**

- Пользователи, роли `admin / experimenter / approver / viewer`, Argon2id, JWT HS256
  с Redis-сессиями и немедленным отзывом токена.
- Feature flags с типом `string / number / bool`, тип флага неизменяем после создания.
- Эксперименты: версии (published-версии неизменяемы), варианты с весами в базисных
  пунктах, targeting DSL, распределение по basis points с проверкой инвариантов на
  уровне БД (deferred constraint trigger).
- Ревью-воркфлоу: approver-группы, `min_approvals`, комментарии, заявка на изменения,
  отклонение; админ может согласовать в обход групп.
- Полный жизненный цикл: `draft → review → approved → running → paused / completed → archived`,
  плюс rollout победителя и откат к контрольной группе.
- Append-only аудит мутаций (`audit_records` защищена триггерами от UPDATE/DELETE/TRUNCATE).
- Metrics Engine: формулы на AST (lexer → parser → validator → query builder) с
  built-in метриками, отчёты по эксперименту, data-quality и CSV-экспорт.

**Runtime (`:8082`)**

- `POST /api/v1/runtime/decide` — решение по in-memory snapshot: ни одного запроса в
  PostgreSQL на hot path.
- Детерминированное назначение варианта: xxhash64 от `subject_id` + соль версии,
  sticky-распределение, allocation в basis points.
- Targeting DSL вычисляется на решении; при отсутствующем атрибуте — fail closed
  (субъект не попадает в эксперимент, отдаётся default флага).
- Атомарный доступ к snapshot в памяти, guard по «свежести» (`max_stale_age`, по
  умолчанию 5 минут) и флаг `degraded=true` в ответе.
- Публикация факта решения в Kafka — fire-and-forget через ограниченный канал,
  решение никогда не блокирует ответ клиенту.

**Analytics (`:8083`)**

- Events API: `POST /events/batch`, `POST /exposures/batch`, `GET /event-types`.
- Каталог типов событий в PostgreSQL с флагом `require_exposure`.
- PII-политика: хэширование `subject_id` (SHA-256 + соль) с версионированием солей.
- Kafka-конвейер `raw → validated → deduped` с DLQ, атрибуция по `decision_id`,
  батч-писатель в ClickHouse.

---

## Архитектура

```mermaid
graph LR
    subgraph clients["Клиенты приложений"]
        APP[Приложение / SDK]
    end

    subgraph control["Control plane"]
        PANEL["panel :8081<br/>Gin · PostgreSQL · Redis"]
        NGINX["nginx :8080<br/>reverse proxy"]
    end

    subgraph delivery["Раздача"]
        RT["runtime :8082<br/>Fiber · in-memory snapshot"]
    end

    subgraph bus["Шина событий"]
        KAFKA["Kafka<br/>snapshot · decisions · analytics"]
    end

    subgraph analytics["Аналитика"]
        AN["analytics :8083<br/>Fiber · Events API"]
        CH["ClickHouse 24.8<br/>ReplacingMergeTree"]
    end

    subgraph obs["Наблюдаемость"]
        PROM[Prometheus]
        LOKI[Loki]
        GRAF[Grafana]
    end

    PG[("PostgreSQL 18<br/>source of truth")]
    REDIS[("Redis 8<br/>snapshot · сессии")]
    S3[("MinIO / S3<br/>аватары")]

    APP --> NGINX
    NGINX --> PANEL
    NGINX --> RT
    NGINX --> AN

    PANEL --> PG
    PANEL --> REDIS
    PANEL --> S3
    PANEL -.->|outbox relay| KAFKA
    PANEL -->|публикация snapshot| REDIS
    PANEL -->|публикация snapshot| KAFKA
    KAFKA -->|consumer group| RT

    RT -.->|decisions (fire-and-forget)| KAFKA
    AN -->|publish| KAFKA
    KAFKA -->|validator → dedup → attribution| AN
    AN -->|batch insert| CH

    PANEL -->|/metrics| PROM
    RT -->|/metrics| PROM
    AN -->|/metrics| PROM
    LOKI -.-> GRAF
    PROM --> GRAF
```

Горячий путь `/decide` целиком укладывается в память процесса:

1. `runtime` держит последний snapshot в атомарной переменной (protobuf-JSON с
   `revision`), обновление — из Redis (публикация panel) и из Kafka-топика snapshot.
2. Запрос: xxhash64(subject) → allocation в basis points → вариант эксперимента либо
   default флага. Отдаётся `decision_id` — ключ, по которому позже связывается
   конверсия с фактом показа.
3. Если snapshot старше `max_stale_age` — ответ помечается `degraded: true`, но
   **продолжает обслуживаться** (fail-open на свежесть, fail-closed на targeting).
4. Если snapshot ещё ни разу не загрузился — `503 SNAPSHOT_UNAVAILABLE`.

Аналитика никогда не участвует в ответе `/decide`: события уходят в Kafka
асинхронно и не блокируют клиента.

---

## Стек и почему именно он

| Слой | Технология | Почему |
| --- | --- | --- |
| Язык | Go 1.27 | Статическая типизация, дешёвые горутины для concurrent-пула, статические бинарники в scratch-образе (CGO_ENABLED=0), предсказуемая латентность без GC-пауз |
| Control plane HTTP | Gin | Требовал зрелой экосистемы middleware (JWT, CORS, recovery) и привычной интеграции с `go-playground/validator` |
| Runtime и Analytics HTTP | Fiber v3 | На порядок меньше аллокаций и overhead на маршрутизацию, что важно для целевого p99 < 10 мс; общая кодовая база с Gin сохраняется через двухфреймворковые middleware-варианты |
| Source of truth | PostgreSQL 18 | Транзакции с `SELECT … FOR UPDATE SKIP LOCKED` для outbox-релея,
 deferred constraint triggers для инвариантов вариантов, jsonb для targeting AST,
 append-only таблица аудита на триггерах |
| Hot-path конфигурация | Redis 8 | Публикация snapshot без базы данных на пути запроса; здесь же сессии и
 отзыв токенов, и pending-store атрибуции |
| Событийная шина | Kafka (KRaft) | Гарантия at-least-once, переигрывание, DLQ, независимые группы
 потребителей с разным темпом; нужна именно для аналитики, где допустима eventual
 доставка |
| Аналитическое хранилище | ClickHouse 24.8 | Колоночное хранение, партиционирование по месяцам, TTL на уровне
 движка, `ReplacingMergeTree` даёт дедупликацию на стороне хранилища |
| Метрики | Prometheus + Grafana | Стандарт индустрии: pull-модель, лёгкие histogram-метрики, готовые
 p99-панели |
| Логи | Zap (JSON) + Loki | Структурированные логи с `request_id`/`trace_id` для сквозной
 корреляции, Loki — лёгкое локальное хранилище логов с тем же Grafana |
| Миграции | goose | SQL-файлы как единственный источник правды, up/down в каждом файле |
| JSON | `goccy/go-json` | Совместим с `encoding/json`, но быстрее на горячих путях |
| Хэширование | `cespare/xxhash/v2` | Быстрый некриптографический хэш для стабильного распределения
 вариантов |
| Пароли | `alexedwards/argon2id` | Современная KDF для хранения паролей |
| Метрики-формулы | Собственный AST | Формулы метрик валидируются и компилируются в SQL ClickHouse, а не
 склеиваются строками |

---

## Быстрый старт

Требования: Go 1.27, Docker или Podman, `make`. Kafka поднимается только через
`make kafka-up` (в `docker-compose.yml` его нет), поэтому для полного стека —
локальные бинарники.

### Путь A. `docker compose` — control plane, раздача флагов, ClickHouse

Быстрый путь для просмотра панели и проверки `/decide`. Kafka не поднимается, поэтому
outbox-релей и аналитический конвейер отключены (в логах — соответствующие warning).

```bash
export BOOTSTRAP_PASSWORD_HASH='<argon2id-хэш, см. ниже>'
export JWT_SECRET_KEY='local-dev-secret'
make up          # postgres, redis, s3, миграции, сборка и старт панели/рантайма/аналитики
make logs        # логи всех сервисов
make down        # остановить
```

Миграции применяются автоматически (`make up` вызывает `make pg-migrate-up`).
Bootstrap-админ создаётся при первом старте, если таблица пользователей пуста.

### Путь B. Полный стек — с Kafka, outbox, конвейером и атрибуцией

```bash
cp config.example.json config.local.json
```

`config.local.json` — локальная копия с `{{ ENV "…" }}`-шаблонами; сам файл в git не
отслеживается. Заполните окружение (для локальных бинарников обязательны также
переменные аналитики и ClickHouse — они не входят в список экспорта Makefile):

```bash
export CONFIG_NAME=config.local.json
export APP_ENVIRONMENT=local
export LOG_LEVEL=debug
export JWT_SECRET_KEY='local-dev-secret'
export BOOTSTRAP_PASSWORD_HASH='<argon2id-хэш>'
export POSTGRES_DSN='postgres://lotty:lottypassword@localhost:5433/labp?sslmode=disable'
export REDIS_ADDRESS=localhost:6379
export KAFKA_BROKERS=localhost:9092
export KAFKA_SNAPSHOT_GROUP_ID=labp-runtime-snapshot
export CLICKHOUSE_DSN='clickhouse://labp:labppassword@localhost:9009/labp'
export S3_BUCKET=labp
export S3_REGION=us-east-1
export S3_ENDPOINT=http://localhost:9000
export S3_ACCESS_KEY=minioadmin
export S3_SECRET_KEY=minioadmin
export RUNTIME_KAFKA_DECISIONS_TOPIC=analytics.decisions.raw
export ANALYTICS_KAFKA_GROUP_ID=labp-analytics
export ANALYTICS_KAFKA_PIPELINE_GROUP_ID=labp-analytics-pipeline
export ANALYTICS_KAFKA_INGEST_GROUP_ID=labp-analytics-ingest
export ANALYTICS_KAFKA_ATTRIBUTION_GROUP_ID=labp-analytics-attribution
export ANALYTICS_EVENTS_TOPIC=analytics.events.raw
export ANALYTICS_EXPOSURES_TOPIC=analytics.exposures.raw
export ANALYTICS_DECISIONS_TOPIC=analytics.decisions.raw
export ANALYTICS_EVENTS_VALIDATED_TOPIC=analytics.events.validated
export ANALYTICS_EVENTS_DEDUPED_TOPIC=analytics.events.deduped
export ANALYTICS_DLQ_TOPIC=analytics.dlq
export ANALYTICS_PII_SALT_V1='local-dev-salt'

make dev-up      # postgres, redis, s3, kafka, clickhouse, prometheus, loki, grafana
make pg-migrate-up
make s3-provision
make run-local   # сборка и запуск panel, runtime, analytics как локальных процессов
```

Остановить: `make stop-local && make dev-down`. Полный сброс с удалением данных:
`make dev-clean`.

Наблюдаемость: Prometheus `:9090`, Loki `:3100`, Grafana `:3000` (`admin` / `admin`,
анонимный просмотр включён).

### Bootstrap-пароль

Панель сравнивает пароль с argon2id-хэшем. Сгенерировать его можно, не выходя из
репозитория (временный файл удаляется сразу):

```bash
cat > tmp-hashpwd.go <<'EOF'
package main

import (
	"fmt"
	"os"

	"github.com/alexedwards/argon2id"
)

func main() {
	hash, err := argon2id.CreateHash(os.Args[1], argon2id.DefaultParams)
	if err != nil {
		panic(err)
	}
	fmt.Println(hash)
}
EOF
go run tmp-hashpwd.go 'my-password' && rm tmp-hashpwd.go
```

### Порты

| Порт | Сервис |
| --- | --- |
| 8080 | nginx (проксирует `/api/v1/{panel,runtime,analytics}`) |
| 8081 | panel |
| 8082 | runtime |
| 8083 | analytics |
| 5433 | PostgreSQL (host) |
| 6379 | Redis |
| 9000 / 9001 | MinIO API / Console |
| 9092 | Kafka |
| 8123 / 9009 | ClickHouse HTTP / native |
| 9090 | Prometheus |
| 3100 | Loki |
| 3000 | Grafana |

---

## Демо-сценарий end-to-end

Полный цикл: флаг → эксперимент → ревью → запуск → решение → событие → атрибуция →
отчёт. Все команды выполняются с поднятым стеком (путь A или B; для аналитики
нужен путь B).

```bash
# 1. Логин bootstrap-админом
TOKEN=$(curl -sS -X POST localhost:8081/api/v1/panel/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.invalid","password":"my-password"}' | jq -r .data.token)
AUTH="Authorization: Bearer $TOKEN"

# 2. Feature flag
FLAG_ID=$(curl -sS -X POST localhost:8081/api/v1/panel/flags \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"key":"checkout_redesign","name":"Checkout redesign","type":"string","default_value":"control"}' \
  | jq -r .data.id)

# 3. Эксперимент: две версии-варианта по 50%, targeting по стране
EXP_ID=$(curl -sS -X POST localhost:8081/api/v1/panel/experiments \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{
        \"flag_id\": \"$FLAG_ID\",
        \"name\": \"Checkout redesign test\",
        \"weights_total\": 10000,
        \"targeting\": \"country == 'DE'\",
        \"variants\": [
          {\"name\":\"control\",\"value\":\"control\",\"weight_bp\":5000,\"is_control\":true},
          {\"name\":\"treatment\",\"value\":\"new-checkout\",\"weight_bp\":5000,\"is_control\":false}
        ]
      }" | jq -r .data.id)

# 4. На ревью и согласование (админ проходит checkEligible с порогом 1)
curl -sS -X POST "localhost:8081/api/v1/panel/experiments/$EXP_ID/submit" \
  -H "$AUTH" -H 'Content-Type: application/json' -d '{"version":1}' > /dev/null

REVIEW_ID=$(curl -sS "localhost:8081/api/v1/panel/reviews?status=open" -H "$AUTH" | jq -r '.data[0].id')
curl -sS -X POST "localhost:8081/api/v1/panel/reviews/$REVIEW_ID/approvals" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"decision":"approve","version":1,"comment":"ok"}' > /dev/null

# 5. Запуск: snapshot уходит в Redis и в Kafka
curl -sS -X POST "localhost:8081/api/v1/panel/experiments/$EXP_ID/start" \
  -H "$AUTH" -H 'Content-Type: application/json' -d '{"version":1}' > /dev/null

# 6. Решение: subject попадает в allocation и получает decision_id
DECIDE=$(curl -sS -X POST localhost:8082/api/v1/runtime/decide \
  -H 'Content-Type: application/json' \
  -d '{"subject_id":"user-42","attributes":{"country":"DE"},"flags":["checkout_redesign"]}')
echo "$DECIDE" | jq

D=$(echo "$DECIDE" | jq -r '.data.flags["checkout_redesign"]')
DECISION_ID=$(echo "$D"   | jq -r '.decision_id')
VARIANT_ID=$(echo "$D"     | jq -r '.variant_id')
VERSION_ID=$(curl -sS "localhost:8081/api/v1/panel/experiments/$EXP_ID" -H "$AUTH" \
  | jq -r '.data.current_version_id')
```

Ответ содержит `flags["checkout_redesign"]` с `value`, `source` (`experiment` либо
`default`), `experiment_id`, `experiment_version`, `variant_id` и `decision_id`.
Пока флаг активен в snapshot, `source` будет `experiment`; иначе — `default`.

Продолжение — события аналитики (`event_id` должен оставаться стабильным: повтор с тем
же `event_id` вернёт `duplicate`, а не создаст дубль):

```bash
# 7. Факт показа: decision_id связывает событие с решением
curl -sS -X POST localhost:8083/api/v1/analytics/exposures/batch \
  -H 'Content-Type: application/json' \
  -d "{\"exposures\":[{
        \"event_id\":\"$(cat /proc/sys/kernel/random/uuid)\",
        \"decision_id\":\"$DECISION_ID\",
        \"subject_id\":\"user-42\",
        \"experiment_id\":\"$EXP_ID\",
        \"experiment_version_id\":\"$VERSION_ID\",
        \"variant_id\":\"$VARIANT_ID\",
        \"flag_id\":\"$FLAG_ID\",
        \"occurred_at\":\"$(date -u +%Y-%m-%dT%H:%M:%S.000Z)\"
      }]}"

# 8. Конверсия, привязанная к тому же decision_id
curl -sS -X POST localhost:8083/api/v1/analytics/events/batch \
  -H 'Content-Type: application/json' \
  -d "{\"events\":[{
        \"event_id\":\"$(cat /proc/sys/kernel/random/uuid)\",
        \"event_type\":\"conversion\",
        \"subject_id\":\"user-42\",
        \"occurred_at\":\"$(date -u +%Y-%m-%dT%H:%M:%S.000Z)\",
        \"decision_id\":\"$DECISION_ID\",
        \"payload\":{\"amount\":49.90,\"currency\":\"EUR\"}
      }]}"

# 9. Проверяем, что событие атрибутировано к эксперименту и варианту
make clickhouse-query q="SELECT event_type, experiment_id, variant_id, subject_id FROM labp.attributed_events ORDER BY received_at DESC LIMIT 5"

# 10. Отчёт по эксперименту, качество данных и CSV-экспорт
curl -sS "localhost:8081/api/v1/panel/reports/$EXP_ID" -H "$AUTH" | jq
curl -sS "localhost:8081/api/v1/panel/reports/$EXP_ID/data-quality" -H "$AUTH" | jq
curl -sS "localhost:8081/api/v1/panel/reports/$EXP_ID?format=csv" -H "$AUTH" -o report.csv
```

Повторите шаг 8 с тем же `event_id` — ответ будет `202 Accepted` со статусом
`duplicate`, а в ClickHouse появится ровно одна строка: это и есть сквозная
идемпотентность (API → Kafka → консьюмер → хранилище).

---

## Проектные решения

### Transactional Outbox

Публикация доменных событий в Kafka реализована паттерном **transactional outbox**:
изменение в PostgreSQL и запись в таблицу `outbox` происходят в одной транзакции,
а отдельный релей доставляет записи в брокер — гарантия **at-least-once**.

`pkg/outbox/publisher.go`:

- Опрос `outbox` каждые 500 мс, до 100 записей за проход.
- `SELECT … FOR UPDATE SKIP LOCKED` — параллельные релеи не мешают друг другу.
- Одна транзакция на одну запись: «ядовитая» запись не блокирует очередь за ней, в
  отличие от batch-транзакции.
- `published` выставляется **только после** подтверждения брокера.
- Экспоненциальный backoff 1s → 5min; после 10 попыток запись переходит в
  `status='dead'` с `last_error` — это dead-letter внутри таблицы.
- Таймаут публикации 10 с; при `UnknownTopic` тема создаётся и публикация
  повторяется один раз.
- Конверт `Envelope{ID, Type, Source, Time, Data}`: `ID` — идентификатор записи
  outbox, по нему консьюмеры дедуплицируют.

Записи пишутся в той же транзакции, что и доменное изменение
(`services/panel/domain/experiments/repository.go`: эксперимент, версия, варианты,
аудит и outbox коммитятся вместе). Если Kafka недоступен, строки накапливаются и
уходят при следующем запуске релея.

### Идемпотентность API

`pkg/middleware/idempotency.go` поддерживает заголовок `Idempotency-Key` для всех
`POST` / `PUT` / `PATCH` в аутентифицированных группах panel.

- **TTL 24 часа** (Redis `SET … EX`).
- Ключ хранится в скоупе `caller | method | route | key` — один и тот же ключ в
  разных маршрутах не конфликтует; чужой пользователь не может перехватить чужой ключ.
- Тело запроса хэшируется SHA-256 (до 1 MiB) и сохраняется вместе с телом ответа,
  статусом и `Content-Type`.
- **Повтор с тем же телом** → сохранённый ответ 2xx и заголовок
  `Idempotent-Replayed: true`.
- **Повтор с другим телом** → `409 Conflict`
  (`idempotency key already used with a different request body`).
- Кэшируются только ответы 2xx; при превышении лимита тела или `Content-Type` не
  `application/json` middleware пропускает запрос без изменений.
- Отказ Redis деградирует в pass-through с warning — идемпотентность не должна ронять
  запись.

Аналитика имеет независимую, PostgreSQL-реализованную идемпотентность по `event_id`
(таблица `event_idempotency_keys`, тот же TTL 24 ч): ключ захватывается **до**
публикации в Kafka, при сбое публикации захваты компенсируются удалением — чтобы
повтор клиента снова был принят.

### Идемпотентные консьюмеры

Kafka даёт at-least-once, поэтому каждый консьюмер обязан быть безразличным к
дубликатам. Здесь это обеспечено на трёх уровнях:

1. **In-memory дедупликатор** — `outbox.Deduper`: map + FIFO-очередь на 1024 id
   (используется дедупликатором конвейера и потребителем snapshot).
2. **Постоянный дедуп в PostgreSQL** — таблица `kafka_dedup_keys`
   (`event_id` PRIMARY KEY): `SELECT EXISTS` + `INSERT … ON CONFLICT DO NOTHING`.
   Совпадение → `consumer.ErrDrop`: запись коммитится и **не** уходит в DLQ.
3. **Дедупликация в хранилище** — `ReplacingMergeTree` в ClickHouse: повторная
   вставка с тем же ключом сольётся при фоновом merge.

`pkg/consumer` задаёт единую политику обработки: до 3 попыток, backoff 1s → 60s,
`ErrDrop` (осознанно дубликат), `ErrNonRetryable` (битое сообщение → сразу в DLQ),
всё остальное — в DLQ с заголовками `dlq.origin_topic`, `dlq.origin_group`,
`dlq.attempts`, `dlq.error`. Offset коммитится в любом случае, что и даёт
at-least-once без потерь.

### Асинхронный пайплайн аналитики

Синхронной обработки аналитики нет вообще: клиент получает `202 Accepted` после
валидации, дальше конвейер работает сам.

```mermaid
graph LR
    CLI[Клиент] -->|events/batch| API["Events API<br/>validate · claim · publish"]
    API -->|analytics.events.raw| K[Kafka]
    K --> V[validator] -->|analytics.events.validated| D[dedup] -->|analytics.events.deduped| AT[attribution]
    API -->|exposures/batch| K2[analytics.exposures.raw]
    K2 --> REG[exposure registry]
    AT --> WR[batch writer]
    REG --> WR
    K --> CH[ClickHouse consumers] --> WR
    WR --> DB[("ClickHouse")]
    D -.->|неретирабельное| DLQ[analytics.dlq]
    V -.->|неретирабельное| DLQ
```

- **Claim-before-publish**: идемпотентный ключ каждого события вставляется до
  публикации; конфликт по `event_id` даёт `duplicate` (тот же хэш запроса) или
  `rejected` (другой хэш).
- **Компенсация**: если публикация в Kafka не удалась, ключи батча удаляются.
- **Коды ответа**: `202` — всё принято (включая дубликаты), `207` — частично,
  `400` — все элементы отклонены, `503` — брокер недоступен.
- **Ключ партиционирования**: `decision_id`, если он есть, иначе `event_id`; для
  показа — `experiment_id`. Это обеспечивает порядок обработки внутри одного
  решения, что критично для атрибуции.
- **Батч-писатель** (`pkg/clickhouse/writer.go`): четыре независимые очереди
  (`events`, `exposures`, `decisions`, `attributed`), flush по размеру (1000) или
  по тику (1 с). При ошибке вставки строки возвращаются в начало очереди (requeue),
  поэтому временная недоступность ClickHouse не теряет данные.
- Коммит оффсета — **после** успешного flush.

### Схемы ClickHouse

`deploy/clickhouse/tables.sql`, все таблицы — `ReplacingMergeTree` с месячным
партиционированием и TTL 90 дней:

| Таблица | Ключ ORDER BY | Партиция | Назначение |
| --- | --- | --- | --- |
| `events_raw` | `event_id` | `toYYYYMM(occurred_at)` | Сырые события |
| `decisions_raw` | `decision_id, flag_key` | `toYYYYMM(created_at)` | Факты решений |
| `exposures` | `event_id` | `toYYYYMM(occurred_at)` | Факты показа |
| `attributed_events` | `event_id` | `toYYYYMM(occurred_at)` | События с привязкой к эксперименту |

Почему так: `ReplacingMergeTree` снимает дубликаты на уровне движка (третий уровень
идемпотентности), партиционирование по месяцу даёт дешёвое удаление старых данных
по целым партициям и ограничивает объём сканирования, TTL освобождает место без
cron-задач. `LowCardinality(String)` для типа события, версии соли и источника
решения, `DateTime64(3,'UTC')` для времени. `subject_id` хранится уже
захэшированным.

### Атрибуция по decision_id

Связь «эксперимент ↔ событие» строится на `decision_id`, который рантайм проставил
в решении и опубликовал в `decisions.raw`.

1. `exposure registry` читает `analytics.exposures.raw` и кладёт
   `attribution:pending:exposure:{decision_id}` в Redis с TTL = окно атрибуции + грейс.
2. Потребитель `analytics.events.deduped` ищет `attribution:pending:exposure:{decision_id}`:
   - **найден** → строка в `attributed_events` с `experiment_id` и `variant_id`, pending
     удаляется;
   - **не найден** и тип события не требует показа (`error`, `latency` — флаг
     `require_exposure = false` в каталоге) → строка пишется с пустыми
     `experiment_id`/`variant_id` (техническое событие не выбрасывается);
   - **не найден** и требует показа → событие кладётся в
     `attribution:pending:conversion:{decision_id}` + ZSET-индекс для sweeper'а.
3. `sweeper` (раз в минуту) выбирает по `ZRANGEBYSCORE` все просроченные конверсии и
   пишет их как «истёкшие» факты с пустыми `experiment_id`/`variant_id` — потеря
   данных не молчаливая, а попадает в data-quality.

Окно атрибуции — 7 дней, грейс для опоздавших событий — 1 час
(`analytics.attribution.window_days` / `late_event_grace_hours`). Каталог типов
событий кэшируется в памяти и перечитывается раз в минуту; ошибка перечитывания
не роняет конвейер — остаётся предыдущая версия.

### Наблюдаемость

- **Логи**: Zap в JSON, единые поля из `pkg/logger` (`request_id`, `trace_id`,
  `route`, `duration_ms`, `user_id`, `error_type`); 5xx логируется как Error, 401/403
  как Warn. Подробности — `docs/logging.md`.
- **Метрики**: `GET /metrics` на всех трёх сервисах, `prometheus/client_golang`:
  `http_requests_total{job,method,path,code}`, `http_request_duration_seconds`,
  `http_response_size_bytes`. Лейбл `path` берётся из шаблона маршрута, а не из
  фактического URL — кардинальность ограничена. Fiber-сервисы используют
  `metrics.MiddlewareFiber("runtime" | "analytics")`, Gin — `metrics.Middleware("panel")`.
- **Prometheus** (`deploy/prometheus/prometheus.yml`): 5 job'ов, `scrape_interval: 15s`,
  TSDB-retention 15 дней.
- **Grafana**: три provisioned-дашборда — `runtime` (латентность p50/p95/p99, RPS,
  доля ошибок, возраст snapshot), `queues` (лаг и число участников групп Kafka,
  оффсеты и лаг по топикам), `analytics` (интенсивность вставок, латентность
  запросов, parts, accepted/rejected/duplicates).
- **Loki**: single-binary, `tsdb`-схема v13, `reject_old_samples_max_age: 168h`.

### Performance SLA

`tests/e2e/runtime/load_test.go` — нагрузочный тест, который падает, если SLA нарушен:

- 256 одновременных соединений, каждое с 100 мс think-time;
- 1 с прогрева (не в статистику) + 3 с измерения;
- считаются p50 / p95 / p99 по клиентским задержкам (nearest-rank);
- **assert: p99 ≤ 10 мс**, любой сетевой сбой или не-2xx — падение теста;
- отдельная проверка: тело ответа `/decide` ≤ 32 KiB;
- таймауты запросов включаются в выборку задержек, а не выбрасываются.

Запуск:

```bash
make dev-up && make test-e2e
```

Тест поднимает реальные бинарники панели и рантайма, готовит фикстуры (флаг, ожидание
попадания в snapshot) и только потом мерит. Тесты помечены `-short` и требуют
`E2E_TEST=1` и суффикс `_e2e` в имени БД — случайно упасть в рабочую базу нельзя.

### Graceful degradation

| Сбой | Поведение |
| --- | --- |
| Redis недоступен (рантайм работает) | Ответы продолжают обслуживаться из in-memory snapshot; попытка refresh — warn, решение не блокируется. Старый snapshot честно помечается `degraded=true` |
| Snapshot ещё не загружался ни разу | `503 SNAPSHOT_UNAVAILABLE` — единственный «жёсткий» отказ |
| Kafka недоступен (рантайм) | Рантайм стартует и обслуживает bootstrap из Redis; консьюмер snapshot лишь логирует ошибку чтения |
| Kafka недоступен (панель) | Relay outbox становится no-op, строки накапливаются в таблице и уходят при восстановлении |
| Kafka недоступен (аналитика) | `/events/batch` и `/exposures/batch` отдают `503 SERVICE_UNAVAILABLE` |
| ClickHouse недоступен | Конвеййер и атрибуция отключаются с предупреждением, HTTP-часть аналитики продолжает работать; накопленные строки requeue'ятся |
| Redis недоступен (панель) | Fail-closed: сессии считаются неактивными, запросы получают `401 token revoked` — панель не выдаёт доступ без проверки сессии |
| PostgreSQL недоступен (панель) | Запросы к доменам возвращают `500`; `GET /health` остаётся liveness-пробой и не пробирует зависимости |

Философия: **fail-open на пути выдачи флага** (деградация лучше ошибки для
продакшена), **fail-closed на доступе и таргетинге** (нет проверки — нет доступа,
нет атрибута — нет попадания в эксперимент).

---

## Тестирование и quality gates

```bash
make check                                  # сборка всех сервисов + go vet + gofmt
go test ./pkg/... ./services/... -short -count=1
```

| Уровень | Что покрывает | Где |
| --- | --- | --- |
| Unit | snapshot reader, consumer framework, ClickHouse writer, outbox, targeting (parser + evaluator), метрики-формулы, idempotency middleware, decision publisher, события/атрибуция/ingest, домены panel (experiments, metrics, reviews, reports) | `pkg/*`, `services/*/domain/*` |
| E2E | auth, users, flags, experiments (жизненный цикл), reviews, метрики, health, типы ответов; рантайм: решение, деградация, отсутствие snapshot, нагрузка | `tests/e2e/{panel,runtime}` |

E2E требует поднятой инфраструктуры и запускается с `-p 1` (съюты делят базу
`labp_e2e` и фиксированные порты):

```bash
make dev-up
make test-e2e        # E2E_TEST=1 go test -p 1 ./tests/e2e/... -count=1
```

---

## Репозиторий

```
services/
├── panel/       control plane (Gin, :8081): auth, users, flags, metrics, experiments, reviews, reports
├── runtime/     decide API (Fiber v3, :8082): allocation, snapshot, decision publishing
└── analytics/   Events API, attribution pipeline, ClickHouse ingest (Fiber v3, :8083)
pkg/             api, audit, auth, clickhouse, config, consumer, database, dto, logger,
                 metrics, middleware, outbox, snapshot, targeting
migrations/      goose SQL: 00001_users … 00011_metrics_engine
deploy/          clickhouse/ (DDL), prometheus/, loki/, grafana/, nginx/
docs/openapi/    panel.yaml, runtime.yaml, analytics.yaml
docs/            logging.md, ai/ (handoff-Notes для агентов), task/ (локально, gitignored)
tests/e2e/       panel, runtime
tools/           s3-provision
```

`docs/ai/`, `docs/task/`, `.opencode/`, `config.local.json` и `.env*` — локальные
служебные файлы, в репозиторий не попадают.

---

## Конфигурация

Единый конфиг для всех трёх сервисов. Порядок загрузки: `$CONFIG_NAME` (абсолютный путь
или базовое имя) → `config.local.json` → `config.json`, искать в `.` и `/labp`.

Шаблон `config.example.json` поддерживает подстановку `{{ ENV "VAR" }}`, поэтому
один и тот же файл используется и compose, и локальными бинарниками. В git
отслеживается только шаблон; `config.local.json` и `.env*` — нет.

- `auth.jwt.secret_key` — обязателен в `environment: prod`; `ValidateSecurity`
  завершает процесс, если секрет остался вида `change-me*`.
- `runtime.http.max_stale_age` — порог «деградации» snapshot (по умолчанию 5 мин).
- `analytics.kafka.*` — топики и группы потребителей; при незаполненных топиках
  конвейер отключается с предупреждением, а не падает.
- `analytics.pii` — хэширование `subject_id` и версии солей.
- `analytics.clickhouse` — размер батча и интервал flush.
- Версия сервиса инжектится через `-ldflags` в `pkg/config.ServiceVersion`.

Секреты в репозиторий не коммитятся: реальные значения живут в `.env` /
`config.local.json` и не должны попадать в диффы или отчёты.

---

## API-контракты

`docs/openapi/` — источник истины по HTTP-контрактам (OpenAPI 3.0.3):

- `panel.yaml` — auth, users, flags, metrics, experiments (жизненный цикл, версии,
  варианты, внутренние guardrail-переходы), reviews, approver-группы, reports.
- `runtime.yaml` — `GET /health`, `POST /decide`.
- `analytics.yaml` — `POST /events/batch`, `POST /exposures/batch`, `GET /event-types`.

Изменение эндпоинтов означает изменение спецификации в том же коммите. Публичный
маршрут рантайма — именно `/api/v1/runtime/decide`, как в спецификации.

---

## CI/CD

`.github/workflows/ci-cd.yml` — три job:

| Job | Что делает |
| --- | --- |
| `quality` | `make check` (сборка + `go vet` + `gofmt`) и `go test ./pkg/... ./services/... -short -count=1` |
| `compose` | `docker compose config --quiet` и сборка образов `panel`, `runtime`, `analytics` |
| `e2e` | `make dev-up` → `make test-e2e` (включая нагрузочный тест с SLA-проверкой), с дампом состояния контейнеров при завершении |

Триггеры: push в `master`, pull request, ручной запуск. Публикация образов и
автоматический деплой на dev-хост пока не настроены — см. статус ниже.

---

## Статус по требованиям

Легенда: **реализовано** — работает в коде; **частично** — есть механика, но не
закрыт критерий; **планируется** — кода нет.

| Требование | Статус | Комментарий |
| --- | --- | --- |
| Transactional Outbox, at-least-once | реализовано | `pkg/outbox`, миграция 00008, SKIP LOCKED, backoff, dead-letter. Покрыт только `experiment.created`; остальные домены публикуют напрямую |
| Идемпотентность `Idempotency-Key` (24 ч, 409) | реализовано | `pkg/middleware/idempotency.go`, покрытие всех мутаций panel |
| Идемпотентные консьюмеры | реализовано | in-memory + PostgreSQL + `ReplacingMergeTree` |
| Сбор событий через Events API → Kafka → ClickHouse | реализовано | 3 стадии конвейера, DLQ, claim-before-publish |
| Схемы ClickHouse с месячным партиционированием | реализовано | 4 таблицы, `ReplacingMergeTree`, TTL 90 дней |
| Атрибуция вне порядка | частично | Есть pending-store, окно, sweeper, expired-факты. Обработка «конверсия пришла раньше показа» не завершается атрибуцией — сценарий закрывается как истёкший факт |
| Prometheus + Loki + Grafana | частично | Конфиги и три дашборда есть, HTTP-метрики всех сервисов доступны. Панели аналитики ждут инструментации счётчиков событий и вставок; Loki не имеет shipper'а логов |
| p99 < 10 мс при 256 соединениях | реализовано | `tests/e2e/runtime/load_test.go`, assert в тесте и в CI |
| Деградация: Redis↓ в рантайме | реализовано | Обслуживание из snapshot + `degraded` |
| Деградация: PostgreSQL↓ в панели | частично | Возвращается `500`, а не `503`; маппинг ошибок БД и health-проба зависимостей не сделаны |
| Автоматические guardrails (пауза/откат по порогам) | частично | Есть внутренние эндпоинты `internal/experiments/{id}/pause` и `/rollback`, поле `guardrail_paused`, аудит и запрет resume без админа. Evaluator с порогами, окнами и триггером не реализован |
| Уведомления Telegram / Slack / Discord | планируется | Кода нет: ни каналов, ни диспетчера, ни dead-letter очереди |
| ADR-документы | планируется | Решения зафиксированы в этом README и в `docs/ai/`, отдельного `docs/adr/` нет |
| OpenAPI 3.1 | частично | Спецификации есть, но в версии 3.0.3 |
| CI/CD | реализовано | Линтеры, форматирование, `go vet`, unit и e2e в GitHub Actions |
| Seed / демо-данные | частично | Есть bootstrap-админ и пошаговый curl-сценарий выше; отдельного seed-скрипта нет |
| E2E аналитического конвейера | планируется | `tests/e2e/` покрывает panel и runtime; Events API → Kafka → ClickHouse проверяется только unit-тестами |
| Автоматический деплой | планируется | Сборка и тесты автоматизированы, публикация образа и выкладка — вручную (`make up`) |

---

## Известные ограничения

- `event_idempotency_keys` и `kafka_dedup_keys` чистятся только при повторном
  обращении к тому же ключу; фонового GC нет, таблицы растут.
- `event_types.schema` не валидируется JSON Schema — только структурные проверки.
- Poison-сообщения, попавшие в консьюмер ClickHouse, коммитятся без DLQ: фреймворк
  `pkg/consumer` используется только конвейерными потребителями.
- `attributed_events` пишет sweeper для истёкших конверсий, но обратного lookup
  «поздно пришёл показа» нет.
- Инструментация `analytics_events_*` и `clickhouse_*` объявлена, но пока не вызывается
  из кода — соответствующие панели Grafana останутся пустыми до появления call sites.
- `snapshot_age_seconds` объявлен без лейбла `job`, поэтому панель «Runtime Snapshot Age»
  его не подхватывает.
- В Loki нет shipper'а (promtail/Alloy), а `retention_period` не задан, поэтому логи
  в стек не попадают, а retention фактически не применяется.
- `docs/openapi/runtime.yaml` и `docs/ai/AI_HANDOFF.md` описывают targeting как
  нереализованный; в текущем коде evaluator подключён к `/decide`.
- Аудит `audit_records` покрывает часть мутаций, а не все.

Полный аудит требований с указанием конкретных мест — в `AGENTS.md` и `docs/task/`.

---

## Лицензия

Код предоставляется «как есть» для учебных и портфолио-задач.
