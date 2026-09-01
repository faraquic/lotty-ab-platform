CONTAINER_ENGINE ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)

GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
GIT_TAG    := $(shell git describe --tags --abbrev=0 2>/dev/null)
ifeq ($(GIT_TAG),)
SERVICE_VERSION := v0.0.0_dev
else ifeq ($(GIT_COMMIT),)
SERVICE_VERSION := $(GIT_TAG)
else
SERVICE_VERSION := $(GIT_TAG)+$(GIT_COMMIT)
endif

LDFLAGS := -X github.com/faraquic/lotty-ab-platform/pkg/config.ServiceVersion=$(SERVICE_VERSION)

PANEL_BIN     ?= bin/panel
RUNTIME_BIN   ?= bin/runtime
ANALYTICS_BIN ?= bin/analytics

PANEL_PORT     ?= 8081
RUNTIME_PORT   ?= 8082
ANALYTICS_PORT ?= 8083
NGINX_PORT     ?= 8080

POSTGRES_IMAGE    ?= docker.io/library/postgres:18-alpine
POSTGRES_NAME     ?= labp-postgres
POSTGRES_PORT     ?= 5433
POSTGRES_DB       ?= labp
POSTGRES_USER     ?= lotty
POSTGRES_PASSWORD ?= lottypassword

REDIS_IMAGE ?= docker.io/library/redis:8-alpine
REDIS_NAME  ?= labp-redis
REDIS_PORT  ?= 6379

S3_IMAGE        ?= docker.io/minio/minio:latest
S3_NAME         ?= labp-s3
S3_PORT         ?= 9000
S3_CONSOLE_PORT ?= 9001
S3_BUCKET       ?= labp
S3_ACCESS_KEY   ?= minioadmin
S3_SECRET_KEY   ?= minioadmin

CPU_LIMIT    ?= 1.0
MEMORY_LIMIT ?= 512m
PIDS_LIMIT   ?= 128

MIGRATIONS_DIR ?= migrations
POSTGRES_DSN   ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

# podman needs :Z on volumes when SELinux is enforcing; docker ignores it
ifeq ($(shell basename $(CONTAINER_ENGINE)),podman)
VOL_OPTS := :Z
else
VOL_OPTS :=
endif

# ─── Help ────────────────────────────────────────────────────────────────────

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Development (local binaries):"
	@echo "  dev-up          start infra containers (PostgreSQL + Redis + S3)"
	@echo "  dev-down        stop infra containers"
	@echo "  dev-clean       stop infra and remove volumes"
	@echo "  dev-status      check which infra containers are running"
	@echo ""
	@echo "  run-local       build + restart all services (fast, no infra restart)"
	@echo "  stop-local      stop all service binaries"
	@echo "  restart-local   stop + run-local"
	@echo "  logs-local      tail panel + runtime + analytics stdout"
	@echo ""
	@echo "  run-panel       build + run only panel"
	@echo "  run-runtime     build + run only runtime"
	@echo "  run-analytics   build + run only analytics"
	@echo ""
	@echo "Build:"
	@echo "  build           compile panel    -> $(PANEL_BIN)"
	@echo "  build-runtime   compile runtime  -> $(RUNTIME_BIN)"
	@echo "  build-analytics compile analytics -> $(ANALYTICS_BIN)"
	@echo "  build-all       compile all three"
	@echo "  vet             go vet ./..."
	@echo "  check           build + vet + gofmt"
	@echo ""
	@echo "Docker Compose (full stack):"
	@echo "  up              build + start all services in docker compose"
	@echo "  down            stop docker compose"
	@echo "  logs            follow docker compose logs"
	@echo "  rebuild         down + up (full rebuild)"
	@echo ""
	@echo "Database:"
	@echo "  pg-migrate-up       apply pending migrations"
	@echo "  pg-migrate-down     rollback last migration"
	@echo "  pg-migrate-status   show migration status"
	@echo "  pg-migrate-new      create new migration (name=create_foo)"
	@echo "  psql                open psql shell"
	@echo ""
	@echo "Tools:"
	@echo "  redis-cli       open redis-cli"
	@echo "  s3-provision    create S3 bucket"
	@echo "  test-e2e        run end-to-end tests"

# ─── Infrastructure ──────────────────────────────────────────────────────────

.PHONY: dev-up
dev-up: pg-up redis-up s3-up
	@echo "Waiting for S3..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do \
		$(CONTAINER_ENGINE) inspect --format='{{.State.Health.Status}}' $(S3_NAME) 2>/dev/null | grep -q healthy && break; \
		echo "  waiting... ($$i)"; \
		sleep 2; \
	done
	$(MAKE) s3-provision

.PHONY: dev-down
dev-down: pg-down redis-down s3-down

.PHONY: dev-clean
dev-clean: pg-clean redis-clean s3-clean

.PHONY: dev-status
dev-status:
	@echo "=== Infrastructure containers ==="
	@$(CONTAINER_ENGINE) ps --filter name=labp-postgres --filter name=labp-redis --filter name=labp-s3 --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || true
	@echo ""
	@echo "=== Service processes ==="
	@pgrep -af 'bin/(panel|runtime|analytics)' 2>/dev/null || echo "  (none running)"

# ─── Local development (fast restart) ────────────────────────────────────────

.PHONY: run-local
run-local: build-all _ensure-infra
	@echo "Stopping old services..."
	@-pkill -f 'bin/panel' 2>/dev/null; sleep 0.2
	@-pkill -f 'bin/runtime' 2>/dev/null; sleep 0.2
	@-pkill -f 'bin/analytics' 2>/dev/null; sleep 0.2
	@echo "Starting panel  on :$(PANEL_PORT)..."
	@./$(PANEL_BIN) &
	@echo "Starting runtime on :$(RUNTIME_PORT)..."
	@./$(RUNTIME_BIN) &
	@echo "Starting analytics on :$(ANALYTICS_PORT)..."
	@./$(ANALYTICS_BIN) &
	@sleep 1
	@echo ""
	@echo "All services running. Endpoints:"
	@echo "  panel     http://localhost:$(PANEL_PORT)/api/v1/panel/"
	@echo "  runtime   http://localhost:$(RUNTIME_PORT)/api/v1/runtime/"
	@echo "  analytics http://localhost:$(ANALYTICS_PORT)/api/v1/analytics/"
	@echo ""
	@echo "  make stop-local    stop all"
	@echo "  make logs-local    tail logs"

.PHONY: stop-local
stop-local:
	@-pkill -f 'bin/panel' 2>/dev/null || true
	@-pkill -f 'bin/runtime' 2>/dev/null || true
	@-pkill -f 'bin/analytics' 2>/dev/null || true
	@echo "All services stopped."

.PHONY: restart-local
restart-local: stop-local run-local

.PHONY: logs-local
logs-local:
	@echo "Tailing service logs (Ctrl+C to stop)..."
	@tail -F /dev/null 2>/dev/null || true
	@echo "Note: services log to stderr. Run them in separate terminals for best output."

.PHONY: run-panel
run-panel: build _ensure-infra
	@-pkill -f 'bin/panel' 2>/dev/null; sleep 0.2
	@echo "Starting panel on :$(PANEL_PORT)..."
	@./$(PANEL_BIN)

.PHONY: run-runtime
run-runtime: build-runtime
	@-pkill -f 'bin/runtime' 2>/dev/null; sleep 0.2
	@echo "Starting runtime on :$(RUNTIME_PORT)..."
	@./$(RUNTIME_BIN)

.PHONY: run-analytics
run-analytics: build-analytics
	@-pkill -f 'bin/analytics' 2>/dev/null; sleep 0.2
	@echo "Starting analytics on :$(ANALYTICS_PORT)..."
	@./$(ANALYTICS_BIN)

# Internal: ensure infra is running, start if not
.PHONY: _ensure-infra
_ensure-infra:
	@$(CONTAINER_ENGINE) inspect $(POSTGRES_NAME) >/dev/null 2>&1 || { echo "PostgreSQL not found. Run 'make dev-up' first."; exit 1; }
	@$(CONTAINER_ENGINE) inspect $(REDIS_NAME) >/dev/null 2>&1 || { echo "Redis not found. Run 'make dev-up' first."; exit 1; }
	@$(CONTAINER_ENGINE) inspect $(S3_NAME) >/dev/null 2>&1 || { echo "S3 not found. Run 'make dev-up' first."; exit 1; }
	@STATUS=$$($(CONTAINER_ENGINE) inspect --format='{{.State.Status}}' $(POSTGRES_NAME) 2>/dev/null); \
		if [ "$$STATUS" != "running" ]; then echo "PostgreSQL is not running. Start it: podman start $(POSTGRES_NAME)"; exit 1; fi
	@STATUS=$$($(CONTAINER_ENGINE) inspect --format='{{.State.Status}}' $(REDIS_NAME) 2>/dev/null); \
		if [ "$$STATUS" != "running" ]; then echo "Redis is not running. Start it: podman start $(REDIS_NAME)"; exit 1; fi
	@STATUS=$$($(CONTAINER_ENGINE) inspect --format='{{.State.Status}}' $(S3_NAME) 2>/dev/null); \
		if [ "$$STATUS" != "running" ]; then echo "S3 is not running. Start it: podman start $(S3_NAME)"; exit 1; fi

# ─── Build ───────────────────────────────────────────────────────────────────

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(PANEL_BIN) ./services/panel

.PHONY: build-runtime
build-runtime:
	go build -ldflags "$(LDFLAGS)" -o $(RUNTIME_BIN) ./services/runtime

.PHONY: build-analytics
build-analytics:
	go build -ldflags "$(LDFLAGS)" -o $(ANALYTICS_BIN) ./services/analytics

.PHONY: build-all
build-all: build build-runtime build-analytics

.PHONY: vet
vet:
	go vet ./...

.PHONY: check
check: build-all vet
	@gofmt -l pkg/ services/ | grep -v '^$$' && echo "^^^ files need formatting" || echo "All clean."

# ─── Docker Compose ──────────────────────────────────────────────────────────

.PHONY: up
up: check-engine
	$(CONTAINER_ENGINE) compose up -d --build

.PHONY: down
down: check-engine
	$(CONTAINER_ENGINE) compose down

.PHONY: logs
logs: check-engine
	$(CONTAINER_ENGINE) compose logs -f

.PHONY: rebuild
rebuild: down up

# ─── Check engine ────────────────────────────────────────────────────────────

.PHONY: check-engine
check-engine:
ifndef CONTAINER_ENGINE
	$(error neither podman nor docker is installed)
endif

# ─── Infrastructure containers ───────────────────────────────────────────────

.PHONY: pg-up
pg-up: check-engine
	$(CONTAINER_ENGINE) run -d \
		--replace \
		--name $(POSTGRES_NAME) \
		-p $(POSTGRES_PORT):5432 \
		-e POSTGRES_DB=$(POSTGRES_DB) \
		-e POSTGRES_USER=$(POSTGRES_USER) \
		-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
		-e PGDATA=/var/lib/postgresql/data/pgdata \
		-v lotty-pgdata:/var/lib/postgresql/data$(VOL_OPTS) \
		--memory=$(MEMORY_LIMIT) \
		--cpus=$(CPU_LIMIT) \
		--pids-limit=$(PIDS_LIMIT) \
		--shm-size=64m \
		--health-cmd="pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB)" \
		--health-interval=5s \
		--health-timeout=3s \
		--health-retries=10 \
		$(POSTGRES_IMAGE) \
	postgres -c shared_buffers=64MB -c max_connections=50 -c fsync=off -c synchronous_commit=off -c full_page_writes=off

.PHONY: pg-down
pg-down: check-engine
	-$(CONTAINER_ENGINE) rm -f $(POSTGRES_NAME)

.PHONY: pg-logs
pg-logs: check-engine
	$(CONTAINER_ENGINE) logs -f $(POSTGRES_NAME)

.PHONY: psql
psql: check-engine
	$(CONTAINER_ENGINE) exec -it $(POSTGRES_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

.PHONY: pg-clean
pg-clean: pg-down
	-$(CONTAINER_ENGINE) volume rm lotty-pgdata

.PHONY: redis-up
redis-up: check-engine
	$(CONTAINER_ENGINE) run -d \
		--replace \
		--name $(REDIS_NAME) \
		-p $(REDIS_PORT):6379 \
		-v lotty-redisdata:/data$(VOL_OPTS) \
		--memory=$(MEMORY_LIMIT) \
		--cpus=$(CPU_LIMIT) \
		--pids-limit=$(PIDS_LIMIT) \
		--health-cmd="redis-cli ping" \
		--health-interval=5s \
		--health-timeout=3s \
		--health-retries=10 \
		$(REDIS_IMAGE) \
	redis-server --save 60 1 --appendonly no --maxmemory 256mb --maxmemory-policy allkeys-lru

.PHONY: redis-down
redis-down: check-engine
	-$(CONTAINER_ENGINE) rm -f $(REDIS_NAME)

.PHONY: redis-logs
redis-logs: check-engine
	$(CONTAINER_ENGINE) logs -f $(REDIS_NAME)

.PHONY: redis-cli
redis-cli: check-engine
	$(CONTAINER_ENGINE) exec -it $(REDIS_NAME) redis-cli

.PHONY: redis-clean
redis-clean: redis-down
	-$(CONTAINER_ENGINE) volume rm lotty-redisdata

.PHONY: s3-up
s3-up: check-engine
	$(CONTAINER_ENGINE) run -d \
		--replace \
		--name $(S3_NAME) \
		-p $(S3_PORT):9000 \
		-p $(S3_CONSOLE_PORT):9001 \
		-e MINIO_ROOT_USER=$(S3_ACCESS_KEY) \
		-e MINIO_ROOT_PASSWORD=$(S3_SECRET_KEY) \
		-v lotty-s3data:/data$(VOL_OPTS) \
		--memory=$(MEMORY_LIMIT) \
		--cpus=$(CPU_LIMIT) \
		--pids-limit=$(PIDS_LIMIT) \
		--health-cmd="mc ready local" \
		--health-interval=5s \
		--health-timeout=3s \
		--health-retries=10 \
		$(S3_IMAGE) \
	server /data --console-address ":9001"

.PHONY: s3-down
s3-down: check-engine
	-$(CONTAINER_ENGINE) rm -f $(S3_NAME)

.PHONY: s3-logs
s3-logs: check-engine
	$(CONTAINER_ENGINE) logs -f $(S3_NAME)

.PHONY: s3-clean
s3-clean: s3-down
	-$(CONTAINER_ENGINE) volume rm lotty-s3data

.PHONY: s3-provision
s3-provision:
	go run ./tools/s3-provision -endpoint http://localhost:$(S3_PORT) -bucket $(S3_BUCKET) -region us-east-1 -access-key $(S3_ACCESS_KEY) -secret-key $(S3_SECRET_KEY)

.PHONY: s3-provision-e2e
s3-provision-e2e:
	go run ./tools/s3-provision -endpoint http://localhost:$(S3_PORT) -bucket $(S3_BUCKET)-e2e -region us-east-1 -access-key $(S3_ACCESS_KEY) -secret-key $(S3_SECRET_KEY)

# ─── Migrations ──────────────────────────────────────────────────────────────

.PHONY: pg-migrate-up
pg-migrate-up:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" up

.PHONY: pg-migrate-down
pg-migrate-down:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" down

.PHONY: pg-migrate-status
pg-migrate-status:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" status

.PHONY: pg-migrate-new
ifndef name
	$(error usage: make pg-migrate-new name=migration_name)
endif
pg-migrate-new:
	go tool goose -dir $(MIGRATIONS_DIR) create $(name) sql

# ─── Tests ───────────────────────────────────────────────────────────────────

.PHONY: test-e2e
test-e2e: s3-provision-e2e
	$(CONTAINER_ENGINE) exec $(POSTGRES_NAME) psql -U $(POSTGRES_USER) -d postgres -c "SELECT 1 FROM pg_database WHERE datname = '$(POSTGRES_DB)_e2e'" | grep -q 1 || \
		$(CONTAINER_ENGINE) exec $(POSTGRES_NAME) psql -U $(POSTGRES_USER) -d postgres -c "CREATE DATABASE $(POSTGRES_DB)_e2e"
	go tool goose -dir $(MIGRATIONS_DIR) postgres "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)_e2e?sslmode=disable" up
	E2E_TEST=1 go test ./tests/e2e/... -v -count=1
