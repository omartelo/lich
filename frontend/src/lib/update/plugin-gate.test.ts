import { describe, expect, it } from "vitest"
import { decidePluginAction } from "./plugin-gate"
import type { Status } from "./plugin-gate"

const status = (over: Partial<Status>): Status => ({
  provider: "claude",
  name: "Claude Code",
  available: true,
  installed: false,
  installedVersion: "",
  latestVersion: "",
  updateAvailable: false,
  ...over,
})

const codex = (over: Partial<Status>): Status =>
  status({ provider: "codex", name: "Codex", ...over })

describe("decidePluginAction", () => {
  it("prompts install when not installed and not dismissed", () => {
    const s = status({ installed: false })
    expect(decidePluginAction([s], false, null)).toEqual({ kind: "install", providers: [s] })
  })

  it("stays silent when not installed but dismissed forever", () => {
    expect(decidePluginAction([status({ installed: false })], true, null)).toEqual({ kind: "none" })
  })

  it("prompts update when a newer version is available", () => {
    const s = status({ installed: true, updateAvailable: true, latestVersion: "0.0.2" })
    expect(decidePluginAction([s], false, null)).toEqual({
      kind: "update",
      version: "0.0.2",
      providers: [s],
    })
  })

  it("skips update already dismissed for that exact version", () => {
    const s = status({ installed: true, updateAvailable: true, latestVersion: "0.0.2" })
    expect(decidePluginAction([s], false, "0.0.2")).toEqual({ kind: "none" })
  })

  it("re-prompts update when a newer version than the dismissed one appears", () => {
    const s = status({ installed: true, updateAvailable: true, latestVersion: "0.0.3" })
    expect(decidePluginAction([s], false, "0.0.2")).toEqual({
      kind: "update",
      version: "0.0.3",
      providers: [s],
    })
  })

  it("stays silent when installed and up to date", () => {
    const s = status({ installed: true, updateAvailable: false, installedVersion: "0.0.2" })
    expect(decidePluginAction([s], false, null)).toEqual({ kind: "none" })
  })

  it("install dismissal does not suppress an update prompt", () => {
    const s = status({ installed: true, updateAvailable: true, latestVersion: "0.0.2" })
    expect(decidePluginAction([s], true, null)).toEqual({
      kind: "update",
      version: "0.0.2",
      providers: [s],
    })
  })

  // A CLI the machine does not have is not a gap the user can close, so it is
  // never a reason to interrupt a startup.
  it("ignores a provider whose CLI is not installed", () => {
    const absent = codex({ available: false })
    expect(decidePluginAction([status({ installed: true }), absent], false, null)).toEqual({
      kind: "none",
    })
  })

  it("lists every provider still missing the plugin", () => {
    const claude = status({ installed: false })
    const other = codex({ installed: false })
    expect(decidePluginAction([claude, other], false, null)).toEqual({
      kind: "install",
      providers: [claude, other],
    })
  })

  // One CLI has the plugin and another does not: the missing install is the
  // bigger gap, and the update is still offered on the next start.
  it("prefers the install prompt when one provider has the plugin and another does not", () => {
    const outdated = status({ installed: true, updateAvailable: true, latestVersion: "0.0.2" })
    const missing = codex({ installed: false })
    expect(decidePluginAction([outdated, missing], false, null)).toEqual({
      kind: "install",
      providers: [missing],
    })
  })

  it("covers every outdated provider in one update prompt", () => {
    const a = status({ installed: true, updateAvailable: true, latestVersion: "0.0.2" })
    const b = codex({ installed: true, updateAvailable: true, latestVersion: "0.0.2" })
    expect(decidePluginAction([a, b], false, null)).toEqual({
      kind: "update",
      version: "0.0.2",
      providers: [a, b],
    })
  })
})
