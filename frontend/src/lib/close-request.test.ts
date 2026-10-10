import { describe, expect, it } from "vitest"
import { closeActionOf, closeOutcome, holdsClose, inLichWindow } from "./close-request"

describe("closeActionOf", () => {
  it("reads a stored choice and asks for anything else", () => {
    expect(closeActionOf("background")).toBe("background")
    expect(closeActionOf("quit")).toBe("quit")
    expect(closeActionOf("ask")).toBe("ask")
    expect(closeActionOf("")).toBe("ask")
    expect(closeActionOf("minimize")).toBe("ask")
  })
})

describe("inLichWindow", () => {
  it("is true only for the page lich's own window opened", () => {
    expect(inLichWindow("http://127.0.0.1:47821/?token=t&shell=1#/")).toBe(true)
    expect(inLichWindow("http://127.0.0.1:47821/?token=t&shell=1&focus=s1")).toBe(true)
    expect(inLichWindow("http://127.0.0.1:47821/?token=t#/")).toBe(false)
  })
})

describe("holdsClose", () => {
  it("lets a kept keep-running through and holds everything else", () => {
    expect(holdsClose("background")).toBe(false)
    expect(holdsClose("ask")).toBe(true)
    expect(holdsClose("quit")).toBe(true)
  })
})

describe("closeOutcome", () => {
  it("quits with no session running, whatever was kept", () => {
    expect(closeOutcome("ask", 0)).toBe("quit")
    expect(closeOutcome("quit", 0)).toBe("quit")
  })

  it("quits when that is the kept answer and asks otherwise", () => {
    expect(closeOutcome("quit", 3)).toBe("quit")
    expect(closeOutcome("ask", 3)).toBe("ask")
  })
})
