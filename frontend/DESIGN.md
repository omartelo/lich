# lich — design language

The visual system for lich's frontend. It is deliberately narrow: one achromatic (zinc) palette, one
component idiom, no brand accent. This file is the source of truth for *how things should look and behave*.
Working rules (commands, state, shadcn workflow) live in `CLAUDE.md`; the tokens themselves live in
`src/index.css`. When this document and the code disagree, the code is a holdout to migrate — see
[Adoption](#adoption).

## The one idea: open surfaces, not boxes

The default shadcn look nests bordered boxes — a card with a `border` inside a panel, a filled pill inside the
card, a bordered chip inside the toolbar. lich rejects that. Elements are defined by **space, hover and
hairline seams**, not by enclosure.

- **Rows, not cards.** A list item has no border and no `bg-card`. It reads as a row; hover and selection
  paint it, nothing encloses it.
- **Borders are structural seams only.** A hairline (`--border`) separates a panel from its neighbour (sidebar
  edge, top tab bar, footer, dock divider). It never wraps an individual list item, badge, or a button at rest.
  Inputs and numeric fields are the one exception — a field needs an edge to read as editable.
- **Badges are text + glyph.** A count or branch is a lucide glyph next to text. No filled pill capsule.
- **State is fill, not chrome.** Selection is a flat `bg-accent` fill. No left accent bar, no ring, no shadow.

If a change adds a border, a pill, or a nested surface, it is probably wrong — reach for spacing or a hover
state first.

## Foundations

### Color

The bundled light/dark themes are zinc and achromatic — there is **no brand accent hue** by default. Palettes are
theme JSON data applied to the tokens mapped in `src/index.css` (`@theme` + CSS variables). Never hardcode a hex or
oklch value in a component; always go through a token.

Surface roles (dark values shown; light is the mirror):

| Token | Role |
| --- | --- |
| `--background` | app ground, terminal-adjacent panes |
| `--sidebar` / `--card` / `--popover` | raised chrome: session sidebar, tabs, footer, dock, menus, dialogs |
| `--accent` | hover and selection fill (the workhorse) |
| `--muted` / `--muted-foreground` | secondary text, icons at rest, paths, meta |
| `--border` | hairline seams (`oklch(1 0 0 / 10%)` in dark) and input edges |
| `--primary` | the single high-emphasis button fill |
| `--foreground` | primary text |

Interaction fills are Tailwind opacity steps on `--accent`, not new tokens:

- **rest** — transparent
- **hover** — `bg-accent/50` … `/60`
- **active / selected** — `bg-accent` (full)

### Semantic color — reserved for meaning

The only chromatic color in the app. It encodes state, never decorates, and is never used as an accent:

| Color | Meaning | Where |
| --- | --- | --- |
| `emerald-500` | done / added | session status ring, diff additions, diff-stat `+n` |
| `amber-500` | waiting on user | session status ring, project-tab badge |
| `muted` spinner | busy / producing | animated status ring |
| `--destructive` (red) | destructive action, deletions | diff removals, "Close session", discard |
| language badge hues | file language | diff file header only (`lang-badge.ts`) |

A busy/done/waiting ring wraps the provider glyph (`SessionStatusIcon`); the same three states badge an
inactive project tab (`ProjectTab`).

### Typography

- **Geist Variable** — all UI text (`--font-sans`). Bundled via `@fontsource-variable/geist`, no CDN.
- **FiraCode Nerd Font Mono** — the terminal, and every monospace context: file paths, code/diff, palette
  subtitles, dense meta. Bundled woff2; the terminal awaits `document.fonts` before opening.
- Scale is small and tight. Titles `text-sm`/`text-2xl` for screen headers; body `text-sm`; meta/paths
  `text-xs`. Section labels are uppercase with `tracking-wide` and `text-muted-foreground`.
- Line up digits with `tabular-nums` wherever they change (zoom %, diff counts, clock).

### Radius, spacing, icons

- `--radius: 0.45rem`. Use the Tailwind `rounded-*` scale that derives from it; don't sprinkle `rounded-lg`
  everywhere. Small controls (list rows, menu items, ghost buttons) sit around `rounded-md`.
- Density is high but breathing: list rows `~9px` vertical padding, `gap-1`–`gap-1.5` between them.
- Icons are **lucide** at `size-3.5`/`size-4`; `size-3` inside dense meta. Provider marks come from
  `ProviderIcon.tsx`, file-type logos from `devicons-react` (`FileIcon.tsx`).

## Interaction states

Every interactive surface follows the same ladder. Focus is always a visible `--ring`; disabled drops opacity
and pointer events.

| Surface | rest | hover | active / selected |
| --- | --- | --- | --- |
| Session row, palette row, settings nav, menu item | transparent | `bg-accent/50–60` | `bg-accent` + `text-accent-foreground` |
| Project tab | `text-muted-foreground` | `bg-accent/60` | `bg-accent` |
| Ghost / footer / toolbar button | `text-muted-foreground` | `bg-accent/50`, `text-foreground` | pressed: `bg-accent` |
| Segmented control option | `text-muted-foreground` | `text-foreground` | `bg-accent` |
| Destructive menu item | `text-destructive` | `bg-destructive/10–15` | — |

## Component patterns

Short specs; the code is the detail. All follow the idiom above.

- **Session row** (`SessionCard`) — borderless. Status-ringed provider glyph + label; mono path with a
  left-fade mask on overflow; branch + PR + diff-stat as text+glyph badges. Active `bg-accent`. Close `×`
  appears on hover only.
- **New session** — a full-width action at the top of the sidebar (dropdown trigger), `bg-accent/55`,
  brightening to `bg-accent` on hover.
- **Project tab** (`ProjectTab`) — text tab, no chip border; inactive tabs badge busy/done/waiting.
- **Diff file** (`FileDiff`) — **borderless section**, not a card. Header is a hover row: chevron, language
  badge, filename, muted dir, diff-stat, ghost actions (attach / discard). Files separated by gap. Added/
  removed lines keep the emerald/red gutter strip — that is meaning.
- **File tree row** (`FilesPanel`) — mono, hover `bg-accent`; chevron + folder/file logo + name.
- **Command palette** — centered dialog on a dimmed backdrop; grouped rows (Sessions / Projects), selected
  `bg-accent`; footer key hints as `<kbd>`.
- **Menu / context menu** — borderless items, hover `bg-accent/50`, hairline separators, destructive in red.
- **Dialog** — hairline-sectioned; body fields with input/select edges; footer buttons right-aligned, one
  primary + ghost cancel.
- **Segmented control** — a bordered track holding ghost options; the chosen option is an `bg-accent` fill.
- **Stepper / numeric field** — icon buttons flanking a bordered value box (`tabular-nums`).
- **Switch** — off `bg-accent`, on `bg-primary`.
- **Toast** (sonner) — popover surface, hairline, semantic glyph.

## Fixed by decision — do not "fix" these

- **Bundled themes stay zinc, no accent hue.** A chromatic palette belongs in a custom theme JSON, not in component
  classes.
- **The layout.** Top tabs · left session sidebar · terminal · right dock · footer. The reskin is skin, not
  structure.
- **Semantic color** and the **diff language badges** — they carry meaning.

## Adoption

The idiom is the standard for all new and touched code. The originally boxed surfaces — the session and
settings cards, the review-panel file diffs, the footer controls, and the filled pill badges — have been
brought into it. Keep new components in line: reach for spacing and a hover state before adding a border, a
`bg-card` panel, or a filled pill.
