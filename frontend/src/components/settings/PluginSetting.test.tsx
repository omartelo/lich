// @vitest-environment jsdom
//
// The plugin pane, mounted for real. What it is about happens between two
// visits to Updates — the rows have to come back painted rather than blank —
// and the outcome of a check is read off the resource the refresh left behind
// rather than awaited, which is a claim about a render and not about a value.
//
// The harness is imported before anything that reaches react-dom, for the
// reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { clearRemoteCache } from "@/lib/remote-cache"
import type { PluginStatus } from "@/lib/api-types"
import { PluginSetting } from "./PluginSetting"

const NOT_INSTALLED: PluginStatus = {
  provider: "claude",
  name: "Claude Code",
  available: true,
  installed: false,
  installedVersion: "",
  latestVersion: "0.9.0",
  updateAvailable: false,
}

const INSTALLED: PluginStatus = {
  ...NOT_INSTALLED,
  installed: true,
  installedVersion: "0.9.0",
}

// An installed plugin one version behind, which is the row that draws a button
// while claiming the plugin is already there.
const OUTDATED: PluginStatus = {
  ...INSTALLED,
  installedVersion: "0.8.0",
  updateAvailable: true,
}

/** What the next Status() call answers. Swapped per test, and mid-test. */
let status: () => Promise<PluginStatus[]> = () => Promise.resolve([NOT_INSTALLED])
const installs: string[] = []
/** A read that never lands, so whatever is on screen came from the cache. */
const pending = () => new Promise<PluginStatus[]>(() => {})

const updates: string[] = []
/** What the machine looks like once an install has run. Swapped per test. */
let installed: () => void = () => {}

vi.mock("@/lib/rpc", () => ({
  AgentPlugin: {
    Status: () => status(),
    Install: (provider: string) => {
      installs.push(provider)
      installed()
      return Promise.resolve(null)
    },
    Update: (provider: string) => {
      updates.push(provider)
      return Promise.resolve(null)
    },
  },
}))

const text = () => document.body.textContent ?? ""

function click(label: string): void {
  const button = [...document.querySelectorAll("button")].find(
    (element) => element.textContent?.trim() === label,
  )
  if (!button) {
    throw new Error(`no "${label}" button on screen: ${text()}`)
  }
  button.click()
}

beforeEach(() => {
  clearRemoteCache()
  installs.length = 0
  updates.length = 0
  installed = () => {}
  status = () => Promise.resolve([NOT_INSTALLED])
})

describe("the plugin pane", () => {
  it("paints the filed rows on the next visit, before the refetch answers", async () => {
    const first = await mountBudget(createElement(PluginSetting))
    await first.act(() => {})
    expect(text()).toContain("Claude Code")
    await first.unmount()

    // The read this mount runs never lands, so a row on screen is the filed
    // answer and nothing else — which is the blank this pane used to show.
    status = pending
    const second = await mountBudget(createElement(PluginSetting))

    expect(text()).toContain("Claude Code")
    await second.unmount()
  })

  it("says the check was made once the refresh has landed", async () => {
    const mounted = await mountBudget(createElement(PluginSetting))
    await mounted.act(() => {})
    expect(text()).not.toContain("Checked.")

    await mounted.act(() => click("Check for updates"))

    expect(text()).toContain("Checked.")
    await mounted.unmount()
  })

  // The outcome is the resource's `error`, not a rejection: refresh folds the
  // failure into state, so nothing is thrown for a catch to read.
  it("says the check failed when the read did", async () => {
    const mounted = await mountBudget(createElement(PluginSetting))
    await mounted.act(() => {})
    // Broken after the pane was painted, so the outcome is this refresh's and
    // not something the mount left behind.
    status = () => Promise.reject(new Error("connection refused"))

    await mounted.act(() => click("Check for updates"))

    expect(text()).toContain("Check failed — are you online?")
    expect(text()).not.toContain("Checked.")
    // Beside the rows, not in place of them: a check that could not be made
    // says nothing about the answer already on the pane.
    expect(text()).toContain("Claude Code")
    await mounted.unmount()
  })

  // The filed answer draws the row, so the button can be offering something
  // that has already happened — or, here, something that cannot: the plugin was
  // removed from a terminal, and the cached row still offers its update. The
  // click re-asks before it acts, and installs what the fresh row is missing.
  it("acts on the fresh row, not the filed one the button was drawn from", async () => {
    status = () => Promise.resolve([OUTDATED])
    const first = await mountBudget(createElement(PluginSetting))
    await first.act(() => {})
    await first.unmount()

    // This visit's read never lands, so the row on screen is the filed one and
    // the button under the pointer is the one the cache drew.
    status = pending
    const second = await mountBudget(createElement(PluginSetting))
    expect(text()).toContain("Update to v0.9.0")

    // What the terminal did in the meantime, answered to the click's own read.
    status = () => Promise.resolve([NOT_INSTALLED])
    await second.act(() => click("Update to v0.9.0"))

    expect(installs).toEqual(["claude"])
    expect(updates).toEqual([])
    expect(text()).toContain("Install")
    await second.unmount()
  })

  it("re-reads after an install and files what it read", async () => {
    const mounted = await mountBudget(createElement(PluginSetting))
    await mounted.act(() => {})
    // Turned by the install itself rather than before the click: the click
    // re-reads first, and a machine that already had the plugin is the case
    // above — not the one where installing is what put it there.
    installed = () => {
      status = () => Promise.resolve([INSTALLED])
    }

    await mounted.act(() => click("Install"))

    expect(installs).toEqual(["claude"])
    expect(text()).toContain("v0.9.0")
    expect(text()).not.toContain("Plugin not installed")
    await mounted.unmount()

    // The filed answer moved with it, so the next visit does not offer an
    // install that has already happened.
    status = pending
    const second = await mountBudget(createElement(PluginSetting))

    expect(text()).toContain("v0.9.0")
    expect(text()).not.toContain("Plugin not installed")
    await second.unmount()
  })
})
