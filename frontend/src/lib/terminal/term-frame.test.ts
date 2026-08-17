import { describe, expect, it } from "vitest"
import { decodeBase64, decodeFrame, encodeFrame } from "./term-frame"

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

// The /events fallback's payload. Every byte has to come back exactly as the PTY
// wrote it: the encoding exists because the JSON envelope cannot carry raw bytes,
// so a decode that went through text would break the sequences it protects.
describe("decodeBase64", () => {
  it("answers the empty payload for an empty string", () => {
    expect(decodeBase64("")).toEqual(new Uint8Array())
  })

  it("round-trips a multi-byte UTF-8 sequence byte for byte", () => {
    // "ok ✓" — the tick is three bytes.
    expect([...decodeBase64("b2sg4pyT")]).toEqual([0x6f, 0x6b, 0x20, 0xe2, 0x9c, 0x93])
  })

  it("keeps a high byte a text decode would replace", () => {
    // 0xff is not valid UTF-8 on its own; the tail of a sequence split across
    // two frames arrives exactly like this.
    expect([...decodeBase64("/w==")]).toEqual([0xff])
  })

  it("keeps an escape sequence intact", () => {
    // ESC [ 2 m
    expect([...decodeBase64("G1sybQ==")]).toEqual([0x1b, 0x5b, 0x32, 0x6d])
  })
})
