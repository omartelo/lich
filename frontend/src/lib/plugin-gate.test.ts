import {describe, expect, it} from "vitest"
import {decidePluginAction} from "./plugin-gate"
import type {Status} from "../../bindings/github.com/omartelo/lich/internal/claudeplugin"

const status = (over: Partial<Status>): Status => ({
  installed: false,
  installedVersion: "",
  latestVersion: "",
  updateAvailable: false,
  ...over,
})

describe("decidePluginAction", () => {
  it("prompts install when not installed and not dismissed", () => {
    expect(decidePluginAction(status({installed: false}), false, null)).toEqual({kind: "install"})
  })

  it("stays silent when not installed but dismissed forever", () => {
    expect(decidePluginAction(status({installed: false}), true, null)).toEqual({kind: "none"})
  })

  it("prompts update when a newer version is available", () => {
    const s = status({installed: true, updateAvailable: true, latestVersion: "0.0.2"})
    expect(decidePluginAction(s, false, null)).toEqual({kind: "update", version: "0.0.2"})
  })

  it("skips update already dismissed for that exact version", () => {
    const s = status({installed: true, updateAvailable: true, latestVersion: "0.0.2"})
    expect(decidePluginAction(s, false, "0.0.2")).toEqual({kind: "none"})
  })

  it("re-prompts update when a newer version than the dismissed one appears", () => {
    const s = status({installed: true, updateAvailable: true, latestVersion: "0.0.3"})
    expect(decidePluginAction(s, false, "0.0.2")).toEqual({kind: "update", version: "0.0.3"})
  })

  it("stays silent when installed and up to date", () => {
    const s = status({installed: true, updateAvailable: false, installedVersion: "0.0.2"})
    expect(decidePluginAction(s, false, null)).toEqual({kind: "none"})
  })

  it("install dismissal does not suppress an update prompt", () => {
    const s = status({installed: true, updateAvailable: true, latestVersion: "0.0.2"})
    expect(decidePluginAction(s, true, null)).toEqual({kind: "update", version: "0.0.2"})
  })
})
