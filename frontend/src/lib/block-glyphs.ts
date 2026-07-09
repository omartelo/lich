// ghostty-web's canvas renderer draws every glyph with ctx.fillText, so Unicode
// block elements (U+2580–U+259F) inherit the font's glyph box — which never
// tiles the renderer's cell grid exactly (cell width/height are rounded up from
// font metrics, plus 2px vertical padding). Adjacent blocks show seams and
// half-block art (e.g. the Claude Code logo) turns into stripes. Native
// terminals synthesize these glyphs as rectangles; this module does the same by
// wrapping the renderer's private renderCellText.

const BLOCK_RANGE_START = 0x2580
const BLOCK_RANGE_END = 0x259f

const FLAG_INVERSE = 16
const FLAG_INVISIBLE = 32
const FLAG_FAINT = 128

/** Rectangle in unit-cell coordinates: [x, y, width, height], each in 0..1. */
export type UnitRect = readonly [number, number, number, number]

export interface BlockGlyph {
  rects: readonly UnitRect[]
  /** Fill opacity — used by the shade characters ░ ▒ ▓. */
  alpha: number
}

// Quadrant bitmasks for U+2596–U+259F: upper-left=1, upper-right=2,
// lower-left=4, lower-right=8.
const QUADRANT_BITS = [
  0b0100, // ▖ U+2596
  0b1000, // ▗ U+2597
  0b0001, // ▘ U+2598
  0b1101, // ▙ U+2599
  0b1001, // ▚ U+259A
  0b0111, // ▛ U+259B
  0b1011, // ▜ U+259C
  0b0010, // ▝ U+259D
  0b0110, // ▞ U+259E
  0b1110, // ▟ U+259F
] as const

const QUADRANT_RECTS: readonly UnitRect[] = [
  [0, 0, 0.5, 0.5],
  [0.5, 0, 0.5, 0.5],
  [0, 0.5, 0.5, 0.5],
  [0.5, 0.5, 0.5, 0.5],
]

const FULL_CELL: UnitRect = [0, 0, 1, 1]

function quadrantGlyph(bits: number): BlockGlyph {
  return {
    rects: QUADRANT_RECTS.filter((_, i) => bits & (1 << i)),
    alpha: 1,
  }
}

/**
 * Geometry for a block-element codepoint, or null when the character is not a
 * block element and should be drawn by the font.
 */
export function blockGlyph(codepoint: number): BlockGlyph | null {
  if (codepoint < BLOCK_RANGE_START || codepoint > BLOCK_RANGE_END) {
    return null
  }
  // ▁▂▃▄▅▆▇█ — lower one-eighth through full block.
  if (codepoint >= 0x2581 && codepoint <= 0x2588) {
    const fraction = (codepoint - 0x2580) / 8
    return { rects: [[0, 1 - fraction, 1, fraction]], alpha: 1 }
  }
  // ▉▊▋▌▍▎▏ — left seven-eighths through left one-eighth.
  if (codepoint >= 0x2589 && codepoint <= 0x258f) {
    const fraction = (0x2590 - codepoint) / 8
    return { rects: [[0, 0, fraction, 1]], alpha: 1 }
  }
  if (codepoint >= 0x2596) {
    return quadrantGlyph(QUADRANT_BITS[codepoint - 0x2596])
  }
  switch (codepoint) {
    case 0x2580: // ▀ upper half
      return { rects: [[0, 0, 1, 0.5]], alpha: 1 }
    case 0x2590: // ▐ right half
      return { rects: [[0.5, 0, 0.5, 1]], alpha: 1 }
    case 0x2591: // ░ light shade
      return { rects: [FULL_CELL], alpha: 0.25 }
    case 0x2592: // ▒ medium shade
      return { rects: [FULL_CELL], alpha: 0.5 }
    case 0x2593: // ▓ dark shade
      return { rects: [FULL_CELL], alpha: 0.75 }
    case 0x2594: // ▔ upper one eighth
      return { rects: [[0, 0, 1, 1 / 8]], alpha: 1 }
    default: // 0x2595 ▕ right one eighth
      return { rects: [[7 / 8, 0, 1 / 8, 1]], alpha: 1 }
  }
}

// Shapes of ghostty-web's private renderer internals we rely on. Kept minimal:
// only the fields the wrapper touches.
interface RenderedCell {
  codepoint: number
  flags: number
  width: number
  grapheme_len: number
  fg_r: number
  fg_g: number
  fg_b: number
  bg_r: number
  bg_g: number
  bg_b: number
}

interface PatchableRenderer {
  ctx: CanvasRenderingContext2D
  metrics: { width: number; height: number }
  theme: { selectionForeground: string }
  renderCellText(cell: RenderedCell, x: number, y: number): void
  isInSelection(x: number, y: number): boolean
}

function fillBlock(
  renderer: PatchableRenderer,
  cell: RenderedCell,
  glyph: BlockGlyph,
  x: number,
  y: number,
): void {
  const { ctx, metrics } = renderer
  if (renderer.isInSelection(x, y)) {
    ctx.fillStyle = renderer.theme.selectionForeground
  } else {
    const inverse = cell.flags & FLAG_INVERSE
    const [r, g, b] = inverse
      ? [cell.bg_r, cell.bg_g, cell.bg_b]
      : [cell.fg_r, cell.fg_g, cell.fg_b]
    ctx.fillStyle = `rgb(${r}, ${g}, ${b})`
  }
  ctx.globalAlpha = cell.flags & FLAG_FAINT ? glyph.alpha * 0.5 : glyph.alpha
  const cellWidth = metrics.width * cell.width
  const px = x * metrics.width
  const py = y * metrics.height
  for (const [rx, ry, rw, rh] of glyph.rects) {
    // Round each edge (not width/height) so adjacent cells share exact pixel
    // boundaries — no anti-aliased seams between tiled blocks.
    const x0 = Math.round(px + rx * cellWidth)
    const y0 = Math.round(py + ry * metrics.height)
    const x1 = Math.round(px + (rx + rw) * cellWidth)
    const y1 = Math.round(py + (ry + rh) * metrics.height)
    ctx.fillRect(x0, y0, x1 - x0, y1 - y0)
  }
  ctx.globalAlpha = 1
}

/**
 * Wraps the renderer's renderCellText so block-element characters are drawn as
 * exact cell-filling rectangles instead of font glyphs. Everything else
 * delegates to the original implementation.
 */
export function patchBlockGlyphs(renderer: unknown): void {
  const target = renderer as PatchableRenderer
  const original = target.renderCellText.bind(target)
  target.renderCellText = (cell, x, y) => {
    const glyph = cell.grapheme_len > 0 ? null : blockGlyph(cell.codepoint)
    if (!glyph) {
      original(cell, x, y)
      return
    }
    if (cell.flags & FLAG_INVISIBLE) {
      return
    }
    fillBlock(target, cell, glyph, x, y)
  }
}
