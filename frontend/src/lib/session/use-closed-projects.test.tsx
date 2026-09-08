// @vitest-environment jsdom
//
// The wiring the palette's Closed group is: the typed term has to reach the
// store, because the store is the only place that can see past the page it
// answers with. A test that filters rows in the window would pass against the
// bug this hook exists to close.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { StrictMode, createElement, useLayoutEffect, useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { RecentProject } from "@/lib/api-types"
import { useClosedProjects } from "@/lib/session/use-closed-projects"

const terms: string[] = []
const countTerms: string[] = []
const missingPaths: string[][] = []

function project(id: string, name: string): RecentProject {
  return { id, name, path: `/src/${id}` }
}

// The fake store searches by name so the test can tell a backend search from a
// window filter: only "old" is answered for a term, and it is never in the list
// the empty term returns. The count runs past the page on purpose.
vi.mock("@/lib/rpc", () => ({
  Store: {
    RecentProjects: (term: string) => {
      terms.push(term)
      if (term === "") {
        return Promise.resolve([project("p1", "recent work")])
      }
      return Promise.resolve(term.includes("old") ? [project("old", "the oldest work")] : [])
    },
    ClosedProjectCount: (term: string) => {
      countTerms.push(term)
      return Promise.resolve(term === "" ? 63 : 1)
    },
  },
  ProjectService: {
    Missing: (paths: string[]) => {
      missingPaths.push(paths)
      return Promise.resolve(paths.filter((p) => p === "/src/old"))
    },
  },
}))

// The debounce is real, so the test waits it out rather than mocking the clock:
// act() and fake timers fight over the same queue, and what is being asserted is
// that a burst of keystrokes costs one call.
const DEBOUNCE_MS = 200

const settled = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, DEBOUNCE_MS + 60))

// The newest frame, which is the one the palette would be painting.
function last<T>(frames: T[]): T | undefined {
  return frames[frames.length - 1]
}

interface Frame {
  rows: RecentProject[]
  total: number
  missing: ReadonlySet<string>
}

function probe(frames: Frame[]) {
  return function Probe({ query, enabled }: { query: string; enabled: boolean }) {
    const closed = useClosedProjects(query, enabled)
    useLayoutEffect(() => {
      frames.push(closed)
    })
    return null
  }
}

function typing(frames: Frame[], typers: ((q: string) => void)[]) {
  return function Typing() {
    const [query, setQuery] = useState("")
    const closed = useClosedProjects(query, true)
    useLayoutEffect(() => {
      frames.push(closed)
      typers.push(setQuery)
    })
    return null
  }
}

beforeEach(() => {
  terms.length = 0
  countTerms.length = 0
  missingPaths.length = 0
})

describe("useClosedProjects", () => {
  it("asks the store for the plain reopen list when nothing is typed", async () => {
    const frames: Frame[] = []
    const Probe = probe(frames)
    const mounted = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "", enabled: true })),
    )
    await settled()

    expect(terms).toEqual([""])
    expect(last(frames)?.rows.map((row) => row.id)).toEqual(["p1"])
    await mounted.unmount()
  })

  it("sends the typed term to the store, so a project past the page is found", async () => {
    const frames: Frame[] = []
    const Probe = probe(frames)
    const mounted = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "old", enabled: true })),
    )
    await settled()

    expect(terms).toContain("old")
    // "old" is not in the list the empty term answers with: a window-side filter
    // could never have produced this row.
    expect(last(frames)?.rows.map((row) => row.id)).toEqual(["old"])
    expect(last(frames)?.missing.has("/src/old")).toBe(true)
    expect(last(missingPaths)).toEqual(["/src/old"])
    await mounted.unmount()
  })

  it("reports every match, not the page, so a cut list can say what it left out", async () => {
    const frames: Frame[] = []
    const Probe = probe(frames)
    const mounted = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "", enabled: true })),
    )
    await settled()

    expect(countTerms).toContain("")
    expect(last(frames)?.total).toBe(63)
    await mounted.unmount()
  })

  it("costs one search for a burst of keystrokes", async () => {
    const frames: Frame[] = []
    const typers: ((q: string) => void)[] = []
    const Typing = typing(frames, typers)
    const mounted = await mountBudget(createElement(Typing))
    await settled()
    terms.length = 0

    for (const word of ["o", "ol", "old"]) {
      last(typers)?.(word)
    }
    await settled()

    expect(terms).toEqual(["old"])
    expect(last(frames)?.rows.map((row) => row.id)).toEqual(["old"])
    await mounted.unmount()
  })

  it("asks for nothing while the palette is closed", async () => {
    const frames: Frame[] = []
    const Probe = probe(frames)
    const mounted = await mountBudget(createElement(Probe, { query: "old", enabled: false }))
    await settled()

    expect(terms).toEqual([])
    expect(last(frames)?.rows).toEqual([])
    expect(last(frames)?.total).toBe(0)
    await mounted.unmount()
  })
})
