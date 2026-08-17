// The xterm and document side of a live terminal: the cell metrics the grid fit
// needs, the mouse encoding a snapshot cannot carry, and the font the renderer
// measures against. Two of the four reach into xterm privates, and every one of
// them needs a real Terminal or a real document — which is why this file is the
// boundary invariant #1 exempts and is excluded from the coverage denominator
// beside term-transport and term-perf. The arithmetic under the fit is
// computeGrid's, in term-fit.ts, and that part is covered.

import type { Terminal } from "@xterm/xterm"
import { computeGrid } from "./term-fit"

// Size used only to ask the font loader for the face; the face is the same at
// any size, and the terminal's real size is the terminalFontSize setting.
const FONT_PROBE_SIZE = 14

// Left gutter so the first column doesn't sit flush against the sidebar/panel
// seam. Subtracted before the grid fit below so it doesn't cost a column.
export const TERMINAL_PADDING_LEFT = 4

// cellDimensions reads the renderer's measured cell size — the same private
// API FitAddon relies on ("TODO: Remove reliance" upstream). Null before the
// first render measure or if xterm ever moves the private; refit then skips,
// keeping the current grid (degrades, never breaks).
export function cellDimensions(term: Terminal): { width: number; height: number } | null {
  const core = (
    term as unknown as {
      _core?: {
        _renderService?: { dimensions?: { css?: { cell?: { width: number; height: number } } } }
      }
    }
  )._core
  const cell = core?._renderService?.dimensions?.css?.cell
  if (!cell || !cell.width || !cell.height) {
    return null
  }
  return cell
}

// mouseEncoding reads which encoding the app selected for mouse reports —
// another private the public API does not expose (term.modes carries the
// protocol only). Undefined if xterm ever moves it, which costs the restore in
// term-modes.ts and nothing else.
export function mouseEncoding(term: Terminal): string | undefined {
  return (term as unknown as { _core?: { coreMouseService?: { activeEncoding?: string } } })._core
    ?.coreMouseService?.activeEncoding
}

// fitTerminal resizes the grid to fill the container edge to edge on the
// right/bottom (replacing xterm's FitAddon, which reserves a scrollbar
// gutter on the right — see term-fit.ts), minus the fixed left gutter above.
// No-op when metrics or size aren't ready, or the grid already fits.
export function fitTerminal(term: Terminal, container: HTMLElement): void {
  const cell = cellDimensions(term)
  if (!cell) {
    return
  }
  const grid = computeGrid(
    container.clientWidth - TERMINAL_PADDING_LEFT,
    container.clientHeight,
    cell,
  )
  if (grid && (grid.cols !== term.cols || grid.rows !== term.rows)) {
    term.resize(grid.cols, grid.rows)
  }
}

// ensureFontLoaded blocks until a font is available. The renderer measures
// cell metrics at open, so bundled fonts (@font-face) must be loaded first;
// system fonts resolve as no-ops and failures fall back to monospace.
export async function ensureFontLoaded(font: string): Promise<void> {
  try {
    await Promise.all([
      document.fonts.load(`${FONT_PROBE_SIZE}px "${font}"`),
      document.fonts.load(`bold ${FONT_PROBE_SIZE}px "${font}"`),
    ])
  } catch {
    // Fall back to the system monospace face.
  }
}
