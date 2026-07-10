import { describe, expect, it } from "vitest"
import { patchGlyphAtlas } from "./glyph-atlas"

const FLAG_BOLD = 1
const FLAG_UNDERLINE = 4
const FLAG_INVERSE = 16
const FLAG_INVISIBLE = 32
const FLAG_FAINT = 128

interface SpriteCtx {
  font: string
  fillStyle: string
  globalAlpha: number
  textBaseline: string
  textAlign: string
  scales: Array<[number, number]>
  texts: Array<{ text: string; x: number; y: number }>
}

function makeFakeCanvas() {
  const ctx: SpriteCtx = {
    font: "",
    fillStyle: "",
    globalAlpha: 1,
    textBaseline: "",
    textAlign: "",
    scales: [],
    texts: [],
  }
  const canvas = {
    width: 0,
    height: 0,
    ctx,
    getContext: () => ({
      ...ctx,
      scale: (sx: number, sy: number) => ctx.scales.push([sx, sy]),
      fillText: (text: string, x: number, y: number) => ctx.texts.push({ text, x, y }),
      set font(v: string) {
        ctx.font = v
      },
      set fillStyle(v: string) {
        ctx.fillStyle = v
      },
      set globalAlpha(v: number) {
        ctx.globalAlpha = v
      },
      set textBaseline(v: string) {
        ctx.textBaseline = v
      },
      set textAlign(v: string) {
        ctx.textAlign = v
      },
    }),
  }
  return canvas as unknown as HTMLCanvasElement & { ctx: SpriteCtx }
}

function makeCell(codepoint: number, { flags = 0, width = 1, hyperlinkId = 0, graphemeLen = 0 } = {}) {
  return {
    codepoint,
    flags,
    width,
    grapheme_len: graphemeLen,
    hyperlink_id: hyperlinkId,
    fg_r: 200,
    fg_g: 100,
    fg_b: 50,
    bg_r: 1,
    bg_g: 2,
    bg_b: 3,
  }
}

function makeRenderer() {
  const draws: Array<{ dx: number; dy: number; w: number; h: number }> = []
  const originals: number[] = []
  const created: Array<ReturnType<typeof makeFakeCanvas>> = []
  const renderer = {
    ctx: {
      drawImage: (_img: unknown, dx: number, dy: number, w: number, h: number) => {
        draws.push({ dx, dy, w, h })
      },
    },
    metrics: { width: 10, height: 20, baseline: 16 },
    fontSize: 14,
    fontFamily: '"Mono"',
    devicePixelRatio: 2,
    theme: { selectionForeground: "#sel" },
    hoveredHyperlinkId: null as unknown,
    hoveredLinkRange: null as unknown,
    isInSelection: () => false,
    rgbToCSS: (r: number, g: number, b: number) => `rgb(${r}, ${g}, ${b})`,
    renderCellText(cell: { codepoint: number }, _x: number, _y: number) {
      originals.push(cell.codepoint)
    },
  }
  patchGlyphAtlas(renderer, () => {
    const canvas = makeFakeCanvas()
    created.push(canvas)
    return canvas
  })
  return { renderer, draws, originals, created }
}

describe("patchGlyphAtlas", () => {
  it("draws a plain glyph from a sprite and reuses it", () => {
    const { renderer, draws, originals, created } = makeRenderer()
    renderer.renderCellText(makeCell(65), 3, 2)
    renderer.renderCellText(makeCell(65), 4, 2)

    expect(originals).toEqual([])
    expect(created).toHaveLength(1)
    expect(draws).toHaveLength(2)
    // padX=5, padY=5: dest = (3*10-5, 2*20-5, 10+10, 20+10)
    expect(draws[0]).toEqual({ dx: 25, dy: 35, w: 20, h: 30 })
    expect(draws[1].dx).toBe(35)
  })

  it("renders the sprite with font, color, dpr scale and baseline offset", () => {
    const { renderer, created } = makeRenderer()
    renderer.renderCellText(makeCell(65, { flags: FLAG_BOLD | FLAG_FAINT }), 0, 0)

    const sprite = created[0]
    expect(sprite.width).toBe(40) // (10 + 2*5) * dpr 2
    expect(sprite.height).toBe(60) // (20 + 2*5) * dpr 2
    expect(sprite.ctx.scales).toEqual([[2, 2]])
    expect(sprite.ctx.font).toBe('bold 14px "Mono"')
    expect(sprite.ctx.fillStyle).toBe("rgb(200, 100, 50)")
    expect(sprite.ctx.globalAlpha).toBe(0.5)
    expect(sprite.ctx.texts).toEqual([{ text: "A", x: 5, y: 21 }]) // padX, padY + baseline
  })

  it("keys sprites by style and resolved color", () => {
    const { renderer, created } = makeRenderer()
    renderer.renderCellText(makeCell(65), 0, 0)
    renderer.renderCellText(makeCell(65, { flags: FLAG_BOLD }), 1, 0)
    renderer.renderCellText(makeCell(65, { flags: FLAG_INVERSE }), 2, 0) // color = bg
    renderer.renderCellText(makeCell(65, { width: 2 }), 3, 0)

    expect(created).toHaveLength(4)
    expect(created[2].ctx.fillStyle).toBe("rgb(1, 2, 3)")
  })

  it("delegates non-cacheable cells to the original", () => {
    const { renderer, draws, originals } = makeRenderer()
    renderer.renderCellText(makeCell(65, { flags: FLAG_UNDERLINE }), 0, 0)
    renderer.renderCellText(makeCell(65, { hyperlinkId: 9 }), 1, 0)
    renderer.renderCellText(makeCell(0x1f600, { graphemeLen: 1 }), 2, 0)
    renderer.renderCellText(makeCell(0x2588), 3, 0) // block glyph: downstream patch
    renderer.renderCellText(makeCell(32), 4, 0) // blank: downstream skip

    expect(draws).toEqual([])
    expect(originals).toEqual([65, 65, 0x1f600, 0x2588, 32])
  })

  it("delegates every cell while a link hover is active", () => {
    const { renderer, draws, originals } = makeRenderer()
    renderer.hoveredLinkRange = { startY: 0 }
    renderer.renderCellText(makeCell(65), 0, 0)

    expect(draws).toEqual([])
    expect(originals).toEqual([65])
  })

  it("draws nothing for invisible cells", () => {
    const { renderer, draws, originals } = makeRenderer()
    renderer.renderCellText(makeCell(65, { flags: FLAG_INVISIBLE }), 0, 0)

    expect(draws).toEqual([])
    expect(originals).toEqual([])
  })

  it("clears the cache when font or metrics change", () => {
    const { renderer, created } = makeRenderer()
    renderer.renderCellText(makeCell(65), 0, 0)
    renderer.metrics = { width: 11, height: 22, baseline: 17 }
    renderer.renderCellText(makeCell(65), 0, 0)

    expect(created).toHaveLength(2)
  })
})
