// @vitest-environment jsdom
//
// The Fork row of a session card, mounted for real: the node-only gate cannot
// tell a disabled item carrying its reason from an item that never rendered,
// and the reason is the whole point of the row.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test } from "vitest"
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from "@/components/ui/context-menu"
import type { Session } from "@/lib/session/sessions"
import { SessionForkItem } from "./SessionForkItem"

function menu(session: Session, onFork: () => void = () => {}) {
  return createElement(
    ContextMenu,
    { open: true },
    createElement(ContextMenuTrigger, null, "card"),
    createElement(ContextMenuContent, null, createElement(SessionForkItem, { session, onFork })),
  )
}

const item = () =>
  [...document.querySelectorAll('[role="menuitem"]')].find((element) =>
    element.textContent?.startsWith("Fork to worktree"),
  )

test("a provider that cannot fork gets the row disabled, naming why", async () => {
  const session: Session = {
    id: "s1",
    label: "crush",
    kind: "crush",
    providerSessionId: "conv-1",
  }
  const mounted = await mountBudget(menu(session))

  const row = item()
  if (!row) {
    throw new Error("Fork row not rendered")
  }
  expect(row.getAttribute("data-disabled")).not.toBeNull()
  expect(row.textContent).toContain("Crush keeps no fork; resume only.")
  await mounted.unmount()
})

test("a provider that forks gets the row live, with no reason to give", async () => {
  const session: Session = {
    id: "s1",
    label: "claude",
    kind: "claude",
    providerSessionId: "conv-1",
  }
  const forks: number[] = []
  const mounted = await mountBudget(menu(session, () => forks.push(1)))

  const row = item()
  if (!row) {
    throw new Error("Fork row not rendered")
  }
  expect(row.getAttribute("data-disabled")).toBeNull()
  expect(row.textContent).toBe("Fork to worktree…")

  await mounted.act(() => {
    ;(row as HTMLElement).click()
  })
  expect(forks).toEqual([1])
  await mounted.unmount()
})

// Nothing to branch yet: the offer would fail, and no provider limit explains
// it, so the row stays off the card as it always has.
test("a forkable session with no conversation gets no row at all", async () => {
  const mounted = await mountBudget(menu({ id: "s1", label: "claude", kind: "claude" }))
  expect(item()).toBeUndefined()
  await mounted.unmount()
})
