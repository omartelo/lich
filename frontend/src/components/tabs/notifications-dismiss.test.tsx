// @vitest-environment jsdom
//
// The bell's X takes a row out of the queue without routing to its session, and
// leaves the menu open for the next one. The node-only gate cannot tell an X
// whose click also reaches the row (navigating away) from one that does not.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { MemoryRouter } from "react-router-dom"
import { expect, test, vi } from "vitest"
import { NotificationsButton } from "./NotificationsButton"

const events = vi.hoisted(() => ({ handlers: new Map<string, (data: unknown) => void>() }))

vi.mock("@/lib/app-events", () => ({
  onAppEvent: (name: string, handler: (data: unknown) => void) => {
    events.handlers.set(name, handler)
    return () => {}
  },
}))

vi.mock("@/lib/rpc", () => ({
  Store: { SetSessionUnread: () => Promise.resolve() },
}))

const activateSession = vi.hoisted(() => vi.fn())
vi.mock("@/providers/projects", () => {
  const value = {
    projects: [{ id: "p1", name: "skipo" }],
    sessions: {
      p1: {
        sessions: [{ id: "s1", label: "Session 7" }],
        activeId: "",
        nextSeq: 2,
      },
    },
    activateSession,
  }
  return { useProjects: () => value }
})

const menuText = (): string =>
  [...document.querySelectorAll('[role="menu"]')].map((node) => node.textContent ?? "").join(" ")

test("the X dismisses a notification without opening its session", async () => {
  const mounted = await mountBudget(
    createElement(MemoryRouter, null, createElement(NotificationsButton)),
  )
  events.handlers.get("session-status")?.({ id: "s1", state: "done" })
  await new Promise((resolve) => setTimeout(resolve, 30))
  ;(document.querySelector('[aria-label^="Notifications"]') as HTMLElement).click()
  await new Promise((resolve) => setTimeout(resolve, 30))
  expect(menuText()).toContain("Session 7")

  ;(document.querySelector('[aria-label="Dismiss Session 7"]') as HTMLElement).click()
  await new Promise((resolve) => setTimeout(resolve, 30))

  expect(activateSession).not.toHaveBeenCalled()
  expect(menuText()).toContain("You're all caught up")
  await mounted.unmount()
})
