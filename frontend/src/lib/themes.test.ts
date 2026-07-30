import { describe, expect, it } from "vitest"
import type { ThemeDefinition } from "./api-types"
import {
  APP_COLOR_TOKENS,
  applyAppTheme,
  BUNDLED_THEMES,
  customThemes,
  mergeThemes,
  resolveTerminalTheme,
  resolveTheme,
  sanitizeTerminalThemePreference,
  sanitizeThemePreference,
  THEME_TEMPLATE_FILENAME,
  themeTemplateJSON,
} from "./themes"

function customTheme(id: string): ThemeDefinition {
  const base = BUNDLED_THEMES[0]
  return {
    ...base,
    id,
    name: "Custom",
    origin: "custom",
    app: { ...base.app, background: "#101010" },
    terminal: { ...base.terminal, background: "#111111" },
  }
}

describe("themes", () => {
  it("keeps bundled themes first and merges custom themes", () => {
    const themes = mergeThemes([customTheme("custom")])
    expect(themes.map((theme) => theme.id)).toEqual(["light", "dark", "custom"])
  })

  it("returns only imported custom themes", () => {
    const themes = mergeThemes([customTheme("custom-a"), customTheme("custom-b")])
    expect(customThemes(themes).map((theme) => theme.id)).toEqual(["custom-a", "custom-b"])
  })

  it("resolves system to the bundled light or dark theme", () => {
    expect(resolveTheme("system", BUNDLED_THEMES, false).id).toBe("light")
    expect(resolveTheme("system", BUNDLED_THEMES, true).id).toBe("dark")
  })

  it("falls back to light when a stored custom theme is missing", () => {
    expect(resolveTheme("missing", BUNDLED_THEMES, false).id).toBe("light")
  })

  it("resolves terminal match to the app theme", () => {
    const appTheme = customTheme("custom")
    expect(resolveTerminalTheme("match", appTheme, BUNDLED_THEMES)).toBe(appTheme)
    expect(resolveTerminalTheme("dark", appTheme, BUNDLED_THEMES).id).toBe("dark")
  })

  it("sanitizes stored theme preferences", () => {
    expect(sanitizeThemePreference("custom-theme")).toBe("custom-theme")
    expect(sanitizeThemePreference("System")).toBe("system")
    expect(sanitizeTerminalThemePreference("match")).toBe("match")
    expect(sanitizeTerminalThemePreference("bad value")).toBe("match")
  })

  it("applies every app token as a CSS variable", () => {
    const values = new Map<string, string>()
    const root = {
      style: {
        setProperty: (key: string, value: string) => values.set(key, value),
      },
    } as unknown as HTMLElement
    applyAppTheme(BUNDLED_THEMES[0], root)
    for (const token of APP_COLOR_TOKENS) {
      expect(values.get(`--${token}`)).toBe(BUNDLED_THEMES[0].app[token])
    }
  })

  it("keeps bundled themes on the same app token set", () => {
    const tokens = [...APP_COLOR_TOKENS].sort()
    for (const theme of BUNDLED_THEMES) {
      expect(Object.keys(theme.app).sort()).toEqual(tokens)
    }
  })

  it("builds a valid import template", () => {
    const template = JSON.parse(themeTemplateJSON()) as Partial<ThemeDefinition>

    expect(THEME_TEMPLATE_FILENAME).toBe("lich-theme-template.json")
    expect(template.id).toBe("my-theme")
    expect(template.name).toBe("My Theme")
    expect(template.scheme).toBe("light")
    expect(template.origin).toBeUndefined()
    expect(Object.keys(template.app ?? {}).sort()).toEqual([...APP_COLOR_TOKENS].sort())
    expect(template.terminal?.background).toBeTruthy()
    expect(template.terminal?.foreground).toBeTruthy()
  })
})
