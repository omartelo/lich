// @vitest-environment jsdom
import { act } from "react"
import { createRoot, type Root } from "react-dom/client"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { hoverHint } from "@/test/hint"
import { Hint } from "./Hint"

let container: HTMLDivElement
let root: Root

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true)
  container = document.createElement("div")
  document.body.append(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
  vi.unstubAllGlobals()
})

async function render(label: string | undefined) {
  await act(async () =>
    root.render(
      <Hint label={label}>
        <button type="button">Go</button>
      </Hint>,
    ),
  )
  return container.querySelector("button") as HTMLButtonElement
}

test("hovering the control shows its label", async () => {
  const button = await render("Open project")
  expect(await hoverHint(button)).toBe("Open project")
})

test("the control is the trigger itself, not a wrapper around it", async () => {
  const button = await render("Open project")
  expect(container.firstElementChild).toBe(button)
  expect(button.dataset.slot).toBe("tooltip-trigger")
})

test.each([undefined, ""])("no label (%j) leaves the control bare", async (label) => {
  const button = await render(label)
  expect(button.dataset.slot).toBeUndefined()
  expect(await hoverHint(button)).toBeUndefined()
})
