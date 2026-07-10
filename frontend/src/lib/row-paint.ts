// ghostty-web's renderLine paints cell backgrounds one fillRect per cell —
// under a TUI colorscheme that's every cell, ~100k fillRect/s while scrolling.
// This patch replaces renderLine with a run-batching version: adjacent cells
// with the same effective background become a single fillRect. The text pass
// is unchanged and dispatches through renderCellText, so the block-glyphs /
// blank-skip wrapper still applies. Touches ghostty-web privates (renderLine,
// ctx, metrics, theme, isInSelection) — revalidate when bumping the pinned
// 0.4.0.

const FLAG_INVERSE = 16

interface PaintedCell {
  flags: number
  width: number
  fg_r: number
  fg_g: number
  fg_b: number
  bg_r: number
  bg_g: number
  bg_b: number
}

interface RowRenderer {
  ctx: CanvasRenderingContext2D
  metrics: { width: number; height: number }
  theme: { background: string; selectionBackground: string }
  isInSelection(x: number, y: number): boolean
  rgbToCSS(r: number, g: number, b: number): string
  renderCellText(cell: PaintedCell, x: number, y: number): void
  renderLine(line: PaintedCell[], row: number, cols: number): void
}

// cellBackground mirrors ghostty-web's renderCellBackground color choice:
// selection wins, INVERSE swaps fg/bg, and all-zero rgb means "default
// background, nothing to paint" (already covered by the row clear).
function cellBackground(renderer: RowRenderer, cell: PaintedCell, x: number, y: number): string | null {
  if (renderer.isInSelection(x, y)) {
    return renderer.theme.selectionBackground
  }
  const inverse = cell.flags & FLAG_INVERSE
  const r = inverse ? cell.fg_r : cell.bg_r
  const g = inverse ? cell.fg_g : cell.bg_g
  const b = inverse ? cell.fg_b : cell.bg_b
  if (r === 0 && g === 0 && b === 0) {
    return null
  }
  return renderer.rgbToCSS(r, g, b)
}

/**
 * Replaces the renderer's renderLine with a version that batches adjacent
 * same-background cells into one fillRect per run.
 */
export function patchRowPaint(renderer: unknown): void {
  const target = renderer as RowRenderer

  target.renderLine = (line, row, cols) => {
    const { ctx, metrics } = target
    const py = row * metrics.height

    ctx.fillStyle = target.theme.background
    ctx.fillRect(0, py, cols * metrics.width, metrics.height)

    let runStyle: string | null = null
    let runStartPx = 0
    let runEndPx = 0
    const flush = () => {
      if (runStyle !== null) {
        ctx.fillStyle = runStyle
        ctx.fillRect(runStartPx, py, runEndPx - runStartPx, metrics.height)
        runStyle = null
      }
    }

    for (let x = 0; x < line.length; x++) {
      const cell = line[x]
      if (cell.width === 0) {
        continue
      }
      const style = cellBackground(target, cell, x, row)
      const px = x * metrics.width
      const cellWidth = metrics.width * cell.width
      if (style !== null && style === runStyle && px === runEndPx) {
        runEndPx = px + cellWidth
        continue
      }
      flush()
      if (style !== null) {
        runStyle = style
        runStartPx = px
        runEndPx = px + cellWidth
      }
    }
    flush()

    for (let x = 0; x < line.length; x++) {
      const cell = line[x]
      if (cell.width !== 0) {
        target.renderCellText(cell, x, row)
      }
    }
  }
}
