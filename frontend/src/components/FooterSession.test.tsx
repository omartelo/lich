// @vitest-environment jsdom
// Contract changed: selected readings are visible without opening a menu.
import { act, createElement, Fragment } from "react"
import { createRoot, type Root } from "react-dom/client"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import type { QuotaPlan } from "@/lib/api-types"
import type { SessionUsage } from "@/lib/session/session-events"
import type { SessionKind } from "@/lib/session/sessions"
import { DEFAULT_FOOTER_LAYOUT, resolveFooterLayout, type FooterLayout } from "@/lib/footer-layout"
import { FooterSession } from "./FooterSession"
import { FooterCheckout } from "./FooterCheckout"
import { FooterSettings } from "./settings/FooterSettings"

const state = vi.hoisted(() => ({
  usage: null as SessionUsage | null,
  plan: null as QuotaPlan | null,
  context: true,
  cost: true,
  budget: 10,
  agent: null as SessionKind | null,
  visibility: { model: true, plan: true, handsOn: true, clock: false },
  layout: null as FooterLayout | null,
  ready: true,
}))
vi.mock("@/lib/session/use-session-usage", () => ({ useSessionUsage: () => state.usage }))
// The footer editor reads the active session to show the model that session is
// actually running, which needs the project context and the router the footer
// itself already has.
vi.mock("@/lib/session/use-active-session", () => ({
  useActiveSession: () => ({
    projectId: "p1",
    sessionId: "s1",
    path: "/repo",
    cwdHost: "",
    checkout: "/repo",
    kind: state.agent ?? "",
    sandboxed: false,
  }),
}))
vi.mock("@/lib/session/use-session-agent", () => ({ useSessionAgent: () => state.agent }))
vi.mock("@/lib/quota/use-plan-quota", () => ({ usePlanQuotaFor: vi.fn(() => state.plan) }))
vi.mock("@/providers/settings", () => ({
  useSettings: () => ({
    showContextUsage: state.context,
    costBudget: state.budget,
    footerVisibility: state.visibility,
    footerLayout: state.layout,
    setFooterLayout: (value: FooterLayout) => {
      state.layout = value
    },
    setCostBudget: (value: number) => {
      state.budget = value
    },
  }),
}))
vi.mock("@/lib/use-cost-readout", () => ({
  useCostReadout: () => state.cost,
  useCostReadoutReady: () => state.ready,
}))
vi.mock("@/lib/cost-readout-store", () => ({
  setCostReadout: vi.fn(async (value: boolean) => {
    state.cost = value
  }),
}))
vi.mock("@/lib/rpc", () => ({ Terminal: { HandsOn: vi.fn(async () => 120) } }))
vi.mock("@/lib/use-now", () => ({ useNow: () => new Date("2026-09-06T12:00:00Z") }))

let root: Root
let container: HTMLDivElement
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true)
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
  Object.assign(state, {
    usage: null,
    plan: null,
    context: true,
    cost: true,
    budget: 10,
    agent: null,
    visibility: { model: true, plan: true, handsOn: true, clock: false },
    layout: null,
    ready: true,
  })
  container = document.createElement("div")
  document.body.append(container)
  root = createRoot(container)
})
afterEach(async () => {
  await act(async () => root.unmount())
  container.remove()
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

const reading = (window: number, costUsd: number | null): SessionUsage => ({
  window,
  costUsd,
  tokens: window / 2,
  percent: 50,
  model: window ? "test-model" : "",
  effort: "",
  costMiss: null,
})
async function render(kind: SessionKind = "codex") {
  const layout = resolveFooterLayout(state.layout, state.visibility, state.context, state.cost)
  await act(async () =>
    root.render(
      createElement(
        Fragment,
        null,
        [...layout.left, ...layout.right].map((item) =>
          createElement(FooterSession, { key: item, sessionId: "active", kind, item }),
        ),
      ),
    ),
  )
}
function button(label: string) {
  const found = Array.from(container.querySelectorAll("button")).find((element) =>
    element.textContent?.startsWith(label),
  )
  if (!found) throw new Error(`Missing button: ${label}`)
  return found
}
async function tooltip(label: string) {
  await act(async () => button(label).focus())
  await vi.waitFor(() => expect(document.querySelector('[role="tooltip"]')).not.toBeNull())
  return document.querySelector('[role="tooltip"]')?.textContent
}

test.each<[SessionKind, number, number | null]>([
  ["claude", 200000, 2],
  ["codex", 200000, 2],
  ["omp", 0, 2],
  ["opencode", 0, 2],
  ["crush", 0, 2],
  ["kiro", 200000, null],
  ["antigravity", 0, null],
  ["cursor", 0, null],
])("%s shows available readings without opening a menu", async (kind, window, cost) => {
  state.usage = window || cost !== null ? reading(window, cost) : null
  await render(kind)
  expect(document.querySelector('[role="dialog"]')).toBeNull()
  expect(container.textContent?.includes("Context window")).toBe(window > 0)
  expect(container.textContent?.includes("Session cost")).toBe(cost !== null)
  expect(container.querySelector('[aria-label^="Context window"]') !== null).toBe(window > 0)
  expect(container.textContent).toContain("Hands-on time")
  expect(container.textContent).toContain("2m")
  expect(container.querySelector("time")).toBeNull()
})

test("model and context visibility are independent", async () => {
  state.usage = reading(200000, null)
  state.context = false
  await render()
  expect(container.textContent).toContain("test-model")
  expect(container.textContent).not.toContain("Context window")
  state.context = true
  state.visibility.model = false
  await render()
  expect(container.textContent).not.toContain("test-model")
  expect(container.textContent).toContain("50%")
})

test("context and budget warnings stay attached to their visible readings", async () => {
  state.usage = { ...reading(200000, 9), percent: 80 }
  await render()
  expect(button("Context window").classList.contains("text-amber-500")).toBe(true)
  expect(button("Session cost").classList.contains("text-amber-500")).toBe(true)
  state.context = false
  state.cost = false
  await render()
  expect(container.textContent).not.toContain("Session cost")
  expect(container.textContent).not.toContain("Context window")
})

test("a locked window stays visible even when another window is fuller", async () => {
  state.plan = {
    provider: "codex",
    name: "Codex",
    status: "ok",
    account: "session account",
    windows: [
      { label: "Session", seconds: 18000, percent: 10, lockedReason: "Limit reached" },
      { label: "Weekly", seconds: 604800, percent: 20 },
    ],
  }
  await render()
  expect(button("Plan usage").textContent).toContain("Locked")
  expect(button("Plan usage").classList.contains("text-destructive")).toBe(true)
  const details = await tooltip("Plan usage")
  expect(details).toContain("Weekly")
  expect(details).toContain("session account")
})

test("plan details retain the provider heading without redundant usage labels", async () => {
  state.plan = {
    provider: "codex",
    name: "Codex",
    plan: "Plus",
    status: "ok",
    windows: [{ label: "Session", seconds: 18000, percent: 50 }],
  }
  await render()
  const details = await tooltip("Plan usage")
  expect(details).toContain("Codex · Plus")
  expect(details).toContain("Session")
  expect(details).not.toContain("Plan usage")
  expect(details).not.toContain("resets in")
})

test.each(["path", "branch"] as const)(
  "checkout %s is text, not a popover trigger",
  async (display) => {
    await act(async () =>
      root.render(
        createElement(FooterCheckout, {
          path: "/project/.worktrees/a-long-worktree",
          branch: "feature/footer",
          display,
        }),
      ),
    )
    expect(container.querySelector("button, [aria-haspopup]")).toBeNull()
    expect(container.querySelector("[title]")?.getAttribute("title")).toBe(
      display === "path" ? "/project/.worktrees/a-long-worktree" : "feature/footer",
    )
  },
)

test("a hosted shell says the cwd is unknown instead of naming the checkout", async () => {
  await act(async () =>
    root.render(
      createElement(FooterCheckout, {
        path: "/project/.worktrees/a-long-worktree",
        branch: "feature/footer",
        display: "path",
        host: "tmux",
      }),
    ),
  )
  expect(container.textContent).toBe("cwd unknown · inside tmux")
  // The path must not survive as the tooltip either: it is a directory the user
  // is not standing in, and a hover would hand it over as though it were.
  expect(container.querySelector("[title]")?.getAttribute("title")).toBe(
    "cwd unknown · inside tmux",
  )
})

test("the branch readout keeps speaking for the checkout while the shell is hosted", async () => {
  // The branch is a fact about the checkout, true wherever the shell went, so
  // only the path readout is passed a host (FooterBar).
  await act(async () =>
    root.render(
      createElement(FooterCheckout, {
        path: "/project/.worktrees/a-long-worktree",
        branch: "feature/footer",
      }),
    ),
  )
  expect(container.textContent).toBe("feature/footer")
})

test.each(["signed-out", "error", "unknown"] as const)(
  "%s never shows quota as zero",
  async (status) => {
    state.plan = { provider: "codex", name: "Codex", status }
    await render()
    expect(container.textContent).not.toContain("Plan usage")
    expect(container.textContent).not.toContain("0%")
  },
)

test("unavailable cost keeps its mark and an explanation reachable by keyboard", async () => {
  state.usage = { ...reading(0, null), costMiss: "mixed-models" }
  await render()
  expect(container.textContent).toContain("$—")
  expect(await tooltip("Cost unavailable")).toContain("switched models")
})

test("a provider started in a shell determines whose plan is shown", async () => {
  state.agent = "claude"
  await render("shell")
  const { usePlanQuotaFor } = await import("@/lib/quota/use-plan-quota")
  expect(usePlanQuotaFor).toHaveBeenCalledWith("claude", "active")
})

test("clock and hands-on time follow independent display choices", async () => {
  state.visibility.clock = true
  state.visibility.handsOn = false
  await render()
  expect(container.querySelector("time")).not.toBeNull()
  expect(container.textContent).not.toContain("Hands-on time")
  const { Terminal } = await import("@/lib/rpc")
  expect(Terminal.HandsOn).not.toHaveBeenCalled()
})

test("zero cost remains a real reading", async () => {
  state.usage = reading(0, 0)
  await render("omp")
  expect(button("Session cost").textContent).toContain("$0")
})

// Contract changed: an item is moved and hidden by dragging it, so the chip has
// no remove button to click. Restore default is the layout change jsdom can
// still make, and it drops the cost item, which is the one that has to carry a
// second write with it.
test("restoring the default writes the layout and turns off the reading it drops", async () => {
  state.layout = { left: ["attach"], right: ["cost", "context"] }
  await act(async () => root.render(createElement(FooterSettings)))
  await act(async () => button("Restore default").click())
  const { setCostReadout } = await import("@/lib/cost-readout-store")
  expect(state.layout).toEqual(DEFAULT_FOOTER_LAYOUT)
  expect(setCostReadout).toHaveBeenCalledWith(false)
  expect(state.cost).toBe(false)
  expect(state.context).toBe(true)
})

// The model chip is the one whose reading is a fact rather than a shape: an
// invented "Codex · GPT-6" named a provider the footer never writes.
test("the editor shows the model the active session is running", async () => {
  state.agent = "claude"
  state.usage = { ...reading(1000, 1), model: "claude-opus-5", effort: "xhigh" }
  await act(async () => root.render(createElement(FooterSettings)))
  expect(container.querySelector('[data-footer-item="model"]')?.textContent).toContain(
    "opus 5 · xhigh",
  )
})

test("the model chip falls back to its name when no session has reported one", async () => {
  state.usage = null
  await act(async () => root.render(createElement(FooterSettings)))
  expect(container.querySelector('[data-footer-item="model"]')?.textContent).toBe("Model")
})

test("the editor waits for the old cost setting before allowing a layout migration", async () => {
  state.ready = false
  await act(async () => root.render(createElement(FooterSettings)))
  expect(container.querySelector<HTMLButtonElement>('[aria-label="Move Model"]')?.disabled).toBe(
    true,
  )
  expect(button("Restore default").disabled).toBe(true)
  expect(state.layout).toBeNull()
})

test("a saved layout controls the order and visibility of readings", async () => {
  state.usage = reading(200000, 2)
  state.layout = { left: ["cost", "context", "model"], right: [] }
  await render()
  // Glyph spacing is presentational; this contract is the order of the readings.
  expect(
    Array.from(container.querySelectorAll("button")).map((item) =>
      item.textContent?.replace(/\s+/g, " "),
    ),
  ).toEqual(["Session cost: $2.00", "Context window: 50%"])
  expect(container.textContent).not.toContain("Hands-on time")
})

test("a failed pricing-setting write leaves the previous layout in place", async () => {
  const { setCostReadout } = await import("@/lib/cost-readout-store")
  vi.mocked(setCostReadout).mockRejectedValueOnce(new Error("offline"))
  state.layout = { left: ["attach"], right: ["cost"] }
  await act(async () => root.render(createElement(FooterSettings)))
  await act(async () => button("Restore default").click())
  expect(state.layout).toEqual({ left: ["attach"], right: ["cost"] })
  expect(container.querySelector('[data-footer-item="cost"]')).not.toBeNull()
})
