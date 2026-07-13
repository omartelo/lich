import { describe, expect, it } from "vitest"
import { isStrayTerminalChild } from "./term-dom"

describe("isStrayTerminalChild", () => {
  it("keeps the canvas + textarea ghostty owns", () => {
    expect(isStrayTerminalChild({ nodeName: "CANVAS" })).toBe(false)
    expect(isStrayTerminalChild({ nodeName: "TEXTAREA" })).toBe(false)
  })

  it("flags editable nodes WebKitGTK slips in (drop, middle-click)", () => {
    expect(isStrayTerminalChild({ nodeName: "DIV" })).toBe(true)
    expect(isStrayTerminalChild({ nodeName: "SPAN" })).toBe(true)
    expect(isStrayTerminalChild({ nodeName: "BR" })).toBe(true)
    // Middle-click primary-selection paste drops a bare text node.
    expect(isStrayTerminalChild({ nodeName: "#text" })).toBe(true)
  })
})
