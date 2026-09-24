# AGENTS.md — Lotty AB Platform

> Практичный справочник для агентов и разработчиков. Все утверждения подтверждены файлами репозитория.
> Последнее обновление: 2026-09-24

## 1. Overview

**Lotty AB Platform** — self-hosted A/B testing platform: feature flags, controlled experiments,
review workflow, runtime variant delivery, event attribution и analytics. Solo pet project,
запускается локально через Docker Compose или бинарники.

Три Go-сервиса + React SPA. PostgreSQL — source of truth для control plane; runtime обслуживает
decide API из in-memory snapshot (Redis).

- **Backend:** Go 1.27, Gin (panel), Fiber v3 (runtime/analytics), pgx/v5, rueidis, aws-sdk-go-v2.
- **Frontend:** React 19 + TypeScript strict + Vite 8, Mantine v8, React Query v5, i18next (en/ru), pnpm.
- **Инфраструктура:** PostgreSQL 18, Redis 8, MinIO (S3), nginx.

## 2. Структура репозитория

```
services/
├── panel/       control plane (Gin, :8081) + домены: auth, users, flags, metrics, experiments, reviews
├── runtime/     decide API (Fiber v3, :8082): xxhash64 buckets, allocation, snapshot
└── analytics/   скелет (Fiber v3, :8083): только health

pkg/             api, auth, config, database, dto, logger, middleware, snapshot
migrations/      goose SQL: 00001_users … 00006_review_widths
tests/e2e/       панель и runtime end-to-end (запускаются с E2E_TEST=1, -p 1)
tools/           s3-provision
deploy/nginx/    nginx.conf — reverse proxy для compose
docs/openapi/    panel.yaml, runtime.yaml, analytics.yaml (source of truth для API)
docs/            logging.md, deploy-dev.md, ai/AI_HANDOFF.md (docs/task — локальные, gitignored)
frontend/        React SPA (см. frontend/AGENTS.md)
config.*.json    конфиги (см. §8)
```

## 3. Архитектура

- **panel :8081** — control plane: JWT (HS256) + Redis-сессии, RBAC, CRUD доменов; PostgreSQL —
  источник истины. Публичные маршруты под `/api/v1/panel`.
- **runtime :8082** — evaluate из in-memory snapshot; публичный `POST /api/v1/runtime/decide`.
- **analytics :8083** — скелет, эндпоинты здоровья.
- **frontend** — SPA, собственный nginx отдаёт `index.html` (SPA fallback) на `frontend:80`
  (`expose: 80`, без отдельного хост-порта).
- **nginx :8080** проксирует `/api/v1/{panel,runtime,analytics}` и `/` → SPA.

Kafka, outbox и ClickHouse-рантайм не реализованы: нет сервисов в `docker-compose.yml` и нет
прямого использования в коде. В `go.mod` `ch-go`/`clickhouse-go` присутствуют только как indirect
(транзитивно через goose) — это не свидетельство интеграции.

## 4. Команды разработки

Все команды запускаются из корня репозитория (подтверждено `Makefile`).

| Задача | Команда |
| --- | --- |
| Инфраструктура | `make dev-up` / `make dev-down` / `make dev-status` / `make dev-clean` |
| Локальный запуск | `make run-local` (билдит Go-сервисы и запускает Vite dev-server) / `make stop-local` / `make restart-local` |
| Точечный запуск | `make run-panel` / `make run-runtime` / `make run-analytics` / `make run-frontend` |
| Сборка Go + SPA | `make build-all` (или `build-panel`, `build-runtime`, `build-analytics`, `build-frontend`; цели `build` нет) |
| Полный стек в Compose | `make up` / `make down` / `make logs` / `make rebuild` |
| Миграции | `make pg-migrate-up` / `pg-migrate-down` / `pg-migrate-status` / `pg-migrate-new name=…` |
| БД / кэш | `make psql` / `make redis-cli` |
| S3 | `make s3-provision` / `make s3-provision-e2e` |

Порты: panel `:8081`, runtime `:8082`, analytics `:8083`, nginx `:8080`, Postgres host `:5433`,
Redis `:6379`, MinIO `:9000`/`:9001`.

Frontend (из `frontend/`): `pnpm dev`, `pnpm build`, `pnpm preview`, `pnpm typecheck`, `pnpm lint`,
`pnpm test`, `pnpm format`.

Требуют Docker/podman и запущенных контейнеров (`make dev-up`): все `make run-*` (через
`_ensure-infra`), `make psql`, `make redis-cli`, `make test-e2e`, `make up/down/logs/rebuild`.
Деструктивны: `make dev-clean` (удаляет volumes с данными), `make pg-migrate-down` (откатывает
последнюю миграцию), `make rebuild`/`make down` (пересобирают/останавливают compose-стек) — не
запускать без явной необходимости и подтверждения.

## 5. Validation (обязательно перед завершением задачи)

```bash
make check                                # build-all + vet + frontend-check + gofmt -l
go test ./pkg/... -short -count=1
go test ./services/... -short -count=1
```

`make check` включает: сборку всех сервисов и SPA, `go vet ./...`, `pnpm typecheck && pnpm lint && pnpm build`.
Важно: `make check` не запускает ни Go-, ни frontend-тесты; `gofmt -l` только печатает список
неформатированных файлов и не роняет цель. Полный локальный прогон: `make check`, затем
`go test ./pkg/... ./services/... -short -count=1` и `pnpm test` из `frontend/`.
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
- **Frontend:** TypeScript strict, без `any`/`@ts-ignore`/`eslint-disable`; Mantine — единственная UI-библиотека;
  JWT только in-memory, никогда не в `localStorage`. Детали — `frontend/AGENTS.md`.
- **Комментарии:** не добавлять без явной просьбы. Без эмодзи.

## 7. Testing

- **Unit (Go):** `pkg/snapshot`, `services/panel/domain/{experiments,metrics,reviews}`,
  `services/runtime/domain/decide`. Запуск: `go test ./pkg/... ./services/... -short -count=1`.
- **E2E:** `tests/e2e/{panel,runtime}`; используют общую БД `labp_e2e`, фиксированные порты
  (panel `18080`; runtime-suite — `18081`/`18083`), bucket `labp-e2e`; обязателен `-p 1`.
  Требуют `make dev-up` и `make test-e2e`.
- **Frontend (Vitest):** `pnpm test` — guard'ы API-ответов и доменная логика (без snapshot-тестов «ради покрытия»).
- **CI:** `.github/workflows/ci-cd.yml` (в `HEAD`) гоняет `go test ./pkg/...` (без `./services/...`),
  frontend tests, `make check`, `make dev-up` + `make test-e2e`, `docker compose config`,
  `docker compose build`; деплой на dev только при push в `master`. Файл в рабочем дереве сейчас
  удалён — статус неясен, перед правками CI свериться с `HEAD`.

## 8. Configuration

- Загрузка: `CONFIG_NAME` (абсолютный или базовое имя) → `config.local.json` → `config.json`,
  поиск в `.` и `/labp` (`pkg/config/config.go`).
- Трекаются: `config.dev.json`, `config.docker.json`. `config.local.json`, `.env*` — gitignored.
- Поддержка `{{ ENV "VAR" }}` шаблонов для подстановки переменных окружения.
- `ServiceVersion` инжектится через `-ldflags` (`pkg/config.ServiceVersion`).
- Дефолт `environment` — `prod` (`defaultConfig`); без конфига `MustLoad` завершает процесс.
  Локально в корне уже может лежать gitignored `config.local.json` — он приоритетнее `config.json`.
- **Секреты:** в трекнутых конфигах только dev-плейсхолдеры (`secret_key: change-me`,
  `minioadmin`, bootstrap `password_hash`). Для prod их необходимо заменить: `ValidateSecurity`
  завершает процесс при `change-me*` в `environment: prod`. Реальные секреты не коммитить и не
  цитировать их значения (в т.ч. из `config.local.json` / `.env*`) в отчётах и диффах.

## 9. Protected / generated files

Не редактировать вручную:

- `bin/`, `frontend/dist/`, `frontend/node_modules/` — сборка (gitignored).
- `go.sum`, `frontend/pnpm-lock.yaml` — генерируются менеджерами пакетов.
- Применённые миграции `migrations/*.sql` — только добавление новых через `make pg-migrate-new`.
- `docs/openapi/*.yaml` — контракт; менять синхронно с backend.

Служебные/локальные (gitignored): `.opencode/`, `docs/task/`, `.vscode/`, `.idea/`, `*.log`.

## 10. Change boundaries

- Не менять зависимости, CI, миграции и документацию без явного запроса.
- Не редактировать чужие домены «попутно»: изменения по задаче — минимальны и локальны.
- `.dockerignore` — whitelist (`go.mod`, `go.sum`, `pkg/`, `services/`), плюс исключение
  `**/*_test.go`; в образ не попадают `migrations/`, `tools/`, конфиги. При добавлении новых
  корневых путей, нужных образу, обновлять `.dockerignore` и `Dockerfile`.
- Публичный API/services/фронтенд-контракты выводить из `docs/openapi/`, не придумывать endpoints.
- Frontend: не добавлять UI-библиотеки, кроме Mantine; не менять дизайн-язык.
- Рабочее дерево может содержать пользовательские изменения (правки, untracked-файлы, удалённый
  CI). Не восстанавливать, не удалять, не форматировать и не переписывать их; работать только в
  границах задачи и сверяться с `git status`.

## 11. Definition of Done

- Код собирается: `make check` зелёный.
- Go unit-тесты (`go test ./pkg/... ./services/... -short -count=1`) зелёные; при изменении
  доменов/API — обновлены или добавлены тесты.
- Frontend: `pnpm typecheck`, `pnpm lint`, `pnpm build`, `pnpm test` зелёные.
- Миграции (если затронуты) применяются вверх/вниз без ошибок; `make pg-migrate-down` откатывает
  последнюю миграцию — выполнять только по явному запросу.
- Нет секретов в диффе, нет сломанных ссылок в документации, нет TODO-заглушек в поставленной задаче.

## 12. Workflow агента

1. Изучить `AGENTS.md` (этот файл) и `frontend/AGENTS.md` для UI-задач; OpenAPI — источник контракта.
2. Определить границы изменения; не трогать лишнее. Свериться с `git status`, не задевать чужие
   изменения.
3. Реализовать минимально достаточное решение в существующем стиле.
4. Прогнать релевантные проверки из §5 и §7; исправить ошибки.
5. Показать краткий отчёт: изменённые файлы, добавленные/убранные зависимости, что сделано,
   результаты проверок (что запускалось, а что нет), известные ограничения.
6. Коммит — только по явному запросу.

## 13. Ссылки

| Документ | Путь |
| --- | --- |
| Frontend brief | `frontend/AGENTS.md` |
| OpenAPI | `docs/openapi/panel.yaml`, `runtime.yaml`, `analytics.yaml` |
| Логирование | `docs/logging.md` |
| Dev deployment | `docs/deploy-dev.md` |
| AI handoff | `docs/ai/AI_HANDOFF.md` |
| ТЗ (локально) | `docs/task/task.md`, `docs/task/todo.md` |
