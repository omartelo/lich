import { describe, expect, it } from "vitest"
import { blockGlyph, patchBlockGlyphs } from "./block-glyphs"
import type { UnitRect } from "./block-glyphs"

function coveredArea(rects: readonly UnitRect[]): number {
  return rects.reduce((sum, [, , w, h]) => sum + w * h, 0)
}

describe("blockGlyph", () => {
  it("returns null for non-block characters", () => {
    expect(blockGlyph("A".codePointAt(0)!)).toBeNull()
    expect(blockGlyph(0x257f)).toBeNull() // box drawing, out of range
    expect(blockGlyph(0x25a0)).toBeNull() // ■, past the range
  })

  it("covers the full cell for █", () => {
    const glyph = blockGlyph(0x2588)!
    expect(glyph.rects).toEqual([[0, 0, 1, 1]])
    expect(glyph.alpha).toBe(1)
  })

  it("maps half blocks to their cell halves", () => {
    expect(blockGlyph(0x2580)!.rects).toEqual([[0, 0, 1, 0.5]]) // ▀
    expect(blockGlyph(0x2584)!.rects).toEqual([[0, 0.5, 1, 0.5]]) // ▄
    expect(blockGlyph(0x258c)!.rects).toEqual([[0, 0, 0.5, 1]]) // ▌
    expect(blockGlyph(0x2590)!.rects).toEqual([[0.5, 0, 0.5, 1]]) // ▐
  })

  it("scales eighth blocks by codepoint", () => {
    expect(blockGlyph(0x2581)!.rects).toEqual([[0, 7 / 8, 1, 1 / 8]]) // ▁
    expect(blockGlyph(0x2587)!.rects).toEqual([[0, 1 / 8, 1, 7 / 8]]) // ▇
    expect(blockGlyph(0x2589)!.rects).toEqual([[0, 0, 7 / 8, 1]]) // ▉
    expect(blockGlyph(0x258f)!.rects).toEqual([[0, 0, 1 / 8, 1]]) // ▏
    expect(blockGlyph(0x2594)!.rects).toEqual([[0, 0, 1, 1 / 8]]) // ▔
    expect(blockGlyph(0x2595)!.rects).toEqual([[7 / 8, 0, 1 / 8, 1]]) // ▕
  })

  it("renders shades as full-cell rects with partial alpha", () => {
    expect(blockGlyph(0x2591)).toEqual({ rects: [[0, 0, 1, 1]], alpha: 0.25 })
    expect(blockGlyph(0x2592)).toEqual({ rects: [[0, 0, 1, 1]], alpha: 0.5 })
    expect(blockGlyph(0x2593)).toEqual({ rects: [[0, 0, 1, 1]], alpha: 0.75 })
  })

  it("composes quadrants with the right coverage", () => {
    // ▘ upper-left only
    expect(blockGlyph(0x2598)!.rects).toEqual([[0, 0, 0.5, 0.5]])
    // ▝ upper-right only
    expect(blockGlyph(0x259d)!.rects).toEqual([[0.5, 0, 0.5, 0.5]])
    // ▛ = everything except lower-right
    const upperLeftHeavy = blockGlyph(0x259b)!
    expect(coveredArea(upperLeftHeavy.rects)).toBe(0.75)
    expect(upperLeftHeavy.rects).not.toContainEqual([0.5, 0.5, 0.5, 0.5])
    // ▜ = everything except lower-left
    const upperRightHeavy = blockGlyph(0x259c)!
    expect(coveredArea(upperRightHeavy.rects)).toBe(0.75)
    expect(upperRightHeavy.rects).not.toContainEqual([0, 0.5, 0.5, 0.5])
    // ▚ diagonal
    expect(blockGlyph(0x259a)!.rects).toEqual([
      [0, 0, 0.5, 0.5],
      [0.5, 0.5, 0.5, 0.5],
    ])
  })

  it("covers every codepoint in the block range", () => {
    for (let cp = 0x2580; cp <= 0x259f; cp++) {
      const glyph = blockGlyph(cp)
      expect(glyph, `U+${cp.toString(16)}`).not.toBeNull()
      expect(glyph!.rects.length).toBeGreaterThan(0)
    }
  })
})

const FLAG_UNDERLINE = 4
const FLAG_STRIKETHROUGH = 8
const FLAG_INVERSE = 16
const FLAG_INVISIBLE = 32
const FLAG_FAINT = 128

interface FillCall {
  x: number
  y: number
  w: number
  h: number
  fillStyle: string
  alpha: number
}

function makeCell(codepoint: number, flags = 0, hyperlinkId = 0) {
  return {
    codepoint,
    flags,
    width: 1,
    grapheme_len: 0,
    hyperlink_id: hyperlinkId,
    fg_r: 255,
    fg_g: 100,
    fg_b: 0,
    bg_r: 0,
    bg_g: 0,
    bg_b: 0,
  }
}

function makeRenderer({ selected = false } = {}) {
  const fillRects: FillCall[] = []
  const textCalls: number[] = []
  const ctx = {
    fillStyle: "",
    globalAlpha: 1,
    fillRect(x: number, y: number, w: number, h: number) {
      fillRects.push({ x, y, w, h, fillStyle: this.fillStyle, alpha: this.globalAlpha })
    },
  }
  const renderer = {
    ctx,
    metrics: { width: 9, height: 19 },
    theme: { selectionForeground: "#abcdef" },
    renderCellText(cell: { codepoint: number }) {
      textCalls.push(cell.codepoint)
    },
    isInSelection: () => selected,
  }
  return { renderer, ctx, fillRects, textCalls }
}

describe("patchBlockGlyphs", () => {
  it("draws block characters as pixel-snapped rects using the cell foreground", () => {
    const { renderer, fillRects, textCalls } = makeRenderer()
    patchBlockGlyphs(renderer)
    renderer.renderCellText(makeCell(0x2590), 2, 1) // ▐ at col 2, row 1

    expect(textCalls).toEqual([])
    expect(fillRects).toHaveLength(1)
    const rect = fillRects[0]
    // Cell spans x 18..27, y 19..38; right half starts at round(18 + 4.5) = 23.
    expect(rect).toMatchObject({ x: 23, y: 19, w: 4, h: 19 })
    expect(rect.fillStyle).toBe("rgb(255, 100, 0)")
    expect(rect.alpha).toBe(1)
  })

  it("delegates non-block characters and grapheme clusters to the original", () => {
    const { renderer, fillRects, textCalls } = makeRenderer()
    patchBlockGlyphs(renderer)
    renderer.renderCellText(makeCell(0x41), 0, 0) // 'A'
    const cluster = { ...makeCell(0x2588), grapheme_len: 2 }
    renderer.renderCellText(cluster, 1, 0)

    expect(fillRects).toEqual([])
    expect(textCalls).toEqual([0x41, 0x2588])
  })

  it("draws nothing for invisible cells", () => {
    const { renderer, fillRects, textCalls } = makeRenderer()
    patchBlockGlyphs(renderer)
    renderer.renderCellText(makeCell(0x2588, FLAG_INVISIBLE), 0, 0)

    expect(fillRects).toEqual([])
    expect(textCalls).toEqual([])
  })

  it("uses the background color for inverse cells", () => {
    const { renderer, fillRects } = makeRenderer()
    patchBlockGlyphs(renderer)
    renderer.renderCellText(makeCell(0x2588, FLAG_INVERSE), 0, 0)

    expect(fillRects[0].fillStyle).toBe("rgb(0, 0, 0)")
  })

  it("halves the alpha for faint cells and restores it afterwards", () => {
    const { renderer, ctx, fillRects } = makeRenderer()
    patchBlockGlyphs(renderer)
    renderer.renderCellText(makeCell(0x2592, FLAG_FAINT), 0, 0) // ▒ faint

    expect(fillRects[0].alpha).toBe(0.25)
    expect(ctx.globalAlpha).toBe(1)
  })

  it("uses the selection foreground inside a selection", () => {
    const { renderer, fillRects } = makeRenderer({ selected: true })
    patchBlockGlyphs(renderer)
    renderer.renderCellText(makeCell(0x2588), 0, 0)

    expect(fillRects[0].fillStyle).toBe("#abcdef")
  })

  it("skips blank cells entirely (space and empty codepoint)", () => {
    const { renderer, fillRects, textCalls } = makeRenderer()
    patchBlockGlyphs(renderer)
    renderer.renderCellText(makeCell(32), 0, 0)
    renderer.renderCellText(makeCell(0), 1, 0)

    expect(fillRects).toEqual([])
    expect(textCalls).toEqual([])
  })

  it("still delegates decorated or linked spaces to the original", () => {
    const { renderer, textCalls } = makeRenderer()
    patchBlockGlyphs(renderer)
    renderer.renderCellText(makeCell(32, FLAG_UNDERLINE), 0, 0)
    renderer.renderCellText(makeCell(32, FLAG_STRIKETHROUGH), 1, 0)
    renderer.renderCellText(makeCell(32, 0, 7), 2, 0)
    const spaceCluster = { ...makeCell(32), grapheme_len: 1 }
    renderer.renderCellText(spaceCluster, 3, 0)

    expect(textCalls).toEqual([32, 32, 32, 32])
  })
})
