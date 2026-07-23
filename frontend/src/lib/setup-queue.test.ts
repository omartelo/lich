import {describe, expect, it} from "vitest"
import {queueSetup, takeSetup} from "./setup-queue"

describe("setup-queue", () => {
  it("returns the mark once, then clears it", () => {
    queueSetup("s1")
    expect(takeSetup("s1")).toBe(true)
    // One-shot: a respawn must not run the setup script again.
    expect(takeSetup("s1")).toBe(false)
  })

  it("returns false for a session never marked", () => {
    expect(takeSetup("never-marked")).toBe(false)
  })

  it("keeps sessions independent", () => {
    queueSetup("a")
    expect(takeSetup("b")).toBe(false)
    expect(takeSetup("a")).toBe(true)
  })
})
