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
  drop: "idle" as const,
  dropFolder: "",
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

const plus = () =>
  [...document.querySelectorAll("button")].find((element) =>
    (element.getAttribute("aria-label") ?? "").startsWith("New session in"),
  )

// A folder has no directory, so its + asks among the checkouts its cards live
// in, one submenu each, and never guesses.
test("a folder spread over two checkouts asks which one its + opens in", async () => {
  const mounted = await mountBudget(
    header({
      name: "Frontend",
      folder: true,
      checkouts: [
        { path: "", label: "elan-app" },
        { path: "/wt/epic-front", label: "epic-front" },
      ],
    }),
  )

  plus()?.click()
  await new Promise((resolve) => setTimeout(resolve, 0))
  expect(document.body.textContent).toContain("New session in Frontend")
  expect(items()).toEqual(["elan-app", "epic-front"])
  await mounted.unmount()
})

test("a folder in one checkout opens there, and says where", async () => {
  const opened: unknown[][] = []
  const mounted = await mountBudget(
    header({
      name: "Apps",
      folder: true,
      checkouts: [{ path: "/wt/epic-front", label: "epic-front" }],
      onNewSession: (...args: unknown[]) => opened.push(args),
    }),
  )

  plus()?.click()
  await new Promise((resolve) => setTimeout(resolve, 0))
  expect(document.body.textContent).toContain("New session in Apps, on epic-front")
  const terminal = [...document.querySelectorAll<HTMLElement>('[role="menuitem"]')].find(
    (element) => element.textContent?.includes("New Terminal"),
  )
  terminal?.click()
  expect(opened).toEqual([["shell", "", "/wt/epic-front"]])
  await mounted.unmount()
})

// The drag finds its targets by this attribute alone, so a header that does
// not take the card must not wear it: a dimmed header that still filed would
// file into a block the user was told was closed to it.
test("only a header that takes the dragged card is a drop target", async () => {
  const target = () => document.querySelector("[data-file-target]")

  for (const drop of ["accepts", "over"] as const) {
    const taking = await mountBudget(header({ drop, dropFolder: "Apps" }))
    expect(target()?.getAttribute("data-file-target")).toBe("Apps")
    await taking.unmount()
  }
  for (const drop of ["idle", "refuses"] as const) {
    const closed = await mountBudget(header({ drop, dropFolder: "Apps" }))
    expect(target()).toBeNull()
    await closed.unmount()
  }
})
