import {describe, expect, it} from "vitest"
import {makeReplayBuffer} from "./replay-buffer"

function chunk(size: number, fill = 0): Uint8Array {
  return new Uint8Array(size).fill(fill)
}

describe("makeReplayBuffer", () => {
  it("queues chunks and drains them in order", () => {
    const buffer = makeReplayBuffer()
    buffer.push(chunk(2, 1))
    buffer.push(chunk(3, 2))
    const drained = buffer.drain()
    expect(drained.map((c) => c.length)).toEqual([2, 3])
    expect(drained[0][0]).toBe(1)
    expect(buffer.bytes()).toBe(0)
    expect(buffer.drain()).toEqual([])
  })

  it("ignores empty chunks", () => {
    const buffer = makeReplayBuffer()
    buffer.push(chunk(0))
    expect(buffer.bytes()).toBe(0)
  })

  it("drops oldest chunks past the cap and reports truncation", () => {
    const buffer = makeReplayBuffer(10)
    buffer.push(chunk(6, 1))
    buffer.push(chunk(6, 2))
    expect(buffer.truncated()).toBe(true)
    const drained = buffer.drain()
    expect(drained.length).toBe(1)
    expect(drained[0][0]).toBe(2)
  })

  it("keeps one oversized chunk rather than dropping everything", () => {
    const buffer = makeReplayBuffer(4)
    buffer.push(chunk(10, 3))
    expect(buffer.bytes()).toBe(10)
    expect(buffer.drain().length).toBe(1)
  })

  it("resets the truncation flag on drain", () => {
    const buffer = makeReplayBuffer(1)
    buffer.push(chunk(1))
    buffer.push(chunk(1))
    expect(buffer.truncated()).toBe(true)
    buffer.drain()
    expect(buffer.truncated()).toBe(false)
  })
})
