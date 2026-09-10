import { describe, expect, it } from "vitest"
import { COST_MISS_REASON, budgetShare, formatCost } from "./session-cost"

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

describe("budgetShare", () => {
  it("is the share of the ceiling already spent", () => {
    expect(budgetShare(5, 10)).toBe(50)
    expect(budgetShare(0.25, 1)).toBe(25)
  })

  // The colour changes at 80 and 95 (see usageColor), so these are the two
  // shares that actually decide what the footer looks like.
  it("reaches the amber and red marks at the spends that earn them", () => {
    expect(budgetShare(8, 10)).toBe(80)
    expect(budgetShare(9.5, 10)).toBe(95)
    expect(budgetShare(7.9, 10)).toBeCloseTo(79)
  })

  it("caps past the ceiling — there is no colour louder than red", () => {
    expect(budgetShare(25, 10)).toBe(100)
  })

  it("is the muted end when no ceiling is set", () => {
    expect(budgetShare(12, 0)).toBe(0)
    expect(budgetShare(12, -5)).toBe(0)
  })

  it("is the muted end for a cost that cannot be one", () => {
    expect(budgetShare(Number.NaN, 10)).toBe(0)
    expect(budgetShare(-1, 10)).toBe(0)
    expect(budgetShare(1, Number.POSITIVE_INFINITY)).toBe(0)
  })
})

// Every reason the backend can send has a sentence here: a marker the user
// cannot read is no better than the blank it replaced.
describe("COST_MISS_REASON", () => {
  it("has a sentence for every reason the contract carries", () => {
    for (const reason of ["mixed-models", "unpriced-model"] as const) {
      expect(COST_MISS_REASON[reason]).toMatch(/\S/)
    }
  })
})
