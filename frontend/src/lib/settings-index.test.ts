import { readdirSync, readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { describe, expect, it } from "vitest"
import { HOTKEY_ACTIONS } from "./hotkeys"
import {
  SETTING_ENTRIES,
  SETTING_SECTIONS,
  allEntries,
  expandEntries,
  hitPath,
  hotkeyEntries,
  searchSettings,
  sectionLabel,
} from "./settings-index"

const find = (query: string) => searchSettings(query, allEntries())

describe("searching the settings", () => {
  it("finds a control by a word the nav never says", () => {
    const titles = find("ssh").map((entry) => entry.title)

    expect(titles).toContain("SSH agent")
  })

  it("finds both Themes, and the path is what tells them apart", () => {
    const groups = find("theme")
      .filter((entry) => entry.title === "Theme")
      .map((entry) => entry.group)

    expect(groups).toEqual(["Interface", "Terminal"])
  })

  // The words nobody would guess from the title are the point of `also`: the
  // block is called "Spend ceiling" and the thing it caps is a cost.
  it("finds a block by a word only its keywords carry", () => {
    expect(find("cost").map((entry) => entry.title)).toContain("Spend ceiling")
  })

  // Ranking, not filtering: a stray substring must not outrank the block the
  // query names.
  it("puts a title that starts with the query first", () => {
    const [first] = find("bin")

    expect(first?.title).toBe("Binary")
  })

  it("ranks a keyword-only match below every title match", () => {
    const ranked = find("session").map((entry) => entry.title)
    const keywordOnly = ranked.indexOf("Default provider")
    const titleMatch = ranked.indexOf("Notify me when a session needs input")

    expect(titleMatch).toBeGreaterThanOrEqual(0)
    expect(keywordOnly).toBeGreaterThan(titleMatch)
  })

  it("answers nothing for an empty box, and for a word that is nowhere", () => {
    expect(find("")).toEqual([])
    expect(find("   ")).toEqual([])
    expect(find("zzz")).toEqual([])
  })

  // A pane is a result of its own: nothing inside Updates is called "updates",
  // and the row for it is right there in the nav.
  it("finds a section by its own name", () => {
    const hit = find("updates")[0]

    expect(hit?.title).toBe("Updates")
    expect(hit?.section).toBe("updates")
  })

  it("ranks a block above the pane it sits in", () => {
    const titles = find("plan").map((entry) => entry.title)

    expect(titles[0]).toBe("Plan usage")
  })

  it("searches the keyboard shortcuts too, which are not blocks", () => {
    const titles = find("worktree").map((entry) => entry.title)

    expect(titles).toContain("New worktree session")
  })

  it("carries every shortcut into the index without writing any of them out", () => {
    expect(hotkeyEntries()).toHaveLength(HOTKEY_ACTIONS.length)
    expect(allEntries()).toHaveLength(
      SETTING_ENTRIES.length + HOTKEY_ACTIONS.length + SETTING_SECTIONS.length,
    )
  })
})

const PROVIDERS = [
  { id: "claude", name: "Claude Code" },
  { id: "codex", name: "Codex" },
]

describe("the blocks that exist once per provider", () => {
  it("becomes one hit per enabled provider, named after it", () => {
    const hits = expandEntries(PROVIDERS).filter((hit) => hit.title === "Binary")

    expect(hits.map((hit) => hit.group)).toEqual(["Claude Code", "Codex"])
    expect(hits.map((hit) => hit.providerId)).toEqual(["claude", "codex"])
  })

  // Not a stranded result: the block genuinely has nowhere to live when no
  // provider is on, and the pane it would open shows the roster instead.
  it("disappears when no provider is enabled", () => {
    const titles = expandEntries([]).map((hit) => hit.title)

    expect(titles).not.toContain("Binary")
    expect(titles).toContain("Default provider")
  })

  it("leaves every other entry alone", () => {
    expect(expandEntries(PROVIDERS).filter((hit) => hit.title === "Zoom")).toHaveLength(1)
  })

  it("searches the expanded hits, so one query answers for each provider", () => {
    const hits = searchSettings("binary", expandEntries(PROVIDERS))

    expect(hits.map((hit) => hit.providerId)).toEqual(["claude", "codex"])
  })
})

describe("the path over a result", () => {
  it("names a section by the label the nav shows", () => {
    expect(sectionLabel("version-control")).toBe("Version Control")
    expect(sectionLabel("nothing-like-it")).toBe("nothing-like-it")
  })

  it("names the section and the group", () => {
    expect(hitPath({ section: "appearance", group: "Terminal", title: "Font" }, "Appearance")).toBe(
      "Appearance \u203a Terminal",
    )
  })

  it("is the section alone when the pane has no groups", () => {
    expect(hitPath({ section: "help", title: "About" }, "Help")).toBe("Help")
  })
})

// The index is written by hand, so it can fall behind the panes silently: a
// block added without an entry here is a control the search cannot find, and
// nothing else in the app would ever say so. This reads the same literals out
// of the source the panes are written in.
describe("the index against the panes it indexes", () => {
  const paneDir = fileURLToPath(new URL("../components/settings", import.meta.url))

  // Every `<SettingBlock ... title=...>` in the settings components, whether the
  // title is a plain string or a template with the project's name in it. Split
  // rather than one regex: a block's other props hold JSX and arrow functions,
  // so "everything up to the closing angle bracket" is not a thing to match.
  function renderedTitles(): string[] {
    const titles: string[] = []
    for (const file of readdirSync(paneDir).filter((name) => name.endsWith(".tsx"))) {
      const source = readFileSync(`${paneDir}/${file}`, "utf8")
      for (const block of source.split("<SettingBlock").slice(1)) {
        const quoted = block.match(/^[\s\S]{0,400}?title="([^"]+)"/)
        const templated = block.match(/^[\s\S]{0,400}?title=\{`([^`$]+)/)
        const title = quoted?.[1] ?? templated?.[1]
        if (title) {
          titles.push(title.trim())
        }
      }
    }
    return titles
  }

  // Every title prop in the same files, block or not: what the entries are
  // allowed to point at.
  function allTitles(): string[] {
    const titles: string[] = []
    for (const file of readdirSync(paneDir).filter((name) => name.endsWith(".tsx"))) {
      const source = readFileSync(`${paneDir}/${file}`, "utf8")
      for (const match of source.matchAll(/title=(?:"([^"]+)"|\{`([^`$]+))/g)) {
        titles.push((match[1] ?? match[2]).trim())
      }
    }
    return titles
  }

  it("has an entry for every block the panes render", () => {
    const indexed = SETTING_ENTRIES.map((entry) => entry.title.toLowerCase())
    const missing = renderedTitles().filter(
      (title) => !indexed.some((known) => title.toLowerCase().startsWith(known)),
    )

    expect(missing).toEqual([])
  })

  // The other direction, against every title in the panes rather than the block
  // ones: an entry may point at a control nested inside a block (the sandbox
  // grants are two switches under one title), but not at one that is gone.
  it("indexes no control that no longer exists", () => {
    const rendered = allTitles().map((title) => title.toLowerCase())
    const stale = SETTING_ENTRIES.filter(
      (entry) => !rendered.some((title) => title.startsWith(entry.title.toLowerCase())),
    ).map((entry) => entry.title)

    expect(stale).toEqual([])
  })

  it("points every entry at a section the nav actually has", () => {
    const ids = SETTING_SECTIONS.map((section) => section.id)
    const orphans = SETTING_ENTRIES.filter((entry) => !ids.includes(entry.section)).map(
      (entry) => entry.title,
    )

    expect(orphans).toEqual([])
  })

  // Guards the guard: a broken extractor that finds nothing would let both
  // checks above pass against an empty list.
  it("reads the blocks out of the source at all", () => {
    expect(renderedTitles().length).toBeGreaterThan(20)
  })
})
