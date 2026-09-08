// @vitest-environment jsdom
//
// The Entrypoint row of a session card, mounted for real: the node-only gate
// cannot tell a disabled item carrying its reason from an item that never
// rendered, and the reason is the whole point of the row.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test } from "vitest"
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from "@/components/ui/context-menu"
import type { Session } from "@/lib/session/sessions"
import { SessionEntrypointItem } from "./SessionEntrypointItem"

function menu(session: Session, onOpen: () => void = () => {}) {
  return createElement(
    ContextMenu,
    { open: true },
    createElement(ContextMenuTrigger, null, "card"),
    createElement(
      ContextMenuContent,
      null,
      createElement(SessionEntrypointItem, { session, onOpen }),
    ),
  )
}

const item = () =>
  [...document.querySelectorAll('[role="menuitem"]')].find((element) =>
    element.textContent?.startsWith("Entrypoint"),
  )

test("an agent card gets the row disabled, naming why", async () => {
  const mounted = await mountBudget(menu({ id: "s1", label: "claude", kind: "claude" }))

  const row = item()
  if (!row) {
    throw new Error("Entrypoint row not rendered")
  }
  expect(row.getAttribute("data-disabled")).not.toBeNull()
  expect(row.textContent).toContain("Entrypoints run in terminal sessions.")
  await mounted.unmount()
})

test("a terminal card gets the row live, with no reason to give", async () => {
  const opened: number[] = []
  const mounted = await mountBudget(
    menu({ id: "s1", label: "term", kind: "shell" }, () => opened.push(1)),
  )

  const row = item()
  if (!row) {
    throw new Error("Entrypoint row not rendered")
  }
  expect(row.getAttribute("data-disabled")).toBeNull()
  expect(row.textContent).toBe("Entrypoint…")

  await mounted.act(() => {
    ;(row as HTMLElement).click()
  })
  expect(opened).toEqual([1])
  await mounted.unmount()
})
