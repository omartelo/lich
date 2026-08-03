import { describe, expect, it } from "vitest"
import type { ThemeDefinition } from "./api-types"
import {
  APP_COLOR_TOKENS,
  applyAppTheme,
  BUNDLED_THEMES,
  bundledThemes,
  customThemes,
  DEFAULT_TERMINAL_THEME,
  DEFAULT_THEME,
  mergeImportedTheme,
  mergeThemes,
  reconcileThemeSelections,
  resolveTerminalTheme,
  resolveTheme,
  sanitizeTerminalThemePreference,
  sanitizeThemePreference,
  selectionsAfterThemeRemoval,
  SYSTEM_THEME,
  THEME_ID_MAX_LENGTH,
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

  it("returns only bundled themes", () => {
    const themes = mergeThemes([customTheme("custom")])
    expect(bundledThemes(themes).map((theme) => theme.id)).toEqual(["light", "dark"])
  })

  it("merges an imported theme by id", () => {
    const original = customTheme("custom")
    const replacement = { ...original, name: "Replacement" }
    const themes = mergeImportedTheme(mergeThemes([original]), replacement)
    expect(themes).toHaveLength(3)
    expect(themes[2].name).toBe("Replacement")
  })

  it("resolves system to the bundled light or dark theme", () => {
    expect(resolveTheme(SYSTEM_THEME, BUNDLED_THEMES, false).id).toBe("light")
    expect(resolveTheme(SYSTEM_THEME, BUNDLED_THEMES, true).id).toBe("dark")
  })

  it("falls back to the system scheme when a stored custom theme is missing", () => {
    expect(resolveTheme("missing", BUNDLED_THEMES, false).id).toBe("light")
    expect(resolveTheme("missing", BUNDLED_THEMES, true).id).toBe("dark")
  })

  it("resolves terminal match to the app theme", () => {
    const appTheme = customTheme("custom")
    expect(resolveTerminalTheme("match", appTheme, BUNDLED_THEMES)).toBe(appTheme)
    expect(resolveTerminalTheme("dark", appTheme, BUNDLED_THEMES).id).toBe("dark")
    expect(resolveTerminalTheme("missing", appTheme, BUNDLED_THEMES)).toBe(appTheme)
  })

  it("reconciles missing stored selections after themes load", () => {
    expect(
      reconcileThemeSelections(
        { theme: "missing-app", terminalTheme: "missing-terminal" },
        BUNDLED_THEMES,
      ),
    ).toEqual({ theme: DEFAULT_THEME, terminalTheme: DEFAULT_TERMINAL_THEME })
    expect(
      reconcileThemeSelections({ theme: "dark", terminalTheme: "light" }, BUNDLED_THEMES),
    ).toEqual({ theme: "dark", terminalTheme: "light" })
  })

  it("resets only selections that use a removed theme", () => {
    expect(
      selectionsAfterThemeRemoval("custom", {
        theme: "custom",
        terminalTheme: "custom",
      }),
    ).toEqual({ theme: DEFAULT_THEME, terminalTheme: DEFAULT_TERMINAL_THEME })
    expect(
      selectionsAfterThemeRemoval("other", { theme: "custom", terminalTheme: "dark" }),
    ).toEqual({ theme: "custom", terminalTheme: "dark" })
  })

  it("sanitizes stored theme preferences", () => {
    expect(sanitizeThemePreference("custom-theme")).toBe("custom-theme")
    expect(sanitizeThemePreference("System")).toBe("system")
    expect(sanitizeTerminalThemePreference("match")).toBe("match")
    expect(sanitizeTerminalThemePreference("bad value")).toBe("match")
    expect(sanitizeThemePreference("a".repeat(THEME_ID_MAX_LENGTH))).toHaveLength(
      THEME_ID_MAX_LENGTH,
    )
    expect(sanitizeThemePreference("a".repeat(THEME_ID_MAX_LENGTH + 1))).toBe(DEFAULT_THEME)
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
    expect(template.id).toBe("purple-night")
    expect(template.name).toBe("Purple Night")
    expect(template.scheme).toBe("dark")
    expect(template.origin).toBeUndefined()
    expect(Object.keys(template.app ?? {}).sort()).toEqual([...APP_COLOR_TOKENS].sort())
    expect(template.terminal?.background).toBeTruthy()
    expect(template.terminal?.foreground).toBeTruthy()
    expect(Object.values({ ...template.app, ...template.terminal }).every(isHexColor)).toBe(true)
  })
})

function isHexColor(value: unknown): boolean {
  return typeof value === "string" && /^#[0-9a-fA-F]{3,8}$/.test(value)
}
