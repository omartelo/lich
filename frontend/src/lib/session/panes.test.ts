import { describe, expect, it } from "vitest"
import {
  addToGroup,
  defaultName,
  dissolveGroup,
  focusAfterRemove,
  formatGroups,
  groupOf,
  movingFrom,
  nextCandidate,
  type PaneGroup,
  parseGroups,
  planAdd,
  removeFromGroups,
  reorderCells,
  resolveGroups,
  stageAction,
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

  // A wall of one draws nothing a solo session does not, so it stops being a
  // group and its survivor goes back among its checkout's cards.
  it("drops a group left with fewer than two live sessions", () => {
    const groups = resolveGroups(
      [group("g1", ["a", "b"]), group("g2", ["c", "gone"]), group("g3", ["gone"])],
      sessions,
    )
    expect(groups.map((g) => g.id)).toEqual(["g1"])
  })

  // The one-group rule is enforced on read too, so a stored value that somehow
  // holds a session twice cannot put it on two walls.
  it("gives a session to the first group that claims it", () => {
    const groups = resolveGroups([group("g1", ["a", "b"]), group("g2", ["b", "c", "d"])], sessions)
    expect(groups.map((g) => g.cells)).toEqual([
      ["a", "b"],
      ["c", "d"],
    ])
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
    expect(addToGroup([group("g1", ["a", "b"])], "g1", "c")[0].cells).toEqual(["a", "b", "c"])
  })

  // At most one group per session, so adding is also a move.
  it("takes the session off whatever wall had it", () => {
    const groups = addToGroup([group("g1", ["a", "b", "d"]), group("g2", ["c"])], "g2", "b")
    expect(groups.map((g) => g.cells)).toEqual([
      ["a", "d"],
      ["c", "b"],
    ])
  })

  it("ends a group the move left below two", () => {
    const groups = addToGroup([group("g1", ["a", "b"]), group("g2", ["c", "d"])], "g2", "b")
    expect(groups.map((g) => g.id)).toEqual(["g2"])
  })
})

describe("removeFromGroups", () => {
  it("keeps a group that still has two members", () => {
    expect(removeFromGroups([group("g1", ["a", "b", "c"])], "b")[0].cells).toEqual(["a", "c"])
  })

  // The pane that leaves takes the group with it: one session is not a split,
  // and the survivor belongs back among its own checkout's cards.
  it("ends the group when only one member would be left", () => {
    expect(removeFromGroups([group("g1", ["a", "b"])], "a")).toEqual([])
  })
})

describe("focusAfterRemove", () => {
  it("hands the cursor to the cell that slid into the empty place", () => {
    expect(focusAfterRemove(["a", "b", "c"], "b")).toBe("c")
  })

  it("falls back to the last cell when the last one left", () => {
    expect(focusAfterRemove(["a", "b", "c"], "c")).toBe("b")
  })

  // "" is "leave the window where it is": nothing is left to focus, or nothing
  // was taken away in the first place.
  it("answers nothing when no cell was freed", () => {
    expect(focusAfterRemove(["a"], "a")).toBe("")
    expect(focusAfterRemove(["a", "b"], "gone")).toBe("")
    expect(focusAfterRemove([], "a")).toBe("")
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

describe("defaultName", () => {
  it("names a wall after the session it grew from", () => {
    expect(defaultName(sessions, "b")).toBe("B")
    expect(defaultName(sessions, "gone")).toBe("Split")
  })
})

describe("nextCandidate", () => {
  it("takes the first card on no wall at all", () => {
    expect(nextCandidate(sessions, [group("g1", ["a", "b"])], "d")).toBe("c")
  })

  it("answers nothing when every session is already on one", () => {
    expect(nextCandidate(sessions, [group("g1", ["a", "b", "c", "d"])], "")).toBe("")
  })

  // The shortcut starts a wall around the active session, so offering that same
  // session is offering nothing: the caller refuses an id equal to the active
  // one, and the press did nothing at all.
  it("skips the active session", () => {
    expect(nextCandidate(sessions, [], "a")).toBe("b")
    expect(nextCandidate([session("a")], [], "a")).toBe("")
  })
})

// The refusal matrix the add affordance documents, and the two things it does
// when it does not refuse.
describe("planAdd", () => {
  // A stage big enough for several panes; the height is what runs out first.
  const roomy = { width: 3000, height: 2000 }
  const base = { sessions, groups: [] as PaneGroup[], current: null, activeId: "a", stage: roomy }

  it("starts a wall around the active session, taking the next free card", () => {
    expect(planAdd(base)).toEqual({ kind: "start", around: "a", sessionId: "b" })
  })

  it("joins the wall the active session is already on", () => {
    const groups = [group("g1", ["a", "b"])]
    expect(planAdd({ ...base, groups, current: groups[0] })).toEqual({
      kind: "join",
      groupId: "g1",
      sessionId: "c",
    })
  })

  it("refuses with nothing left to show", () => {
    const groups = [group("g1", ["a", "b", "c", "d"])]
    expect(planAdd({ ...base, groups, current: groups[0] }).kind).toBe("none")
  })

  it("refuses with no active session to show anything beside", () => {
    expect(planAdd({ ...base, activeId: "" }).kind).toBe("none")
  })

  it("refuses to add the active session to itself", () => {
    expect(planAdd({ ...base, sessionId: "a" }).kind).toBe("none")
  })

  // A stage 500px wide takes two panes as two rows (neither fits beside the
  // other), so 320px of height is exactly enough for both and 300 is not.
  it("refuses when one more pane would be too small to read", () => {
    expect(planAdd({ ...base, stage: { width: 500, height: 300 } }).kind).toBe("none")
    expect(planAdd({ ...base, stage: { width: 500, height: 320 } }).kind).toBe("start")
  })

  // Taking a session off somebody else's arrangement is the user's decision, so
  // a caller that has not asked gets a no-op rather than a silent move.
  it("refuses a session on another wall until the move is asked for", () => {
    const groups = [group("g1", ["b"])]
    expect(planAdd({ ...base, groups, sessionId: "b" }).kind).toBe("none")
    expect(planAdd({ ...base, groups, sessionId: "b", move: true })).toEqual({
      kind: "start",
      around: "a",
      sessionId: "b",
    })
  })
})

describe("stageAction", () => {
  // What the card's own label reads off: a session drawing in a pane right now
  // is taken off the stage, and any other is put on it. Membership of a parked
  // wall is not the question — that card is not showing.
  it("takes a member of the wall on screen off it", () => {
    const groups = [group("g1", ["a", "b"])]
    expect(stageAction(groups, groups[0], "b")).toEqual({ kind: "remove" })
  })

  it("asks before moving a member of a parked wall", () => {
    const groups = [group("g1", ["a", "b"]), group("g2", ["c"])]
    expect(stageAction(groups, groups[1], "b")).toEqual({ kind: "confirm", from: groups[0] })
    expect(stageAction(groups, null, "b")).toEqual({ kind: "confirm", from: groups[0] })
  })

  it("adds a session on no wall at all", () => {
    expect(stageAction([group("g1", ["a"])], null, "d")).toEqual({ kind: "add" })
  })
})

describe("reorderCells", () => {
  it("puts a wall's panes in the order the drag named", () => {
    const groups = [group("g1", ["a", "b", "c"]), group("g2", ["d"])]
    expect(reorderCells(groups, "g1", ["c", "a", "b"]).map((g) => g.cells)).toEqual([
      ["c", "a", "b"],
      ["d"],
    ])
  })

  // A card added to the wall while its siblings were being dragged is not in the
  // dropped order; writing it would take that session straight back off.
  it("drops an order that no longer names the wall's exact members", () => {
    const groups = [group("g1", ["a", "b", "c"])]
    expect(reorderCells(groups, "g1", ["c", "a"])[0].cells).toEqual(["a", "b", "c"])
    expect(reorderCells(groups, "g1", ["c", "a", "b", "d"])[0].cells).toEqual(["a", "b", "c"])
    expect(reorderCells(groups, "g1", ["a", "b", "b"])[0].cells).toEqual(["a", "b", "c"])
  })

  it("leaves the walls alone for a group that is gone", () => {
    const groups = [group("g1", ["a"])]
    expect(reorderCells(groups, "gone", ["a"])).toEqual(groups)
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
