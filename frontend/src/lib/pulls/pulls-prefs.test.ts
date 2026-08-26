import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  readLastPull,
  readPullsFilter,
  readPullsQuery,
  readPullsSort,
  readPullsTab,
  writeLastPull,
  writePullsFilter,
  writePullsQuery,
  writePullsSort,
  writePullsTab,
} from "./pulls-prefs"

// The suite runs in node, which has no localStorage. These are the prefs that
// have to survive leaving the screen, so the storage is stubbed and the round
// trip through it is what is checked.
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

describe("the stored sort", () => {
  it("reads the default with nothing stored", () => {
    expect(readPullsSort()).toBe("updated")
  })

  it("round-trips a chosen sort", () => {
    writePullsSort("failing")

    expect(readPullsSort()).toBe("failing")
  })

  // A pref must never be able to break a launch: a sort from another build, or a
  // hand-edited one, reads as the default rather than as an unsortable column.
  it("reads a value this build does not know as the default", () => {
    stored.set("lich.pulls.sort", "by-vibes")

    expect(readPullsSort()).toBe("updated")
  })
})

describe("the stored quick filter", () => {
  it("reads every pull request with nothing stored", () => {
    expect(readPullsFilter("proj-1")).toBe("all")
  })

  it("round-trips a chosen filter", () => {
    writePullsFilter("proj-1", "failing")

    expect(readPullsFilter("proj-1")).toBe("failing")
  })

  it("reads a value this build does not know as the default", () => {
    stored.set("lich.pulls.filter.proj-1", "mine")

    expect(readPullsFilter("proj-1")).toBe("all")
  })

  // It narrows one repository's list, so it is that repository's.
  it("keeps two projects apart", () => {
    writePullsFilter("proj-1", "failing")
    writePullsFilter("proj-2", "drafts")

    expect(readPullsFilter("proj-1")).toBe("failing")
    expect(readPullsFilter("proj-2")).toBe("drafts")
  })

  it("reads and writes nothing without a project", () => {
    writePullsFilter("", "failing")

    expect(stored.size).toBe(0)
    expect(readPullsFilter("")).toBe("all")
  })
})

describe("the stored tab", () => {
  it("reads the overview with nothing stored", () => {
    expect(readPullsTab()).toBe("overview")
  })

  it("round-trips a chosen tab", () => {
    writePullsTab("files")

    expect(readPullsTab()).toBe("files")
  })

  it("reads a value this build does not know as the default", () => {
    stored.set("lich.pulls.tab", "blame")

    expect(readPullsTab()).toBe("overview")
  })
})

describe("the stored filter box", () => {
  it("reads empty with nothing stored", () => {
    expect(readPullsQuery("proj-1")).toBe("")
  })

  // Free text: whatever was typed is what comes back, qualifiers and all.
  it("round-trips what was typed", () => {
    writePullsQuery("proj-1", "is:merged review:approved cache")

    expect(readPullsQuery("proj-1")).toBe("is:merged review:approved cache")
  })

  it("round-trips an emptied box", () => {
    writePullsQuery("proj-1", "is:merged")
    writePullsQuery("proj-1", "")

    expect(readPullsQuery("proj-1")).toBe("")
  })

  // The reason this is not one global box: a query left on `is:merged` would
  // otherwise make the next project's list look empty of open pull requests.
  it("keeps two projects apart", () => {
    writePullsQuery("proj-1", "is:merged")

    expect(readPullsQuery("proj-2")).toBe("")
  })

  it("reads and writes nothing without a project", () => {
    writePullsQuery("", "is:merged")

    expect(stored.size).toBe(0)
    expect(readPullsQuery("")).toBe("")
  })
})

describe("the remembered pull request", () => {
  it("answers 0 for a project with none", () => {
    expect(readLastPull("proj-1")).toBe(0)
  })

  it("round-trips a selection", () => {
    writeLastPull("proj-1", 388)

    expect(readLastPull("proj-1")).toBe(388)
  })

  // Two repositories are two reviews: one must never open on the other's number.
  it("keeps two projects apart", () => {
    writeLastPull("proj-1", 388)
    writeLastPull("proj-2", 12)

    expect(readLastPull("proj-1")).toBe(388)
    expect(readLastPull("proj-2")).toBe(12)
  })

  // Nothing forgets a selection: a lookup that fails is transient far more often
  // than it is a pull request that is gone, and the two look the same from here.
  it("survives a selection being written again", () => {
    writeLastPull("proj-1", 388)
    writeLastPull("proj-1", 12)

    expect(readLastPull("proj-1")).toBe(12)
  })

  // 0 is the screen's own "no pull request selected", so storing it would be
  // storing nothing — and a hand-edited or foreign value reads the same way.
  it.each([null, "", "0", "-3", "1.5", "latest"])("reads %j as no selection", (raw) => {
    if (raw !== null) {
      stored.set("lich.pulls.last.proj-1", raw)
    }

    expect(readLastPull("proj-1")).toBe(0)
  })

  it("never writes a number that is not a selection", () => {
    writeLastPull("proj-1", 0)

    expect(stored.has("lich.pulls.last.proj-1")).toBe(false)
  })

  // Every reader is handed a project id straight from the route, which is empty
  // before one is open.
  it("reads and writes nothing without a project", () => {
    writeLastPull("", 388)

    expect(stored.size).toBe(0)
    expect(readLastPull("")).toBe(0)
  })
})
