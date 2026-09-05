import { beforeEach, describe, expect, it, vi } from "vitest"
import { storedGroups, writeGroups } from "./panes-store"

// The suite runs in node, which has no localStorage; the pref is stubbed so
// the round trip through it is what is checked.
const stored = new Map<string, string>()

vi.stubGlobal("localStorage", {
  getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => {
    stored.set(key, value)
  },
  removeItem: (key: string) => {
    stored.delete(key)
  },
})

beforeEach(() => {
  stored.clear()
})

const group = { id: "g1", name: "wall", cells: ["s1", "s2"], cols: [], rows: [] }

describe("storedGroups", () => {
  it("answers an identity-stable empty array for a project with nothing stored", () => {
    expect(storedGroups("p1")).toEqual([])
    expect(storedGroups("p1")).toBe(storedGroups("p1"))
  })

  it("answers the one empty array for no project at all, and reads nothing to do it", () => {
    stored.set("", "poison")
    expect(storedGroups("")).toEqual([])
    expect(storedGroups("")).toBe(storedGroups(""))
  })

  it("keeps the parsed value while the stored string is unchanged", () => {
    writeGroups("p1", [group])
    const first = storedGroups("p1")
    expect(first).toMatchObject([{ id: "g1", cells: ["s1", "s2"] }])
    expect(storedGroups("p1")).toBe(first)
  })
})

describe("writeGroups", () => {
  it("writes nothing for no project — there is no key to write under", () => {
    writeGroups("", [group])
    expect(stored.size).toBe(0)
  })
})
