import { beforeEach, describe, expect, it, vi } from "vitest"
import { useInject } from "./use-inject"

// useInject is a useCallback around one write, so the hook is exercised by
// standing in for the dispatcher rather than by rendering: what is under test
// is the closure it returns, not React.
vi.mock("react", () => ({ useCallback: (fn: unknown) => fn }))

const write = vi.fn()
vi.mock("@/lib/rpc", () => ({
  Terminal: { Write: (id: string, text: string) => write(id, text) },
}))

describe("useInject", () => {
  beforeEach(() => {
    write.mockReset()
  })

  // Nothing wraps this write, so a newline in an injected reference sends the
  // prompt on the agent's behalf. The paths come out of a repository tree, and
  // a file can be committed under any name its filesystem took.
  it("never writes a control character a committed path carried", () => {
    const inject = useInject("s1")

    expect(inject("@src/a.ts\nrm -rf ~ ")).toBe(true)
    expect(inject("@src/\x1b[201~b.ts ")).toBe(true)

    expect(write.mock.calls).toEqual([
      ["s1", "@src/a.tsrm -rf ~ "],
      ["s1", "@src/[201~b.ts "],
    ])
  })

  it("leaves an ordinary reference alone", () => {
    useInject("s1")("@src/lib/rpc.ts:42 ")

    expect(write).toHaveBeenCalledWith("s1", "@src/lib/rpc.ts:42 ")
  })

  // A session-less surface is a no-op, and the false is what tells a caller its
  // text was not taken.
  it("writes nothing without a session", () => {
    expect(useInject("")("@src/a.ts ")).toBe(false)
    expect(write).not.toHaveBeenCalled()
  })
})
