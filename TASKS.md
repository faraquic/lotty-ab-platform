# TASKS

Status of the panel service. Panel-scope only; the broader platform roadmap lives in `task.md`.

## Done

- [x] Project skeleton: Go module, config loader (JSON + `{{ ENV }}` templates + defaults), zap logger
- [x] HTTP stack: gin router, middleware chain (Recovery → RequestID → Logger → CORS via gin-contrib)
- [x] Request IDs: UUIDv7 validation/generation, `X-Request-ID` propagation into logs
- [x] Response envelope helpers (`pkg/api`) with validator-backed request decoding
- [x] PostgreSQL access: pgx pool with ping-on-start (`pkg/database/postgres.go`)
- [x] Redis access: go-redis client, non-fatal on connection failure
- [x] Users domain: CRUD API (create/list/get/patch/soft-delete), bcrypt password hashing, role CHECK constraint
- [x] Auth domain: `POST /login` issuing JWT (HS256) via `lib/auth.Tokenizer`
- [x] Auth middleware (`appleboy/gin-jwt/v2`): Bearer validation on `/users/*`, envelope-style 401s
- [x] Redis sessions: issued tokens stored under `panel:sessions:<sha256>` with TTL; middleware checks revocation (fails open if Redis is down)
- [x] Migrations: goose SQL migrations v1–v2 (users table, email + soft delete), `make pg-migrate-*`
- [x] Dev environment: Makefile targets for podman/docker containers (Postgres 5433, Redis 6379) with resource limits and healthchecks
- [x] OpenAPI spec: `docs/openapi/panel.yaml` (users + login, bearer security scheme declared)
- [x] First commit baseline

## Known gaps

- [ ] **Tests were deleted** — unit/integration/e2e suites existed and passed; need to be re-added (fake-repo unit tests, real-DB integration, httptest e2e; see CONVENTIONS.md interface-seam pattern that enables them)
- [ ] No RBAC checks (roles exist in schema, nothing consumes them)
- [ ] No refresh tokens / logout / token revocation endpoint (revocation *mechanism* exists — delete the Redis session key)
- [ ] No `/health` and `/ready` endpoints (needed for platform criterion B9)
- [ ] No CI pipeline, lint/format tooling config (golangci-lint), or Makefile targets for them
- [ ] OpenAPI spec drifts from code manually — consider codegen or contract tests later

## Next steps (suggested order)

1. Re-add test suites (unit → integration → e2e) per previous patterns; add Makefile target `test`
2. Auth middleware: verify Bearer JWT, inject user id into request context; protect `/users/*`; wire RBAC role checks
3. `/health` + `/ready` endpoints (pg ping required, redis optional)
4. Feature flags domain (model/migrations/CRUD) — first step toward experiment lifecycle
5. Experiments domain: lifecycle Draft → Review → Approved → Running (+ approver groups)
6. Redis Pub/Sub invalidation channel for runtime config distribution
7. golangci-lint + CI workflow running build/vet/lint/tests
