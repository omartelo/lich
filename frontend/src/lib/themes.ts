import type { ThemeDefinition } from "@/lib/api-types"
import darkTheme from "@/themes/dark.json"
import lightTheme from "@/themes/light.json"

export const SYSTEM_THEME = "system"
export const MATCH_TERMINAL_THEME = "match"
export const DEFAULT_THEME = SYSTEM_THEME
export const DEFAULT_TERMINAL_THEME = MATCH_TERMINAL_THEME
export const LIGHT_THEME_ID = "light"
export const DARK_THEME_ID = "dark"

export type Theme = typeof SYSTEM_THEME | string
export type TerminalTheme = typeof MATCH_TERMINAL_THEME | string
export type ResolvedTheme = ThemeDefinition
type ThemeImportTemplate = Omit<ThemeDefinition, "origin">

const BUNDLED = [lightTheme, darkTheme].map((theme) =>
  normalizeTheme(theme as unknown as ThemeDefinition, "bundled"),
)
const BUNDLED_ORDER = new Map(BUNDLED.map((theme, index) => [theme.id, index]))

export const APP_COLOR_TOKENS = Object.keys(BUNDLED[0].app)
export const BUNDLED_THEMES = mergeThemes(BUNDLED)
export const THEME_TEMPLATE_FILENAME = "lich-theme-template.json"

export function mergeThemes(
  themes: readonly ThemeDefinition[] | null | undefined,
): ThemeDefinition[] {
  const byId = new Map<string, ThemeDefinition>()
  for (const theme of BUNDLED) {
    byId.set(theme.id, normalizeTheme(theme, "bundled"))
  }
  for (const theme of themes ?? []) {
    const normalized = normalizeTheme(theme)
    byId.set(normalized.id, normalized)
  }
  return [...byId.values()].sort((a, b) => {
    if (a.origin !== b.origin) {
      return a.origin === "bundled" ? -1 : 1
    }
    if (a.origin === "bundled") {
      return (BUNDLED_ORDER.get(a.id) ?? 0) - (BUNDLED_ORDER.get(b.id) ?? 0)
    }
    return a.name.localeCompare(b.name)
  })
}

export function resolveTheme(
  selected: Theme,
  themes: readonly ThemeDefinition[],
  systemDark: boolean,
): ThemeDefinition {
  if (selected === SYSTEM_THEME) {
    return findTheme(themes, systemDark ? DARK_THEME_ID : LIGHT_THEME_ID)
  }
  return findTheme(themes, selected)
}

export function resolveTerminalTheme(
  selected: TerminalTheme,
  appTheme: ThemeDefinition,
  themes: readonly ThemeDefinition[],
): ThemeDefinition {
  if (selected === MATCH_TERMINAL_THEME) {
    return appTheme
  }
  return findTheme(themes, selected)
}

export function applyAppTheme(theme: ThemeDefinition, root: HTMLElement): void {
  for (const token of APP_COLOR_TOKENS) {
    root.style.setProperty(`--${token}`, theme.app[token])
  }
}

export function customThemes(themes: readonly ThemeDefinition[]): ThemeDefinition[] {
  return themes.filter((theme) => theme.origin === "custom")
}

export function themeTemplateJSON(): string {
  const base = BUNDLED[0]
  const template = {
    id: "my-theme",
    name: "My Theme",
    scheme: base.scheme,
    app: { ...base.app },
    terminal: { ...base.terminal },
  } satisfies ThemeImportTemplate
  return `${JSON.stringify(template, null, 2)}\n`
}

export function sanitizeThemePreference(raw: string | null): Theme {
  return safePreference(raw) ?? DEFAULT_THEME
}

export function sanitizeTerminalThemePreference(raw: string | null): TerminalTheme {
  return safePreference(raw) ?? DEFAULT_TERMINAL_THEME
}

function findTheme(themes: readonly ThemeDefinition[], id: string): ThemeDefinition {
  return (
    themes.find((theme) => theme.id === id) ??
    themes.find((theme) => theme.id === LIGHT_THEME_ID) ??
    normalizeTheme(lightTheme as unknown as ThemeDefinition, "bundled")
  )
}

function safePreference(raw: string | null): string | null {
  if (!raw || raw.length > 64) {
    return null
  }
  return /^[a-z0-9][a-z0-9._-]{0,63}$/.test(raw) ? raw : null
}

function normalizeTheme(
  theme: ThemeDefinition,
  origin: ThemeDefinition["origin"] = theme.origin,
): ThemeDefinition {
  return {
    id: theme.id,
    name: theme.name,
    scheme: theme.scheme,
    origin,
    app: { ...theme.app },
    terminal: { ...theme.terminal },
  }
}
