import { describe, expect, it } from "vitest"
import { formatCost } from "./session-cost"

describe("formatCost", () => {
  it("shows cents once there are dollars to anchor them", () => {
    expect(formatCost(12.476)).toBe("$12.48")
    expect(formatCost(1)).toBe("$1.00")
  })

  it("keeps three decimals below a dollar, where the early signal lives", () => {
    expect(formatCost(0.42)).toBe("$0.420")
    expect(formatCost(0.004)).toBe("$0.004")
  })

  // A row of zeros would read as free. The floor says "small", not "nothing".
  it("floors a fraction of a cent instead of rounding it away", () => {
    expect(formatCost(0.0004)).toBe("<$0.001")
  })

  it("shows a true zero as zero", () => {
    expect(formatCost(0)).toBe("$0")
  })

  // The value crosses a process boundary; a negative or non-finite total is a
  // bug upstream, and rendering nothing beats rendering nonsense about money.
  it("renders nothing for a value that cannot be a cost", () => {
    expect(formatCost(-1)).toBe("")
    expect(formatCost(Number.NaN)).toBe("")
    expect(formatCost(Number.POSITIVE_INFINITY)).toBe("")
  })
})
