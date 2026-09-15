import { describe, expect, it } from "vitest"
import { shownLayout, SPLIT_MIN_WIDTH_PX } from "./diff-layout"

describe("shownLayout", () => {
  it("draws split only when the card is wide enough", () => {
    expect(shownLayout("split", SPLIT_MIN_WIDTH_PX)).toBe("split")
    expect(shownLayout("split", SPLIT_MIN_WIDTH_PX - 1)).toBe("unified")
  })

  it("never turns unified into split", () => {
    expect(shownLayout("unified", 4000)).toBe("unified")
  })
})
