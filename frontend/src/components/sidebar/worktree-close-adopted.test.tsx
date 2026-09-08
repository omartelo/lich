// @vitest-environment jsdom
//
// The keep-or-remove confirmation, mounted for real: removing a checkout lich
// did not create is allowed, and the only thing standing between the user and
// their own directory is what this dialog says. The node-only gate cannot read a
// sentence, and the sentence is the whole safeguard.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test } from "vitest"
import type { Session } from "@/lib/session/sessions"
import { WorktreeCloseDialogs } from "./WorktreeCloseDialogs"
import type { WorktreeClose } from "./useWorktreeClose"

const PATH = "/home/dev/checkouts/by-hand"

const session: Session = { id: "s1", label: "by-hand", kind: "claude", path: PATH }

function dialogs(adopted: boolean, onRemove: () => void = () => {}) {
  const close: WorktreeClose = {
    requestClose: () => {},
    pendingRunning: null,
    closeAnyway: () => {},
    pendingClose: session,
    pendingAdopted: adopted,
    pendingForce: null,
    cancel: () => {},
    keep: () => {},
    remove: onRemove,
    forceRemove: () => {},
  }
  return createElement(WorktreeCloseDialogs, { close })
}

const removeButton = () =>
  [...document.querySelectorAll("button")].find((b) => b.textContent === "Remove worktree")

test("an adopted checkout names its absolute path and says lich did not make it", async () => {
  const mounted = await mountBudget(dialogs(true))

  const text = document.body.textContent ?? ""
  expect(text).toContain(PATH)
  expect(text).toContain("lich did not create the worktree at")
  // The reversal: the removal is on offer, behind that sentence.
  expect(removeButton()).toBeDefined()
  await mounted.unmount()
})

test("a checkout lich created is named too, without the whose-directory sentence", async () => {
  const mounted = await mountBudget(dialogs(false))

  const text = document.body.textContent ?? ""
  expect(text).toContain(PATH)
  expect(text).not.toContain("lich did not create")
  expect(removeButton()).toBeDefined()
  await mounted.unmount()
})

test("Remove worktree hands the click to the flow, adopted or not", async () => {
  const clicks: boolean[] = []
  const mounted = await mountBudget(dialogs(true, () => clicks.push(true)))

  const button = removeButton()
  if (!button) {
    throw new Error("Remove worktree button not rendered")
  }
  await mounted.act(() => button.click())
  expect(clicks).toEqual([true])
  await mounted.unmount()
})
