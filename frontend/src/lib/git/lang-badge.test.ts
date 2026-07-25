import { describe, expect, it } from "vitest"
import { languageAbbr, splitPath } from "./lang-badge"

describe("languageAbbr", () => {
  it("maps known extensions", () => {
    expect(languageAbbr("src/App.tsx").abbr).toBe("TS")
    expect(languageAbbr("internal/project/difftext.go").abbr).toBe("GO")
    expect(languageAbbr("index.css").abbr).toBe("CSS")
  })

  it("uppercases unknown extensions", () => {
    const badge = languageAbbr("build/config.kdl")
    expect(badge.abbr).toBe("KDL")
  })

  it("truncates long unknown extensions to three chars", () => {
    expect(languageAbbr("a.gradle").abbr).toBe("GRA")
  })

  it("falls back to a dot for extensionless files", () => {
    expect(languageAbbr("Taskfile").abbr).toBe("•")
  })

  it("treats dotfiles as extensionless", () => {
    expect(languageAbbr(".gitignore").abbr).toBe("•")
  })
})

describe("splitPath", () => {
  it("splits directory and basename", () => {
    expect(splitPath("src/components/App.tsx")).toEqual({
      dir: "src/components",
      base: "App.tsx",
    })
  })

  it("handles paths without a directory", () => {
    expect(splitPath("main.go")).toEqual({ dir: "", base: "main.go" })
  })
})
