import { describe, expect, it } from "vitest"
import { metricsFromMeasurement, patchFontMetrics } from "./font-metrics"

describe("metricsFromMeasurement", () => {
  it("derives cell size from the font bounding box", () => {
    const metrics = metricsFromMeasurement({
      width: 8.4,
      fontBoundingBoxAscent: 14.3,
      fontBoundingBoxDescent: 4.2,
    })
    expect(metrics).toEqual({ width: 9, height: 19, baseline: 14 })
  })

  it("returns null when the browser lacks font bounding box support", () => {
    expect(metricsFromMeasurement({ width: 8.4 })).toBeNull()
    expect(
      metricsFromMeasurement({ width: 8.4, fontBoundingBoxAscent: 0, fontBoundingBoxDescent: 3 }),
    ).toBeNull()
  })
})

function makeRenderer(measured: Partial<TextMetrics>) {
  let measureCalls = 0
  const renderer = {
    fontSize: 14,
    fontFamily: '"Test Font", monospace',
    metrics: { width: 9, height: 15, baseline: 11 },
    measureFont() {
      measureCalls++
      return { width: 9, height: 15, baseline: 11 }
    },
    remeasureFont() {
      this.metrics = this.measureFont()
    },
  }
  const requested: Array<[number, string]> = []
  const measure = (fontSize: number, fontFamily: string) => {
    requested.push([fontSize, fontFamily])
    return measured as TextMetrics
  }
  return { renderer, measure, requested, originalCalls: () => measureCalls }
}

describe("patchFontMetrics", () => {
  it("remeasures immediately with bounding-box metrics", () => {
    const { renderer, measure, requested, originalCalls } = makeRenderer({
      width: 8.4,
      fontBoundingBoxAscent: 14.3,
      fontBoundingBoxDescent: 4.2,
    })
    patchFontMetrics(renderer, measure)

    expect(renderer.metrics).toEqual({ width: 9, height: 19, baseline: 14 })
    expect(requested).toEqual([[14, '"Test Font", monospace']])
    expect(originalCalls()).toBe(0)
  })

  it("keeps the override across later remeasures (font changes)", () => {
    const { renderer, measure } = makeRenderer({
      width: 8.4,
      fontBoundingBoxAscent: 14.3,
      fontBoundingBoxDescent: 4.2,
    })
    patchFontMetrics(renderer, measure)
    renderer.metrics = { width: 0, height: 0, baseline: 0 }
    renderer.remeasureFont()

    expect(renderer.metrics).toEqual({ width: 9, height: 19, baseline: 14 })
  })

  it("falls back to the original measurement without bounding box support", () => {
    const { renderer, measure, originalCalls } = makeRenderer({ width: 8.4 })
    patchFontMetrics(renderer, measure)

    expect(renderer.metrics).toEqual({ width: 9, height: 15, baseline: 11 })
    expect(originalCalls()).toBe(1)
  })
})
