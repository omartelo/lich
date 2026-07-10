import {describe, expect, it} from "vitest"
import {patchPooledGetLine} from "./getline-pool"

interface FakeCell {
  codepoint: number
}

function makeTarget(cols: number, rows: number) {
  const pool: FakeCell[] = Array.from({length: cols * rows}, (_, i) => ({
    codepoint: i,
  }))
  return {
    _cols: cols,
    _rows: rows,
    updates: 0,
    pool,
    update() {
      this.updates++
      return 0
    },
    getViewport() {
      return this.pool
    },
    getLine(_row: number): object[] | null {
      throw new Error("original getLine must be replaced")
    },
  }
}

describe("patchPooledGetLine", () => {
  it("returns references into the pool, not copies", () => {
    const target = makeTarget(3, 2)
    patchPooledGetLine(target)
    const row = target.getLine(1)
    expect(row).toHaveLength(3)
    expect(row?.[0]).toBe(target.pool[3])
    expect(row?.[2]).toBe(target.pool[5])
  })

  it("reuses the same row array across calls (zero allocation)", () => {
    const target = makeTarget(4, 3)
    patchPooledGetLine(target)
    expect(target.getLine(2)).toBe(target.getLine(2))
  })

  it("reflects in-place pool mutation, and refreshes via update()", () => {
    const target = makeTarget(2, 2)
    patchPooledGetLine(target)
    const row = target.getLine(0) as FakeCell[]
    target.pool[0].codepoint = 99
    expect(row[0].codepoint).toBe(99)
    expect(target.updates).toBe(1)
  })

  it("returns null out of range", () => {
    const target = makeTarget(2, 2)
    patchPooledGetLine(target)
    expect(target.getLine(-1)).toBeNull()
    expect(target.getLine(2)).toBeNull()
  })

  it("rebuilds the row mapping after a resize", () => {
    const target = makeTarget(2, 2)
    patchPooledGetLine(target)
    expect((target.getLine(1) as FakeCell[])[0].codepoint).toBe(2)
    // Grow like initCellPool does: same array, more cells, new dimensions.
    target._cols = 3
    target._rows = 3
    for (let i = 4; i < 9; i++) {
      target.pool.push({codepoint: i})
    }
    const row = target.getLine(1) as FakeCell[]
    expect(row).toHaveLength(3)
    expect(row[0]).toBe(target.pool[3])
  })

  it("tolerates a missing wasmTerm", () => {
    expect(() => patchPooledGetLine(undefined)).not.toThrow()
  })
})
