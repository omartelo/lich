// @vitest-environment jsdom
//
// A fold stored while the block had siblings outlives them: once the block is
// the only one, the sidebar draws it with no header, which is the only control
// that unfolds it.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { HashRouter } from "react-router-dom"
import { afterEach, expect, test } from "vitest"
import { readGroupCollapsed, writeGroupCollapsed } from "@/lib/session/group-prefs"
import { ROOT_GROUP_KEY } from "@/lib/session/sidebar-groups"
import { ProjectsContext, type ProjectsValue } from "@/providers/projects-context"
import { SessionGroup } from "./SessionGroup"

const PROJECT = "p1"

function block(showHeader: boolean) {
  const props = {
    projectId: PROJECT,
    sortId: ROOT_GROUP_KEY,
    pinned: false,
    stage: null,
    folder: "",
    onRenameGroup: () => {},
    onDissolveGroup: () => {},
    folders: [],
    onFile: () => {},
    onNewFolder: () => {},
    path: "",
    sessions: [],
    projectPath: "/project",
    activeId: "",
    stageIds: [],
    onStageToggle: () => {},
    onGroupDelegates: () => {},
    onFork: () => {},
    showHeader,
    sortable: true,
    onReorder: () => {},
    onClose: () => {},
    pullsActive: false,
    onPulls: () => {},
    onClosePulls: () => {},
    delegateGroups: [],
    providers: [],
  }
  return createElement(
    HashRouter,
    null,
    createElement(
      ProjectsContext.Provider,
      { value: {} as ProjectsValue },
      createElement(SessionGroup, props),
    ),
  )
}

const hidden = () => document.querySelector('[aria-hidden="true"]') !== null

afterEach(() => {
  writeGroupCollapsed(PROJECT, ROOT_GROUP_KEY, false)
})

test("a lone block draws its cards even when it was folded", async () => {
  writeGroupCollapsed(PROJECT, ROOT_GROUP_KEY, true)
  const mounted = await mountBudget(block(false))

  expect(hidden()).toBe(false)
  // The fold is the user's and comes back with a sibling, so it is kept.
  expect(readGroupCollapsed(PROJECT, ROOT_GROUP_KEY)).toBe(true)
  await mounted.unmount()
})

test("a block with a header stays folded", async () => {
  writeGroupCollapsed(PROJECT, ROOT_GROUP_KEY, true)
  const mounted = await mountBudget(block(true))

  expect(hidden()).toBe(true)
  await mounted.unmount()
})
