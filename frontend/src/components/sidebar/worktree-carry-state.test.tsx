// @vitest-environment jsdom
//
// The + button's working-tree row: the project's own checkout offered as a base
// with its uncommitted work, never opened on, and handed back as the checkout to
// copy only when it was picked.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import type { GitStatus } from "@/lib/git/use-git-status"
import { WorktreeDialog } from "./WorktreeDialog"

const polled = vi.hoisted(() => ({ paths: [] as string[], status: null as GitStatus | null }))

vi.mock("@/lib/git/use-git-status", () => ({
  useGitStatus: (path: string) => {
    polled.paths.push(path)
    return polled.status
  },
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    ListBranches: () => Promise.resolve({ local: ["develop", "main"], remote: [], worktrees: [] }),
    WorktreeSetup: () => Promise.resolve(null),
  },
}))

vi.mock("@/lib/use-sandbox-choice", () => ({
  useSandboxChoice: () => ({ available: false, confined: false, setConfined() {}, answer: "" }),
}))

type Created = [base: string, carryFrom: string]

function dialog(created: Created[]) {
  return createElement(WorktreeDialog, {
    open: true,
    onOpenChange: () => {},
    projectPath: "/repo",
    projectId: "p1",
    providerId: "claude",
    currentBranch: "main",
    onCreate: (_name, base, _remote, _sandbox, _prompt, carryFrom) => {
      created.push([base, carryFrom])
      return Promise.resolve()
    },
    onResume: () => {},
  })
}

Element.prototype.scrollIntoView = () => {}

const rows = () => [...document.querySelectorAll<HTMLElement>('[role="option"]')]
const selected = () => rows().find((row) => row.getAttribute("aria-selected") === "true")
const treeRow = () => rows().find((row) => row.textContent?.includes("working tree"))

function createButton(): HTMLButtonElement {
  const button = [...document.querySelectorAll("button")].find(
    (b) => b.textContent === "Create worktree",
  )
  if (!button) {
    throw new Error("Create worktree button not rendered")
  }
  return button
}

beforeEach(() => {
  polled.paths = []
  polled.status = { branch: "main", files: 4, added: 12, deleted: 3, head: "abc1234", base: null }
})

test("a dirty project checkout is offered as a base, but the dialog opens on its branch", async () => {
  const created: Created[] = []
  const mounted = await mountBudget(dialog(created))
  await mounted.act(async () => {})

  expect(polled.paths).toContain("/repo")
  expect(treeRow()?.textContent).toContain("main · working tree")
  expect(treeRow()?.textContent).toContain("4 files this checkout has not committed")
  expect(selected()?.textContent).toBe("main")

  await mounted.act(() => createButton().click())

  expect(created).toEqual([["main", ""]])
  await mounted.unmount()
})

test("picking the working-tree row carries the project checkout", async () => {
  const created: Created[] = []
  const mounted = await mountBudget(dialog(created))
  await mounted.act(async () => {})

  const row = treeRow()
  if (!row) {
    throw new Error("working-tree row not rendered")
  }
  await mounted.act(() => row.click())
  await mounted.act(() => createButton().click())

  expect(created).toEqual([["main", "/repo"]])
  await mounted.unmount()
})

test("a clean project checkout offers no working-tree row", async () => {
  polled.status = { branch: "main", files: 0, added: 0, deleted: 0, head: "abc1234", base: null }
  const created: Created[] = []
  const mounted = await mountBudget(dialog(created))
  await mounted.act(async () => {})

  expect(treeRow()).toBeUndefined()
  await mounted.unmount()
})
