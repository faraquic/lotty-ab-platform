CONTAINER_ENGINE ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)

# Build version: prefer the latest git tag + short commit, e.g. v1.12.4+abc1234.
# Falls back to a dev marker when not in a git checkout.
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

PANEL_BIN ?= bin/panel

POSTGRES_IMAGE   ?= docker.io/library/postgres:18-alpine
POSTGRES_NAME    ?= labp-postgres
POSTGRES_PORT    ?= 5433
POSTGRES_DB      ?= labp
POSTGRES_USER    ?= lotty
POSTGRES_PASSWORD?= lottypassword

CPU_LIMIT        ?= 1.0
MEMORY_LIMIT     ?= 512m
PIDS_LIMIT       ?= 100

MIGRATIONS_DIR   ?= migrations
POSTGRES_DSN     ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

REDIS_IMAGE      ?= docker.io/library/redis:8-alpine
REDIS_NAME       ?= labp-redis
REDIS_PORT       ?= 6379

S3_IMAGE         ?= docker.io/minio/minio:latest
S3_NAME          ?= labp-s3
S3_PORT          ?= 9000
S3_CONSOLE_PORT  ?= 9001
S3_BUCKET        ?= labp
S3_ACCESS_KEY    ?= minioadmin
S3_SECRET_KEY    ?= minioadmin

.PHONY: help
help:
	@echo "dev-up    - start all dev containers (PostgreSQL + Redis + S3/MinIO)"
	@echo "dev-down  - stop and remove all dev containers"
	@echo "dev-clean - stop all containers and remove their volumes"
	@echo ""
	@echo "Build:"
	@echo "build     - compile the panel service with version ldflags ($(SERVICE_VERSION))"
	@echo ""
	@echo "PostgreSQL:"
	@echo "pg-up      - start PostgreSQL dev container"
	@echo "pg-down    - stop and remove PostgreSQL container"
	@echo "pg-logs    - follow PostgreSQL logs"
	@echo "pg-psql    - open psql shell inside container"
	@echo "pg-clean   - remove PostgreSQL container and its volume"
	@echo "pg-migrate-up     - apply all pending migrations (goose)"
	@echo "pg-migrate-down   - rollback last migration (goose)"
	@echo "pg-migrate-status - show migration status (goose)"
	@echo "pg-migrate-new name=foo - create new migration file"
	@echo ""
	@echo "Redis:"
	@echo "redis-up      - start Redis dev container"
	@echo "redis-down    - stop and remove Redis container"
	@echo "redis-logs    - follow Redis logs"
	@echo "redis-cli     - open redis-cli shell inside container"
	@echo "redis-clean   - remove Redis container and its volume"
	@echo ""
	@echo "S3 (MinIO):"
	@echo "s3-up      - start MinIO dev container"
	@echo "s3-down    - stop and remove MinIO container"
	@echo "s3-logs    - follow MinIO logs"
	@echo "s3-clean   - remove MinIO container and its volume"

.PHONY: check-engine
check-engine:
ifndef CONTAINER_ENGINE
	$(error neither podman nor docker is installed)
endif

# podman needs :Z on volumes when SELinux is enforcing; docker ignores it
ifeq ($(shell basename $(CONTAINER_ENGINE)),podman)
VOL_OPTS := :Z
else
VOL_OPTS :=
endif

.PHONY: dev-up
dev-up: pg-up redis-up s3-up

.PHONY: dev-down
dev-down: pg-down redis-down s3-down

.PHONY: dev-clean
dev-clean: pg-clean redis-clean s3-clean

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

.PHONY: pg-psql
pg-psql: check-engine
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

.PHONY: pg-migrate-up
pg-migrate-up:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" up

.PHONY: pg-migrate-down
pg-migrate-down:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" down

.PHONY: pg-migrate-status
pg-migrate-status:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" status

# usage: make pg-migrate-new name=create_flags
.PHONY: pg-migrate-new
ifndef name
	$(error usage: make pg-migrate-new name=migration_name)
endif
pg-migrate-new:
	go tool goose -dir $(MIGRATIONS_DIR) create $(name) sql

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(PANEL_BIN) ./services/panel/cmd
