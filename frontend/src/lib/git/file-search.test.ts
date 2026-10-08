import { describe, expect, it } from "vitest"
import type { SearchResult } from "@/lib/api-types"
import { groupMatches, highlightSegments, searchFootnote, searchSummary } from "./file-search"

const hit = (path: string, line: number) => ({ path, line, text: "x" })

function result(matches: number, cut = false, tooLarge = 0): SearchResult {
  return { matches: Array.from({ length: matches }, (_, i) => hit("a", i + 1)), cut, tooLarge }
}

describe("groupMatches", () => {
  it("keeps the backend's order, one group per file", () => {
    const groups = groupMatches([hit("a.ts", 1), hit("a.ts", 9), hit("b.go", 2)])
    expect(groups.map((g) => [g.path, g.hits.map((h) => h.line)])).toEqual([
      ["a.ts", [1, 9]],
      ["b.go", [2]],
    ])
  })

  it("answers nothing for nothing", () => {
    expect(groupMatches([])).toEqual([])
  })
})

describe("highlightSegments", () => {
  it("marks every occurrence, whatever its case, and keeps the text as written", () => {
    expect(highlightSegments("Foo bar FOO", "foo")).toEqual([
      { text: "Foo", hit: true },
      { text: " bar ", hit: false },
      { text: "FOO", hit: true },
    ])
  })

  it("treats the query as text, not a pattern", () => {
    expect(highlightSegments("a.b axb", "a.b")).toEqual([
      { text: "a.b", hit: true },
      { text: " axb", hit: false },
    ])
  })

  it("shows a line plain when the query is not in it or is empty", () => {
    expect(highlightSegments("abc", "zzz")).toEqual([{ text: "abc", hit: false }])
    expect(highlightSegments("abc", "")).toEqual([{ text: "abc", hit: false }])
  })

  it("shows a line plain when lowercasing moves its offsets", () => {
    expect(highlightSegments("İstanbul", "stan")).toEqual([{ text: "İstanbul", hit: false }])
  })
})

describe("searchSummary", () => {
  it("counts lines and files, singular and plural", () => {
    expect(searchSummary(result(1), 1)).toBe("1 match in 1 file")
    expect(searchSummary(result(3), 2)).toBe("3 matches in 2 files")
  })

  it("says a cut answer is a floor, without a file count it cannot know", () => {
    expect(searchSummary(result(500, true), 7)).toBe("500+ matches")
  })
})

describe("searchFootnote", () => {
  it("is empty for an answer that read everything", () => {
    expect(searchFootnote(result(3))).toBe("")
  })

  it("names the cap and the files skipped for size", () => {
    expect(searchFootnote(result(500, true, 1))).toBe(
      "Showing the first 500 matching lines. Narrow the search. 1 file over 1 MB not searched.",
    )
    expect(searchFootnote(result(0, false, 2))).toBe("2 files over 1 MB not searched.")
  })
})
