import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import {
  getLocale,
  LOCALES,
  matchLocale,
  resetLocale,
  setLocale,
  subscribeLocale,
  t,
  UI_LANGUAGE_PREF,
} from "./i18n"
import { en } from "./locales/en"
import { es } from "./locales/es"
import { ptBR } from "./locales/pt-br"

function stubStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial))
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => data.set(key, value),
    removeItem: (key: string) => data.delete(key),
  })
  return data
}

beforeEach(() => resetLocale())
afterEach(() => {
  vi.unstubAllGlobals()
  resetLocale()
})

describe("the interface language", () => {
  it("is English with no storage and no browser language", () => {
    vi.stubGlobal("navigator", undefined)
    expect(getLocale()).toBe("en")
  })

  it("follows the browser language until someone picks one", () => {
    stubStorage()
    vi.stubGlobal("navigator", { language: "pt-PT" })
    expect(getLocale()).toBe("pt-BR")
  })

  it("keeps a stored choice over the browser language", () => {
    stubStorage({ [UI_LANGUAGE_PREF]: "en" })
    vi.stubGlobal("navigator", { language: "pt-BR" })
    expect(getLocale()).toBe("en")
  })

  it("persists a choice and tells every subscriber", () => {
    const data = stubStorage()
    const listener = vi.fn()
    subscribeLocale(listener)

    setLocale("pt-BR")

    expect(data.get(UI_LANGUAGE_PREF)).toBe("pt-BR")
    expect(listener).toHaveBeenCalledTimes(1)
    expect(t("common.action.cancel")).toBe("Cancelar")
  })

  it("does not wake subscribers for the language already showing", () => {
    stubStorage({ [UI_LANGUAGE_PREF]: "en" })
    const listener = vi.fn()
    subscribeLocale(listener)
    getLocale()

    setLocale("en")

    expect(listener).not.toHaveBeenCalled()
  })
})

describe("matchLocale", () => {
  it("matches exactly, then by language, then falls back to English", () => {
    expect(matchLocale("pt-BR")).toBe("pt-BR")
    expect(matchLocale("pt-br")).toBe("pt-BR")
    expect(matchLocale("pt")).toBe("pt-BR")
    expect(matchLocale("en-GB")).toBe("en")
    expect(matchLocale("fr-FR")).toBe("en")
    expect(matchLocale("")).toBe("en")
    expect(matchLocale(null)).toBe("en")
  })
})

describe("t", () => {
  it("fills placeholders by name", () => {
    expect(t("settings.language.promptSaveFailed", { error: "boom" })).toBe(
      "Could not save the prompt language: boom",
    )
  })

  it("picks the plural form each language's rules select", () => {
    expect(t("common.count.file", { count: 0 })).toBe("0 files")
    expect(t("common.count.file", { count: 1 })).toBe("1 file")
    expect(t("common.count.file", { count: 2 })).toBe("2 files")
    stubStorage()
    setLocale("pt-BR")
    // Portuguese counts zero as singular.
    expect(t("common.count.file", { count: 0 })).toBe("0 arquivo")
    expect(t("common.count.file", { count: 1 })).toBe("1 arquivo")
    expect(t("common.count.file", { count: 2 })).toBe("2 arquivos")
  })

  it("rejects misuse at compile time", () => {
    // @ts-expect-error a key that does not exist
    expect(() => t("sidebar.nothing.here")).toThrow()
    // @ts-expect-error a placeholder left unfilled
    expect(() => t("settings.language.promptSaveFailed")).toThrow()
    // @ts-expect-error a plural with no count
    expect(() => t("common.count.file")).toThrow()
    // @ts-expect-error params for a message that takes none
    t("common.action.cancel", { error: "x" })
  })
})

type Tree = { [key: string]: Tree | string }

function leaves(tree: Tree, prefix = ""): Map<string, string> {
  const found = new Map<string, string>()
  for (const [key, value] of Object.entries(tree)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === "string") {
      found.set(path, value)
    } else {
      for (const [leaf, text] of leaves(value, path)) found.set(leaf, text)
    }
  }
  return found
}

const placeholders = (text: string) => [...text.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort()

// tsc already fails on a missing or extra key; this is what it cannot see: a
// translation that renames, drops or invents a placeholder.
describe("every locale against English", () => {
  const source = leaves(en as unknown as Tree)
  const locales = { "pt-BR": ptBR, es }

  it("covers every locale the picker offers", () => {
    expect(Object.keys(locales)).toEqual(LOCALES.filter((locale) => locale !== "en"))
  })

  for (const [locale, catalog] of Object.entries(locales)) {
    it(`${locale} uses the placeholders English uses`, () => {
      const translated = leaves(catalog as unknown as Tree)
      const mismatched: string[] = []
      for (const [key, text] of translated) {
        const english = source.get(key) ?? source.get(key.replace(/\.\w+$/, ".other"))
        const allowed = new Set(placeholders(english ?? "").concat("count"))
        if (english === undefined || placeholders(text).some((name) => !allowed.has(name))) {
          mismatched.push(key)
        }
      }
      for (const [key, text] of source) {
        const needed = placeholders(text).filter((name) => name !== "count")
        const mine = translated.get(key) ?? translated.get(key.replace(/\.\w+$/, ".other"))
        if (mine === undefined || needed.some((name) => !placeholders(mine).includes(name))) {
          mismatched.push(key)
        }
      }
      expect(mismatched).toEqual([])
    })
  }
})

describe("Spanish", () => {
  it("is what every Spanish browser language resolves to", () => {
    for (const tag of ["es", "es-ES", "es-MX", "es-419", "ES-ar"]) {
      expect(matchLocale(tag)).toBe("es")
    }
  })

  it("renders messages, with the plural rule Spanish uses", () => {
    stubStorage()
    setLocale("es")
    expect(t("common.action.cancel")).toBe("Cancelar")
    expect(t("settings.language.promptSaveFailed", { error: "boom" })).toBe(
      "No se pudo guardar el idioma de los prompts: boom",
    )
    // Spanish, like English, counts zero as plural.
    expect(t("common.count.file", { count: 0 })).toBe("0 archivos")
    expect(t("common.count.file", { count: 1 })).toBe("1 archivo")
    expect(t("common.count.file", { count: 2 })).toBe("2 archivos")
  })
})
