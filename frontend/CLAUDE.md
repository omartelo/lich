# lich frontend

Extends the repo root `CLAUDE.md`. Rules here are frontend-specific and override the root where they conflict.
The visual system is `DESIGN.md` — read it before touching UI.

## What it is

React 18 + TypeScript + Vite, built to a static bundle that the Go binary embeds and serves to a Chromium
`--app` window. There is no dev server in production; the backend is the only origin.

- **Service shapes are hand-owned.** `src/lib/api-types.ts` mirrors the Go structs' JSON tags. Change a Go
  struct → update this file in the same change. There is no codegen.
- **Terminal** is xterm.js 6 + WebGL. **Code / diff** is CodeMirror 6. **Icons** are lucide; file-type logos
  are `devicons-react`. **Toasts** are sonner. **DnD** is dnd-kit.

## Stack specifics

- **Tailwind v4, CSS-first.** No `tailwind.config.*` — tokens and `@theme` live in `src/index.css`, wired via
  `@tailwindcss/vite`. Add a design token there, never inline a raw color in a component.
- **shadcn v4** (`components.json`: style `base-vega`, base color zinc, cssVariables). Primitives under
  `@/components/ui` are built on **base-ui** (`@base-ui/react`), not Radix.
- **`cn`** from `@/lib/utils` (`clsx` + `tailwind-merge`) is the only class-merging helper. `@` aliases `src/`.

## Commands

`pnpm` only — `npm i` errors out (see root memory). From `frontend/`:

```bash
pnpm build        # tsc typecheck + vite build (the real gate)
pnpm test         # vitest run
pnpm check        # biome format + lint check (CI runs this; errors fail)
pnpm format       # biome, writing the fixes
```

Wart: `pnpm test`/`build` can trip an implicit install that dies on `ERR_PNPM_IGNORED_BUILDS`. When it does,
run the binaries directly: `./node_modules/.bin/vitest run` or `pnpm exec vitest run`.

## Formatting and lint — biome

**biome** (`biome.jsonc`) is the frontend's gofmt + vet, in one binary. There is no prettier and no eslint;
`eslint-disable` comments are dead here — the directive is `biome-ignore <rule>: <reason>`.

- 2-space indent, double quotes, no semicolons, `lineWidth` 100 (measured as the least-churn width over the
  source at adoption).
- A few recommended rules are off on purpose, each with its reason in `biome.jsonc` — read it before turning
  one back on. Import order is deliberately not enforced.
- Warnings do not fail the gate; errors do. The standing warning backlog is the a11y one: `role="button"`
  spans that take a click but no key, and the `role="separator"` resize handles.

## State — no state library

zustand and tanstack-query were both rejected (PR #13). The pattern is a **module-level store +
`useSyncExternalStore`**; see any `src/lib/*-store.ts`. Backend→frontend events are **global** payloads
`{ id, ... }` on the events channel, never per-session sockets — subscribe once, filter by id. Do not add a
state dep; extend the store pattern.

## Testing

- vitest runs in the **node environment — there is no jsdom**. The gate covers pure logic (stores, parsers,
  reducers, `lib/*`); it does **not** render components. A base-ui/DOM render crash passes the suite green.
- So: verify any render-path change by hand in `task dev`, and check base-ui contracts (below). Assert the
  empty/default/error branch of a component's data, not just the happy path.
- Coverage bar is 80% on the logic that *is* testable. Don't chase coverage by rendering in node.

## Adapting a shadcn component to lich

Never hand-write a `ui/` primitive — add it with the CLI (the `shadcn` skill), then **adapt it to `DESIGN.md`
before use**. What the CLI drops is stock shadcn: bordered boxes, `bg-card`, `rounded-lg`, filled pill badges.
Bring it into the idiom:

1. **Strip enclosure.** Remove `border` / `bg-card` from list items and cards → borderless rows. Keep a
   hairline `border` only on inputs, selects, numeric fields, and structural panel seams.
2. **State is fill.** rest transparent · hover `bg-accent/50–60` · active `bg-accent text-accent-foreground`.
   No left bar, no selection ring, no shadow.
3. **Badges → text + glyph.** Delete filled pill capsules (`bg-*/10 rounded-full`); use a lucide glyph + text.
   Chromatic color only if it is semantic (state / language).
4. **Buttons.** Default to `ghost` in chrome; reserve one `primary` per surface. Keep the `Button` variant
   scale (primary / secondary / outline / ghost / destructive) — don't invent new ones.
5. **Radius from the token.** Lean on `--radius`; small controls `rounded-md`. Don't paper `rounded-lg` over
   everything.
6. **Tokens only.** No raw hex/oklch; route through the `--*` variables and Tailwind opacity steps.

### base-ui gotchas

- **`forwardRef` is load-bearing.** base-ui composition (`render={<Button/>}` on `Menu.Trigger`,
  `Dialog.Close`, etc.) injects a ref; a plain function component silently drops it and the trigger never
  registers. `ui/button.tsx` already forwards — mirror it in any custom trigger.
- **Menu parts need their wrappers.** e.g. `Menu.GroupLabel` requires a surrounding `Menu.Group`. Because the
  gate can't render, a broken composition ships green — smoke it in `task dev`.

## Conventions

- **English** for all code, comments, docs and identifiers. Comments explain *why*, only for a non-obvious
  rule or behaviour — never narrate readable code.
- Small focused functions (< 50 lines), cohesive files (200–400 lines, 800 max), no nesting past 4 levels.
- Props typed with a named `interface`; no `React.FC`; avoid `any` (`unknown` + narrow at boundaries).
