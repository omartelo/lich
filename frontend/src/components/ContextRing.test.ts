import { describe, expect, it } from "vitest"
import { usageColor } from "./ContextRing"

describe("usageColor", () => {
  it("is muted below 80%", () => {
    expect(usageColor(0)).toBe("text-muted-foreground")
    expect(usageColor(79)).toBe("text-muted-foreground")
  })

  it("turns amber from 80% up to 95%", () => {
    expect(usageColor(80)).toBe("text-amber-500")
    expect(usageColor(94)).toBe("text-amber-500")
  })

  it("turns red from 95%", () => {
    expect(usageColor(95)).toBe("text-red-500")
    expect(usageColor(100)).toBe("text-red-500")
  })
})
