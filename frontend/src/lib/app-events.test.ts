import {describe, expect, it, vi} from "vitest"
import {dispatchEnvelope} from "./app-events"

type Callback = (data: unknown) => void

function registryWith(name: string, cb: Callback): Map<string, Set<Callback>> {
  return new Map([[name, new Set([cb])]])
}

describe("dispatchEnvelope", () => {
  it("routes the envelope data to the named handlers", () => {
    const cb = vi.fn()
    dispatchEnvelope(
      registryWith("session-status", cb),
      '{"name":"session-status","data":{"id":"s1","state":"busy"}}',
    )
    expect(cb).toHaveBeenCalledWith({id: "s1", state: "busy"})
  })

  it("delivers undefined data for payload-less events", () => {
    const cb = vi.fn()
    dispatchEnvelope(registryWith("terminal:exit:s1", cb), '{"name":"terminal:exit:s1"}')
    expect(cb).toHaveBeenCalledWith(undefined)
  })

  it("ignores unknown events, missing names and garbage", () => {
    const cb = vi.fn()
    const registry = registryWith("a", cb)
    dispatchEnvelope(registry, '{"name":"b","data":1}')
    dispatchEnvelope(registry, '{"data":1}')
    dispatchEnvelope(registry, "not json")
    expect(cb).not.toHaveBeenCalled()
  })

  it("fans out to every callback registered for the name", () => {
    const first = vi.fn()
    const second = vi.fn()
    const registry = new Map([["x", new Set([first, second])]])
    dispatchEnvelope(registry, '{"name":"x","data":"y"}')
    expect(first).toHaveBeenCalledWith("y")
    expect(second).toHaveBeenCalledWith("y")
  })
})
