# AGENTS

Guidance for AI coding agents working in this repository.

## Read first

- `PROJECT_CONTEXT.md` — architecture, stack, data schemas, file structure, config schema
- `CONVENTIONS.md` — coding patterns and error handling rules; follow them for any new code
- `TASKS.md` — current implementation status and the agreed order of next steps

## Working rules

1. Match existing patterns: domain code goes in `services/panel/internal/domain/<name>/` with `handler.go`, `dto.go`, `model.go` (if needed), `service.go`, `repository.go`.
2. Services depend on repository interfaces declared next to them; SQL lives only in repositories.
3. Respond only through `pkg/api` envelope helpers; validate request bodies via `api.ValidateRequest`; map sentinel errors to HTTP codes in a single `respondError` per handler.
4. Schema changes require a new goose migration in `migrations/` (never edit applied ones).
5. Before finishing any task: `go build ./... && go vet ./...` must pass.
6. Do not commit unless explicitly asked.
