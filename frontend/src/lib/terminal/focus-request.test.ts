import { describe, expect, it } from "vitest"
import { onTerminalFocusRequest, requestTerminalFocus } from "./focus-request"

describe("requestTerminalFocus", () => {
  it("notifies the session's own listener, and only that one", () => {
    let mine = 0
    let theirs = 0
    onTerminalFocusRequest("s1", () => mine++)
    onTerminalFocusRequest("s2", () => theirs++)
    requestTerminalFocus("s1")
    expect(mine).toBe(1)
    expect(theirs).toBe(0)
  })

  it("is an event, not a state: two requests in a row notify twice", () => {
    let seen = 0
    onTerminalFocusRequest("s3", () => seen++)
    requestTerminalFocus("s3")
    requestTerminalFocus("s3")
    expect(seen).toBe(2)
  })

  it("stops notifying once unsubscribed", () => {
    let seen = 0
    const off = onTerminalFocusRequest("s4", () => seen++)
    off()
    requestTerminalFocus("s4")
    expect(seen).toBe(0)
  })
})
