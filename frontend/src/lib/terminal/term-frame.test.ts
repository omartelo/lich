import { describe, expect, it } from "vitest"
import { decodeFrame, encodeFrame } from "./term-frame"

describe("term-frame", () => {
  it("round-trips id and payload", () => {
    const frame = encodeFrame("sess-1", new Uint8Array([1, 2, 3]))
    expect(frame).not.toBeNull()
    const decoded = decodeFrame(frame as Uint8Array)
    expect(decoded?.id).toBe("sess-1")
    expect(Array.from(decoded?.payload ?? [])).toEqual([1, 2, 3])
  })

  it("round-trips an empty payload", () => {
    const frame = encodeFrame("s", new Uint8Array(0))
    const decoded = decodeFrame(frame as Uint8Array)
    expect(decoded?.id).toBe("s")
    expect(decoded?.payload).toHaveLength(0)
  })

  it("handles multi-byte ids", () => {
    const frame = encodeFrame("sessão-1", new Uint8Array([9]))
    const decoded = decodeFrame(frame as Uint8Array)
    expect(decoded?.id).toBe("sessão-1")
  })

  it("rejects invalid ids", () => {
    expect(encodeFrame("", new Uint8Array(0))).toBeNull()
    expect(encodeFrame("x".repeat(256), new Uint8Array(0))).toBeNull()
  })

  it("rejects malformed frames", () => {
    expect(decodeFrame(new Uint8Array(0))).toBeNull()
    expect(decodeFrame(new Uint8Array([0, 65]))).toBeNull()
    expect(decodeFrame(new Uint8Array([10, 65]))).toBeNull()
  })
})
