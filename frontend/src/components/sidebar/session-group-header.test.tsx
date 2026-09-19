// @vitest-environment jsdom
//
// The block header's own menu, mounted for real: whether a block offers to file
// its whole list is a prop the node-only gate cannot see, and offering it on a
// filtered list files part of a group while saying it filed the group.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test } from "vitest"
import { SessionGroupHeader } from "./SessionGroupHeader"

const base = {
  name: "wt-b",
  fixed: false,
  launch: true,
  folder: false,
  count: 3,
  mark: null,
  collapsed: false,
  isDragging: false,
  providers: [],
  projectId: "p1",
  activatorRef: () => {},
  activatorProps: {},
  onToggle: () => {},
  onNewSession: () => {},
}

function header(overrides: Record<string, unknown> = {}) {
  return createElement(SessionGroupHeader, { ...base, ...overrides })
}

const options = () =>
  [...document.querySelectorAll("button")].find((element) =>
    (element.getAttribute("aria-label") ?? "").startsWith("Options for"),
  )

const items = () =>
  [...document.querySelectorAll('[role="menuitem"]')].map((element) => element.textContent?.trim())

// The filtered case: the sidebar withholds the handlers, and the header must
// then carry no menu at all rather than an empty one.
test("a block with nothing to say about itself has no options button", async () => {
  const mounted = await mountBudget(header())

  expect(options()).toBeUndefined()
  await mounted.unmount()
})

test("a checkout's block offers to file its whole list", async () => {
  const mounted = await mountBudget(
    header({ folders: ["Apps"], onFileAll: () => {}, onNewFolder: () => {} }),
  )

  options()?.click()
  await new Promise((resolve) => setTimeout(resolve, 0))
  expect(items()).toContain("Move group to folder")
  await mounted.unmount()
})

// A folder renames and comes apart; it is already filed, so it is never offered
// filing of its own.
test("a folder's block renames and ungroups, and is not offered filing", async () => {
  const mounted = await mountBudget(
    header({
      name: "Design system",
      folder: true,
      launch: false,
      onRename: () => {},
      onDissolve: () => {},
    }),
  )

  options()?.click()
  await new Promise((resolve) => setTimeout(resolve, 0))
  expect(items()).toEqual(["Rename folder", "Ungroup"])
  await mounted.unmount()
})

test("a folder says how many sessions it holds; a checkout does not", async () => {
  const folder = await mountBudget(header({ name: "Apps", folder: true, count: 14 }))
  expect(document.body.textContent).toContain("14")
  await folder.unmount()

  const checkout = await mountBudget(header({ count: 14 }))
  expect(document.body.textContent).not.toContain("14")
  await checkout.unmount()
})

// Folding hides the cards' rings, so the block has to carry what they were
// saying. Amber outranks emerald, and an open block draws neither.
test("a folded block draws the mark it was given, an open one draws none", async () => {
  const folded = await mountBudget(header({ collapsed: true, mark: "wait" }))
  expect(document.body.textContent).toContain("waiting on you")
  await folded.unmount()

  const open = await mountBudget(header({ collapsed: false, mark: "wait" }))
  expect(document.body.textContent).not.toContain("waiting on you")
  await open.unmount()
})
