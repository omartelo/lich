import { describe, expect, it } from "vitest"
import { computeGrid } from "./term-fit"

const cell = { width: 8, height: 16 }

describe("computeGrid", () => {
  it("floors partial cells so the grid stays whole", () => {
    expect(computeGrid(101, 49, cell)).toEqual({ cols: 12, rows: 3 })
  })

  it("fills the full width — no reserved scrollbar gutter", () => {
    // 800px / 8px = 100 cols exactly; the old FitAddon reserved 15px and lost a column.
    expect(computeGrid(800, 160, cell)).toEqual({ cols: 100, rows: 10 })
  })

  it("clamps to at least 1x1 for a sub-cell container", () => {
    expect(computeGrid(4, 4, cell)).toEqual({ cols: 1, rows: 1 })
  })

  it("returns null for a zero-size container (hidden/unmounted)", () => {
    expect(computeGrid(0, 480, cell)).toBeNull()
    expect(computeGrid(640, 0, cell)).toBeNull()
  })

  it("returns null before font metrics resolve", () => {
    expect(computeGrid(640, 480, { width: 0, height: 0 })).toBeNull()
  })
})
