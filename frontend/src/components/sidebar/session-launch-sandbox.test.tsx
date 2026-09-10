// @vitest-environment jsdom
//
// The New session menu putting the "Ask each time" rung's question, mounted for
// real: the answer is what the menu does on a click, and the node-only gate
// cannot tell an extra step from a card that opened.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { ProviderState } from "@/lib/providers-store"
import type { SandboxAnswer } from "@/lib/use-sandbox-choice"
import { SessionLaunchMenuItems } from "./SessionLaunchMenuItems"

const store = vi.hoisted(() => ({ backend: "bubblewrap", rungs: {} as Record<string, string> }))

vi.mock("@/lib/rpc", () => ({
  System: { SandboxBackend: () => Promise.resolve(store.backend) },
  Store: { GetSetting: (key: string) => Promise.resolve(store.rungs[key] ?? "") },
}))

beforeEach(() => {
  store.backend = "bubblewrap"
  store.rungs = {}
})

const claude: ProviderState = {
  id: "claude",
  name: "Claude Code",
  binary: "claude",
  installed: true,
  source: "path",
  enabled: true,
  docs: "",
}

type Opened = [string, SandboxAnswer]

// A project of its own per test: the rung store caches by (provider, project)
// and never clears — the same cache a card reads, so two tests sharing a
// project would share the first one's answer.
function menu(opened: Opened[], projectId: string) {
  return createElement(
    DropdownMenu,
    { open: true },
    createElement(DropdownMenuTrigger, null, "+"),
    createElement(
      DropdownMenuContent,
      null,
      createElement(SessionLaunchMenuItems, {
        providers: [claude],
        terminalLabel: "Terminal" as const,
        projectId,
        onNewSession: (kind, sandbox) => opened.push([kind, sandbox]),
      }),
    ),
  )
}

const item = (label: string) =>
  [...document.querySelectorAll('[role="menuitem"],[role="menuitemcheckbox"]')].find(
    (element) => element.textContent === label,
  ) as HTMLElement | undefined

test("the Ask rung puts the question before the card, and the answer opens with it", async () => {
  store.rungs["provider.claude.sandbox"] = "ask"
  const opened: Opened[] = []
  const mounted = await mountBudget(menu(opened, "asks"))

  const provider = item("Claude Code")
  if (!provider) {
    throw new Error("provider item not rendered")
  }
  await mounted.act(() => provider.click())
  expect(opened).toEqual([])
  expect(document.body.textContent).toContain("Run confined")

  // The rung's own answer is the confined one, so the box the user leaves
  // untouched is the one the unattended callers would have taken.
  const open = item("Open session")
  if (!open) {
    throw new Error("open item not rendered")
  }
  await mounted.act(() => open.click())
  expect(opened).toEqual([["claude", "on"]])
  await mounted.unmount()
})

test("unticking the box is what opens the session on the machine", async () => {
  store.rungs["provider.claude.sandbox"] = "ask"
  const opened: Opened[] = []
  const mounted = await mountBudget(menu(opened, "unticks"))

  await mounted.act(() => item("Claude Code")?.click())
  await mounted.act(() => item("Run confined")?.click())
  await mounted.act(() => item("Open session")?.click())

  expect(opened).toEqual([["claude", "off"]])
  await mounted.unmount()
})

// Back leaves the menu standing on its list, so the question can be reached
// again — and the answer to it never opens a card by itself.
test("Back puts the menu on its list without opening anything", async () => {
  store.rungs["provider.claude.sandbox"] = "ask"
  const opened: Opened[] = []
  const mounted = await mountBudget(menu(opened, "back"))

  await mounted.act(() => item("Claude Code")?.click())
  await mounted.act(() => item("Back")?.click())

  expect(opened).toEqual([])
  expect(item("Claude Code")).toBeDefined()
  expect(item("Terminal")).toBeDefined()
  await mounted.unmount()
})

// Every other rung answers for itself, in the store, on the spawn. Asking there
// would be a step with one honest answer.
test.each(["", "off", "worktrees", "everywhere"])(
  "the %s rung opens on the click",
  async (rung) => {
    store.rungs["provider.claude.sandbox"] = rung
    const opened: Opened[] = []
    const mounted = await mountBudget(menu(opened, `rung-${rung}`))

    await mounted.act(() => item("Claude Code")?.click())

    expect(opened).toEqual([["claude", ""]])
    expect(document.body.textContent).not.toContain("Run confined")
    await mounted.unmount()
  },
)

// A rung is a question about confinement, and a machine that cannot confine has
// nothing to ask about: the menu opens the card and the spawn leaves it alone.
test("a machine with no sandbox backend is never asked", async () => {
  store.backend = ""
  store.rungs["provider.claude.sandbox"] = "ask"
  const opened: Opened[] = []
  const mounted = await mountBudget(menu(opened, "no-backend"))

  await mounted.act(() => item("Claude Code")?.click())

  expect(opened).toEqual([["claude", ""]])
  await mounted.unmount()
})
