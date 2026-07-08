// ghostty-web sizes terminal cells from the ink bounds of "M" plus a 2px fudge.
// Ink bounds ≈ cap height, so cells come out ~20% shorter than the font's real
// line height and the grid is squatter than any native terminal — block art
// (e.g. the Claude Code logo) renders horizontally stretched. Measure with the
// font bounding box (full ascent + descent) instead, matching how native
// terminals derive line height.

export interface CellMetrics {
  width: number
  height: number
  baseline: number
}

interface MeasurableRenderer {
  fontSize: number
  fontFamily: string
  measureFont(): CellMetrics
  remeasureFont(): void
}

/**
 * Cell metrics from a TextMetrics measurement of "M", or null when the
 * browser does not report font bounding boxes (falls back to the original).
 */
export function metricsFromMeasurement(measure: {
  width: number
  fontBoundingBoxAscent?: number
  fontBoundingBoxDescent?: number
}): CellMetrics | null {
  const ascent = measure.fontBoundingBoxAscent
  const descent = measure.fontBoundingBoxDescent
  if (!ascent || descent === undefined) {
    return null
  }
  return {
    width: Math.ceil(measure.width),
    height: Math.ceil(ascent + descent),
    baseline: Math.round(ascent),
  }
}

function measureWithCanvas(fontSize: number, fontFamily: string): TextMetrics {
  const ctx = document.createElement("canvas").getContext("2d")!
  ctx.font = `${fontSize}px ${fontFamily}`
  return ctx.measureText("M")
}

/**
 * Replaces the renderer's measureFont with a font-bounding-box based
 * implementation and remeasures immediately. remeasureFont (used on font
 * changes) picks up the override, so later font switches stay correct.
 */
export function patchFontMetrics(
  renderer: unknown,
  measure: (fontSize: number, fontFamily: string) => TextMetrics = measureWithCanvas,
): void {
  const target = renderer as MeasurableRenderer
  const original = target.measureFont.bind(target)
  target.measureFont = () => {
    const metrics = metricsFromMeasurement(measure(target.fontSize, target.fontFamily))
    return metrics ?? original()
  }
  target.remeasureFont()
}
