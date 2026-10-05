// @vitest-environment jsdom
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import { SubagentCardsSetting } from "./SubagentCardsSetting"

const written: string[] = []
let stored = ""

vi.mock("@/lib/rpc", () => ({
  Store: {
    GetSetting: () => Promise.resolve(stored),
    SetSetting: (key: string, scope: string, value: string) => {
      written.push(`${key}|${scope}|${value}`)
      return Promise.resolve()
    },
  },
}))

beforeEach(() => {
  written.length = 0
  stored = ""
})

const render = (providerId: string, providerName: string) =>
  mountBudget(createElement(SubagentCardsSetting, { providerId, providerName }))

const toggle = () =>
  document.querySelector<HTMLElement>('[aria-label="Run Claude Code subagents as lich sessions"]')

test("is on with nothing stored, and turning it off stores false globally", async () => {
  const mounted = await render("claude", "Claude Code")
  await mounted.act(() => {})

  expect(toggle()?.getAttribute("aria-checked")).toBe("true")
  await mounted.act(() => toggle()?.click())
  expect(written).toContain("provider.claude.subagentCards||false")
  await mounted.unmount()
})

test("reads a stored false as off", async () => {
  stored = "false"
  const mounted = await render("claude", "Claude Code")
  await mounted.act(() => {})

  expect(toggle()?.getAttribute("aria-checked")).toBe("false")
  await mounted.unmount()
})

test("offers no switch to a provider whose subagents cannot become sessions", async () => {
  const mounted = await render("codex", "Codex")
  await mounted.act(() => {})
  expect(document.body.textContent).not.toContain("Subagents as lich sessions")
  await mounted.unmount()
})
