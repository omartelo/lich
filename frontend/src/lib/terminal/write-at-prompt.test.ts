// The one thing this has to get right is the order: nothing is written into a
// session that has no prompt yet. So both calls are stubbed and what is asserted
// is that the write did not happen until Ready said so.
import { describe, expect, it, vi } from "vitest"
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

  it("writes straight away when the session is already at a prompt", async () => {
    ready.mockReset()
    write.mockReset()
    ready.mockResolvedValue(true)

    await writeAtPrompt("s2", "fix CI")

    expect(ready).toHaveBeenCalledTimes(1)
    expect(write).toHaveBeenCalledWith("s2", "fix CI")
  })
})
