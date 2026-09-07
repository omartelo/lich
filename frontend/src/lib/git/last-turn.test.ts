import { describe, expect, it } from "vitest"
import { lastTurnNotice, turnSwitchable } from "./last-turn"

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

describe("turnSwitchable", () => {
  // The state a card is restored in: quiet since the launch, with a turn the
  // backend read back off the workspace database. Withholding the switch here
  // is what used to hide it.
  it("offers the switch to a restored session that has not reported yet", () => {
    expect(turnSwitchable(false, true)).toBe(true)
  })

  // The other half, unchanged: a session that has reported has a turn boundary,
  // so the switch is earned before its first turn has finished.
  it("offers the switch to a session that has reported", () => {
    expect(turnSwitchable(true, false)).toBe(true)
  })

  // A provider that reports no state has no turn to bracket, and with nothing
  // on record there is none to show: the working tree alone.
  it("withholds the switch from a session with neither", () => {
    expect(turnSwitchable(false, false)).toBe(false)
  })
})
