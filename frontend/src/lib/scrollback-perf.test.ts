import {describe, expect, it, vi} from "vitest"
import {patchScrollGate, patchScrollbackCache} from "./scrollback-perf"

function makeRenderer(dirty = false) {
  const theme = {background: "#000"}
  return {
    theme,
    hoveredHyperlinkId: 0,
    previousHoveredHyperlinkId: 0,
    hoveredLinkRange: null as unknown,
    previousHoveredLinkRange: null as unknown,
    selectionManager: {getDirtySelectionRows: () => new Set<number>()},
    paints: 0,
    buffer: {isDirty: () => dirty},
    render(_buffer: {isDirty(): boolean}) {
      this.paints++
    },
  }
}

describe("patchScrollGate", () => {
  it("skips repeated renders while scrolled and unchanged", () => {
    const r = makeRenderer()
    patchScrollGate(r)
    r.render(r.buffer, false, 5, null, 1)
    r.render(r.buffer, false, 5, null, 1)
    r.render(r.buffer, false, 5, null, 1)
    expect(r.paints).toBe(1)
  })

  it("never skips at the bottom or when forced", () => {
    const r = makeRenderer()
    patchScrollGate(r)
    r.render(r.buffer, false, 0, null, 1)
    r.render(r.buffer, false, 0, null, 1)
    r.render(r.buffer, true, 5, null, 1)
    expect(r.paints).toBe(3)
  })

  it("renders when viewportY or scrollbar opacity changes", () => {
    const r = makeRenderer()
    patchScrollGate(r)
    r.render(r.buffer, false, 5, null, 1)
    r.render(r.buffer, false, 6, null, 1)
    r.render(r.buffer, false, 6, null, 0.5)
    expect(r.paints).toBe(3)
  })

  it("renders while the buffer is dirty", () => {
    const r = makeRenderer(true)
    patchScrollGate(r)
    r.render(r.buffer, false, 5, null, 1)
    r.render(r.buffer, false, 5, null, 1)
    expect(r.paints).toBe(2)
  })

  it("renders when hover repaint is pending", () => {
    const r = makeRenderer()
    patchScrollGate(r)
    r.render(r.buffer, false, 5, null, 1)
    r.hoveredHyperlinkId = 7
    r.render(r.buffer, false, 5, null, 1)
    expect(r.paints).toBe(2)
  })

  it("renders when selection rows are dirty", () => {
    const r = makeRenderer()
    patchScrollGate(r)
    r.render(r.buffer, false, 5, null, 1)
    r.selectionManager = {getDirtySelectionRows: () => new Set([3])}
    r.render(r.buffer, false, 5, null, 1)
    expect(r.paints).toBe(2)
  })

  it("renders when the theme object changes", () => {
    const r = makeRenderer()
    patchScrollGate(r)
    r.render(r.buffer, false, 5, null, 1)
    r.theme = {background: "#fff"}
    r.render(r.buffer, false, 5, null, 1)
    expect(r.paints).toBe(2)
  })
})

function makeTerm() {
  const writeSpy = vi.fn()
  const resizeSpy = vi.fn()
  return {
    fetches: 0,
    getScrollbackLine(index: number): object[] | null {
      this.fetches++
      return index < 0 ? null : [{codepoint: index}]
    },
    write: writeSpy as (data: unknown) => void,
    resize: resizeSpy as (cols: number, rows: number) => void,
    writeSpy,
    resizeSpy,
  }
}

describe("patchScrollbackCache", () => {
  it("caches rows per index", () => {
    const t = makeTerm()
    patchScrollbackCache(t)
    const first = t.getScrollbackLine(10)
    expect(t.getScrollbackLine(10)).toBe(first)
    expect(t.fetches).toBe(1)
  })

  it("does not cache null rows", () => {
    const t = makeTerm()
    patchScrollbackCache(t)
    expect(t.getScrollbackLine(-1)).toBeNull()
    expect(t.getScrollbackLine(-1)).toBeNull()
    expect(t.fetches).toBe(2)
  })

  it("invalidates on write and resize, delegating to the originals", () => {
    const t = makeTerm()
    patchScrollbackCache(t)
    t.getScrollbackLine(10)
    t.write("x")
    t.getScrollbackLine(10)
    expect(t.fetches).toBe(2)
    t.resize(80, 24)
    t.getScrollbackLine(10)
    expect(t.fetches).toBe(3)
    expect(t.writeSpy).toHaveBeenCalledWith("x")
    expect(t.resizeSpy).toHaveBeenCalledWith(80, 24)
  })

  it("evicts oldest entries beyond the cap", () => {
    const t = makeTerm()
    patchScrollbackCache(t)
    for (let i = 0; i < 257; i++) {
      t.getScrollbackLine(i)
    }
    t.getScrollbackLine(0)
    expect(t.fetches).toBe(258)
  })
})
