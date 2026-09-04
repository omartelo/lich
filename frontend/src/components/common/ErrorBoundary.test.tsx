// @vitest-environment jsdom
//
// The bug class this file exists for is the one the node gate cannot see: a
// component throws while rendering, nothing catches it, and React unmounts the
// whole tree — in lich's case the window, sidebar and terminals included. So
// the assertions are about what is left standing, not about the boundary's own
// state: the sibling subtree, and a child that renders again after a retry.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { ErrorBoundary } from "./ErrorBoundary"

const MESSAGE = "files.map is not a function"

// Read at render time rather than taken as a prop: the boundary is showing its
// fallback when the flag flips, so nothing it holds would be re-read until the
// retry re-renders the child.
let crash = true

function Panel() {
  if (crash) {
    throw new Error(MESSAGE)
  }
  return <p>panel content</p>
}

function tree() {
  return (
    <div>
      <ErrorBoundary label="The panel">
        <Panel />
      </ErrorBoundary>
      <p>the sidebar</p>
    </div>
  )
}

function retryButton(): HTMLButtonElement {
  const button = [...document.querySelectorAll("button")].find(
    (candidate) => candidate.textContent?.trim() === "Try again",
  )
  if (!button) {
    throw new Error("the fallback offers no retry")
  }
  return button
}

// React 18 rethrows a caught error through a DOM event so devtools can break on
// it, and jsdom prints every unhandled one — a caught error would read as a
// failed run. Marking it handled is what makes the suite's output honest.
const swallow = (event: ErrorEvent) => event.preventDefault()

beforeEach(() => {
  window.addEventListener("error", swallow)
})

afterEach(() => {
  window.removeEventListener("error", swallow)
  crash = true
  vi.restoreAllMocks()
})

test("a throwing child leaves its siblings alive, and retry re-renders it", async () => {
  // React reports every caught error on its own; the fallback is what the test
  // reads, and an unmocked console.error would only make the run look failed.
  vi.spyOn(console, "error").mockImplementation(() => {})

  const mounted = await mountBudget(tree())
  expect(document.body.textContent).toContain("The panel stopped rendering")
  // On screen, not only in the console: the window it would have been read in
  // is the one this fallback stands in for.
  expect(document.body.textContent).toContain(MESSAGE)
  expect(document.body.textContent).toContain("the sidebar")
  expect(document.body.textContent).not.toContain("panel content")

  crash = false
  await mounted.act(() => {
    retryButton().click()
  })
  expect(document.body.textContent).toContain("panel content")
  expect(document.body.textContent).not.toContain("The panel stopped rendering")
  expect(document.body.textContent).toContain("the sidebar")

  await mounted.unmount()
})
