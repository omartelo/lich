import { describe, expect, it } from "vitest"
import { foldConflicts, NAMED, splitConflictPath } from "./conflict-files"

describe("foldConflicts", () => {
  it("names every file when they all fit", () => {
    const files = ["a.txt", "b.txt"]
    expect(foldConflicts(files, false)).toEqual({ shown: files, hidden: 0 })
  })

  it("names six and holds the seventh back", () => {
    const files = ["1", "2", "3", "4", "5", "6", "7"]
    expect(foldConflicts(files, false)).toEqual({ shown: files.slice(0, 6), hidden: 1 })
  })

  it("holds nothing back at exactly the limit", () => {
    const files = ["1", "2", "3", "4", "5", "6"]
    expect(foldConflicts(files, false).hidden).toBe(0)
    expect(foldConflicts(files, false).shown).toHaveLength(NAMED)
  })

  it("names none inline once the whole list is open", () => {
    expect(foldConflicts(["1", "2", "3", "4", "5", "6", "7"], true).shown).toEqual([])
  })

  it("answers for a list that is empty", () => {
    expect(foldConflicts([], false)).toEqual({ shown: [], hidden: 0 })
  })
})

describe("splitConflictPath", () => {
  it("cuts where the directory ends", () => {
    expect(splitConflictPath("internal/project/pr.go")).toEqual({
      dir: "internal/project/",
      name: "pr.go",
    })
  })

  it("makes a file at the root all name", () => {
    expect(splitConflictPath("CHANGELOG.md")).toEqual({ dir: "", name: "CHANGELOG.md" })
  })

  it("makes a path ending in a separator all directory", () => {
    expect(splitConflictPath("docs/")).toEqual({ dir: "docs/", name: "" })
  })

  it("answers for the empty path", () => {
    expect(splitConflictPath("")).toEqual({ dir: "", name: "" })
  })
})
