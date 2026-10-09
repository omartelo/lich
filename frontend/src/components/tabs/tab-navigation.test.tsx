// @vitest-environment jsdom
//
// The window is Chromium, and Chromium shows the URL of any hovered <a href> in
// a status bubble at the corner of the window: for a tab that URL carries the
// session token. The tabs navigate without being links so there is nothing to
// show.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget, type RenderBudget } from "@/test/render-budget"
import { DndContext } from "@dnd-kit/core"
import { createElement, type ReactElement } from "react"
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom"
import { expect, test, vi } from "vitest"
import { HomeTab } from "./HomeTab"
import { ProjectTab } from "./ProjectTab"

function Location(): ReactElement {
  return createElement("output", null, useLocation().pathname)
}

function inRouter(tab: ReactElement): ReactElement {
  return createElement(
    MemoryRouter,
    { initialEntries: ["/start"] },
    createElement(DndContext, null, tab),
    createElement(
      Routes,
      null,
      createElement(Route, { path: "*", element: createElement(Location) }),
    ),
  )
}

const pathname = (): string => document.querySelector("output")?.textContent ?? ""

async function click(mounted: RenderBudget, selector: string): Promise<void> {
  const node = document.querySelector(selector)
  if (!(node instanceof HTMLElement)) {
    throw new Error(`${selector} not rendered`)
  }
  await mounted.act(() => node.click())
}

test("the home tab navigates without being a link", async () => {
  const mounted = await mountBudget(
    inRouter(createElement(HomeTab, { to: "/projects/home", active: false })),
  )

  expect(document.querySelector("a[href]")).toBeNull()
  await click(mounted, '[aria-label="Home"]')
  expect(pathname()).toBe("/projects/home")
  await mounted.unmount()
})

test("a project tab navigates without being a link, and its × closes without navigating", async () => {
  const onClose = vi.fn()
  const project = { id: "p1", name: "lich", path: "/src/lich" }
  const mounted = await mountBudget(
    inRouter(
      createElement(ProjectTab, {
        project,
        sessionIds: [],
        to: "/projects/p1",
        active: false,
        onClose,
      }),
    ),
  )

  expect(document.querySelector("a[href]")).toBeNull()
  await click(mounted, '[aria-label="Close lich"]')
  expect(onClose).toHaveBeenCalledOnce()
  expect(pathname()).toBe("/start")

  await click(mounted, '[title="/src/lich"]')
  expect(pathname()).toBe("/projects/p1")
  await mounted.unmount()
})
