// @vitest-environment jsdom
//
// The bug class this file exists for is the one the node gate cannot see: a
// component throws while rendering, nothing catches it, and React unmounts the
// whole tree — in lich's case the window, sidebar and terminals included. So
// the assertions are about what is left standing, not about the boundary's own
// state: the sibling subtree, a child that renders again after a retry, and the
// terminal a caught stage hands back.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { useEffect, useReducer, useRef } from "react"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import {
  attachTerminal,
  detachTerminal,
  disposeTerminal,
  type LiveTerminal,
  terminalEntry,
} from "@/lib/terminal/terminal-registry"
import { ErrorBoundary } from "./ErrorBoundary"

const MESSAGE = "files.map is not a function"
const SESSION = "s1"
const SCROLLBACK = "the conversation so far"

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

// A pane, drawn the way the stage draws one: a container this component owns,
// with the session's host node attached into it and detached again on unmount.
// xterm needs a canvas and a measured font, so the terminal itself stands in —
// which object comes back is what the test is about.
function Pane() {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const entry = terminalEntry(SESSION)
  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }
    attachTerminal(entry, container)
    return () => detachTerminal(entry)
  }, [entry])
  return <div ref={containerRef} />
}

// Re-renders the boundary's children from outside it, which is what turns a
// healthy stage into a throwing one.
let repaint = () => {}

function Stage() {
  const [, bump] = useReducer((count: number) => count + 1, 0)
  repaint = bump
  return (
    <ErrorBoundary label="The stage" retry="Reload the stage">
      <Panel />
      <Pane />
    </ErrorBoundary>
  )
}

function retryButton(label = "Try again"): HTMLButtonElement {
  const button = [...document.querySelectorAll("button")].find(
    (candidate) => candidate.textContent?.trim() === label,
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
  disposeTerminal(SESSION)
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

test("a retry that throws again offers the reload instead of another retry", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {})
  const reload = vi.fn()
  vi.spyOn(window, "location", "get").mockReturnValue({
    ...window.location,
    reload,
  } as unknown as Location)

  const mounted = await mountBudget(tree())
  expect(retryButton().textContent).toContain("Try again")

  // The retry lands on a subtree that is still broken, which is the loop this
  // exists to end: the offer changes rather than repeating.
  await mounted.act(() => {
    retryButton().click()
  })
  expect(document.body.textContent).toContain("The panel stopped rendering")
  expect(document.body.textContent).toContain(MESSAGE)
  expect(() => retryButton()).toThrow()

  await mounted.act(() => {
    retryButton("Reload the window").click()
  })
  expect(reload).toHaveBeenCalledTimes(1)

  await mounted.unmount()
})

test("a throw over the stage keeps the terminal it unmounts, and gives it back", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {})
  crash = false

  const entry = terminalEntry(SESSION)
  const live = {
    term: { rows: 24, refresh: vi.fn() },
    dispose: vi.fn(),
  } as unknown as LiveTerminal
  entry.live = live

  const mounted = await mountBudget(<Stage />)
  entry.host.textContent = SCROLLBACK
  expect(document.body.textContent).toContain(SCROLLBACK)

  await mounted.act(() => {
    crash = true
    repaint()
  })
  // The stage is gone and says so; the terminal it took down with it is not.
  expect(document.body.textContent).toContain("The stage stopped rendering")
  expect(document.body.textContent).not.toContain(SCROLLBACK)
  expect(entry.live).toBe(live)
  expect(live.dispose).not.toHaveBeenCalled()

  crash = false
  await mounted.act(() => {
    retryButton("Reload the stage").click()
  })
  // The same terminal, in the same node, with what was in it — no snapshot and
  // no round trip through the backend.
  expect(entry.live).toBe(live)
  expect(entry.host.textContent).toBe(SCROLLBACK)
  expect(document.body.contains(entry.host)).toBe(true)
  expect(document.body.textContent).not.toContain("The stage stopped rendering")

  await mounted.unmount()
})
