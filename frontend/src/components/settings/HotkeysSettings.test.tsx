// @vitest-environment jsdom
//
// The recorder saying what a rebind costs the terminal. A bound chord is
// stopped in the window capture phase and never reaches the PTY, so recording
// Ctrl+R takes the shell's history search — the pane has to connect the two,
// and only a real mount says whether the line reached the row.
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import { DEFAULT_HOTKEYS, UNASSIGNED, type Hotkeys } from "@/lib/hotkeys"
import { HotkeysSettings } from "./HotkeysSettings"

const bindings = vi.hoisted(() => ({ hotkeys: {} as Hotkeys }))

vi.mock("@/providers/settings", () => ({
  useSettings: () => ({
    hotkeys: bindings.hotkeys,
    setHotkey: () => {},
    resetHotkey: () => {},
  }),
}))

test("says what a chord the terminal spends will cost, and nothing for one it does not", async () => {
  // Every other action unbound, so the count below is these two rows and
  // nothing the defaults happen to spend.
  bindings.hotkeys = {
    ...(Object.fromEntries(Object.keys(DEFAULT_HOTKEYS).map((id) => [id, UNASSIGNED])) as Hotkeys),
    // The shell's history search, taken by a rebind.
    settings: { mod: true, shift: false, alt: false, key: "r" },
    // Ctrl+Shift+letter reaches no TUI at all, so it costs nothing.
    newSession: { mod: true, shift: true, alt: false, key: "r" },
  }

  const mounted = await mountBudget(createElement(HotkeysSettings))
  const text = document.body.textContent ?? ""

  expect(text).toContain("Ctrl+R is the shell's history search; sessions will no longer see it.")
  expect(text.match(/sessions will no longer see it/g)).toHaveLength(1)
  await mounted.unmount()
})
