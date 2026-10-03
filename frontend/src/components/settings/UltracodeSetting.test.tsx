// @vitest-environment jsdom
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import { UltracodeSetting } from "./UltracodeSetting"

const written: string[] = []

vi.mock("@/lib/rpc", () => ({
  Store: {
    GetSetting: () => Promise.resolve(""),
    SetSetting: (key: string, scope: string, value: string) => {
      written.push(`${key}|${scope}|${value}`)
      return Promise.resolve()
    },
  },
}))

beforeEach(() => {
  written.length = 0
})

const render = (providerId: string, providerName: string) =>
  mountBudget(createElement(UltracodeSetting, { providerId, providerName }))

test("turns ultracode on for Claude Code, globally", async () => {
  const mounted = await render("claude", "Claude Code")
  await mounted.act(() => {})

  const ultracode = document.querySelector<HTMLElement>(
    '[aria-label="Start Claude Code sessions in ultracode"]',
  )
  expect(ultracode).not.toBeNull()
  await mounted.act(() => ultracode?.click())
  expect(written).toContain("provider.claude.ultracode||true")
  await mounted.unmount()
})

test("offers no ultracode to a provider without one", async () => {
  const mounted = await render("codex", "Codex")
  await mounted.act(() => {})
  expect(document.body.textContent).not.toContain("Ultracode")
  await mounted.unmount()
})
