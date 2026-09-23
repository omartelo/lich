import { describe, expect, it } from "vitest"
import { departed, EXIT_MS, holdDeparted, withClosing } from "./use-closing-sessions"
import type { Session } from "./sessions"

function session(id: string): Session {
  return { id, label: id, kind: "claude" }
}

const [a, b, c, d] = [session("a"), session("b"), session("c"), session("d")]

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

describe("holdDeparted", () => {
  it("keeps cards closed in separate renders in the order they were drawn", () => {
    const first = holdDeparted([], [a, b, c, d], [a, c, d])
    const second = holdDeparted(first, [a, c, d], [a, d])
    expect(withClosing([a, d], second)).toEqual([a, b, c, d])
  })

  it("hands back the same list when nothing left", () => {
    const held = holdDeparted([], [a, b], [a])
    expect(holdDeparted(held, [a], [a, c])).toBe(held)
  })

  // Back while its card was still going, then closed again: one entry, in the
  // slot it holds now, so its exit is timed from the second close.
  it("holds a card closed twice once, at its new slot", () => {
    const held = holdDeparted([], [a, b, c], [a, c])
    const again = holdDeparted(held, [c, a, b], [c, a])
    expect(again).toEqual([{ session: b, index: 2 }])
    expect(again).not.toBe(held)
  })
})

it("holds a closed card long enough for its exit to play", () => {
  // Pinned, not derived: the transition is timed from this constant, so a change
  // to one is only correct with the same change to the other.
  expect(EXIT_MS).toBe(180)
})
