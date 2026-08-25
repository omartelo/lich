import { describe, expect, it } from "vitest"
import { lastTurnNotice } from "./last-turn"

describe("lastTurnNotice", () => {
  it("draws the diff only when there is one to draw", () => {
    expect(lastTurnNotice("ok", 3)).toBe("diff")
  })

  // The distinction the whole feature turns on: a turn that changed nothing
  // must never be reported as a turn nobody recorded, or the other way round.
  it("keeps a turn that changed nothing apart from one nobody recorded", () => {
    expect(lastTurnNotice("empty", 0)).toBe("empty")
    expect(lastTurnNotice("unavailable", 0)).toBe("unrecorded")
  })

  // Before the first read lands, and for a session whose provider never
  // reported: neither is an empty turn.
  it("reads no answer at all as unrecorded", () => {
    expect(lastTurnNotice(null, 0)).toBe("unrecorded")
  })

  // An "ok" whose text yielded no files — a mode-only change, a binary the
  // parser skips — has nothing to show, and "nothing changed" would be a claim
  // the backend never made.
  it("does not promote an unrenderable diff to an empty turn", () => {
    expect(lastTurnNotice("ok", 0)).toBe("unrecorded")
  })
})
