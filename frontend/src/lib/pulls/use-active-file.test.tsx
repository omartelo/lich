// @vitest-environment jsdom
//
// The remembered selection of the changed-files tree, mounted for real. What
// this feature is about happens between two mounts of a component — the Files
// tab is destroyed by every tab switch — and no pure test can see that; the
// prefs module underneath is covered by pulls-prefs.test.ts.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { StrictMode, createElement, useLayoutEffect, useState } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { useActiveFile } from "@/lib/pulls/use-active-file"

const A = "https://github.com/o/r/pull/1"
const B = "https://github.com/o/r/pull/2"

// Every frame the hook *committed*, so a mount reads as a sequence rather than
// as a final value: a selection laid down and then dropped shows up here as an
// extra null, and only a committed frame proves the user saw it.
//
// Recorded from a layout effect, never from the render body. A render that
// updates state during it runs again from its base state, and StrictMode runs
// every pass twice — so the body sees values that were never painted, and an
// assertion written against them reads a correct hook as an oscillation.
//
// Mounted inside StrictMode, as the app runs (main.tsx): React discards and
// replays a render that updates state during it, and holding this hook's
// bookkeeping in a ref would let the replay skip the re-read it guards.
function probe(frames: (string | null)[], initial: string) {
  let move: (next: string) => void = () => {}
  let select: (path: string) => void = () => {}
  function Probe() {
    const [pullRequest, setPullRequest] = useState(initial)
    move = setPullRequest
    const [active, choose] = useActiveFile(pullRequest)
    select = choose
    useLayoutEffect(() => {
      frames.push(active)
    })
    return null
  }
  return {
    element: createElement(StrictMode, null, createElement(Probe)),
    move: (next: string) => move(next),
    select: (path: string) => select(path),
  }
}

/** The distinct selections a mount passed through, in order. */
function states(frames: (string | null)[]): (string | null)[] {
  return frames.filter((state, i) => state !== frames[i - 1])
}

beforeEach(() => {
  localStorage.clear()
})

describe("the changed-files selection", () => {
  it("comes back marked on the next mount of the tab", async () => {
    const frames: (string | null)[] = []
    const first = probe(frames, A)
    const mounted = await mountBudget(first.element)
    await mounted.act(() => first.select("internal/pty/pty.go"))
    await mounted.unmount()

    frames.length = 0
    const second = probe(frames, A)
    const back = await mountBudget(second.element)

    // One state for the whole mount: the file was marked from the first frame
    // and never left it. A tab that paints unmarked first is the bug this pins.
    expect(states(frames)).toEqual(["internal/pty/pty.go"])
    await back.unmount()
  })

  // The Files tab is not remounted when the list column moves to another pull
  // request, so this is driven on a *live* component: a remount would seed the
  // marker from scratch and never exercise the re-read at all.
  it("re-reads when the pull request moves under it", async () => {
    const seed = probe([], B)
    const seeded = await mountBudget(seed.element)
    await seeded.act(() => seed.select("frontend/src/main.tsx"))
    await seeded.unmount()

    const frames: (string | null)[] = []
    const live = probe(frames, A)
    const mounted = await mountBudget(live.element)
    await mounted.act(() => live.select("internal/pty/pty.go"))
    frames.length = 0
    await mounted.act(() => live.move(B))

    // The other pull request's own file, and never A's under B's tree.
    expect(states(frames)).toEqual(["frontend/src/main.tsx"])
    await mounted.unmount()
  })

  it("marks nothing for a pull request never read before", async () => {
    const frames: (string | null)[] = []
    const fresh = probe(frames, A)
    const mounted = await mountBudget(fresh.element)

    expect(states(frames)).toEqual([null])
    await mounted.unmount()
  })
})
