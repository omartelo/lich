import { beforeEach, describe, expect, it } from "vitest"
import { clearRemoteCache, readRemoteCache, writeRemoteCache } from "./remote-cache"

// The cap is pinned here rather than imported: a test that derives its bounds
// from the constant it is guarding moves with it and stops guarding anything.
const MAX_ENTRIES = 32

beforeEach(() => {
  clearRemoteCache()
})

describe("the remote answer cache", () => {
  it("reads back what was filed", () => {
    writeRemoteCache("pulls.detail /repo #12", { number: 12 })

    expect(readRemoteCache("pulls.detail /repo #12")).toEqual({ number: 12 })
  })

  it("answers undefined for a key never filed", () => {
    expect(readRemoteCache("pulls.detail /repo #12")).toBeUndefined()
  })

  // Why every caller names itself in its key: a pull request's diff and its
  // conversation are asked for in the very same words otherwise.
  it("keeps two callers naming the same request apart", () => {
    writeRemoteCache("pulls.diff /repo 12", "diff")
    writeRemoteCache("pulls.conversation /repo 12", "conversation")

    expect(readRemoteCache("pulls.diff /repo 12")).toBe("diff")
    expect(readRemoteCache("pulls.conversation /repo 12")).toBe("conversation")
  })

  it("replaces the answer under a key that is filed again", () => {
    writeRemoteCache("pulls.list /repo open", ["stale"])
    writeRemoteCache("pulls.list /repo open", ["fresh"])

    expect(readRemoteCache("pulls.list /repo open")).toEqual(["fresh"])
  })

  it("holds the cap without evicting", () => {
    for (let i = 0; i < MAX_ENTRIES; i++) {
      writeRemoteCache(`pulls.diff pr-${i}`, i)
    }

    expect(readRemoteCache("pulls.diff pr-0")).toBe(0)
    expect(readRemoteCache(`pulls.diff pr-${MAX_ENTRIES - 1}`)).toBe(MAX_ENTRIES - 1)
  })

  it("evicts the oldest answer once past the cap", () => {
    for (let i = 0; i <= MAX_ENTRIES; i++) {
      writeRemoteCache(`pulls.diff pr-${i}`, i)
    }

    expect(readRemoteCache("pulls.diff pr-0")).toBeUndefined()
    expect(readRemoteCache("pulls.diff pr-1")).toBe(1)
    expect(readRemoteCache(`pulls.diff pr-${MAX_ENTRIES}`)).toBe(MAX_ENTRIES)
  })

  // Re-filing is what a refetch does on every focus, so the answer being read
  // over and over must not be the one eviction takes.
  it("counts a re-filed answer as the newest", () => {
    for (let i = 0; i < MAX_ENTRIES; i++) {
      writeRemoteCache(`pulls.diff pr-${i}`, i)
    }
    writeRemoteCache("pulls.diff pr-0", "revalidated")
    writeRemoteCache("pulls.diff one-too-many", true)

    expect(readRemoteCache("pulls.diff pr-0")).toBe("revalidated")
    expect(readRemoteCache("pulls.diff pr-1")).toBeUndefined()
  })
})
