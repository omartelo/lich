import { describe, expect, it } from "vitest"
import {
  cellAt,
  dragTrack,
  fits,
  grid,
  MIN_PANE_WIDTH,
  MIN_TRACK,
  offsetOf,
  rowLength,
  rowTracks,
  tracks,
} from "./pane-grid"

// The two stages the layout is actually argued about: a laptop window with the
// sidebar and the dock taking their share, and a 34" ultrawide.
const LAPTOP = 900
const ULTRAWIDE = 3100

describe("grid", () => {
  it("keeps one pane whole", () => {
    expect(grid(1, LAPTOP)).toEqual({ cols: 1, rows: 1 })
  })

  it("puts eight panes four across on an ultrawide and two across on a laptop", () => {
    expect(grid(8, ULTRAWIDE)).toEqual({ cols: 4, rows: 2 })
    expect(grid(8, LAPTOP)).toEqual({ cols: 2, rows: 4 })
  })

  it("splits two side by side on a laptop and stacks them on a narrow window", () => {
    expect(grid(2, LAPTOP)).toEqual({ cols: 2, rows: 1 })
    expect(grid(2, 700)).toEqual({ cols: 1, rows: 2 })
  })

  // The literal is pinned rather than derived from the constant: this width is
  // the readability argument the whole layout rests on, and a test that read the
  // constant would follow it wherever it moved instead of noticing.
  it("takes another row exactly when a pane would fall under the readable width", () => {
    expect(MIN_PANE_WIDTH).toBe(420)
    expect(grid(2, 840)).toEqual({ cols: 2, rows: 1 })
    expect(grid(2, 839)).toEqual({ cols: 1, rows: 2 })
  })

  it("stacks when nothing fits side by side at all", () => {
    expect(grid(3, 300)).toEqual({ cols: 1, rows: 3 })
  })
})

describe("fits", () => {
  it("allows another pane while they all stay readable", () => {
    expect(fits(8, ULTRAWIDE, 800)).toBe(true)
  })

  it("refuses one that would leave them too short", () => {
    expect(fits(8, LAPTOP, 600)).toBe(false)
  })
})

describe("cellAt / rowLength", () => {
  it("fills rows left to right in list order", () => {
    const layout = { cols: 4, rows: 2 }
    expect(cellAt(0, layout)).toEqual({ col: 0, row: 0 })
    expect(cellAt(4, layout)).toEqual({ col: 0, row: 1 })
    expect(cellAt(7, layout)).toEqual({ col: 3, row: 1 })
  })

  it("reports a short last row", () => {
    const layout = { cols: 4, rows: 2 }
    expect(rowLength(0, 7, layout)).toBe(4)
    expect(rowLength(1, 7, layout)).toBe(3)
  })
})

describe("tracks", () => {
  it("spreads equally when nothing usable is stored", () => {
    expect(tracks([], 4)).toEqual([0.25, 0.25, 0.25, 0.25])
  })

  it("keeps a stored layout that still describes this many tracks", () => {
    expect(tracks([0.6, 0.4], 2)).toEqual([0.6, 0.4])
  })

  // The grid reshapes whenever a pane is added or the window resizes, and a
  // layout dragged for four columns says nothing about three.
  it("falls back to equal when the stored layout is for another grid", () => {
    expect(tracks([0.6, 0.4], 3)).toEqual([1 / 3, 1 / 3, 1 / 3])
  })

  it("refuses a stored layout that does not add up, or that hides a pane", () => {
    expect(tracks([0.9, 0.9], 2)).toEqual([0.5, 0.5])
    expect(tracks([0.99, 0.01], 2)).toEqual([0.5, 0.5])
  })
})

describe("rowTracks", () => {
  it("spreads a short row over the whole width", () => {
    expect(rowTracks([0.4, 0.3, 0.3], 3)).toEqual([0.4, 0.3, 0.3])
    const short = rowTracks([0.5, 0.25, 0.25], 2)
    expect(short[0]).toBeCloseTo(2 / 3)
    expect(short.reduce((sum, value) => sum + value, 0)).toBeCloseTo(1)
  })
})

describe("offsetOf", () => {
  it("measures a track's leading edge", () => {
    expect(offsetOf([0.5, 0.25, 0.25], 0)).toBe(0)
    expect(offsetOf([0.5, 0.25, 0.25], 2)).toBeCloseTo(0.75)
  })
})

describe("dragTrack", () => {
  it("takes from one track and gives to its neighbour", () => {
    const next = dragTrack([0.5, 0.5], 0, 0.1)
    expect(next[0]).toBeCloseTo(0.6)
    expect(next[1]).toBeCloseTo(0.4)
  })

  it("never collapses a pane, so the session in it stays reachable", () => {
    expect(dragTrack([0.5, 0.5], 0, -1)).toEqual([MIN_TRACK, 1 - MIN_TRACK])
    expect(dragTrack([0.5, 0.5], 0, 1)).toEqual([1 - MIN_TRACK, MIN_TRACK])
  })

  it("leaves every other track alone", () => {
    const next = dragTrack([0.25, 0.25, 0.5], 0, 0.05)
    expect(next[2]).toBe(0.5)
  })

  it("ignores a boundary that is not between two tracks", () => {
    expect(dragTrack([0.5, 0.5], 1, 0.1)).toEqual([0.5, 0.5])
    expect(dragTrack([1], 0, 0.1)).toEqual([1])
  })
})
