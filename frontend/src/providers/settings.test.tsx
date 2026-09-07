// @vitest-environment jsdom
//
// Where the hotkey bindings come from, and the one migration off the page's own
// storage. This is rendered rather than left to the pure half (hotkeys.test.ts)
// because the migration runs exactly once per install: it writes the workspace
// copy and drops the page entry, and a launch that gets that order wrong takes
// every rebind with it, with nothing left to read it back from.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { DEFAULT_HOTKEYS, HOTKEYS_SETTING_KEY, LEGACY_HOTKEYS_KEY } from "@/lib/hotkeys"
import type { Combo, Hotkeys } from "@/lib/hotkeys"
import { SettingsProvider, useSettings } from "@/providers/settings"

const settings = new Map<string, string>()
const writes: string[] = []

vi.mock("@/lib/rpc", () => ({
  Store: {
    GetSetting: (key: string, scope: string) =>
      Promise.resolve(settings.get(`${key}@${scope}`) ?? ""),
    SetSetting: (key: string, scope: string, value: string) => {
      if (key === HOTKEYS_SETTING_KEY) {
        writes.push(`${key}@${scope}`)
      }
      settings.set(`${key}@${scope}`, value)
      return Promise.resolve(null)
    },
  },
  // The theme list is the provider's other boot call; it hangs, which leaves the
  // bundled themes standing and keeps this file about the bindings.
  Themes: new Proxy({}, { get: () => () => new Promise<never>(() => {}) }),
}))

const MINE: Combo = { mod: true, shift: true, alt: false, key: "j" }
const rebound = (combo: Combo) => JSON.stringify({ ...DEFAULT_HOTKEYS, newSession: combo })

/** The bindings as a consumer sees them, once every boot read has settled. */
async function mountSettings(): Promise<Hotkeys> {
  let seen: Hotkeys = DEFAULT_HOTKEYS
  function Probe() {
    seen = useSettings().hotkeys
    return null
  }
  const mounted = await mountBudget(createElement(SettingsProvider, null, createElement(Probe)))
  await mounted.act(() => {})
  await mounted.unmount()
  return seen
}

beforeEach(() => {
  settings.clear()
  writes.length = 0
  localStorage.clear()
})

describe("the hotkey bindings", () => {
  it("reads them from the workspace, without writing anything back", async () => {
    settings.set(`${HOTKEYS_SETTING_KEY}@`, rebound(MINE))

    expect((await mountSettings()).newSession).toEqual(MINE)
    expect(writes).toEqual([])
  })

  it("migrates the page's copy into the workspace and drops it", async () => {
    localStorage.setItem(LEGACY_HOTKEYS_KEY, rebound(MINE))

    expect((await mountSettings()).newSession).toEqual(MINE)
    expect(writes).toEqual([`${HOTKEYS_SETTING_KEY}@`])
    expect(localStorage.getItem(LEGACY_HOTKEYS_KEY)).toBeNull()

    // And the launch after it reads the workspace, with nothing left to migrate.
    writes.length = 0
    expect((await mountSettings()).newSession).toEqual(MINE)
    expect(writes).toEqual([])
  })

  it("leaves a stale page copy standing under the workspace one", async () => {
    const theirs: Combo = { mod: true, shift: true, alt: false, key: "y" }
    settings.set(`${HOTKEYS_SETTING_KEY}@`, rebound(theirs))
    localStorage.setItem(LEGACY_HOTKEYS_KEY, rebound(MINE))

    expect((await mountSettings()).newSession).toEqual(theirs)
    expect(writes).toEqual([])
  })

  it("answers the defaults for an install that has neither", async () => {
    expect(await mountSettings()).toEqual(DEFAULT_HOTKEYS)
    expect(writes).toEqual([])
  })

  it("writes a rebind through to the workspace", async () => {
    let rebind: (() => void) | undefined
    function Probe() {
      const { setHotkey } = useSettings()
      rebind = () => setHotkey("newSession", MINE)
      return null
    }
    const mounted = await mountBudget(createElement(SettingsProvider, null, createElement(Probe)))
    await mounted.act(() => {})
    await mounted.act(() => rebind?.())

    expect(settings.get(`${HOTKEYS_SETTING_KEY}@`)).toContain('"key":"j"')
    await mounted.unmount()
  })
})
