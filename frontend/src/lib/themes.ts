import type { ThemeDefinition } from "@/lib/api-types"
import darkTheme from "../../../themes/dark.json"
import lightTheme from "../../../themes/light.json"

export const SYSTEM_THEME = "system"
export const MATCH_TERMINAL_THEME = "match"
export const DEFAULT_THEME = SYSTEM_THEME
export const DEFAULT_TERMINAL_THEME = MATCH_TERMINAL_THEME
export const LIGHT_THEME_ID = "light"
export const DARK_THEME_ID = "dark"
export const BUNDLED_THEME_ORIGIN = "bundled"
export const CUSTOM_THEME_ORIGIN = "custom"
export const DARK_THEME_SCHEME = "dark"
export const THEME_TEMPLATE_FILENAME = "lich-theme-template.json"

export type Theme = typeof SYSTEM_THEME | string
export type TerminalTheme = typeof MATCH_TERMINAL_THEME | string
export type ResolvedTheme = ThemeDefinition

const BUNDLED = [lightTheme, darkTheme].map((theme) =>
  normalizeTheme(theme as unknown as ThemeDefinition),
)
const BUNDLED_ORDER = new Map(BUNDLED.map((theme, index) => [theme.id, index]))

export const APP_COLOR_TOKENS = Object.keys(BUNDLED[0].app)
export const BUNDLED_THEMES = mergeThemes(BUNDLED)

export function mergeThemes(
  themes: readonly ThemeDefinition[] | null | undefined,
): ThemeDefinition[] {
  const byId = new Map<string, ThemeDefinition>()
  for (const theme of BUNDLED) {
    byId.set(theme.id, normalizeTheme(theme))
  }
  for (const theme of themes ?? []) {
    const normalized = normalizeTheme(theme)
    byId.set(normalized.id, normalized)
  }
  return [...byId.values()].sort((a, b) => {
    if (a.origin !== b.origin) {
      return a.origin === BUNDLED_THEME_ORIGIN ? -1 : 1
    }
    if (a.origin === BUNDLED_THEME_ORIGIN) {
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
  const systemID = systemDark ? DARK_THEME_ID : LIGHT_THEME_ID
  // mergeThemes always seeds the bundled pair, so the last fallback only
  // covers a caller that passed a list which never went through it.
  const systemTheme = themes.find((theme) => theme.id === systemID) ?? BUNDLED[0]
  if (selected === SYSTEM_THEME) {
    return systemTheme
  }
  return themes.find((theme) => theme.id === selected) ?? systemTheme
}

export function resolveTerminalTheme(
  selected: TerminalTheme,
  appTheme: ThemeDefinition,
  themes: readonly ThemeDefinition[],
): ThemeDefinition {
  if (selected === MATCH_TERMINAL_THEME) {
    return appTheme
  }
  return themes.find((theme) => theme.id === selected) ?? appTheme
}

export function applyAppTheme(theme: ThemeDefinition, root: HTMLElement): void {
  for (const token of APP_COLOR_TOKENS) {
    root.style.setProperty(`--${token}`, theme.app[token])
  }
}

export function customThemes(themes: readonly ThemeDefinition[]): ThemeDefinition[] {
  return themes.filter((theme) => theme.origin === CUSTOM_THEME_ORIGIN)
}

export function bundledThemes(themes: readonly ThemeDefinition[]): ThemeDefinition[] {
  return themes.filter((theme) => theme.origin === BUNDLED_THEME_ORIGIN)
}

// A Select trigger renders the raw value unless the Root is handed the
// value→label map: the items only register themselves once the popup has been
// opened, so without this the closed trigger reads "system", not "System".
export function themeSelectItems(
  themes: readonly ThemeDefinition[],
  automaticID: string,
  automaticLabel: string,
): Record<string, string> {
  const items: Record<string, string> = { [automaticID]: automaticLabel }
  for (const theme of themes) {
    items[theme.id] = theme.name
  }
  return items
}

export function mergeImportedThemes(
  themes: readonly ThemeDefinition[],
  imported: readonly ThemeDefinition[],
): ThemeDefinition[] {
  return mergeThemes([...themes, ...imported])
}

// repoLabel shortens a clone URL to the "owner/repo" a user recognizes; the
// full URL is kept for the tooltip, since only the tail fits a theme row.
export function repoLabel(url: string): string {
  const trimmed = url
    .trim()
    .replace(/\.git$/, "")
    .replace(/[/\\]+$/, "")
  const segments = trimmed.split(/[/\\:]+/).filter(Boolean)
  return segments.slice(-2).join("/") || trimmed
}

export interface ThemeSelections {
  theme: Theme
  terminalTheme: TerminalTheme
}

export function reconcileThemeSelections(
  selections: ThemeSelections,
  themes: readonly ThemeDefinition[],
): ThemeSelections {
  const ids = new Set(themes.map((item) => item.id))
  return {
    theme:
      selections.theme === SYSTEM_THEME || ids.has(selections.theme)
        ? selections.theme
        : DEFAULT_THEME,
    terminalTheme:
      selections.terminalTheme === MATCH_TERMINAL_THEME || ids.has(selections.terminalTheme)
        ? selections.terminalTheme
        : DEFAULT_TERMINAL_THEME,
  }
}

// adoptStoredSelections resolves the selections a launch starts from against
// the workspace copy, which is the durable one: the boot cache lives in the
// Chromium profile, and a profile Chromium recreates comes back empty while the
// workspace database survives. A setting that was never written reads as "" —
// an install whose cache is still the only record — so the cache is adopted and
// written back once, which is what `persist` reports.
export function adoptStoredSelections(
  stored: ThemeSelections,
  cached: ThemeSelections,
): { selections: ThemeSelections; persist: boolean } {
  return {
    selections: {
      theme: stored.theme || cached.theme,
      terminalTheme: stored.terminalTheme || cached.terminalTheme,
    },
    persist: !stored.theme || !stored.terminalTheme,
  }
}

export function selectionsAfterThemeRemoval(
  removedID: string,
  selections: ThemeSelections,
): ThemeSelections {
  return {
    theme: selections.theme === removedID ? DEFAULT_THEME : selections.theme,
    terminalTheme:
      selections.terminalTheme === removedID ? DEFAULT_TERMINAL_THEME : selections.terminalTheme,
  }
}

function normalizeTheme(theme: ThemeDefinition): ThemeDefinition {
  return {
    id: theme.id,
    name: theme.name,
    scheme: theme.scheme,
    origin: theme.origin,
    ...(theme.source ? { source: { ...theme.source } } : {}),
    app: { ...theme.app },
    terminal: { ...theme.terminal },
  }
}
