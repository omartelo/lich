import {describe, expect, it} from "vitest"
import {decideUpdateAction} from "./app-update-gate"
import type {AppUpdateStatus} from "./api-types"

const status = (over: Partial<AppUpdateStatus>): AppUpdateStatus => ({
  currentVersion: "0.7.0",
  latestVersion: "0.8.0",
  updateAvailable: true,
  canSelfApply: false,
  releaseUrl: "https://github.com/omartelo/lich/releases/tag/v0.8.0",
  installCommand: "curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh",
  ...over,
})

describe("decideUpdateAction", () => {
  it("prompts an update when one is available and not dismissed", () => {
    const action = decideUpdateAction(status({}), null)
    expect(action).toEqual({
      kind: "update",
      version: "0.8.0",
      canSelfApply: false,
      releaseUrl: "https://github.com/omartelo/lich/releases/tag/v0.8.0",
      installCommand: "curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh",
    })
  })

  it("carries the distro install command through", () => {
    const action = decideUpdateAction(status({installCommand: "yay -S lich-bin"}), null)
    expect(action).toMatchObject({kind: "update", installCommand: "yay -S lich-bin"})
  })

  it("carries canSelfApply through for the self-apply platforms", () => {
    const action = decideUpdateAction(status({canSelfApply: true}), null)
    expect(action).toMatchObject({kind: "update", canSelfApply: true})
  })

  it("stays silent when no update is available", () => {
    expect(decideUpdateAction(status({updateAvailable: false}), null)).toEqual({kind: "none"})
  })

  it("stays silent when dismissed for this exact version", () => {
    expect(decideUpdateAction(status({}), "0.8.0")).toEqual({kind: "none"})
  })

  it("re-prompts when a newer version arrives than the dismissed one", () => {
    const action = decideUpdateAction(status({latestVersion: "0.9.0"}), "0.8.0")
    expect(action).toMatchObject({kind: "update", version: "0.9.0"})
  })
})
