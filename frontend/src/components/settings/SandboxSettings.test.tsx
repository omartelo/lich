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

// What the agent holds, swapped between reads so a second one is visible.
let agentKeys: string[] = []

vi.mock("@/lib/rpc", () => ({
  System: {
    SandboxBackend: () => Promise.resolve("bubblewrap"),
    SSHAgentKeys: () => Promise.resolve(agentKeys),
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

// The two grants are the credentials a confined session gets back, and each is
// read as a smaller promise than it makes — the whole point of the sentence is
// the half after the first full stop.
test("each grant says what it hands over", async () => {
  const mounted = await mountBudget(createElement(SandboxSettings))
  await mounted.act(() => {})

  expect(document.body.textContent).toContain(
    "Signs with every identity in your agent, against any host, for as long as the session runs.",
  )
  expect(document.body.textContent).toContain(
    "The agent can read the token out of its environment and spend it outside this repository.",
  )
  await mounted.unmount()
})

// ssh-add is run in a terminal outside this window, so the tab back is when the
// list has to stop being the one read at pane open.
test("re-reads the agent when the window comes back", async () => {
  agentKeys = []
  const mounted = await mountBudget(createElement(SandboxSettings))
  await mounted.act(() => {})
  expect(document.body.textContent).toContain("Nothing loaded.")

  agentKeys = ["me@example.com (ED25519 256)"]
  await mounted.act(() => {
    window.dispatchEvent(new Event("focus"))
  })
  await mounted.act(() => {})

  expect(document.body.textContent).toContain("Loaded: me@example.com (ED25519 256)")
  agentKeys = []
  await mounted.unmount()
})
