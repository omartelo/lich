// @vitest-environment jsdom
//
// The one thing draft-store exists for, and the one thing a node test cannot
// see: prose typed into a box survives the box being destroyed. Every way the
// pull request screen destroys one is an unmount — the tab strip is a ternary
// between component types, folding a file drops the diff body, and a refetched
// diff rebuilds every CodeMirror widget — so a remount is the whole test.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { StrictMode, createElement, useLayoutEffect } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { draftKey, draftStore } from "./draft-store"
import { useDraft } from "./use-draft"

const PR = "https://github.com/omartelo/lich/pull/389"

// Records what each commit saw, from a layout effect rather than the render
// body: the body sees passes React never committed (use-remote-resource.test).
function box(seen: (string | null)[], id = PR, kind: "body" | "comment" | "reply" = "comment") {
  let type: (next: string | null) => void = () => {}
  function Box() {
    const [draft, setDraft] = useDraft(kind, id)
    type = setDraft
    useLayoutEffect(() => {
      seen.push(draft)
    })
    return null
  }
  return {
    element: createElement(StrictMode, null, createElement(Box)),
    type: (next: string | null) => type(next),
  }
}

beforeEach(() => {
  for (const kind of ["body", "comment", "reply"] as const) {
    draftStore.set(draftKey(kind, PR), null)
  }
})

describe("a draft being typed", () => {
  it("is still there when the box is built again", async () => {
    const seen: (string | null)[] = []
    const first = box(seen)
    const mounted = await mountBudget(first.element)
    await mounted.act(() => first.type("half a comment"))
    await mounted.unmount()

    seen.length = 0
    const second = box(seen)
    const again = await mountBudget(second.element)

    // The first frame of the new box, before any effect could fetch anything.
    expect(seen[0]).toBe("half a comment")
    await again.unmount()
  })

  it("comes back empty once it has been sent", async () => {
    const seen: (string | null)[] = []
    const first = box(seen)
    const mounted = await mountBudget(first.element)
    await mounted.act(() => first.type("a comment"))
    // What every send path does: null, not "", so the box closes rather than
    // reopening empty.
    await mounted.act(() => first.type(null))
    await mounted.unmount()

    seen.length = 0
    const second = box(seen)
    const again = await mountBudget(second.element)

    expect(seen[0]).toBeNull()
    await again.unmount()
  })

  // Two threads on one diff, two boxes open at once: the store is keyed, and a
  // box must read its own key rather than the last one written.
  it("does not reach a box with another id", async () => {
    const mine: (string | null)[] = []
    const theirs: (string | null)[] = []
    const first = box(mine, "PRRT_one", "reply")
    const other = box(theirs, "PRRT_two", "reply")

    const a = await mountBudget(first.element)
    const b = await mountBudget(other.element)
    await a.act(() => first.type("about this line"))

    expect(mine[mine.length - 1]).toBe("about this line")
    expect(theirs[theirs.length - 1]).toBeNull()
    await a.unmount()
    await b.unmount()
    draftStore.set(draftKey("reply", "PRRT_one"), null)
  })
})
