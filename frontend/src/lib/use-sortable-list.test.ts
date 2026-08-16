import { describe, expect, it } from "vitest"
import type { Modifier } from "@dnd-kit/core"
import { withinList } from "./use-sortable-list"

type Args = Parameters<Modifier>[0]

const rect = (top: number, height: number, left = 0, width = 200) => ({
  top,
  left,
  width,
  height,
  right: left + width,
  bottom: top + height,
})

// A list of three 100px rows starting at y=100, and the middle one dragged.
const args = (
  transform: { x: number; y: number },
  dragged = rect(200, 100),
  container = rect(100, 300),
) =>
  ({
    containerNodeRect: container,
    draggingNodeRect: dragged,
    transform: { ...transform, scaleX: 1, scaleY: 1 },
  }) as unknown as Args

describe("withinList", () => {
  it("leaves a drag that stays inside the list untouched", () => {
    expect(withinList(args({ x: 0, y: 50 }))).toMatchObject({ x: 0, y: 50 })
    expect(withinList(args({ x: 0, y: -50 }))).toMatchObject({ x: 0, y: -50 })
  })

  it("stops a drag at the end of the list instead of letting it leave", () => {
    // The row's bottom (300) reaches the container's (400) after 100px.
    expect(withinList(args({ x: 0, y: 400 }))).toMatchObject({ y: 100 })
    // And its top (200) reaches the container's (100) after -100px.
    expect(withinList(args({ x: 0, y: -400 }))).toMatchObject({ y: -100 })
  })

  it("clamps horizontally too, for the tab strip", () => {
    // A 100px tab sitting at x=300 in a strip that runs from 0 to 600.
    const strip = rect(0, 40, 0, 600)
    const tab = rect(0, 40, 300, 100)
    expect(withinList(args({ x: 500, y: 0 }, tab, strip))).toMatchObject({ x: 200 })
    expect(withinList(args({ x: -500, y: 0 }, tab, strip))).toMatchObject({ x: -300 })
  })

  it("passes the transform through before the rects are measured", () => {
    const unmeasured = {
      containerNodeRect: null,
      draggingNodeRect: null,
      transform: { x: 4, y: 900, scaleX: 1, scaleY: 1 },
    } as unknown as Args
    expect(withinList(unmeasured)).toMatchObject({ x: 4, y: 900 })
  })
})
