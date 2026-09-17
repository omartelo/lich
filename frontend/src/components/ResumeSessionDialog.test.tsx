// @vitest-environment jsdom
//
// The resume prompt, mounted for real: the checkbox only means something if
// the button that closes the prompt is the one that writes it.
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import type { Session } from "@/lib/session/sessions"
import { ResumeSessionDialog } from "./ResumeSessionDialog"

vi.mock("@/lib/rpc", () => ({
  Store: { GetSetting: () => Promise.resolve("") },
  Providers: {
    Detect: () =>
      Promise.resolve([
        { id: "claude", name: "Claude Code", binary: "claude", installed: true, source: "path" },
      ]),
  },
}))

const session: Session = { id: "s1", label: "docs", kind: "claude", providerSessionId: "conv-1" }

function button(label: string): HTMLButtonElement {
  const found = [...document.querySelectorAll("button")].find((b) => b.textContent === label)
  if (!found) {
    throw new Error(`no ${label} button`)
  }
  return found
}

async function mount() {
  const calls = { startNew: 0, resume: 0, remembered: [] as string[] }
  const mounted = await mountBudget(
    createElement(ResumeSessionDialog, {
      session,
      onStartNew: () => calls.startNew++,
      onResume: () => calls.resume++,
      onRemember: (kind, choice) => calls.remembered.push(`${kind}=${choice}`),
    }),
  )
  await mounted.act(() => {})
  return { mounted, calls }
}

test("names the provider the default would be stored for", async () => {
  const { mounted } = await mount()
  expect(document.body.textContent).toContain("Always do this for Claude Code")
  await mounted.unmount()
})

test("an unticked answer remembers nothing", async () => {
  const { mounted, calls } = await mount()
  await mounted.act(() => button("Resume").click())
  expect(calls.resume).toBe(1)
  expect(calls.remembered).toEqual([])
  await mounted.unmount()
})

test("a ticked answer is stored as the provider's default", async () => {
  const { mounted, calls } = await mount()
  const box = document.getElementById("resume-remember")
  await mounted.act(() => box?.click())
  await mounted.act(() => button("Start new").click())
  expect(calls.startNew).toBe(1)
  expect(calls.remembered).toEqual(["claude=fresh"])
  await mounted.unmount()
})
