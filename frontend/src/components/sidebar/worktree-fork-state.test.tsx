// @vitest-environment jsdom
//
// The fork's working-tree row, mounted for real: which row a fork opens on and
// what the Fork button hands back are both decided inside the dialog's render,
// and the node-only gate cannot see either.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import type { GitStatus } from "@/lib/git/use-git-status"
import { WorktreeDialog } from "./WorktreeDialog"

const source = vi.hoisted(() => ({ status: null as GitStatus | null }))

vi.mock("@/lib/git/use-git-status", () => ({
  // Only the forked session's checkout is polled; the + button passes "".
  useGitStatus: (path: string) => (path ? source.status : null),
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    ListBranches: () =>
      Promise.resolve({ local: ["icy-glacier", "main"], remote: [], worktrees: [] }),
    WorktreeSetup: () => Promise.resolve(null),
  },
}))

vi.mock("@/lib/use-sandbox-choice", () => ({
  useSandboxChoice: () => ({ available: false, confined: false, setConfined() {}, answer: "" }),
}))

type Created = [name: string, base: string, remote: boolean, sandbox: string, carryFrom: string]

function dialog(created: Created[]) {
  return createElement(WorktreeDialog, {
    open: true,
    onOpenChange: () => {},
    projectPath: "/repo",
    projectId: "p1",
    providerId: "claude",
    currentBranch: "main",
    onCreate: (name, base, baseIsRemote, sandbox, _prompt, carryFrom) => {
      created.push([name, base, baseIsRemote, sandbox, carryFrom])
      return Promise.resolve()
    },
    onResume: () => {},
    forkOf: { label: "claudin", path: "/wt/icy-glacier" },
  })
}

// jsdom lays nothing out and so implements no scrolling; the dialog keeps the
// selected row in view on every change.
Element.prototype.scrollIntoView = () => {}

const rows = () => [...document.querySelectorAll('[role="option"]')]
const selected = () => rows().find((row) => row.getAttribute("aria-selected") === "true")
const forkButton = () =>
  [...document.querySelectorAll("button")].find((button) => button.textContent === "Fork")

beforeEach(() => {
  source.status = {
    branch: "icy-glacier",
    files: 4,
    added: 12,
    deleted: 3,
    head: "abc1234",
    base: null,
  }
})

test("a dirty fork opens on its own working tree and carries that checkout", async () => {
  const created: Created[] = []
  const mounted = await mountBudget(dialog(created))
  await mounted.act(async () => {})

  const row = selected()
  expect(row?.textContent).toContain("icy-glacier · working tree")
  expect(row?.textContent).toContain("4 files")

  const button = forkButton()
  if (!button) {
    throw new Error("Fork button not rendered")
  }
  await mounted.act(() => button.click())

  // The base is the branch every other row would have named; the checkout rides
  // alongside it as what the new worktree is copied from.
  expect(created).toEqual([["", "icy-glacier", false, "", "/wt/icy-glacier"]])
  await mounted.unmount()
})

test("a fork with nothing uncommitted has no working-tree row, and carries nothing", async () => {
  source.status = {
    branch: "icy-glacier",
    files: 0,
    added: 0,
    deleted: 0,
    head: "abc1234",
    base: null,
  }
  const created: Created[] = []
  const mounted = await mountBudget(dialog(created))
  await mounted.act(async () => {})

  expect(rows().some((row) => row.textContent?.includes("working tree"))).toBe(false)
  expect(selected()?.textContent).toBe("icy-glacier")

  const button = forkButton()
  if (!button) {
    throw new Error("Fork button not rendered")
  }
  await mounted.act(() => button.click())

  expect(created).toEqual([["", "icy-glacier", false, "", ""]])
  await mounted.unmount()
})

test("a detached source offers no working-tree row: git would refuse the base", async () => {
  source.status = {
    branch: "",
    files: 4,
    added: 12,
    deleted: 3,
    head: "abc1234",
    base: null,
  }
  const created: Created[] = []
  const mounted = await mountBudget(dialog(created))
  await mounted.act(async () => {})

  expect(rows().some((row) => row.textContent?.includes("working tree"))).toBe(false)
  expect(selected()?.textContent).toBe("main")
  await mounted.unmount()
})

const search = () =>
  document.querySelector('[aria-label="Search base branches"]') as HTMLInputElement | null

// Typing into the filter, the way a user narrows a long branch list. jsdom
// needs the native setter to make React see the change.
async function type(mounted: { act: (run: () => void) => Promise<void> }, value: string) {
  const input = search()
  if (!input) {
    throw new Error("search field not rendered")
  }
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set
  await mounted.act(() => {
    setter?.call(input, value)
    input.dispatchEvent(new Event("input", { bubbles: true }))
  })
}

test("a filter that hides the working-tree row moves the selection off it", async () => {
  const created: Created[] = []
  const mounted = await mountBudget(dialog(created))
  await mounted.act(async () => {})
  await type(mounted, "main")

  expect(rows().some((row) => row.textContent?.includes("working tree"))).toBe(false)
  expect(selected()?.textContent).toBe("main")

  const button = forkButton()
  if (!button) {
    throw new Error("Fork button not rendered")
  }
  await mounted.act(() => button.click())

  expect(created).toEqual([["", "main", false, "", ""]])
  await mounted.unmount()
})

test("the row going away takes its selection with it, rather than leaving no base", async () => {
  const created: Created[] = []
  const mounted = await mountBudget(dialog(created))
  await mounted.act(async () => {})
  expect(selected()?.textContent).toContain("working tree")

  // The forked session commits while the dialog is open: the row it was
  // selecting is gone by the next render.
  source.status = {
    branch: "icy-glacier",
    files: 0,
    added: 0,
    deleted: 0,
    head: "def5678",
    base: null,
  }
  await type(mounted, "icy")

  expect(selected()?.textContent).toBe("icy-glacier")

  const button = forkButton()
  if (!button) {
    throw new Error("Fork button not rendered")
  }
  await mounted.act(() => button.click())

  expect(created).toEqual([["", "icy-glacier", false, "", ""]])
  await mounted.unmount()
})
