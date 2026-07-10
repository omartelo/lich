import { describe, expect, it } from "vitest"
import { patchRowPaint } from "./row-paint"

const FLAG_INVERSE = 16

interface FillCall {
  x: number
  y: number
  w: number
  h: number
  fillStyle: string
}

function makeCell(bg: [number, number, number], { flags = 0, width = 1 } = {}) {
  return {
    flags,
    width,
    fg_r: 10,
    fg_g: 20,
    fg_b: 30,
    bg_r: bg[0],
    bg_g: bg[1],
    bg_b: bg[2],
  }
}

function makeRenderer({ selectedCols = [] as number[] } = {}) {
  const fillRects: FillCall[] = []
  const textCalls: number[] = []
  const renderer = {
    ctx: {
      fillStyle: "",
      fillRect(x: number, y: number, w: number, h: number) {
        fillRects.push({ x, y, w, h, fillStyle: this.fillStyle })
      },
    },
    metrics: { width: 10, height: 20 },
    theme: { background: "#000", selectionBackground: "#sel" },
    isInSelection: (x: number) => selectedCols.includes(x),
    rgbToCSS: (r: number, g: number, b: number) => `rgb(${r}, ${g}, ${b})`,
    renderCellText(_cell: unknown, x: number) {
      textCalls.push(x)
    },
    renderLine(_line: unknown[], _row: number, _cols: number) {},
  }
  patchRowPaint(renderer)
  return { renderer, fillRects, textCalls }
}

describe("patchRowPaint", () => {
  it("merges a uniform-background row into the clear plus one run", () => {
    const { renderer, fillRects } = makeRenderer()
    const line = Array.from({ length: 5 }, () => makeCell([40, 40, 60]))
    renderer.renderLine(line, 2, 80)

    expect(fillRects).toEqual([
      { x: 0, y: 40, w: 800, h: 20, fillStyle: "#000" },
      { x: 0, y: 40, w: 50, h: 20, fillStyle: "rgb(40, 40, 60)" },
    ])
  })

  it("breaks runs on background changes and skips default-background cells", () => {
    const { renderer, fillRects } = makeRenderer()
    const line = [
      makeCell([1, 1, 1]),
      makeCell([1, 1, 1]),
      makeCell([0, 0, 0]), // default bg: hole in the middle
      makeCell([2, 2, 2]),
    ]
    renderer.renderLine(line, 0, 4)

    expect(fillRects.slice(1)).toEqual([
      { x: 0, y: 0, w: 20, h: 20, fillStyle: "rgb(1, 1, 1)" },
      { x: 30, y: 0, w: 10, h: 20, fillStyle: "rgb(2, 2, 2)" },
    ])
  })

  it("uses the selection background and inverse colors", () => {
    const { renderer, fillRects } = makeRenderer({ selectedCols: [0] })
    const line = [
      makeCell([5, 5, 5]), // selected
      makeCell([0, 0, 0], { flags: FLAG_INVERSE }), // inverse: paints fg color
    ]
    renderer.renderLine(line, 0, 2)

    expect(fillRects.slice(1)).toEqual([
      { x: 0, y: 0, w: 10, h: 20, fillStyle: "#sel" },
      { x: 10, y: 0, w: 10, h: 20, fillStyle: "rgb(10, 20, 30)" },
    ])
  })

  it("keeps a run going across a wide cell and its zero-width spacer", () => {
    const { renderer, fillRects, textCalls } = makeRenderer()
    const line = [
      makeCell([9, 9, 9], { width: 2 }),
      makeCell([9, 9, 9], { width: 0 }), // spacer: no paint, no text
      makeCell([9, 9, 9]),
    ]
    renderer.renderLine(line, 0, 3)

    expect(fillRects.slice(1)).toEqual([{ x: 0, y: 0, w: 30, h: 20, fillStyle: "rgb(9, 9, 9)" }])
    expect(textCalls).toEqual([0, 2])
  })

  it("dispatches the text pass through the current renderCellText", () => {
    const { renderer, textCalls } = makeRenderer()
    const seen: number[] = []
    renderer.renderCellText = (_cell: unknown, x: number) => {
      seen.push(x)
    }
    renderer.renderLine([makeCell([0, 0, 0]), makeCell([0, 0, 0])], 0, 2)

    expect(seen).toEqual([0, 1])
    expect(textCalls).toEqual([])
  })
})
