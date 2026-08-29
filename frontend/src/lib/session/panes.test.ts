import { describe, expect, it } from "vitest"
import {
  besideCandidate,
  clampRatio,
  dragRatio,
  formatBeside,
  NO_BESIDE,
  parseBeside,
  RATIO_MAX,
  RATIO_MIN,
  resolveBeside,
} from "./panes"
import type { Session } from "./sessions"

const session = (id: string): Session => ({ id, label: id, kind: "claude" })
const sessions = [session("a"), session("b"), session("c")]

describe("clampRatio", () => {
  it("passes a ratio inside the bounds through", () => {
    expect(clampRatio(0.5)).toBe(0.5)
  })

  // The literals are pinned rather than derived from the constants: this is the
  // boundary the readable-width argument bought, and a test that reads the
  // constant would follow it wherever it moved instead of noticing.
  it("clamps either pane to a quarter of the stage", () => {
    expect(clampRatio(0.1)).toBe(0.25)
    expect(clampRatio(0.9)).toBe(0.75)
    expect(RATIO_MIN).toBe(0.25)
    expect(RATIO_MAX).toBe(0.75)
  })
})

describe("parseBeside", () => {
  it("reads a bare id as the right-hand pane", () => {
    expect(parseBeside("b")).toEqual({ id: "b", side: "right" })
  })

  it("reads the stored side", () => {
    expect(parseBeside("b@left")).toEqual({ id: "b", side: "left" })
  })

  // A pref must never be able to break a launch, so anything unrecognised reads
  // as the default rather than as a third side.
  it("answers the default for nothing and for a value it does not know", () => {
    expect(parseBeside(null)).toEqual(NO_BESIDE)
    expect(parseBeside("")).toEqual(NO_BESIDE)
    expect(parseBeside("b@sideways")).toEqual({ id: "b", side: "right" })
  })

  it("round-trips through the stored form", () => {
    for (const beside of [
      { id: "b", side: "right" },
      { id: "b", side: "left" },
    ] as const) {
      expect(parseBeside(formatBeside(beside))).toEqual(beside)
    }
  })
})

describe("resolveBeside", () => {
  it("keeps a stored id that is still a card, on the side it was left", () => {
    expect(resolveBeside("b", sessions, "a")).toEqual({ id: "b", side: "right" })
    expect(resolveBeside("b@left", sessions, "a")).toEqual({ id: "b", side: "left" })
  })

  it("answers nothing when nothing is stored", () => {
    expect(resolveBeside("", sessions, "a")).toEqual(NO_BESIDE)
  })

  // The four ways a session leaves the workspace — a close, a park, a worktree
  // removal, an undone create — all end here, which is why the id is derived on
  // read instead of maintained by each of them.
  it("drops an id that is no longer in the project", () => {
    expect(resolveBeside("gone", sessions, "a")).toEqual(NO_BESIDE)
  })

  it("drops an id that has become the active one", () => {
    expect(resolveBeside("a", sessions, "a")).toEqual(NO_BESIDE)
    expect(resolveBeside("a@left", sessions, "a")).toEqual(NO_BESIDE)
  })
})

describe("besideCandidate", () => {
  it("takes the next card in the project's own order", () => {
    expect(besideCandidate(sessions, "a")).toBe("b")
  })

  it("wraps past the last card", () => {
    expect(besideCandidate(sessions, "c")).toBe("a")
  })

  it("declines when there is nothing to put beside", () => {
    expect(besideCandidate([session("a")], "a")).toBe("")
    expect(besideCandidate([], "")).toBe("")
  })

  it("starts at the first card when the active one is not in the list", () => {
    expect(besideCandidate(sessions, "gone")).toBe("a")
  })
})

describe("dragRatio", () => {
  it("moves the seam with the pointer", () => {
    expect(dragRatio(0.5, 100, 200, 1000)).toBeCloseTo(0.6)
    expect(dragRatio(0.5, 200, 100, 1000)).toBeCloseTo(0.4)
  })

  it("clamps a drag that would collapse a pane", () => {
    expect(dragRatio(0.5, 0, 900, 1000)).toBe(RATIO_MAX)
    expect(dragRatio(0.5, 900, 0, 1000)).toBe(RATIO_MIN)
  })

  it("holds the ratio when the container has no width to divide by", () => {
    expect(dragRatio(0.5, 0, 400, 0)).toBe(0.5)
  })
})
