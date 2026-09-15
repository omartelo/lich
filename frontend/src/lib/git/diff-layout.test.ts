import { describe, expect, it } from "vitest"
import { shownLayout } from "./diff-layout"

describe("shownLayout", () => {
  it("draws split only on a card measured wide enough", () => {
    expect(shownLayout("split", true)).toBe("split")
    expect(shownLayout("split", false)).toBe("unified")
    expect(shownLayout("split", null)).toBe("unified")
  })

  it("never turns unified into split", () => {
    expect(shownLayout("unified", true)).toBe("unified")
  })
})
