import { describe, expect, it } from "vitest"
import type { ThemeDefinition } from "./api-types"
import {
  adoptStoredTheme,
  APP_COLOR_TOKENS,
  applyAppTheme,
  BUNDLED_THEMES,
  bundledThemes,
  customThemes,
  DEFAULT_THEME,
  mergeImportedThemes,
  mergeThemes,
  reconcileTheme,
  repoLabel,
  resolveTheme,
  themeAfterRemoval,
  SYSTEM_THEME,
  THEME_TEMPLATE_FILENAME,
  themeSelectItems,
} from "./themes"

// The name is a parameter because mergeThemes orders custom themes by it: a
// fixture that gave every theme the same name made that comparator a no-op and
// left the picker's order unasserted.
function customTheme(id: string, name = "Custom"): ThemeDefinition {
  const base = BUNDLED_THEMES[0]
  return {
    ...base,
    id,
    name,
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

  // Ids deliberately in the opposite order to the names: only an order taken
  // from the name can produce this one, which is what the picker shows.
  it("orders custom themes by name, after the bundled pair", () => {
    const themes = mergeThemes([customTheme("aaa", "Zinc Night"), customTheme("zzz", "Amber Dusk")])
    expect(themes.map((theme) => theme.id)).toEqual(["light", "dark", "zzz", "aaa"])
  })

  it("returns only bundled themes", () => {
    const themes = mergeThemes([customTheme("custom")])
    expect(bundledThemes(themes).map((theme) => theme.id)).toEqual(["light", "dark"])
  })

  it("merges an imported theme by id", () => {
    const original = customTheme("custom")
    const replacement = { ...original, name: "Replacement" }
    const themes = mergeImportedThemes(mergeThemes([original]), [replacement])
    expect(themes).toHaveLength(3)
    expect(themes[2].name).toBe("Replacement")
  })

  it("merges a whole installed pack at once", () => {
    const themes = mergeImportedThemes(BUNDLED_THEMES, [
      customTheme("pack-a"),
      customTheme("pack-b"),
    ])
    expect(themes.map((theme) => theme.id)).toEqual(["light", "dark", "pack-a", "pack-b"])
  })

  it("keeps the repository source of an installed theme", () => {
    const installed = {
      ...customTheme("installed"),
      source: { url: "https://github.com/you/lich-themes.git", version: "1.2.0" },
    }
    const merged = mergeThemes([installed])
    expect(merged[2].source).toEqual(installed.source)
    // The merge must copy the source, not alias the caller's object.
    expect(merged[2].source).not.toBe(installed.source)
    expect(mergeThemes([customTheme("plain")])[2].source).toBeUndefined()
  })

  it("shortens a clone URL to owner/repo", () => {
    expect(repoLabel("https://github.com/you/lich-themes.git")).toBe("you/lich-themes")
    expect(repoLabel("git@github.com:you/lich-themes.git")).toBe("you/lich-themes")
    expect(repoLabel("https://gitlab.com/team/group/themes/")).toBe("group/themes")
    expect(repoLabel("/home/you/themes")).toBe("you/themes")
  })

  it("resolves system to the bundled light or dark theme", () => {
    expect(resolveTheme(SYSTEM_THEME, BUNDLED_THEMES, false).id).toBe("light")
    expect(resolveTheme(SYSTEM_THEME, BUNDLED_THEMES, true).id).toBe("dark")
  })

  it("falls back to the system scheme when a stored custom theme is missing", () => {
    expect(resolveTheme("missing", BUNDLED_THEMES, false).id).toBe("light")
    expect(resolveTheme("missing", BUNDLED_THEMES, true).id).toBe("dark")
  })

  // One selection colors both surfaces, so the palette the terminal paints is
  // the resolved theme's own, with nothing left to resolve separately.
  it("carries a terminal palette on the theme the app resolved to", () => {
    expect(resolveTheme("dark", BUNDLED_THEMES, false).terminal.background).toBeTruthy()
    expect(resolveTheme(SYSTEM_THEME, BUNDLED_THEMES, true).terminal.foreground).toBeTruthy()
  })

  it("reconciles a missing stored selection after themes load", () => {
    expect(reconcileTheme("missing-theme", BUNDLED_THEMES)).toBe(DEFAULT_THEME)
    expect(reconcileTheme("dark", BUNDLED_THEMES)).toBe("dark")
    expect(reconcileTheme(SYSTEM_THEME, BUNDLED_THEMES)).toBe(SYSTEM_THEME)
  })

  it("prefers the stored selection over the boot cache", () => {
    expect(adoptStoredTheme("dracula", SYSTEM_THEME)).toEqual({
      theme: "dracula",
      persist: false,
    })
  })

  it("adopts the boot cache and asks for a write-back when nothing is stored", () => {
    expect(adoptStoredTheme("", "dracula")).toEqual({ theme: "dracula", persist: true })
  })

  it("resets the selection only when the removed theme is the one in use", () => {
    expect(themeAfterRemoval("custom", "custom")).toBe(DEFAULT_THEME)
    expect(themeAfterRemoval("other", "custom")).toBe("custom")
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

  it("maps every selectable value to the label the trigger shows", () => {
    const themes = mergeThemes([customTheme("purple-night")])
    const items = themeSelectItems(themes, SYSTEM_THEME, "System")

    expect(items[SYSTEM_THEME]).toBe("System")
    expect(items.light).toBe("Light")
    expect(items["purple-night"]).toBe("Custom")
    expect(Object.keys(items)).toHaveLength(themes.length + 1)
  })

  // The template itself is a Go asset now — internal/themes proves it parses,
  // carries every token and survives the real validator. All that is left here
  // is the name the save dialog is seeded with.
  it("names the template file the save dialog offers", () => {
    expect(THEME_TEMPLATE_FILENAME).toBe("lich-theme-template.json")
  })
})
