// @vitest-environment jsdom
//
// What a surface paints while a check is in flight. The hook is rendered rather
// than left to a pure half because the whole behaviour is a frame: the verdict
// on screen on the way back in, before anything has answered.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { refreshPath } from "@/lib/path-refresh"
import { Providers } from "@/lib/rpc"
import { NO_SETTLE, useBinaryCheck } from "@/lib/use-binary-check"
import type { BinaryCheck } from "@/lib/api-types"

vi.mock("@/lib/rpc", () => ({
  Providers: {
    Verify: vi.fn(),
    RefreshPath: vi.fn(),
  },
}))

const verify = vi.mocked(Providers.Verify)
const refreshRPC = vi.mocked(Providers.RefreshPath)

const found = (path: string): BinaryCheck => ({ path, status: "ok" })

const last = (frames: (BinaryCheck | null)[]) => frames[frames.length - 1]

// Every value each render answered with, in order, so a null frame between two
// verdicts is visible rather than averaged away.
async function paint(bin: string): Promise<{ frames: (BinaryCheck | null)[] }> {
  const frames: (BinaryCheck | null)[] = []
  function Probe() {
    frames.push(useBinaryCheck(bin, NO_SETTLE))
    return null
  }
  const mounted = await mountBudget(createElement(Probe))
  // The settle is a timer even at zero, so the answer lands a macrotask later.
  await mounted.act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
  await mounted.unmount()
  return { frames }
}

beforeEach(() => {
  verify.mockReset()
  refreshRPC.mockReset()
})

describe("useBinaryCheck", () => {
  // Each test names its own binary: the verdicts are module-level, which is the
  // point of them, so a shared name would let one test answer the next.
  it("answers null for a value it has never checked", async () => {
    verify.mockResolvedValue(found("/usr/bin/first"))

    const { frames } = await paint("first")

    expect(frames[0]).toBeNull()
    expect(last(frames)).toEqual(found("/usr/bin/first"))
  })

  it("paints the last verdict on a remount, with no null frame", async () => {
    verify.mockResolvedValue(found("/usr/bin/second"))
    await paint("second")

    const { frames } = await paint("second")

    expect(frames).not.toContain(null)
    expect(frames[0]).toEqual(found("/usr/bin/second"))
    // And it is still re-checked behind what is on screen.
    expect(verify).toHaveBeenCalledTimes(2)
  })

  it("starts a value it has not seen at null, however many it has", async () => {
    verify.mockResolvedValue(found("/usr/bin/third"))
    await paint("third")
    verify.mockResolvedValue(found("/opt/fourth"))

    const { frames } = await paint("fourth")

    expect(frames[0]).toBeNull()
    expect(last(frames)).toEqual(found("/opt/fourth"))
  })

  // A verdict is resolved through the pinned $PATH, so a re-read makes every one
  // of them a fresh question rather than a repeat of the answer already filed.
  it("re-checks when the pinned PATH moves", async () => {
    verify.mockResolvedValue(found("/usr/bin/fifth"))
    await paint("fifth")
    refreshRPC.mockResolvedValue(null)
    await refreshPath()
    verify.mockResolvedValue(found("/opt/fifth"))

    const { frames } = await paint("fifth")

    expect(verify).toHaveBeenCalledTimes(2)
    expect(frames[0]).toBeNull()
    expect(last(frames)).toEqual(found("/opt/fifth"))
  })
})
