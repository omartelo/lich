import { describe, expect, it } from "vitest"
import { exitMarker, exitNotice, readSessionExit } from "./session-exit"

describe("readSessionExit", () => {
  it("keeps the reported status", () => {
    expect(readSessionExit({ code: 130 })).toEqual({ code: 130 })
    expect(readSessionExit({ code: 0 })).toEqual({ code: 0 })
  })

  it("reads a missing or unusable code as no status at all", () => {
    // Never 0: an exit lich did not observe must not read as a clean one.
    expect(readSessionExit({}).code).toBeNull()
    expect(readSessionExit(null).code).toBeNull()
    expect(readSessionExit(undefined).code).toBeNull()
    expect(readSessionExit("130").code).toBeNull()
    expect(readSessionExit({ code: "130" }).code).toBeNull()
  })
})

describe("exitNotice", () => {
  it("names a failing code and stays quiet about the rest", () => {
    expect(exitNotice({ code: 130 })).toBe("Session ended with code 130")
    expect(exitNotice({ code: 1 })).toBe("Session ended with code 1")
    expect(exitNotice({ code: 0 })).toBe("Session ended")
    expect(exitNotice({ code: null })).toBe("Session ended")
  })
})

describe("exitMarker", () => {
  it("carries the code into the scrollback when there is one", () => {
    expect(exitMarker({ code: 3 })).toBe("\r\n[process exited: 3]\r\n")
    expect(exitMarker({ code: null })).toBe("\r\n[process exited]\r\n")
  })
})
