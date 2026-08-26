# TASKS

Status of the panel service. Panel-scope only; the broader platform roadmap lives in `task.md`.

## Done

- [x] Project skeleton: Go module, config loader (`$CONFIG_NAME` / `config.local.json` / `config.json`, `{{ ENV }}` templates + defaults), zap logger
- [x] HTTP stack: gin router, middleware chain (Recovery → RequestID → Logger → CORS via gin-contrib)
- [x] Request IDs: UUIDv7 validation/generation, `X-Request-ID` propagation into logs
- [x] Response envelope helpers (`pkg/api`) with validator-backed request decoding
- [x] PostgreSQL access: pgx pool with ping-on-start (`pkg/database/postgres.go`)
- [x] Redis access: **rueidis** client (URL or `host:port`), non-fatal on connection failure
- [x] Users domain: CRUD API (create/list/get/patch/soft-delete), bcrypt password hashing, role CHECK constraint, limit/offset pagination
- [x] Auth domain: `POST /login` issuing JWT (HS256) via `pkg/auth.Tokenizer`; role claim; `expires_at` in response
- [x] Auth middleware (`appleboy/gin-jwt/v3`): Bearer validation on `/users/*` (admin-only), session revocation + role gate inside `Authorizer`, envelope-style 401/403
- [x] Redis sessions: tokens stored under `labp:panel:auth:jwt:<uid>:<sha256>` with TTL; per-user revocation via SCAN+DEL (fails open if Redis is down)
- [x] Stale-role fix: role change/delete revokes all of the user's sessions — old tokens die immediately
- [x] Self-management protections: self-delete and self-role-change forbidden (403); last-admin demotion/deletion forbidden (409)
- [x] Bootstrap first admin: on startup, if users table is empty and `auth.bootstrap.password` is set (min 8 chars), an admin is created; otherwise a warning explains how to do it
- [x] Prod-hardening checks at startup: weak/default JWT secret → fatal error in prod (warn elsewhere); CORS `*` → warning
- [x] Migrations: goose SQL migrations v1–v2 (users table, email + soft delete), `make pg-migrate-*`
- [x] Dev environment: Makefile targets for podman/docker containers (Postgres 5433, Redis 6379) with resource limits and healthchecks
- [x] OpenAPI spec: `docs/openapi/panel.yaml` (login + users CRUD, health probes, bearer security scheme)
- [x] Health endpoints: `/health` (liveness, 200 `OK`) and `/ready` (readiness JSON with service/database/cache components; 503 when DB down, Redis optional)
- [x] Tests for `pkg/api`: envelope helpers (`OK`/`Error`/`InternalError`) and `ValidateRequest` edge cases (valid, unknown fields ignored, malformed JSON, missing/short/invalid fields, wrong type, empty body, oversized → `413`); fixed `ValidateRequest` so the body-size guard is deterministic (goccy/go-json swallowed `*http.MaxBytesError`)
- [x] Tests for `pkg/auth`: `JWTManager` generate/parse — round-trip, expiry ≈ now+ttl, role embedded, zero user id rejected; parse rejects expired (`ErrExpiredToken`), empty/garbage/wrong-secret/tampered/`none`-alg/non-numeric-subject/negative-subject/invalid-base64, and accepts a token lacking `exp` (existing behavior)
- [x] First commit baseline

## Known gaps

- [ ] **Tests were deleted** — only `pkg/api` has been re-covered so far; the rest (fake-repo unit tests, real-DB integration, httptest e2e) still need re-adding (interface-seam pattern in CONVENTIONS.md enables them)
- [ ] RBAC beyond admin-only `/users`: experimenter/approver/viewer route matrices arrive with the experiments domain
- [ ] No refresh tokens / logout endpoint (revocation *mechanism* exists — deleting the Redis session key kills a token instantly)
- [ ] No `/health` and `/ready` endpoints (needed for platform criterion B9)
- [ ] No CI pipeline, lint/format tooling config (golangci-lint), or Makefile targets for them
- [ ] OpenAPI spec drifts from code manually — consider codegen or contract tests later

## Next steps (suggested order)

1. Re-add test suites (unit → integration → e2e); add Makefile target `test`
2. Feature flags domain (model/migrations/CRUD) — first step toward experiment lifecycle
4. Experiments domain: lifecycle Draft → Review → Approved → Running (+ approver groups & approval thresholds from task.md §0.3)
5. Redis Pub/Sub invalidation channel for runtime config distribution
6. golangci-lint + CI workflow running build/vet/lint/tests
