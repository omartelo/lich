// @vitest-environment jsdom
//
// The Sandbox pane, mounted for real. The rung is keyed by provider and `shell`
// is not one, so a terminal in a project on Everywhere runs on the machine —
// the ladder cannot say that with a row, and the node-only gate cannot tell a
// sentence that renders from one that was written and never reached the pane.
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import { SandboxSettings } from "./SandboxSettings"

vi.mock("@/lib/rpc", () => ({
  System: {
    SandboxBackend: () => Promise.resolve("bubblewrap"),
    SSHAgentKeys: () => Promise.resolve([]),
  },
  Store: { GetSetting: () => Promise.resolve("") },
  Providers: {
    Detect: () =>
      Promise.resolve([
        { id: "claude", name: "Claude Code", binary: "claude", installed: true, source: "path" },
      ]),
  },
}))

test("says the sandbox never confines a terminal", async () => {
  const mounted = await mountBudget(createElement(SandboxSettings))
  await mounted.act(() => {})

  expect(document.body.textContent).toContain(
    "Terminals are never confined; the sandbox is for agents working unattended.",
  )
  await mounted.unmount()
})
