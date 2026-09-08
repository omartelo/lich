// @vitest-environment jsdom
//
// The two halves of the plain-folder tree the gate's node environment cannot
// see: the footnote a partial listing owes the reader, and the tick that
// re-reads a folder git has no status for. Both are render paths (the panel's
// other suites are pure logic), so this one opts into jsdom the way the render
// budgets do. The footnote's own wording is pinned next to treeFootnote.
//
// The harness has to be imported before anything that reaches react-dom (see
// @/test/render-budget), which is why it is first.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { buildTree } from "@/lib/git/file-tree"
import type { DiffFile } from "@/lib/git/diff"
import { GIT_POLL } from "@/lib/git/use-git-status"
import { TreeBody, usePlainFolderTick } from "./FilesPanel"

const NO_STATS = new Map<string, DiffFile>()

interface BodyOptions {
  cut?: boolean
  hidden?: string[]
  files?: string[]
}

function body({ cut = false, hidden = [], files = ["a.txt", "src/main.go"] }: BodyOptions) {
  return createElement(TreeBody, {
    tree: buildTree(files),
    query: "",
    active: "",
    toggled: new Set<string>(),
    onToggled: () => {},
    stats: NO_STATS,
    cut,
    hidden,
    loading: false,
    failed: false,
    onOpen: () => {},
    onEditor: () => {},
  })
}

const CUT_LINE = "This folder has more files than the tree can list."
const HIDDEN_LINE = "Hidden: build, node_modules."

test("a cut listing says so under the rows", async () => {
  const budget = await mountBudget(body({ cut: true }))
  const text = document.body.textContent ?? ""
  expect(text).toContain(CUT_LINE)
  // Under the tree, not instead of it: the rows the walk did reach still show.
  expect(text.indexOf("a.txt")).toBeLessThan(text.indexOf(CUT_LINE))
  await budget.unmount()
})

// The judgement call the ignore set makes is answered on screen: a folder that
// keeps its own source under build/ is one line away from the reason.
test("a filtered listing names the directories it stepped over", async () => {
  const budget = await mountBudget(body({ hidden: ["build", "node_modules"] }))
  const text = document.body.textContent ?? ""
  expect(text).toContain(HIDDEN_LINE)
  expect(text).not.toContain(CUT_LINE)
  expect(text.indexOf("a.txt")).toBeLessThan(text.indexOf(HIDDEN_LINE))
  await budget.unmount()
})

test("a listing that reached the end and hid nothing says nothing", async () => {
  const budget = await mountBudget(body({}))
  const text = document.body.textContent ?? ""
  expect(text).not.toContain(CUT_LINE)
  expect(text).not.toContain("Hidden:")
  await budget.unmount()
})

// The empty branch is where the line matters most: a filter that matched
// nothing in a listing that was cut has not searched the folder, and saying only
// "No file matches" would report that as a finished answer.
test("an empty tree still admits what it left out", async () => {
  const budget = await mountBudget(body({ cut: true, hidden: ["build"], files: [] }))
  const text = document.body.textContent ?? ""
  expect(text).toContain("No files here")
  expect(text).toContain("Hidden: build.")
  expect(text).toContain(CUT_LINE)
  await budget.unmount()
})

function probe(on: boolean) {
  return createElement(function Probe() {
    return createElement("span", null, `tick ${usePlainFolderTick(on)}`)
  })
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

test("a plain folder re-reads on the git poller's idle cadence", async () => {
  const budget = await mountBudget(probe(true))
  expect(document.body.textContent).toContain("tick 0")
  await budget.act(() => {
    vi.advanceTimersByTime(GIT_POLL.slowMs)
  })
  expect(document.body.textContent).toContain("tick 1")
  await budget.act(() => {
    vi.advanceTimersByTime(GIT_POLL.slowMs * 2)
  })
  expect(document.body.textContent).toContain("tick 3")
  await budget.unmount()
})

// A repository keys its tree off the git status it already polls, so a second
// timer on the same path would be one walk per tick for nothing.
test("a repository is left to its git status", async () => {
  const budget = await mountBudget(probe(false))
  await budget.act(() => {
    vi.advanceTimersByTime(GIT_POLL.slowMs * 5)
  })
  expect(document.body.textContent).toContain("tick 0")
  await budget.unmount()
})
