import { describe, expect, it } from "vitest"
import { contextColor } from "./ContextRing"

describe("contextColor", () => {
  it("is muted below 80%", () => {
    expect(contextColor(0)).toBe("text-muted-foreground")
    expect(contextColor(79)).toBe("text-muted-foreground")
  })

  it("turns amber from 80% up to 95%", () => {
    expect(contextColor(80)).toBe("text-amber-500")
    expect(contextColor(94)).toBe("text-amber-500")
  })

  it("turns red from 95%", () => {
    expect(contextColor(95)).toBe("text-red-500")
    expect(contextColor(100)).toBe("text-red-500")
  })
})
