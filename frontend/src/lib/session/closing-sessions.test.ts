import { describe, expect, it } from "vitest"
import { departed, EXIT_MS, withClosing } from "./use-closing-sessions"
import type { Session } from "./sessions"

function session(id: string): Session {
  return { id, label: id, kind: "claude" }
}

const [a, b, c] = [session("a"), session("b"), session("c")]

describe("departed", () => {
  it("names the sessions that left, with the slot each held", () => {
    expect(departed([a, b, c], [a, c])).toEqual([{ session: b, index: 1 }])
  })

  it("is empty when the list only grew or was reordered", () => {
    expect(departed([a, b], [a, b, c])).toEqual([])
    expect(departed([a, b], [b, a])).toEqual([])
  })
})

describe("withClosing", () => {
  it("puts a closing card back in the slot it left", () => {
    expect(withClosing([a, c], [{ session: b, index: 1 }])).toEqual([a, b, c])
  })

  it("keeps several in order, whatever order they closed in", () => {
    expect(
      withClosing(
        [b],
        [
          { session: c, index: 2 },
          { session: a, index: 0 },
        ],
      ),
    ).toEqual([a, b, c])
  })

  it("appends one whose slot is past the end of what is left", () => {
    expect(withClosing([a], [{ session: c, index: 2 }])).toEqual([a, c])
  })
})

it("holds a closed card long enough for its exit to play", () => {
  // Pinned, not derived: the transition is timed from this constant, so a change
  // to one is only correct with the same change to the other.
  expect(EXIT_MS).toBe(180)
})
