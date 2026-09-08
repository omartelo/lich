import { beforeEach, describe, expect, it, vi } from "vitest"
import { Providers } from "./rpc"
import { PATH_REREAD_FAILED, pathVersion, refreshPath, subscribePathRefresh } from "./path-refresh"

vi.mock("./rpc", () => ({ Providers: { RefreshPath: vi.fn() } }))

const refreshRPC = vi.mocked(Providers.RefreshPath)

describe("refreshPath", () => {
  beforeEach(() => {
    refreshRPC.mockReset()
  })

  it("wakes every check drawn from the pin once the shell answered", async () => {
    refreshRPC.mockResolvedValue(null)
    const woken = vi.fn()
    const stop = subscribePathRefresh(woken)
    const before = pathVersion()

    await refreshPath()

    expect(woken).toHaveBeenCalledTimes(1)
    expect(pathVersion()).toBe(before + 1)
    stop()
  })

  // A shell that did not answer leaves the pin as it was, so nothing re-checked
  // would be news — the surface has to say so instead of showing a stale scan.
  it("reports the failure and leaves the checks alone", async () => {
    refreshRPC.mockRejectedValue(new Error("shell never answered"))
    const woken = vi.fn()
    const stop = subscribePathRefresh(woken)
    const before = pathVersion()

    await expect(refreshPath()).rejects.toThrow(PATH_REREAD_FAILED)

    expect(woken).not.toHaveBeenCalled()
    expect(pathVersion()).toBe(before)
    stop()
  })
})
