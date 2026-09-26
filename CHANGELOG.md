# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Targeting DSL (pkg/targeting)**: New standalone package implementing the targeting expression grammar:
  - Grammar: `expr := or_expr`, `or_expr := and_expr ("OR" and_expr)*`, `and_expr := unary_expr ("AND" unary_expr)*`, `unary_expr := "NOT" unary_expr | primary`, `primary := comparison | "(" expr ")"`, `comparison := field operator literal | field "IN" array | field "NOT IN" array`, `operator := "==" | "!=" | ">" | ">=" | "<" | "<="`
  - Canonical JSON AST storage in DB (jsonb), API accepts/returns DSL string
  - Parser, formatter, validator with comprehensive tests
  - Backend: `ValidateVariants` now validates variant values against flag type (`string`/`number`/`bool`) at write time (create/version/set_variants/check_ready), not only on rollout
  - Frontend: typed value input per flag type (text for string, number input for number, select for bool), DSL validation in UI
  - OpenAPI updated: `targeting` now `string|null` with DSL description; `ExperimentVariantInput.value` description updated

- **Unified Health Endpoint**: `/health` now returns the full component readiness report (previously `/ready`). `/ready` removed from all services (panel, runtime, analytics).
  - Single fetch instead of two per service
  - OpenAPI specs updated for all three services
  - Frontend probes (`probes.ts`) simplified to single request
  - Status page (`/status`) and `StatusPage` component removed
  - `status-nav-button` in header repurposed as indicator only (no navigation)
  - `OverallStatusPill` reused in header; polling interval 20s unchanged

- **Server-side Query Filters**: All list endpoints support pagination + search via query parameters using `ShouldBindQuery`:
  - `GET /users`: `q` (name/email), `role` (admin|experimenter|approver|viewer)
  - `GET /flags`: `q` (key/name)
  - `GET /experiments`: `q` (name), `flag_id` (uuid)
  - `GET /approver-groups`: `q` (name)
  - OpenAPI updated with new parameters

- **Role-based Member Validation**: 
  - `AddMember` (approver groups) — only `approver` role allowed (422 otherwise)
  - `SetExperimenterGroup` — only `experimenter` role allowed (422 otherwise)

- **Frontend Search UI**: Debounced search inputs (300ms) added to Flags and Users pages, using server-side `q` parameter.

### Changed

- **Experiments Table**: Removed `Version` column (was showing `v—` since list endpoint didn't load `current_version`).
- **Flag Selector in Create Experiment**: Displays only flag name; searchable by both name and key.
- **Allocation Display**: Now percentages with 2 decimals (`50.00%`) instead of basis points (`5000`); consistent across create/version/variants editor/details.
- **Duplicate Create Buttons**: Removed from empty states (Users, Flags, Experiments, Metrics); header button remains.
- **Navigation**: Users tab hidden for non-admin roles.
- **Assignments Display**: Full name + role only (email removed).
- **Review Detail Page**: New route `/reviews/:id` (was modal-only); `ReviewDetailsView` component extracted.
- **Approver Groups Table**: Edit action moved to separate `IconPencil` in Actions column (was text button in modal); View remains `IconEye`.
- **Lifecycle Actions Panel**: Moved to right-top card (`Actions`) with vertical stack and icons:
  - `Save` (form submit via `form="experiment-overview-form"`)
  - Transitions: `Submit`, `Start`, `Pause`, `Resume`, `Complete`, `Roll out`, `Archive`, `Guardrail pause/rollback`
  - Each with appropriate Tabler icon
- **Form Reset on Success**: Create modals (users, flags, experiments, version, groups) reset via key-based remount + form.reset() on close/success.
- **Group Form Key Rotation**: New group creation increments key to force remount and clear stale data.
- **Review Table Filters**: Status filter moved to table header icon (same pattern as experiments).
- **Variant Options**: Rollout/Complete modals show `name — value (control)` for all variants including control.
- **Metric Polling**: Added `refetchInterval: 60s` to metrics list query.
- **Create Version Modal**: Targeting input uses DSL string; weight in % with 2 decimals; form reset on close.

### Removed

- `/ready` endpoint from panel, runtime, analytics (merged into `/health`)
- `/status` route and `StatusPage` component
- `ReviewDetailsModal` component (replaced by `ReviewDetailsView` + `ReviewDetailsPage`)
- `status.openStatus` i18n key (unused, kept in locale files)

### Fixed

- Create forms (Users, Flags, Experiments, Version, Groups) now properly reset on submit/close — no stale data on reopen.
- Variant value type mismatch now caught at write time (422) instead of failing at rollout.
- Control variant now selectable in Rollout/Complete modals (was silently rejected by backend type mismatch).
- `status-nav-button` no longer navigates; serves as health indicator only.

### Documentation

- OpenAPI specs (`docs/openapi/panel.yaml`, `runtime.yaml`, `analytics.yaml`): health endpoints, query params, targeting DSL, variant value validation notes.
- `docs/deploy-dev.md`: Updated health check verification to use `/health` endpoint.

### Infrastructure

- New `pkg/targeting/` package (pure Go, no external deps)
- `pkg/api/query.go` — shared list query binding helper
- Frontend targeting DSL parser (`frontend/src/features/experiments/lib/targeting.ts`)
- New `ReviewDetailsView.tsx` and `ReviewDetailsPage.tsx` components
- Updated e2e tests for targeting string and member role validation

### Dependencies

- No new external dependencies (all pure Go / existing frontend deps)

---

## [0.1.0] - 2026-09-17

### Added

- Initial implementation of Lotty AB Platform: feature flags, experiments, reviews, metrics
- Panel (Gin), Runtime (Fiber v3), Analytics (Fiber v3) services
- React 19 + Mantine v8 frontend
- PostgreSQL 18, Redis 8, MinIO S3, nginx reverse proxy
- JWT auth with in-memory sessions, RBAC (admin, experimenter, approver, viewer)
- Goose migrations, Docker Compose deployment