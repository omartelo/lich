// @vitest-environment jsdom
//
// The reopen menu lists five closed projects and the store keeps far more than
// that. Nothing in the menu used to say so, which is the whole reason this line
// exists, and the node-only gate cannot tell a line that renders from one that
// never did.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import type { RecentProject } from "@/lib/api-types"
import { OpenProjectMenu } from "./OpenProjectMenu"

let closedCount = 0

function project(id: string): RecentProject {
  return { id, name: id, path: `/src/${id}` }
}

vi.mock("@/lib/rpc", () => ({
  Store: {
    RecentProjects: () =>
      Promise.resolve(["p1", "p2", "p3", "p4", "p5", "p6"].map((id) => project(id))),
    ClosedProjectCount: () => Promise.resolve(closedCount),
  },
  ProjectService: {
    Missing: () => Promise.resolve([]),
  },
}))

// One frozen value, not a fresh object per render: the menu refetches whenever
// `projects` changes identity, and a new array each render is an endless loop.
vi.mock("@/providers/projects", () => {
  const value = { projects: [], openProject: () => {}, openRecent: () => {} }
  return { useProjects: () => value }
})

// base-ui opens the menu on the trigger's click, so the test opens it the way a
// user does rather than reaching for a prop the component does not take.
async function openMenu(): Promise<void> {
  const trigger = document.querySelector('[aria-label="Open project"]')
  if (!(trigger instanceof HTMLElement)) {
    throw new Error("menu trigger not rendered")
  }
  trigger.click()
  await new Promise((resolve) => setTimeout(resolve, 30))
}

const menuText = (): string =>
  [...document.querySelectorAll('[role="menu"]')].map((node) => node.textContent ?? "").join(" ")

beforeEach(() => {
  closedCount = 0
})

test("the menu points at the palette for the closed projects it does not list", async () => {
  closedCount = 30
  const mounted = await mountBudget(createElement(OpenProjectMenu))
  await new Promise((resolve) => setTimeout(resolve, 30))
  await openMenu()

  // Five rows are listed of thirty closed, so twenty-five are only reachable
  // through the palette.
  expect(menuText()).toContain("25 more closed projects")
  await mounted.unmount()
})

test("one project left over is named in the singular", async () => {
  closedCount = 6
  const mounted = await mountBudget(createElement(OpenProjectMenu))
  await new Promise((resolve) => setTimeout(resolve, 30))
  await openMenu()

  expect(menuText()).toContain("1 more closed project ")
  await mounted.unmount()
})

test("a menu that lists every closed project says nothing about the palette", async () => {
  closedCount = 5
  const mounted = await mountBudget(createElement(OpenProjectMenu))
  await new Promise((resolve) => setTimeout(resolve, 30))
  await openMenu()

  // The menu is open, so an absent line is an absent line and not an unopened
  // menu: the assertion below would pass either way.
  expect(menuText()).toContain("Recent projects")
  expect(menuText()).not.toContain("more closed project")
  await mounted.unmount()
})
