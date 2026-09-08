// @vitest-environment jsdom
//
// The sandbox block of a session's tooltip, mounted for real: what it says is
// the whole of what the card's shield means, and the node-only gate cannot tell
// a sentence that changed from one that never rendered.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import type { Session } from "@/lib/session/sessions"
import { Tooltip } from "@/components/ui/tooltip"
import { SessionTooltip } from "./SessionTooltip"

vi.mock("@/lib/app-events", () => ({ onAppEvent: () => () => {}, dispatchEnvelope: () => {} }))

// The rung each project is on, as the settings store would answer it. Keyed by
// project so every test owns its own answer: the rung store asks once per
// (provider, project) and keeps what it got, which is the point of it.
const rungs = vi.hoisted(() => new Map<string, string>())

// Everything but the two calls the sandbox readout makes hangs, so the git
// status and the pull request stay at their loading branch and nothing else
// lands mid-assertion.
vi.mock("@/lib/rpc", () => {
  const never = () => new Promise<never>(() => {})
  const hangs = new Proxy({}, { get: () => never })
  return {
    endpoint: () => ({ base: "http://localhost", token: "token" }),
    Terminal: hangs,
    DropService: hangs,
    ProjectService: hangs,
    Git: hangs,
    Pulls: hangs,
    Fonts: hangs,
    AgentPlugin: hangs,
    AppUpdate: hangs,
    PatchNotes: hangs,
    Providers: hangs,
    Quota: hangs,
    Themes: hangs,
    Store: {
      GetSetting: (_key: string, projectID: string) =>
        Promise.resolve(projectID ? (rungs.get(projectID) ?? "") : ""),
    },
    System: { SandboxBackend: () => Promise.resolve("bubblewrap") },
  }
})

function tooltip(session: Session, projectId: string) {
  return createElement(
    Tooltip,
    { open: true },
    createElement(SessionTooltip, { session, path: "/repo", projectId }),
  )
}

const text = () => document.querySelector('[data-slot="tooltip-content"]')?.textContent ?? ""

function session(patch: Partial<Session>): Session {
  return { id: "s1", label: "claude", kind: "claude", ...patch }
}

const MOVED = "Opened before the sandbox setting changed; reopen the session to apply it."

test("a confined session on a rung that still confines reads as it always did", async () => {
  rungs.set("p-agrees-on", "everywhere")
  const mounted = await mountBudget(tooltip(session({ sandboxed: true }), "p-agrees-on"))
  expect(text()).toContain("Sandboxed")
  expect(text()).toContain("Set when the session opened; reopen it to change.")
  expect(text()).not.toContain(MOVED)
  await mounted.unmount()
})

// The rung the session opened on is the rung it is still on, so its lack of a
// shield is not news and the block stays off the tooltip entirely.
test("an unconfined session on a rung that confines nothing says nothing", async () => {
  rungs.set("p-agrees-off", "off")
  const mounted = await mountBudget(tooltip(session({}), "p-agrees-off"))
  expect(text()).not.toContain("Sandboxed")
  expect(text()).not.toContain(MOVED)
  await mounted.unmount()
})

test("a session the rung would confine today says why it is not", async () => {
  rungs.set("p-moved-on", "everywhere")
  const mounted = await mountBudget(tooltip(session({}), "p-moved-on"))
  expect(text()).toContain("Not sandboxed")
  expect(text()).toContain(MOVED)
  await mounted.unmount()
})

// The other direction of the same move: the shield on the card is accurate, and
// the sentence is what says the ladder no longer agrees with it.
test("a confined session the rung would now leave out says so too", async () => {
  rungs.set("p-moved-off", "off")
  const mounted = await mountBudget(tooltip(session({ sandboxed: true }), "p-moved-off"))
  expect(text()).toContain("Sandboxed")
  expect(text()).toContain(MOVED)
  await mounted.unmount()
})

// What the whole change exists for: the dotfiles the private home did not get,
// named on the card instead of discovered as a git that has forgotten who you
// are.
test("a confined session names what the sandbox skipped for being a symlink", async () => {
  rungs.set("p-links", "everywhere")
  const mounted = await mountBudget(
    tooltip(
      session({ sandboxed: true, sandboxSkippedLinks: [".gitconfig", ".ssh/known_hosts"] }),
      "p-links",
    ),
  )
  expect(text()).toContain("Not mounted (symlinks): .gitconfig, .ssh/known_hosts")
  await mounted.unmount()
})

// Nothing skipped is nothing to say: the line is absent rather than empty.
test("a confined session that skipped nothing draws no line", async () => {
  rungs.set("p-no-links", "everywhere")
  const mounted = await mountBudget(tooltip(session({ sandboxed: true }), "p-no-links"))
  expect(text()).toContain("Sandboxed")
  expect(text()).not.toContain("Not mounted")
  await mounted.unmount()
})
