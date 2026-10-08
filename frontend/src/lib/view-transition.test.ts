import { afterEach, describe, expect, it, vi } from "vitest"
import { morph, SIDEBAR_MORPH } from "./view-transition"

function stubPage(reducedMotion: boolean) {
  const startViewTransition = vi.fn((run: () => void) => run())
  vi.stubGlobal("window", { matchMedia: () => ({ matches: reducedMotion }) })
  vi.stubGlobal("document", { startViewTransition })
  return startViewTransition
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("morph", () => {
  it("runs the update inside a view transition", () => {
    const startViewTransition = stubPage(false)
    const update = vi.fn()
    morph(update)
    expect(startViewTransition).toHaveBeenCalledOnce()
    expect(update).toHaveBeenCalledOnce()
  })

  it("applies the update without a transition when the user asks for reduced motion", () => {
    const startViewTransition = stubPage(true)
    const update = vi.fn()
    morph(update)
    expect(startViewTransition).not.toHaveBeenCalled()
    expect(update).toHaveBeenCalledOnce()
  })
})

describe("SIDEBAR_MORPH.session", () => {
  it("names a session whose id starts with a digit as a valid custom ident", () => {
    expect(SIDEBAR_MORPH.session("0f3c")).toBe("sidebar-session-0f3c")
  })
})
