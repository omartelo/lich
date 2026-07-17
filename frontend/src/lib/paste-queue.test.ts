import {describe, expect, it} from "vitest"
import {queuePaste, takePaste} from "./paste-queue"

describe("paste-queue", () => {
  it("returns queued text once, then clears it", () => {
    queuePaste("s1", "curl … | sh")
    expect(takePaste("s1")).toBe("curl … | sh")
    // One-shot: a second take finds nothing.
    expect(takePaste("s1")).toBeUndefined()
  })

  it("returns undefined for a session with nothing queued", () => {
    expect(takePaste("never-queued")).toBeUndefined()
  })

  it("keeps sessions independent", () => {
    queuePaste("a", "cmd-a")
    queuePaste("b", "cmd-b")
    expect(takePaste("b")).toBe("cmd-b")
    expect(takePaste("a")).toBe("cmd-a")
  })
})
