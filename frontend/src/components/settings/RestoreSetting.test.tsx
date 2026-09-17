// @vitest-environment jsdom
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import { RestoreSetting } from "./RestoreSetting"

const written: string[] = []

vi.mock("@/lib/rpc", () => ({
  Store: {
    GetSetting: () => Promise.resolve("resume"),
    SetSetting: (key: string, scope: string, value: string) => {
      written.push(`${key}|${scope}|${value}`)
      return Promise.resolve()
    },
  },
}))

test("draws the stored choice and writes a new one globally", async () => {
  const mounted = await mountBudget(
    createElement(RestoreSetting, { providerId: "codex", providerName: "Codex" }),
  )
  await mounted.act(() => {})
  expect(document.body.textContent).toContain("Restored cards continue their conversation on open.")

  const startNew = [...document.querySelectorAll("button")].find(
    (b) => b.textContent === "Start new",
  )
  await mounted.act(() => startNew?.click())
  expect(written).toEqual(["provider.codex.on-restore||fresh"])
  await mounted.unmount()
})

test("is absent for a kind that cannot resume", async () => {
  const mounted = await mountBudget(
    createElement(RestoreSetting, { providerId: "shell", providerName: "Terminal" }),
  )
  await mounted.act(() => {})
  expect(document.body.textContent).not.toContain("Restored sessions")
  await mounted.unmount()
})
