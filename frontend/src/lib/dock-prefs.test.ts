import { beforeEach, describe, expect, it, vi } from "vitest"
import { readDiffSource, writeDiffSource } from "./dock-prefs"

// The suite runs in node, which has no localStorage. This is the pref that has
// to survive the dock unmounting, so the storage is stubbed and the round trip
// through it is what is checked.
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

describe("the Review tab's source", () => {
  it("is the working tree until one is chosen", () => {
    expect(readDiffSource()).toBe("worktree")
  })

  it("comes back the way the panel was left", () => {
    writeDiffSource("turn")
    expect(readDiffSource()).toBe("turn")
    writeDiffSource("worktree")
    expect(readDiffSource()).toBe("worktree")
  })

  // A source this build cannot render would leave the panel asking for a diff
  // nothing answers, with a switch showing a value it does not have.
  it("reads a value from another build as the working tree", () => {
    stored.set("lich.dock.source", "staged")
    expect(readDiffSource()).toBe("worktree")
  })
})
