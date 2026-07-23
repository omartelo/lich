import {afterEach, describe, expect, it, vi} from "vitest"
import {registerUpdateChecker, runUpdateCheck} from "./update-check"
import type {AppUpdateStatus} from "./api-types"

const status: AppUpdateStatus = {
  currentVersion: "0.14.0",
  latestVersion: "0.14.0",
  updateAvailable: false,
  canSelfApply: false,
  releaseUrl: "https://github.com/omartelo/lich/releases/tag/v0.14.0",
  installCommand: "",
}

afterEach(() => registerUpdateChecker(null))

describe("runUpdateCheck", () => {
  it("rejects when no checker is registered", async () => {
    await expect(runUpdateCheck()).rejects.toThrow("update checker not ready")
  })

  it("runs the registered checker and returns its status", async () => {
    const checker = vi.fn().mockResolvedValue(status)
    registerUpdateChecker(checker)
    await expect(runUpdateCheck()).resolves.toEqual(status)
    expect(checker).toHaveBeenCalledOnce()
  })

  it("rejects again after the checker unregisters", async () => {
    registerUpdateChecker(vi.fn().mockResolvedValue(status))
    registerUpdateChecker(null)
    await expect(runUpdateCheck()).rejects.toThrow("update checker not ready")
  })

  it("propagates the checker's failure", async () => {
    registerUpdateChecker(vi.fn().mockRejectedValue(new Error("offline")))
    await expect(runUpdateCheck()).rejects.toThrow("offline")
  })
})
