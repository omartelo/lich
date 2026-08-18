import { describe, expect, it } from "vitest"
import { formatModel } from "./model-name"

describe("formatModel", () => {
  it("drops the claude- prefix and dots the version", () => {
    expect(formatModel("claude-opus-4-8")).toBe("opus 4.8")
    expect(formatModel("claude-sonnet-5")).toBe("sonnet 5")
    expect(formatModel("claude-fable-5")).toBe("fable 5")
  })

  it("strips a trailing date snapshot", () => {
    expect(formatModel("claude-haiku-4-5-20251001")).toBe("haiku 4.5")
  })

  it("leaves an id without a version split stripped but intact", () => {
    expect(formatModel("claude-experimental")).toBe("experimental")
  })

  it("leaves another provider's model id intact", () => {
    expect(formatModel("gpt-5.6-sol")).toBe("gpt-5.6-sol")
  })
})
