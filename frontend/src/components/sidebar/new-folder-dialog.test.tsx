// @vitest-environment jsdom
//
// The dialog that names a folder, mounted for real: the node-only gate cannot
// tell a refused empty name from a name that was never typed, and refusing it is
// the whole of what this dialog decides.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test } from "vitest"
import { NewFolderDialog } from "./NewFolderDialog"

const field = () => document.querySelector<HTMLInputElement>("#new-folder-name")

const button = (label: string) =>
  [...document.querySelectorAll("button")].find((element) => element.textContent?.trim() === label)

function type(value: string) {
  const input = field()
  if (!input) {
    throw new Error("name field not rendered")
  }
  // The value setter is called off the prototype so React's own onChange sees
  // the change, the way it would from a keystroke.
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set
  setter?.call(input, value)
  input.dispatchEvent(new Event("input", { bubbles: true }))
}

function dialog(overrides: Partial<Parameters<typeof NewFolderDialog>[0]> = {}) {
  return createElement(NewFolderDialog, {
    open: true,
    onOpenChange: () => {},
    count: 1,
    existing: [],
    onCreate: () => {},
    ...overrides,
  })
}

test("a name of nothing but spaces creates no folder", async () => {
  let created = ""
  const mounted = await mountBudget(dialog({ onCreate: (name) => (created = name) }))

  type("   ")
  button("Create")?.click()

  expect(created).toBe("")
  await mounted.unmount()
})

test("the typed name is trimmed before it files anything", async () => {
  let created = ""
  const mounted = await mountBudget(dialog({ onCreate: (name) => (created = name) }))

  type("  Design system  ")
  button("Create")?.click()

  expect(created).toBe("Design system")
  await mounted.unmount()
})

// A name the project already holds is not a second folder — filing into the
// existing one is what happens, so the dialog says so before it does.
test("an existing name files into that folder instead of making another", async () => {
  let created = ""
  const mounted = await mountBudget(
    dialog({ existing: ["Apps"], onCreate: (name) => (created = name) }),
  )

  type("Apps")
  expect(document.body.textContent).toContain("already exists")
  button("Move here")?.click()

  expect(created).toBe("Apps")
  await mounted.unmount()
})

test("filing a whole block says how many sessions it is about to file", async () => {
  const mounted = await mountBudget(dialog({ count: 4 }))

  expect(document.body.textContent).toContain("these 4 sessions")
  await mounted.unmount()
})
