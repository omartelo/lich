// fillText is the most expensive canvas call in WebKit (~12µs per glyph:
// shaping + raster on every call). ghostty-web issues one per non-blank cell,
// every frame. This patch caches each rendered glyph on a small offscreen
// canvas — keyed by codepoint, font style, faintness, cell width and resolved
// color — and blits it with drawImage (~1-3µs). Only plain single-codepoint
// cells are cached; graphemes/emoji, decorations (underline/strikethrough) and
// hyperlink hover delegate to the original renderCellText. Touches ghostty-web
// privates (renderCellText, ctx, metrics, theme, fontSize, fontFamily,
// devicePixelRatio, hovered*) — revalidate when bumping the pinned 0.4.0.

const FLAG_BOLD = 1
const FLAG_ITALIC = 2
const FLAG_UNDERLINE = 4
const FLAG_STRIKETHROUGH = 8
const FLAG_INVERSE = 16
const FLAG_INVISIBLE = 32
const FLAG_FAINT = 128

const BLOCK_RANGE_START = 0x2580
const BLOCK_RANGE_END = 0x259f

// Overflow resets the whole cache; an LRU only earns its keep if a real session
// ever cycles >4096 live glyph+color combos.
const MAX_SPRITES = 4096

interface AtlasCell {
  codepoint: number
  flags: number
  width: number
  grapheme_len: number
  hyperlink_id: number
  fg_r: number
  fg_g: number
  fg_b: number
  bg_r: number
  bg_g: number
  bg_b: number
}

interface AtlasRenderer {
  ctx: CanvasRenderingContext2D
  metrics: { width: number; height: number; baseline: number }
  fontSize: number
  fontFamily: string
  devicePixelRatio: number
  theme: { selectionForeground: string }
  hoveredHyperlinkId: unknown
  hoveredLinkRange: unknown
  isInSelection(x: number, y: number): boolean
  rgbToCSS(r: number, g: number, b: number): string
  renderCellText(cell: AtlasCell, x: number, y: number): void
}

// isCacheable gates the fast path: anything the original draws beyond a plain
// colored glyph (decorations, clusters, link underlines) falls through.
function isCacheable(renderer: AtlasRenderer, cell: AtlasCell): boolean {
  return (
    cell.grapheme_len === 0 &&
    cell.codepoint > 32 &&
    !(cell.codepoint >= BLOCK_RANGE_START && cell.codepoint <= BLOCK_RANGE_END) &&
    (cell.flags & (FLAG_UNDERLINE | FLAG_STRIKETHROUGH)) === 0 &&
    cell.hyperlink_id === 0 &&
    !renderer.hoveredHyperlinkId &&
    !renderer.hoveredLinkRange
  )
}

// cellForeground mirrors the original renderCellText color choice.
function cellForeground(renderer: AtlasRenderer, cell: AtlasCell, x: number, y: number): string {
  if (renderer.isInSelection(x, y)) {
    return renderer.theme.selectionForeground
  }
  const inverse = cell.flags & FLAG_INVERSE
  return inverse
    ? renderer.rgbToCSS(cell.bg_r, cell.bg_g, cell.bg_b)
    : renderer.rgbToCSS(cell.fg_r, cell.fg_g, cell.fg_b)
}

/**
 * Wraps the renderer's renderCellText with a glyph sprite cache. makeCanvas is
 * injectable for tests; production uses a plain DOM canvas.
 */
export function patchGlyphAtlas(
  renderer: unknown,
  makeCanvas: () => HTMLCanvasElement = () => document.createElement("canvas"),
): void {
  const target = renderer as AtlasRenderer
  const original = target.renderCellText.bind(target)
  const sprites = new Map<string, HTMLCanvasElement>()
  // Snapshot of everything baked into the sprites; when it changes (font
  // switch, remeasure) every sprite is stale.
  let spriteMetrics = target.metrics
  let spriteFont = `${target.fontSize}px ${target.fontFamily}`

  const makeSprite = (cell: AtlasCell, fontStyle: string, color: string): HTMLCanvasElement | null => {
    const { width: cellWidth, height: cellHeight, baseline } = target.metrics
    const dpr = target.devicePixelRatio || 1
    // Padding absorbs ink that overhangs the cell box (italic lean,
    // descenders); the blit offsets by the same amount.
    const padX = Math.ceil(cellWidth / 2)
    const padY = Math.ceil(cellHeight / 4)
    const w = cellWidth * cell.width + padX * 2
    const h = cellHeight + padY * 2

    const canvas = makeCanvas()
    canvas.width = w * dpr
    canvas.height = h * dpr
    const ctx = canvas.getContext("2d")
    if (!ctx) {
      return null
    }
    ctx.scale(dpr, dpr)
    ctx.font = `${fontStyle}${target.fontSize}px ${target.fontFamily}`
    ctx.textBaseline = "alphabetic"
    ctx.textAlign = "left"
    ctx.fillStyle = color
    if (cell.flags & FLAG_FAINT) {
      ctx.globalAlpha = 0.5
    }
    ctx.fillText(String.fromCodePoint(cell.codepoint), padX, padY + baseline)
    return canvas
  }

  target.renderCellText = (cell, x, y) => {
    if (cell.flags & FLAG_INVISIBLE) {
      return
    }
    if (!isCacheable(target, cell)) {
      original(cell, x, y)
      return
    }

    const currentFont = `${target.fontSize}px ${target.fontFamily}`
    if (target.metrics !== spriteMetrics || currentFont !== spriteFont) {
      sprites.clear()
      spriteMetrics = target.metrics
      spriteFont = currentFont
    }

    let fontStyle = ""
    if (cell.flags & FLAG_ITALIC) {
      fontStyle += "italic "
    }
    if (cell.flags & FLAG_BOLD) {
      fontStyle += "bold "
    }
    const color = cellForeground(target, cell, x, y)
    const key = `${cell.codepoint}|${fontStyle}|${cell.flags & FLAG_FAINT}|${cell.width}|${color}`

    let sprite = sprites.get(key)
    if (!sprite) {
      const made = makeSprite(cell, fontStyle, color)
      if (!made) {
        original(cell, x, y)
        return
      }
      if (sprites.size >= MAX_SPRITES) {
        sprites.clear()
      }
      sprites.set(key, made)
      sprite = made
    }

    const { width: cellWidth, height: cellHeight } = target.metrics
    const padX = Math.ceil(cellWidth / 2)
    const padY = Math.ceil(cellHeight / 4)
    target.ctx.drawImage(
      sprite,
      x * cellWidth - padX,
      y * cellHeight - padY,
      cellWidth * cell.width + padX * 2,
      cellHeight + padY * 2,
    )
  }
}
