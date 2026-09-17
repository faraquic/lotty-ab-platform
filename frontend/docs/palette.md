# Lotty AB Platform — цветовая палитра (frontend)

> Строгий нейтральный серый (Neutral/Zinc), холодный slate-оттенок устранён.
> Источники истины: `src/styles/global.css` (базовые токены),
> `src/styles/interactions.css` (свечение, фоны, поверхности, навигация),
> `src/app/providers/AppProviders.tsx` (тема Mantine).
> Этот файл — описание, а не источник: меняешь цвет — меняй CSS и обновляй таблицу.

## Переключение темы

- Ключ в `localStorage`: `labp-color-scheme`, значения `system | light | dark`, по умолчанию `system`.
- `system` следует за `prefers-color-scheme` и отслеживает смену темы ОС.
- Переключение — через селектор `:root[data-mantine-color-scheme='light' | 'dark']`.
- Шрифт везде: `Inter, -apple-system, …`, радиус по умолчанию `md`, `primaryColor: 'green'`.
- **Синий запрещён**: в теме шкала `blue` перемаплена на `green`, ни один дефолт Mantine не даст синего пикселя.

## Базовые токены

| Токен | Light | Dark |
|---|---|---|
| `--mantine-color-body` (фон страницы) | `#fafafa` | `#121212` |
| `--mantine-color-text` (основной текст) | `#171717` | `#f5f5f5` |
| `--mantine-color-dimmed` (вторичный текст) | `#737373` | `#a3a3a3` |
| `--mantine-color-default` (поверхность/сайдбар) | `#ffffff` | `#171717` |
| `--mantine-color-default-border` (бордер) | `#e5e5e5` | `#333333` |
| `--input-bd-hover` (бордер инпута при наведении) | `#a3a3a3` | `#525252` |
| `--input-focus` (фокус инпута) | `#16a34a` | `#22c55e` |

## Фоны и поверхности (solid)

Сайдбар и контент — в единой цветовой температуре; разделение только за счёт
лёгкой разницы в светлоте, а не смены оттенка.

| Поверхность | Light | Dark |
|---|---|---|
| Страница / контент (`.login-bg`, `.app-bg`) | `#fafafa` | `#121212` |
| Сайдбар / карточки (`--mantine-color-default`) | `#ffffff` | `#171717` |
| Шапка `.app-header` (frosted, `blur(8px)`) | `rgba(255, 255, 255, 0.85)`, бордер `#e5e5e5` | `rgba(23, 23, 23, 0.85)`, бордер `#333333` |
| Нижняя панель `.panel-illuminate` (frosted, `blur(8px)`) | `rgba(255, 255, 255, 0.85)`, бордер `#e5e5e5` | `rgba(23, 23, 23, 0.85)`, бордер `#333333` |
| Hover `.panel-illuminate` (только бордер+тень, фон не меняется) | бордер `rgba(22, 163, 74, 0.5)`, тень `0 0 10px rgba(22, 163, 74, 0.14)` | бордер `rgba(34, 197, 94, 0.55)`, тень `0 0 12px rgba(34, 197, 94, 0.18)` |

Курсор на внутреннем контроле панели: светится только контрол, карточка остаётся спокойной
(бордер возвращается к `#e5e5e5` / `#333333`, тень снимается).

## Инпуты (непрозрачные, без примесей)

В тёмной теме инпуты чуть темнее поверхности — выглядят вдавленными.

| Состояние | Light | Dark |
|---|---|---|
| Фон | `#ffffff` | `#0a0a0a` |
| Бордер | `#d4d4d4` | `#262626` |
| Hover — бордер | `#16a34a` | `#22c55e` |
| Focus — бордер + halo (тонкое кольцо, без размытого неонового свечения) | `#16a34a`, `0 0 0 1px #16a34a` | `#22c55e`, `0 0 0 1px #22c55e` |

Error-состояние инпутов — красное, стили Mantine, не переопределять.

## Навигация и активные элементы (альфа-свечение, `.app-nav-link`)

Плотного тёмно-зелёного фона нет: активный пункт — прозрачность, ложится на любой фон.
Неслоёный CSS перебивает собственные hover/active-фоны Mantine NavLink;
активность приходит через `active`-проп (`[data-active]`).

| Элемент | Состояние | Стили (Light & Dark) |
|---|---|---|
| Пункт меню | Базовое | текст dimmed (`#737373` / `#a3a3a3`), фон transparent |
| Пункт меню | Hover | текст text (`#171717` / `#f5f5f5`), фон `rgba(163, 163, 163, 0.08)` |
| Active (напр. «Главная») | — | текст `#15803d` (light) / `#22c55e` (dark), фон `rgba(22, 163, 74, 0.12)` |

## Primary green (кнопки, акценты)

| Элемент | Light | Dark |
|---|---|---|
| `.btn-glow-green` фон / hover | `#16a34a` / `#15803d` | `#22c55e` / `#16a34a` |
| Текст кнопки | `#ffffff` | `#ffffff` |
| Внутренний блик | `inset 0 1px 0 rgba(255, 255, 255, 0.15)` | `inset 0 1px 0 rgba(255, 255, 255, 0.2)` |
| Hover-тень (направленная вниз) | `0 4px 12px rgba(22, 163, 74, 0.2)` | `0 4px 12px rgba(34, 197, 94, 0.15)` |
| Active (нажатие) | `scale(0.98)` + `inset 0 2px 4px rgba(0, 0, 0, 0.3)` | то же |
| Focus-outline | `2px solid #16a34a`, offset `2px` | `2px solid #22c55e`, offset `2px` |
| `.accent-word` (слово «A/B» в заголовке) | `#16a34a` | `#22c55e` |

## Статусы (имена палитры Mantine, резолвятся под тему автоматически)

| Смысл | Цвет | Где |
|---|---|---|
| OK / healthy / operational | `green` | StatusPage header, `ServiceStatusSection`, бейджи компонентов `ok` |
| Degraded / warning / partial | `yellow` | те же + нотификация `auth-required` |
| Down / unhealthy / outage | `red` | те же + ошибки |
| Loading-индикатор (`OverallStatusPill`) | `#a3a3a3` / `#525252` (dark) | пульсирующая точка |
| Success-нотификации (`login-success`, `signed-out`, users CRUD) | `green` | bottom-right, лимит 5 |
| Error-нотификации | `red` | bottom-right |

## Роли пользователей (бейджи `variant="light"`, `lib/roleBadge.ts`)

| Роль | Цвет |
|---|---|
| `admin` | `red` |
| `experimenter` | `green` |
| `approver` | `violet` |
| `viewer` | `gray` |

Destructive actions — `red` (`ActionIcon` удаления, `confirmProps` модалки, alert ошибки списка).

## Поверхности StatusPage (`.status-service-item`, выводятся из токенов через `color-mix`)

| Состояние | Фон | Бордер |
|---|---|---|
| Collapsed | `var(--mantine-color-default)` | `default-border 45%` |
| Hover | `default 93% + white 7%` | `default-border 65%` |
| Expanded (`[data-active]`) | `default 96% + black 4%` | `default-border 60%` |
| Expanded + hover | `default 94% + black 6%` | `default-border 70%` |

Собственный hover контролей Accordion нейтрализован (`transparent`) — иначе конфликтный прямоугольник.
Иерархия без скачков: collapsed < hover < expanded.

## Спиннер / индикатор статуса

- `.segmented-spinner__group`: 6 однородных SVG-дуг (`currentColor`, opacity 1), вращение группы `1.2s linear infinite`, `prefers-reduced-motion` отключает.
- Цвет наследуется от контейнера — цвет агрегата (`green` / `yellow` / `red`).

## Правила (не нарушать без явной задачи)

- Только функциональные акценты: cyan, blue→green, green, purple/violet, amber/yellow, red, gray.
- Запрещены: Tailwind, glassmorphism, тяжёлые тени, огромные градиенты, crypto-neon.
- Токены Mantine (`--mantine-color-*`) предпочтительнее хардкода; кастом — только в `src/styles/`.
- Не оверрайдить внутренности компонентов Mantine через CSS без обоснования в отчёте.
- `prefers-reduced-motion: reduce` отключает все transitions/animations интеракций.
