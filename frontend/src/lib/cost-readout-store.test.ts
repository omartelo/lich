import { beforeEach, describe, expect, it, vi } from "vitest"
import { costReadoutStore, resetCostReadoutStore, setCostReadout } from "./cost-readout-store"

// The stored setting is the boundary; stub it so the test can assert both what
// the store reads and what a toggle writes back.
const getSetting = vi.fn()
const setSetting = vi.fn()
vi.mock("@/lib/rpc", () => ({
  Store: {
    GetSetting: (key: string, projectID: string) => getSetting(key, projectID),
    SetSetting: (key: string, projectID: string, value: string) =>
      setSetting(key, projectID, value),
  },
}))

// flush lets the store's one load settle.
const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

beforeEach(() => {
  resetCostReadoutStore()
  getSetting.mockReset().mockResolvedValue("")
  setSetting.mockReset().mockResolvedValue(null)
})

describe("costReadoutStore", () => {
  // Off is what a subscription plan must see, and it is also what the page
  // shows for the instant before the stored value arrives.
  it("reads off before anything is loaded", () => {
    expect(costReadoutStore.get()).toBe(false)
  })

  it("loads the stored flag on the first subscriber", async () => {
    getSetting.mockResolvedValue("true")
    let notified = 0
    costReadoutStore.subscribe(() => notified++)

    await flush()

    expect(getSetting).toHaveBeenCalledWith("usage.cost", "")
    expect(costReadoutStore.get()).toBe(true)
    expect(notified).toBe(1)
  })

  it("loads once however many subscribers mount", async () => {
    costReadoutStore.subscribe(() => {})
    costReadoutStore.subscribe(() => {})
    costReadoutStore.subscribe(() => {})

    await flush()

    expect(getSetting).toHaveBeenCalledTimes(1)
  })

  it("treats anything but the stored true as off", async () => {
    getSetting.mockResolvedValue("1")
    costReadoutStore.subscribe(() => {})

    await flush()

    expect(costReadoutStore.get()).toBe(false)
  })

  // An unreachable backend must not turn a money readout on by accident.
  it("stays off when the setting cannot be read", async () => {
    getSetting.mockRejectedValue(new Error("no backend"))
    costReadoutStore.subscribe(() => {})

    await flush()

    expect(costReadoutStore.get()).toBe(false)
  })

  it("writes a toggle through and updates the page at once", () => {
    let notified = 0
    costReadoutStore.subscribe(() => notified++)

    setCostReadout(true)

    expect(costReadoutStore.get()).toBe(true)
    expect(notified).toBe(1)
    expect(setSetting).toHaveBeenCalledWith("usage.cost", "", "true")
  })

  it("stops notifying an unsubscribed listener", () => {
    let notified = 0
    const off = costReadoutStore.subscribe(() => notified++)
    off()

    setCostReadout(true)

    expect(notified).toBe(0)
  })

  it("announces when the initial value settles without adding value notifications", async () => {
    const ready = vi.fn()
    const value = vi.fn()
    const off = costReadoutStore.subscribeReady(ready)
    costReadoutStore.subscribe(value)
    expect(costReadoutStore.isReady()).toBe(false)
    await flush()
    expect(costReadoutStore.isReady()).toBe(true)
    expect(ready).toHaveBeenCalledTimes(1)
    expect(value).not.toHaveBeenCalled()
    off()
  })

  it("rolls back a failed write and reports the error to the layout editor", async () => {
    setSetting.mockRejectedValueOnce(new Error("offline"))
    await expect(setCostReadout(true)).rejects.toThrow("offline")
    expect(costReadoutStore.get()).toBe(false)
  })

  it("does not let a late initial read overwrite a user choice", async () => {
    let finish: (value: string) => void = () => {}
    getSetting.mockImplementationOnce(
      () =>
        new Promise<string>((resolve) => {
          finish = resolve
        }),
    )
    costReadoutStore.subscribe(() => {})
    await setCostReadout(true)
    finish("false")
    await flush()
    expect(costReadoutStore.get()).toBe(true)
    expect(costReadoutStore.isReady()).toBe(true)
  })
})
