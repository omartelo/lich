import { beforeEach, describe, expect, it, vi } from "vitest"
import { PINNED_GROUP_KEY, ROOT_GROUP_KEY } from "./sessions"
import { readGroupCollapsed, writeGroupCollapsed } from "./group-prefs"

// The suite runs in node, which has no localStorage. A fold has to survive
// leaving the project, so the storage is stubbed and the round trip is what is
// checked.
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

describe("a folded session group", () => {
  it("reads unfolded with nothing stored", () => {
    expect(readGroupCollapsed("proj-1", ROOT_GROUP_KEY)).toBe(false)
  })

  it("round-trips a fold", () => {
    writeGroupCollapsed("proj-1", "/home/dev/wt/feature", true)

    expect(readGroupCollapsed("proj-1", "/home/dev/wt/feature")).toBe(true)
  })

  it("round-trips an unfold", () => {
    writeGroupCollapsed("proj-1", ROOT_GROUP_KEY, true)
    writeGroupCollapsed("proj-1", ROOT_GROUP_KEY, false)

    expect(readGroupCollapsed("proj-1", ROOT_GROUP_KEY)).toBe(false)
  })

  // Unfolded is the default, so it is stored by not being stored.
  it("leaves nothing behind for an unfolded group", () => {
    writeGroupCollapsed("proj-1", ROOT_GROUP_KEY, true)
    writeGroupCollapsed("proj-1", ROOT_GROUP_KEY, false)

    expect(stored.size).toBe(0)
  })

  // A pref must never be able to break a launch: a value from another build, or
  // a hand-edited one, reads as unfolded rather than as a hidden block.
  it.each(["yes", "1", ""])("reads %j as unfolded", (raw) => {
    stored.set(`lich.sidebar.collapsed.proj-1:${ROOT_GROUP_KEY}`, raw)

    expect(readGroupCollapsed("proj-1", ROOT_GROUP_KEY)).toBe(false)
  })

  // The whole reason the project id is in the key: these two stand-ins are not
  // checkout paths, so every project has a block under each of them.
  it.each([ROOT_GROUP_KEY, PINNED_GROUP_KEY])("keeps two projects' %s apart", (key) => {
    writeGroupCollapsed("proj-1", key, true)

    expect(readGroupCollapsed("proj-1", key)).toBe(true)
    expect(readGroupCollapsed("proj-2", key)).toBe(false)
  })

  it("keeps two groups of one project apart", () => {
    writeGroupCollapsed("proj-1", ROOT_GROUP_KEY, true)

    expect(readGroupCollapsed("proj-1", PINNED_GROUP_KEY)).toBe(false)
  })
})
