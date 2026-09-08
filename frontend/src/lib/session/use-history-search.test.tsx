// @vitest-environment jsdom
//
// The wiring the History tab's search is: the typed term has to reach the store,
// because the store is the only place that can see past the page it answers
// with. A test that filters rows in the window would pass against the bug this
// hook exists to close.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { StrictMode, createElement, useLayoutEffect, useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { ClosedSession } from "@/lib/api-types"
import { useHistorySearch } from "@/lib/session/use-history-search"
import type { PaletteHistory } from "@/lib/session/command-palette"

const terms: string[] = []
const branchPaths: string[][] = []
// The store's backfill, as the hook sees it: a count that comes down by itself,
// one session per answer, and never moves unless somebody asks again.
const backlog = { left: 0 }

function row(id: string, label: string, snippet = ""): ClosedSession {
  return {
    id,
    snippet,
    truncated: false,
    projectId: "p1",
    projectName: "alpha",
    projectPath: "/repo",
    label,
    kind: "claude",
    path: `/wt/${id}`,
    parkedBranch: `parked/${id}`,
    closedAt: 1_700_000_000,
  }
}

// The fake store searches by name so the test can tell a backend search from a
// window filter: only "old" is answered for a term, and it is never in the list
// the empty term returns. Its totals are larger than its pages, which is the
// shape the real store answers a term matching more than one page in.
vi.mock("@/lib/rpc", () => ({
  Store: {
    ClosedSessions: (term: string) => {
      terms.push(term)
      const indexing = backlog.left
      backlog.left = Math.max(0, backlog.left - 1)
      if (term === "") {
        return Promise.resolve({ sessions: [row("s1", "recent work")], total: 7, indexing })
      }
      const hit = term.includes("old")
      return Promise.resolve({
        sessions: hit ? [row("old", "the oldest work")] : [],
        total: hit ? 42 : 0,
        indexing,
      })
    },
  },
  ProjectService: {
    Missing: (paths: string[]) => Promise.resolve(paths.filter((p) => p === "/wt/old")),
    BranchesOf: (paths: string[]) => {
      branchPaths.push(paths)
      return Promise.resolve(Object.fromEntries(paths.map((p) => [p, `branch${p}`])))
    },
  },
}))

// The debounce is real, so the test waits it out rather than mocking the clock:
// act() and fake timers fight over the same queue, and what is being asserted is
// that a burst of keystrokes costs one call.
const DEBOUNCE_MS = 200

const settled = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, DEBOUNCE_MS + 60))

// The hook's own poll interval while the store reports a backfill running. The
// polls are waited out rather than counted off the clock: the interval and the
// debounce chain, so what is being asserted is where the sequence lands, never
// how long one lap of it took.
const POLL_MS = 1000

// The newest frame, which is the one the palette would be painting.
function last<T>(frames: T[]): T | undefined {
  return frames[frames.length - 1]
}

function probe(
  frames: PaletteHistory[][],
  forgets: ((id: string) => void)[],
  totals: number[] = [],
  backlogs: number[] = [],
) {
  return function Probe({ query, enabled }: { query: string; enabled: boolean }) {
    const { rows, total, indexing, forget } = useHistorySearch(query, enabled)
    useLayoutEffect(() => {
      frames.push(rows)
      totals.push(total)
      backlogs.push(indexing)
      forgets.push(forget)
    })
    return null
  }
}

function typing(frames: PaletteHistory[][], typers: ((q: string) => void)[]) {
  return function Typing() {
    const [query, setQuery] = useState("")
    const { rows } = useHistorySearch(query, true)
    useLayoutEffect(() => {
      frames.push(rows)
      typers.push(setQuery)
    })
    return null
  }
}

beforeEach(() => {
  terms.length = 0
  branchPaths.length = 0
  backlog.left = 0
})

describe("useHistorySearch", () => {
  it("asks the store for the plain history when nothing is typed", async () => {
    const frames: PaletteHistory[][] = []
    const Probe = probe(frames, [])
    const budget = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "", enabled: true })),
    )
    await budget.act(settled)

    expect(terms).toEqual([""])
    const rows = last(frames) ?? []
    expect(rows.map((r) => r.id)).toEqual(["s1"])
    // The branch is joined on from git, not from the row: the store has none.
    expect(rows[0].branch).toBe("branch/wt/s1")
    expect(rows[0].gone).toBe(false)
    expect(last(branchPaths)).toEqual(["/wt/s1"])
    await budget.unmount()
  })

  it("sends the typed term to the store, once for a burst of keystrokes", async () => {
    const frames: PaletteHistory[][] = []
    const typers: ((q: string) => void)[] = []
    const Probe = typing(frames, typers)
    const budget = await mountBudget(createElement(StrictMode, null, createElement(Probe)))
    await budget.act(settled)
    expect(terms).toEqual([""])

    // Three keystrokes inside one debounce window buy one search, not three.
    await budget.act(async () => {
      for (const q of ["o", "ol", "old"]) {
        last(typers)?.(q)
        await Promise.resolve()
      }
    })
    await budget.act(settled)

    expect(terms).toEqual(["", "old"])
    // The row the term returns was never in the empty-term list, so no filter
    // over what the window already held could have produced it.
    const rows = last(frames) ?? []
    expect(rows.map((r) => r.id)).toEqual(["old"])
    // Its checkout is gone, which is the row that forgets rather than resumes.
    expect(rows[0].gone).toBe(true)
    await budget.unmount()
  })

  it("stays idle while the palette is closed", async () => {
    const frames: PaletteHistory[][] = []
    const Probe = probe(frames, [])
    const budget = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "old", enabled: false })),
    )
    await budget.act(settled)

    expect(terms).toEqual([])
    expect(last(frames)).toEqual([])
    await budget.unmount()
  })

  it("drops a forgotten row without asking the store again", async () => {
    const frames: PaletteHistory[][] = []
    const forgets: ((id: string) => void)[] = []
    const totals: number[] = []
    const Probe = probe(frames, forgets, totals)
    const budget = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "", enabled: true })),
    )
    await budget.act(settled)
    expect(last(totals)).toBe(7)

    await budget.act(() => {
      last(forgets)?.("s1")
    })
    expect(last(frames)).toEqual([])
    expect(terms).toEqual([""])
    // The count comes down with the row, so a page that was never cut does not
    // start claiming it was.
    expect(last(totals)).toBe(6)
    await budget.unmount()
  })

  it("carries the whole match count beside the page it got", async () => {
    const frames: PaletteHistory[][] = []
    const totals: number[] = []
    const Probe = probe(frames, [], totals)
    const budget = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "old", enabled: true })),
    )
    await budget.act(settled)

    // One row in hand, 42 matched: without the second number the list would
    // present its page as the whole answer.
    expect((last(frames) ?? []).map((r) => r.id)).toEqual(["old"])
    expect(last(totals)).toBe(42)
    await budget.unmount()
  })

  it("counts nothing while the palette is closed", async () => {
    const totals: number[] = []
    const Probe = probe([], [], totals)
    const budget = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "old", enabled: false })),
    )
    await budget.act(settled)

    expect(last(totals)).toBe(0)
    await budget.unmount()
  })

  it("asks again while the store is still indexing, until the backlog is gone", async () => {
    // Two sessions left to index when the first search lands. Nothing else will
    // ask on the user's behalf, so a hook that stopped here would leave the
    // header reporting a count that only moves when somebody types.
    backlog.left = 2
    const backlogs: number[] = []
    const Probe = probe([], [], [], backlogs)
    const budget = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "old", enabled: true })),
    )
    await budget.act(settled)
    expect(last(backlogs)).toBe(2)
    expect(terms).toEqual(["old"])

    // Two more rounds of the poll, which is what it takes to walk a backlog of
    // two down to nothing: nobody typed in between, so every ask after the
    // first is the hook's own. Each lap is its own act: a frame is only readable
    // once React has been let commit it.
    for (let lap = 0; lap < 12 && last(backlogs) !== 0; lap++) {
      await budget.act(() => new Promise((resolve) => setTimeout(resolve, POLL_MS / 2)))
    }
    expect(last(backlogs)).toBe(0)
    expect(terms).toEqual(["old", "old", "old"])

    // And then it stops: the poll chains off the count it was told, so a
    // backlog of zero is the end of it.
    await budget.act(
      () => new Promise((resolve) => setTimeout(resolve, POLL_MS + DEBOUNCE_MS + 60)),
    )
    expect(terms).toEqual(["old", "old", "old"])
    await budget.unmount()
  })

  it("keeps the branch the store matched on the row, beside the one git named", async () => {
    const frames: PaletteHistory[][] = []
    const Probe = probe(frames, [])
    const budget = await mountBudget(
      createElement(StrictMode, null, createElement(Probe, { query: "", enabled: true })),
    )
    await budget.act(settled)

    const rows = last(frames) ?? []
    expect(rows[0].branch).toBe("branch/wt/s1")
    expect(rows[0].parkedBranch).toBe("parked/s1")
    await budget.unmount()
  })
})
