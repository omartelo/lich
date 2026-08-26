import { beforeEach, describe, expect, it } from "vitest"
import { fileBrowse, updateFileBrowse } from "./file-browse"

const A = "/checkouts/one"
const B = "/checkouts/two"

// Nothing clears the store, so each case works on paths of its own rather than
// on what the case before it left behind.
let seq = 0
let a = A
let b = B

beforeEach(() => {
  seq++
  a = `${A}/${seq}`
  b = `${B}/${seq}`
})

describe("a checkout's browse", () => {
  it("is all defaults for one never browsed", () => {
    expect(fileBrowse(a)).toEqual({ query: "", open: "", selected: "", toggled: new Set() })
  })

  // The point of the store: a tab switch unmounts the panel, so each half of
  // the browse is written by whichever handler moved it and none of them may
  // take the others down with it.
  it("keeps the rest of itself when one part moves", () => {
    updateFileBrowse(a, { query: "store" })
    updateFileBrowse(a, { toggled: new Set(["src", "src/lib"]) })
    updateFileBrowse(a, {
      open: "src/lib/git-status-store.ts",
      selected: "src/lib/git-status-store.ts",
    })
    updateFileBrowse(a, { open: "" })

    expect(fileBrowse(a)).toEqual({
      query: "store",
      open: "",
      selected: "src/lib/git-status-store.ts",
      toggled: new Set(["src", "src/lib"]),
    })
  })

  // The rule the panel used to keep by clearing on a path change: one worktree's
  // filter and folds must never narrow another worktree's tree.
  it("belongs to one checkout and never to the next", () => {
    updateFileBrowse(a, { query: "store", selected: "src/lib/rpc.ts" })
    expect(fileBrowse(b)).toEqual({ query: "", open: "", selected: "", toggled: new Set() })

    updateFileBrowse(b, { query: "panel" })
    expect(fileBrowse(a).query).toBe("store")
    expect(fileBrowse(b).selected).toBe("")
  })

  // A panel with no session yet has no checkout to file this under, and one
  // shared empty key would hand the next checkout the last one's browse.
  it("writes nothing for a panel with no checkout", () => {
    updateFileBrowse("", { query: "store" })
    expect(fileBrowse("")).toEqual({ query: "", open: "", selected: "", toggled: new Set() })
  })
})
