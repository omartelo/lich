import { describe, expect, it } from "vitest"
import { layoutHighlights, splitLeadIn } from "./patch-notes-layout"

describe("splitLeadIn", () => {
  it("splits the bold lead-in from the paragraph behind it", () => {
    expect(splitLeadIn("**A thing.** It does `stuff` now.")).toEqual({
      headline: "A thing.",
      body: "It does `stuff` now.",
    })
  })

  it("keeps an item that is only its lead-in as a headline with no body", () => {
    expect(splitLeadIn("**A thing.**")).toEqual({ headline: "A thing.", body: "" })
  })

  it("keeps an item without a lead-in whole", () => {
    expect(splitLeadIn("Plain item.")).toEqual({ headline: "Plain item.", body: "" })
    expect(splitLeadIn("**Unclosed bold")).toEqual({ headline: "**Unclosed bold", body: "" })
  })
})

describe("layoutHighlights", () => {
  const important = { kind: "important", text: "The headline." }
  const note = { kind: "note", text: "A detail." }
  const warning = { kind: "warning", text: "Do this." }

  it("leads with the first important block and keeps the rest in order", () => {
    expect(layoutHighlights([note, important, warning])).toEqual({
      lead: important,
      callouts: [note, warning],
    })
  })

  it("demotes a second important block to a callout", () => {
    const second = { kind: "important", text: "Another headline." }
    expect(layoutHighlights([important, second])).toEqual({ lead: important, callouts: [second] })
  })

  it("has no lead without an important block", () => {
    expect(layoutHighlights([note])).toEqual({ lead: null, callouts: [note] })
    expect(layoutHighlights(null)).toEqual({ lead: null, callouts: [] })
  })
})
