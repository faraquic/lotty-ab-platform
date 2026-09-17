# AGENTS.md — Lotty AB Platform (frontend)

> Engineering brief for AI agents working on this React app.
> Read this file before starting any task. Do not guess what is not specified here.
>
> Last updated: 2026-09-17

## Project overview

- React + TypeScript + Vite SPA: operator UI for Lotty AB Platform (feature flags, experiments, analytics).
- **Goal:** control-plane UI over the panel API (`/api/v1/panel`, OpenAPI in `docs/openapi/panel.yaml`).
  Auth shell is done; next is domain pages (flags, experiments, reviews, metrics) behind `RequireAuth`,
  one OpenAPI-transmitted endpoint at a time.
- Current UI library is **Mantine — the only UI library in this project**.
  Do not install `primereact`, `primeicons`, `@primeuix/themes`, and do not
  migrate components to any other kit. Maximize use of Mantine components.
- Backend contract for login is provided (`docs/openapi/panel.yaml`, `POST /api/v1/panel/login`)
  and integrated in `src/features/auth/` (fetch + React Query mutation, in-memory JWT only).

## Tech stack

| Area | Choice |
|------|--------|
| UI components | Mantine only: `@mantine/core`, `@mantine/form`, `@mantine/hooks`, `@mantine/modals`, `@mantine/notifications`; icons `@tabler/icons-react`; fonts `@fontsource/inter` |
| Routing | React Router v7 (`createBrowserRouter` in `src/app/router/router.tsx`): `/login` + `/status` public, `/` under `RequireAuth` (`AppLayout` + `RootIndexStub`), `*` → `NotFoundPage` |
| Local UI state | React state/hooks only |
| Server state / API client | `fetch` + `@tanstack/react-query` v5. Integrated: login. Other domains only after their OpenAPI contract is transmitted |
| Language | TypeScript strict mode (`tsconfig.app.json`) |
| Build / lint / format | Vite, ESLint, Prettier |
| Package manager | `pnpm@12.4.2` (`packageManager` field; `pnpm-lock.yaml` is committed; `npm run <script>` also works) |

## Implementation status

### Built

- **Auth shell** (`src/features/auth/`): `LoginForm` (Mantine `useForm` client validation) + `LoginPage`
  (`queryClient.clear()` on login success so a new account never sees the previous session's cache);
  `api/login.ts` (`POST {base}/login`, runtime shape guards), `api/useLoginMutation.ts` (React Query),
  `types.ts` (`LoginApiError` with `http`/`network`/`unexpected` kinds),
  `lib/loginErrorMessage.ts` (401/400/403/404/422/429/5xx mapping; appends backend `message (CODE)`,
  5xx via `serverErrorDetail` with status/code/message), `lib/authSession.ts` (in-memory JWT,
  `useSyncExternalStore`: `set/get/clear/useAuthSession`),   `RequireAuth.tsx` (redirect to `/login`
  with `auth-required` notification; logged-in users are bounced `/login` → `/`),
  `LogoutButton.tsx` (header button: `clearAuthSession` + `queryClient.clear()` + `signed-out`
  notification + redirect to `/login`),
  `lib/devLoginDefaults.ts` (prefills `root@labp.net` / `root!@#$` only when `import.meta.env.DEV`;
  overridable via `VITE_DEV_LOGIN_EMAIL` / `VITE_DEV_LOGIN_PASSWORD`; compiled out of the prod bundle).
- **Status Page** (`src/features/status/` + `src/pages/StatusPage.tsx`, public route `/status`):
  `types.ts` (service/component/overall statuses), `api/probes.ts` (health+ready fetch per service,
  5s `AbortSignal` timeout, probe latency, never throws — failures are DOWN data),
  `api/useServiceStatus.ts` (React Query, `refetchInterval: 20_000`, `placeholderData` keeps data),
  Accordion в контролируемом `multiple`-режиме (`useState<string[]>` — cards раскрываются независимо),
  `lib/aggregate.ts` (pure service/overall calculation), `lib/format.ts` (time/latency formatting),
  `components/ServiceStatusSection.tsx` (Mantine Accordion `separated` + `classNames`:
  coherent surface system via `.status-service-item` in `interactions.css` — collapsed/hover/expanded
  derive from `--mantine-color-default` via color-mix, subtle borders, spacing instead of
  dividers; rows: indicator, name, inline `service · version · environment`, latency, status badge;
  components as compact icon badges shown with messages only for non-ok; expanded meta + footer), wide public layout (Container md, dominant global status
  header with Last checked + manual Refresh via query `refetch`), Back button to `/` (`status.back`),
  `components/OverallStatusPill.tsx` (dot + short overall label, reused probes/aggregate), vitest suites
  (`aggregate.test.ts`, `probes.test.ts` with stubbed fetch). No auth headers, no JWT reads.
- **App shell**: `AppLayout` (Mantine `AppShell` with left `Navbar` (Home + Users `NavLink`s,
  `Burger` below `sm`) + header with Status button → `/status`, minimal `OverallStatusPill`,
  `CurrentUserChip` (avatar + name + role badge from `/me`, skeleton while loading) and logout), `RootIndexStub`, `NotFoundPage`,
  `BottomBar` (theme/locale selects), `MetricIcon` (shared colored icon box, not used by any page yet),
  `SegmentedSpinner` (shared 6-arc SVG spinner, uniform segments, `currentColor`, group rotation via `interactions.css`; StatusPage header shows it as a ring (`size={56}`) with centered status icon from `overallDisplay` mapping inside `.status-indicator` grid, color from overall status, decorative `aria-hidden`).
- **Users page** (`src/features/users/` + `src/pages/UsersPage.tsx`, authenticated route `/users`
  behind `RequireAuth`, nav button in `AppLayout` header next to Status):
  `types.ts` (`UserRole` enum guard, `User`/`PaginationMeta`/`UserListResponse`/`UserResponse`
  shape guards, `UsersApiError` with `http`/`network`/`unexpected` kinds),
  `api/users.ts` (fetch + `Authorization: Bearer` via `getAuthToken()`, 401 clears the in-memory
  session so `RequireAuth` bounces to `/login`; list query key `['users', { limit, offset }`;
  `limit`/`offset` only — no search/filter/sort; DELETE expects 204; avatar upload/delete as
  multipart `FormData`, empty form deletes),
  `api/useUsers.ts` (React Query v5: list with `placeholderData`, detail, `/me`, create/update/
  delete/avatar mutations with list+detail invalidation),
  `lib/usersErrorMessage.ts` (400/401/403/404/409/422/429/5xx mapping with backend `message (CODE)`),
  `lib/rules.ts` (self-delete/self-role-change gating hints, changed-fields-only PATCH builder,
  pagination helpers), `lib/format.ts` (locale date-time),
  `components/UsersTable.tsx` (Mantine Table: avatar/name/email/role badge/created/actions),
  `components/CreateUserModal.tsx` (full_name/email/password/role, client validation),
  `components/UserDetailsModal.tsx` (read-only name, editable email+role, role locked for self,
  admin-only avatar upload/remove, id/created/updated meta),
  `components/CurrentUserChip.tsx` (header chip, reused `/me` query),
  `lib/roleBadge.ts` (shared role → badge color),
  delete via `modals.openConfirmModal`
  with name/email confirmation, loading skeletons, empty state, error state with Retry,
  403 list hint for non-admins (users CRUD is admin-only server-side), i18n `users.*` (en/ru),
  vitest suites (`users.test.ts` with stubbed fetch, `rules.test.ts`). No password edit —
  `UpdateUserRequest` allows only `email`/`role`.
- **Flags page** (`src/features/flags/` + `src/pages/FlagsPage.tsx`, authenticated route `/flags`
  behind `RequireAuth`, sidebar `NavLink`):
  `types.ts` (`FlagType` enum guard (`string`/`number`/`bool`), per-type `default_value`
  shape guards, nullable `created_by`/`updated_by` via users `parseUser`, `FlagsApiError`),
  `api/flags.ts` (same fetch/auth/401 pattern as users; list key `['flags', { limit, offset }]`,
  DELETE expects 204), `api/useFlags.ts` (list/detail/create/update/delete + invalidation),
  `lib/flagsErrorMessage.ts` (same mapping, `flags.errors.*`),
  `lib/flagValue.ts` (typed default parse/format/equality: number validated via `JSON.parse`
  exactly like the backend; changed-fields-only PATCH builder — empty key/name/description
  are omitted because the backend treats them as «keep», description can never be cleared),
  `lib/format.ts` (locale date-time),
  `components/FlagsTable.tsx` (key/name/type badge/default/updated/actions;
  type colors: string cyan, number violet, bool amber),
  `components/FlagDefaultInput.tsx` (shared typed default editor: text for string/number,
  select for bool), `components/CreateFlagModal.tsx` (key 3–128/name/type/typed default/
  optional description), `components/FlagDetailsModal.tsx` (type read-only + immutability note,
  editable key/name/default/description, creator/updater names when present),
  delete via `modals.openConfirmModal`, skeletons/empty/error+Retry, i18n `flags.*` (en/ru),
  vitest suites (`flags.test.ts`, `flagValue.test.ts`).
  Writes are admin-only server-side: non-admins get a read-only UI (`readOnlyHint`, no
  create/delete controls, fields disabled) — backend remains authoritative (403).
- **Feedback**: `<Notifications position="bottom-right" limit={5}>`; stable ids
  (`login-success`, `login-error`, `auth-required`, `signed-out`) — repeats update instead of stacking.
- **i18n**: `en`/`ru` (`src/i18n/`, typed resources, browser detector with `localStorage` cache).
- **API wiring**: base URLs from `VITE_*_API_BASE_URL` (`panel`, `runtime`, `analytics`),
  fallbacks `/api/v1/{panel,runtime,analytics}` (`src/shared/config/api.ts`); dev proxy for all three
  (`vite.config.ts` → `localhost:8081/8082/8083`);
  docker image builds SPA into nginx (`frontend/Dockerfile` + `frontend/nginx.conf`: SPA fallback to
  `index.html`, `/health` → `ok`), served by compose nginx at `/`.

### Not built

- Domain pages: experiments, reviews, metrics — no UI, no API clients.
- Authenticated API calls beyond users/flags: no `Authorization` header wiring yet for other domains
  (users and flags reuse `getAuthToken()` for their `Bearer` headers).
- Logout UI is client-side only (backend has no `/logout`; JWT session is in-memory, nothing to revoke server-side — Redis session entries expire via TTL), token refresh, session restore.

### Known limitations

- Session lives in memory only: page reload logs the user out and `RequireAuth` sends them to `/login`
  (tokens must never go to `localStorage`).
- Backend 4xx/5xx messages are English-only; shown verbatim inside translated templates for detail.

## UI component policy

- Mantine is the only UI library. Use ready-made Mantine components first. Do not hand-roll standard UI elements when Mantine offers a fit, and do not introduce another component kit.
- Mapping: buttons → `Button`; text fields → `TextInput` / `PasswordInput` / `Textarea` (+ `useForm` from `@mantine/form`); selection → `Select`, `MultiSelect`, `Checkbox`, `Radio`, `Switch`, `DatePickerInput` (from `@mantine/dates`, install only when needed); feedback → `Alert`, `notifications`/`Notifications`, `modals`/`ModalsProvider`, `Loader`, `Progress`, `Skeleton`; data → `Table`, `Pagination` (compose, do not build custom tables/pagination); navigation and layout → `AppShell`, `NavLink`, `Tabs`, `Breadcrumbs`, `Burger`, `Drawer`, `Modal`, `Menu`.
- Never build custom buttons, selects, modals, tooltips, toasts, tables, or pagination without an explicit reason stated in the final report.
- Custom components are allowed only for product-specific composition: `AppLayout`, `PageHeader`, `EmptyState`, `ErrorState`, `StatusIndicator`, `EntitySummary`, and similar domain blocks.
- Do not wrap a Mantine component in your own wrapper without a reason.

## Styling and theme policy

- Style: compact technical control panel / developer tool. Strict neutral gray surfaces
  (Neutral/Zinc, no slate tint), small radii, high readability.
  Light: body `#fafafa`, surface `#ffffff`, border `#e5e5e5`, dimmed `#737373`;
  dark: body `#121212`, surface `#171717`, border `#333333`, dimmed `#a3a3a3`.
- Functional accents only: cyan, blue, green, purple, amber, red.
- Forbidden unless explicitly tasked: Tailwind, glassmorphism, heavy shadows, huge gradients, crypto-neon visuals.
- Prefer Mantine theme tokens (`createTheme` in `src/app/providers/AppProviders.tsx`, CSS vars like `--mantine-color-*`); custom interaction classes live in `src/styles/interactions.css` (`.btn-glow-green`, `.panel-illuminate`, `.login-bg`, `.accent-word`), global tokens in `src/styles/global.css`. Full palette reference (light/dark, glows, surfaces): `docs/palette.md` — update it when changing colors. Do not override Mantine component internals via CSS unless justified in the report.
- Theme preference: `type ThemePreference = 'system' | 'light' | 'dark'`, default `system`, stored in `localStorage` (existing key: `labp-color-scheme`). `system` follows `prefers-color-scheme` and tracks OS changes only while preference is `system`.
- Theme state: Mantine `localStorageColorSchemeManager` + `BottomBar` selects today; there is no
  `src/features/theme` yet — create it only when theme logic outgrows `BottomBar`. Never scatter theme state across pages.

## Architecture

- Feature-first. Pages compose layout + feature components; a feature owns its UI logic, types, and (later) its API client.
- `shared/` holds only generic, feature-agnostic code (`lib/`, `types/`, `config/`); reusable UI blocks live in `src/components/ui/`.
- Actual layout (keep it, create folders only when actually needed):
  `src/app/{providers,router,layout}/`, `src/pages/`, `src/features/{auth,…}/` (each: components + `api/` + `types.ts` + `lib/` as needed),
  `src/components/ui/`, `src/shared/{lib,types,config}/`, `src/i18n/locales/{en,ru}/`, `src/styles/`.
- `src/pages/UsersPage.tsx` / `src/pages/FlagsPage.tsx` are the domain pages; their
  tables/modals live in `src/features/users/components/` and `src/features/flags/components/`.
  Generic pagination math lives in `src/shared/lib/pagination.ts` (re-exported by users `rules.ts`).
- No upfront folders/abstractions without real usage. No giant `App.tsx` or giant page components. Keep local state next to its component; server state stays separate from UI state later.
- Strict TypeScript: no `any`, no `@ts-ignore`, no `eslint-disable`, no disabling strict mode.

## Backend and security boundaries

The login contract (`POST /api/v1/panel/login`, see `docs/openapi/panel.yaml`) is transmitted
and implemented. For every other endpoint it is still **forbidden** to:

- guess endpoints, DTOs, auth format, tokens, cookies, CORS settings, or user roles;
- create fake APIs, mock auth, JWT integration, network requests, or router guards.

API layer appears only after the contract. Never store auth/refresh tokens or secrets in `localStorage` (theme preference only).

Rule for the next domain (flags, experiments, reviews, metrics, users): transmit its OpenAPI fragment first,
then implement `types.ts` → `api/` (fetch + shape guards) → React Query hook → UI behind `RequireAuth`,
reusing `getAuthToken()` for the `Authorization` header on the first authenticated call.

## Quality checklist

- Server-driven scenarios include `loading`, `error`, and `empty` states.
- Destructive actions require `modals.openConfirmModal` (once data/mutations exist).
- Keyboard accessibility: buttons, forms, links, inputs focusable and operable; forms submit on Enter where expected; correct `label`s and `aria` attributes.
- No inline object styles without need; component CSS stays in style files.
- No new dependency without justification: what problem it solves, why Mantine/React cannot cover it, size and impact on the project. New deps and architectural decisions require user confirmation **before** installing.

## Agent workflow

1. Briefly list the files you plan to change before touching them.
2. If the task implies a new dependency or architectural decision, ask for confirmation first — never install automatically.
3. Implement only the current task's scope; do not rewrite unrelated code.
4. After changes, run (in `frontend/`): `pnpm typecheck`, `pnpm lint`, `pnpm build`, `pnpm test`. Fix all reported errors.
5. Final report must include: changed files; added/removed dependencies; what was implemented; results of all checks; known limitations and what was deliberately left out.

## Commands

Run from `frontend/`:

```bash
pnpm dev         # Vite dev server
pnpm typecheck   # tsc -b --noEmit
pnpm lint        # eslint .
pnpm build       # tsc -b && vite build
pnpm test        # vitest run
pnpm preview     # preview production build
pnpm format      # prettier --write "src/**/*.{ts,tsx,css,json}"
```

Note: repo root `/AGENTS.md` describes the Go backend; this file governs the React frontend only.
