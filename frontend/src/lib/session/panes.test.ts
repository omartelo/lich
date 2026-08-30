import { describe, expect, it } from "vitest"
import {
  addToGroup,
  defaultName,
  dissolveGroup,
  formatGroups,
  groupOf,
  movingFrom,
  nextCandidate,
  type PaneGroup,
  parseGroups,
  removeFromGroups,
  reorderGroups,
  resolveGroups,
  swapCells,
  updateGroup,
} from "./panes"
import type { Session } from "./sessions"

const session = (id: string): Session => ({ id, label: id.toUpperCase(), kind: "claude" })
const sessions = [session("a"), session("b"), session("c"), session("d")]

const group = (id: string, cells: string[], name = id): PaneGroup => ({
  id,
  name,
  cells,
  cols: [],
  rows: [],
})

describe("parseGroups", () => {
  it("reads what formatGroups wrote", () => {
    const groups = [group("g1", ["a", "b"], "orchestrator"), group("g2", ["c"])]
    expect(parseGroups(formatGroups(groups))).toEqual(groups)
  })

  // A pref must never be able to break a launch, so anything unreadable costs
  // the user their arrangement and nothing else.
  it("answers no groups for anything it cannot read", () => {
    expect(parseGroups(null)).toEqual([])
    expect(parseGroups("")).toEqual([])
    expect(parseGroups("{oh no")).toEqual([])
    expect(parseGroups('{"id":"g1"}')).toEqual([])
  })

  it("drops entries that are not groups and keeps the ones that are", () => {
    expect(parseGroups('[{"nope":1},{"id":"g1","cells":["a"]}]')).toEqual([
      { id: "g1", name: "", cells: ["a"], cols: [], rows: [] },
    ])
  })
})

describe("resolveGroups", () => {
  it("drops cells whose session is gone", () => {
    expect(resolveGroups([group("g1", ["a", "gone", "b"])], sessions)[0].cells).toEqual(["a", "b"])
  })

  // A group of one is still something the user named and can add to; a group of
  // none has nothing left for the name to be about.
  it("keeps a group of one and drops a group of none", () => {
    const groups = resolveGroups([group("g1", ["a"]), group("g2", ["gone"])], sessions)
    expect(groups.map((g) => g.id)).toEqual(["g1"])
  })

  // The one-group rule is enforced on read too, so a stored value that somehow
  // holds a session twice cannot put it on two walls.
  it("gives a session to the first group that claims it", () => {
    const groups = resolveGroups([group("g1", ["a", "b"]), group("g2", ["b", "c"])], sessions)
    expect(groups.map((g) => g.cells)).toEqual([["a", "b"], ["c"]])
  })
})

describe("groupOf", () => {
  it("finds the wall a session is on, and answers null for one on none", () => {
    const groups = [group("g1", ["a", "b"]), group("g2", ["c"])]
    expect(groupOf(groups, "b")?.id).toBe("g1")
    expect(groupOf(groups, "d")).toBeNull()
  })
})

describe("movingFrom", () => {
  const groups = [group("g1", ["a", "b"]), group("g2", ["c"])]

  it("names the wall a session would be taken off", () => {
    expect(movingFrom(groups, groups[1], "a")?.id).toBe("g1")
  })

  // Nothing to ask about: the session is on no wall, or already on this one.
  it("answers null when nothing else loses a member", () => {
    expect(movingFrom(groups, groups[0], "a")).toBeNull()
    expect(movingFrom(groups, groups[0], "d")).toBeNull()
    expect(movingFrom(groups, null, "d")).toBeNull()
  })

  // Starting a wall around a session that is on another one is still a move,
  // even though there is no current wall to move it into yet.
  it("still names it when the user is on no wall at all", () => {
    expect(movingFrom(groups, null, "c")?.id).toBe("g2")
  })
})

describe("addToGroup", () => {
  it("appends to the named group", () => {
    expect(addToGroup([group("g1", ["a"])], "g1", "b")[0].cells).toEqual(["a", "b"])
  })

  // At most one group per session, so adding is also a move.
  it("takes the session off whatever wall had it", () => {
    const groups = addToGroup([group("g1", ["a", "b"]), group("g2", ["c"])], "g2", "b")
    expect(groups.map((g) => g.cells)).toEqual([["a"], ["c", "b"]])
  })

  it("drops a group the move emptied", () => {
    const groups = addToGroup([group("g1", ["b"]), group("g2", ["c"])], "g2", "b")
    expect(groups.map((g) => g.id)).toEqual(["g2"])
  })
})

describe("removeFromGroups", () => {
  it("keeps the group at one member", () => {
    expect(removeFromGroups([group("g1", ["a", "b"])], "b")[0].cells).toEqual(["a"])
  })

  it("ends the group when its last member leaves", () => {
    expect(removeFromGroups([group("g1", ["a"])], "a")).toEqual([])
  })
})

describe("updateGroup / dissolveGroup", () => {
  it("changes only the group named", () => {
    const groups = updateGroup([group("g1", ["a"]), group("g2", ["b"])], "g2", { name: "renamed" })
    expect(groups.map((g) => g.name)).toEqual(["g1", "renamed"])
  })

  it("takes a wall apart without touching the others", () => {
    expect(dissolveGroup([group("g1", ["a"]), group("g2", ["b"])], "g1").map((g) => g.id)).toEqual([
      "g2",
    ])
  })
})

describe("reorderGroups", () => {
  it("puts the walls in the order the drag named", () => {
    const groups = [group("g1", ["a"]), group("g2", ["b"]), group("g3", ["c"])]
    expect(reorderGroups(groups, ["g3", "g1", "g2"]).map((g) => g.id)).toEqual(["g3", "g1", "g2"])
  })

  // An id set that raced a dissolve must not cost the user a group.
  it("keeps a wall the drag did not name", () => {
    const groups = [group("g1", ["a"]), group("g2", ["b"])]
    expect(reorderGroups(groups, ["g2"]).map((g) => g.id)).toEqual(["g2", "g1"])
  })
})

describe("defaultName", () => {
  it("names a wall after the session it grew from", () => {
    expect(defaultName(sessions, "b")).toBe("B")
    expect(defaultName(sessions, "gone")).toBe("Split")
  })
})

describe("nextCandidate", () => {
  it("takes the first card on no wall at all", () => {
    expect(nextCandidate(sessions, [group("g1", ["a", "b"])])).toBe("c")
  })

  it("answers nothing when every session is already on one", () => {
    expect(nextCandidate(sessions, [group("g1", ["a", "b", "c", "d"])])).toBe("")
  })
})

describe("swapCells", () => {
  it("trades two places", () => {
    expect(swapCells(["a", "b", "c"], 0, 2)).toEqual(["c", "b", "a"])
  })

  it("leaves the list alone for a drop that goes nowhere", () => {
    expect(swapCells(["a", "b"], 1, 1)).toEqual(["a", "b"])
    expect(swapCells(["a", "b"], 0, 5)).toEqual(["a", "b"])
  })
})
