// The one thing this has to get right is the order: nothing is written into a
// session that has no prompt yet. So both calls are stubbed and what is asserted
// is that the write did not happen until Ready said so.
import { describe, expect, it, vi } from "vitest"
import { handoffHolds } from "./handoff-store"
import { writeAtPrompt } from "./write-at-prompt"

const ready = vi.fn()
const write = vi.fn()

vi.mock("@/lib/rpc", () => ({
  Terminal: {
    Ready: (id: string) => ready(id),
    Write: (id: string, data: string) => write(id, data),
  },
}))

describe("writeAtPrompt", () => {
  it("waits for the prompt before writing", async () => {
    ready.mockResolvedValueOnce(false).mockResolvedValueOnce(false).mockResolvedValueOnce(true)

    await writeAtPrompt("s1", "resolve the conflicts")

    expect(ready).toHaveBeenCalledTimes(3)
    expect(write).toHaveBeenCalledWith("s1", "resolve the conflicts")
  })

  // The wait is minutes long at its worst and looks exactly like a click that
  // did nothing, so the card it is waiting on says so for as long as it lasts
  // (handoff-store).
  it("marks the session it is waiting on, and clears the mark when it lands", async () => {
    ready.mockReset()
    write.mockReset()
    let held: boolean | null = null
    ready.mockImplementationOnce(() => {
      return Promise.resolve(false)
    })
    ready.mockImplementationOnce(() => {
      held = handoffHolds.get("s3")
      return Promise.resolve(true)
    })

    await writeAtPrompt("s3", "resolve the conflicts")

    expect(held).toBe(true)
    expect(handoffHolds.get("s3")).toBe(false)
  })

  // A handoff that never reached a prompt is not waiting for one either: the
  // caller words what was lost, and the card goes back to normal.
  it("clears the mark when the wait runs out", async () => {
    ready.mockReset()
    write.mockReset()
    ready.mockResolvedValue(false)
    vi.useFakeTimers()
    const wait = writeAtPrompt("s4", "fix CI").catch((err: unknown) => err)
    await vi.advanceTimersByTimeAsync(6 * 60 * 1000)
    vi.useRealTimers()

    expect(await wait).toBeInstanceOf(Error)
    expect(handoffHolds.get("s4")).toBe(false)
    expect(write).not.toHaveBeenCalled()
  })

  it("writes straight away when the session is already at a prompt", async () => {
    ready.mockReset()
    write.mockReset()
    ready.mockResolvedValue(true)

    await writeAtPrompt("s2", "fix CI")

    expect(ready).toHaveBeenCalledTimes(1)
    expect(write).toHaveBeenCalledWith("s2", "fix CI")
  })
})
