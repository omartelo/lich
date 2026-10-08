// @vitest-environment jsdom
//
// The two race dialogs, mounted for real: Race agents asks for two agents and a
// task before it starts anything and names the branches it will make, and
// Consolidate opens on a prompt naming every raced branch. Antigravity and Crush
// are drawn but cannot be picked in either.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import type { ProviderState } from "@/lib/providers-store"
import type { ProviderKind } from "@/lib/session/sessions"
import { ConsolidateDialog } from "./ConsolidateDialog"
import { RaceAgentsDialog } from "./RaceAgentsDialog"

vi.mock("@/lib/git/use-git-status", () => ({ useGitStatus: () => null }))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    ListBranches: () => Promise.resolve({ local: ["main"], remote: [], worktrees: [] }),
    WorktreeSetup: () => Promise.resolve(null),
    BranchesOf: (paths: string[]) =>
      Promise.resolve(Object.fromEntries(paths.map((path) => [path, path.split("/").pop()]))),
  },
}))

vi.mock("@/lib/use-sandbox-choice", () => ({
  useSandboxChoice: () => ({ available: false, confined: false, setConfined() {}, answer: "" }),
}))

const provider = (id: ProviderKind, name: string): ProviderState => ({
  id,
  name,
  binary: id,
  installed: true,
  source: "path",
  enabled: true,
  docs: "",
})

const providers = [
  provider("claude", "Claude Code"),
  provider("codex", "Codex"),
  provider("crush", "Crush"),
]

Element.prototype.scrollIntoView = () => {}

const button = (text: string) =>
  [...document.querySelectorAll("button")].find((b) => b.textContent === text)

const agentBox = (name: string) =>
  [...document.querySelectorAll("label")]
    .find((label) => label.textContent === name)
    ?.querySelector<HTMLElement>('[role="checkbox"]')

// React tracks a field's value itself, so a test sets it the way the browser
// would: through the native setter, then the input event.
function type(field: HTMLTextAreaElement | HTMLInputElement, value: string) {
  const proto = Object.getPrototypeOf(field)
  Object.getOwnPropertyDescriptor(proto, "value")?.set?.call(field, value)
  field.dispatchEvent(new Event("input", { bubbles: true }))
}

interface Started {
  name: string
  base: string
  task: string
  agents: ProviderKind[]
}

function race(started: Started[]) {
  return createElement(RaceAgentsDialog, {
    open: true,
    onOpenChange: () => {},
    projectPath: "/repo",
    projectId: "p1",
    providers,
    defaultProvider: "claude",
    currentBranch: "main",
    onStart: (name, base, _sandbox, task, agents) => {
      started.push({ name, base: base.branch, task, agents })
      return Promise.resolve()
    },
  })
}

test("a race waits for a second agent and a task, then names a branch per agent", async () => {
  const started: Started[] = []
  const mounted = await mountBudget(race(started))
  await mounted.act(async () => {})

  expect(agentBox("Claude Code")?.getAttribute("aria-checked")).toBe("true")
  expect(button("Start 2 agents")?.disabled).toBe(true)
  expect(document.body.textContent).toContain("Pick at least two agents.")

  await mounted.act(() => agentBox("Codex")?.click())
  const task = document.querySelector<HTMLTextAreaElement>("#race-task")
  if (!task) {
    throw new Error("task field not rendered")
  }
  expect(button("Start 2 agents")?.disabled).toBe(true)
  await mounted.act(() => type(task, "Fix the auth redirect"))
  expect(document.body.textContent).toContain(
    "Branches: fix-the-auth-redirect-claude, fix-the-auth-redirect-codex",
  )
  await mounted.act(() => button("Start 2 agents")?.click())

  expect(started).toEqual([
    {
      name: "fix-the-auth-redirect",
      base: "main",
      task: "Fix the auth redirect",
      agents: ["claude", "codex"],
    },
  ])
  await mounted.unmount()
})

test("a provider that cannot race is drawn dead, with the reason", async () => {
  const mounted = await mountBudget(race([]))
  await mounted.act(async () => {})

  expect(agentBox("Crush")?.hasAttribute("data-disabled")).toBe(true)
  expect(document.body.textContent).toContain(
    "Crush cannot run here: its first start in a worktree asks a question lich cannot see past.",
  )
  await mounted.unmount()
})

test("a consolidation opens on a prompt naming every raced branch", async () => {
  const consolidated: { name: string; prompt: string; agent: ProviderKind }[] = []
  const mounted = await mountBudget(
    createElement(ConsolidateDialog, {
      folder: "fix",
      onClose: () => {},
      projectPath: "/repo",
      projectId: "p1",
      paths: ["/wt/fix-claude", "/wt/fix-codex"],
      task: "fix the redirect",
      providers,
      defaultProvider: "claude",
      currentBranch: "main",
      onConsolidate: (name, _base, _sandbox, prompt, agent) => {
        consolidated.push({ name, prompt, agent })
        return Promise.resolve()
      },
    }),
  )
  await mounted.act(async () => {})

  const prompt = document.querySelector<HTMLTextAreaElement>("#consolidate-prompt")
  expect(prompt?.value).toContain("- fix-claude, at /wt/fix-claude")
  expect(prompt?.value).toContain("- fix-codex, at /wt/fix-codex")
  expect(prompt?.value).toContain("fix the redirect")
  expect(document.body.textContent).toContain("Branch: fix-consolidated")

  await mounted.act(() => agentBox("Codex")?.click())
  await mounted.act(() => button("Consolidate")?.click())

  expect(consolidated).toHaveLength(1)
  expect(consolidated[0].name).toBe("fix-consolidated")
  expect(consolidated[0].agent).toBe("codex")
  expect(consolidated[0].prompt).toContain("git diff main...<branch>")
  await mounted.unmount()
})
