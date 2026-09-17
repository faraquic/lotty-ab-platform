# labp

React 19 + TypeScript + Vite frontend for Lotty AB Platform.

## Requirements

- Node.js (LTS)
- pnpm (`paru -S pnpm` or https://pnpm.io/installation)

## Setup

```bash
pnpm install
```

## Run

```bash
pnpm dev      # dev server
pnpm preview  # preview production build
```

## Checks

```bash
pnpm typecheck  # tsc -b --noEmit (strict mode)
pnpm lint       # eslint . (check only, no auto-fix)
pnpm format     # prettier --write
pnpm build      # tsc -b && vite build
```

## Stack

- React 19 + TypeScript (strict) + Vite
- React Router (`/ ` → LoginPage)
- Mantine (`@mantine/core`, `hooks`, `form`, `notifications`, `modals`) + `@tabler/icons-react`
- `@tanstack/react-query` (QueryClient base, no API hooks yet)
- i18next + react-i18next + i18next-browser-languagedetector (local `ru`/`en` bundles)
- Inter via `@fontsource/inter` (bundled, offline-friendly)
- ESLint (check-only) + Prettier, pnpm

## Design tokens

Dark Mode stylekit (slate system) implemented on Mantine without Tailwind.
Interaction layer lives in `src/styles/interactions.css`, palette in `src/styles/global.css`.

| Token | Dark | Light (mirrored) |
|-------|------|------------------|
| Background | `#0f172a` (slate-900) | `#f8fafc` (slate-50) |
| Surface | `#1e293b` (slate-800) | `#ffffff` |
| Border | `#334155` (slate-700) | `#e2e8f0` (slate-200) |
| Text / dimmed | `#f1f5f9` / `#94a3b8` | `#0f172a` / `#64748b` |

Stylekit → implementation mapping:

- Primary button → `.btn-glow-green`: green-600 `#16a34a`, hover green-500 `#22c55e`,
  inset top-edge glow, intensified glow on dark hover, `active:scale(0.98)` + inset depression,
  `focus-visible` ring + 2px offset, `200ms ease-out`, visible disabled state.
- Inputs → Mantine defaults on slate vars; focus ring blue-500.
- Panels → `.panel-illuminate`: border brightens on hover
  (dark `#334155` → `#64748b`, light mirrored darker).
- Accent word → `.accent-word`: `#16a34a` on light, `#22c55e` on dark.
- Stock Mantine `blue` / `green` / `red` / `yellow` — accents, errors, warnings, rare info.
- `prefers-reduced-motion` disables transform/transition.
- Banned: `bg-black`, pure-white text (except on the green button), light shadows on dark.
- Focus is green everywhere (`primaryColor: 'green'`, `--input-focus` per scheme).
  Blue is fully removed from the UI: no blue props anywhere, and the stock `blue`
  scale is remapped to green in `createTheme` so no component default can render blue.
- Ambient background: `BackgroundWaves` — five thin green contour lines stacked from
  mid-screen down, each lower layer dimmer (0.16 → 0.04), slow alternate drift
  (14–30s), static under `prefers-reduced-motion`.

Geometry and type: `defaultRadius: 'md'` (8px), Inter (400/500/600/700) with system fallback.
Icons: `MetricIcon` (`components/ui`) — Tabler glyph in an 8px geometric shape with a
12% translucent accent background (`color-mix`), no glow or gradients.

## Directory structure

```
src/
  app/
    App.tsx            # root component
    providers/         # AppProviders: Mantine + Query + Notifications + Modals
    router/            # createBrowserRouter, single route / → LoginPage
  components/
    ui/                # BottomBar (theme/locale dropdowns), MetricIcon
  features/
    auth/              # LoginForm (@mantine/form validation)
  i18n/
    index.ts           # i18next init + CustomTypeOptions augmentation
    locales/
      en/common.json
      ru/common.json
  pages/
    LoginPage.tsx      # the only page (UI only, no backend)
  shared/
    lib/               # queryClient (retry: 1, no refetch on focus, 30s stale)
    types/             # ThemePreference, Locale
  styles/
    global.css         # reset + per-scheme palette overrides
```

## Adding translations

1. Add the key to **both** `src/i18n/locales/en/common.json` and `src/i18n/locales/ru/common.json`
   (`en` is the `CustomTypeOptions` source, so keys must match it).
2. Use it via `const { t } = useTranslation();` — never hardcode user-facing strings.
3. `fallbackLng` is `'en'`; no HTTP backend, translations are bundled.

## Changing the theme

- Preference type: `'system' | 'light' | 'dark'` (default `'system'`).
- Managed by Mantine's official `localStorageColorSchemeManager`
  (key `labp-color-scheme`, `defaultColorScheme="auto"`) in `AppProviders`.
- `system` maps to Mantine `auto` and follows the OS live via `matchMedia`.
- `index.html` contains an inline script mirroring Mantine's `ColorSchemeScript`
  to avoid a theme flash before mount.
- Switch theme and language with the fixed `BottomBar` (two dropdown `Select`s).
