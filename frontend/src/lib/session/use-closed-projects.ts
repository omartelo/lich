import { useEffect, useState } from "react"
import type { RecentProject } from "@/lib/api-types"
import { ProjectService, Store } from "@/lib/rpc"

// The closed projects are read per keystroke once the term reaches the store,
// so the typing is waited out rather than fired on: the same bargain the parked
// sessions and the transcript search make (use-history-search).
const DEBOUNCE_MS = 200

export interface ClosedProjects {
  /** One page of matches, newest close first. */
  rows: RecentProject[]
  /** Every match, page or no page, so the group can say what it left out. */
  total: number
  /** The rows whose directory is gone: they relocate rather than reopen. */
  missing: ReadonlySet<string>
}

// useClosedProjects asks the store for the closed projects matching `query` and
// checks where their directories went. Idle (no rows, no calls) while the
// palette is closed.
//
// The term goes to the backend rather than filtering rows already in hand: the
// store answers with one page, so a project closed further back than that page
// is only reachable if the match happens in the query. That page is what the
// reopen menu lists too, which is why the count comes back with it.
//
// An empty query skips the debounce: it is the list the palette opens with, and
// nothing was typed to wait out. Every effect run supersedes the one before it,
// so a reply that lands after the query moved on is dropped rather than painted
// over newer rows.
export function useClosedProjects(query: string, enabled: boolean): ClosedProjects {
  const [rows, setRows] = useState<RecentProject[]>([])
  const [total, setTotal] = useState(0)
  const [missing, setMissing] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    if (!enabled) {
      setRows([])
      setTotal(0)
      return
    }
    let live = true
    const timer = window.setTimeout(
      () => {
        void Store.RecentProjects(query).then((recentRows) => {
          const recents = recentRows ?? []
          if (!live) {
            return
          }
          setRows(recents)
          setTotal(recents.length)
          // Asked for after the rows are up: what the count and the filesystem
          // say is what a row says about itself, and a failed check must not
          // cost the palette its entries.
          void Store.ClosedProjectCount(query).then((count) => {
            if (live) {
              setTotal(count)
            }
          })
          void ProjectService.Missing(recents.map((row) => row.path)).then((gone) => {
            if (live) {
              setMissing(new Set(gone ?? []))
            }
          })
        })
      },
      query.trim() === "" ? 0 : DEBOUNCE_MS,
    )
    return () => {
      live = false
      window.clearTimeout(timer)
    }
  }, [query, enabled])

  return { rows, total, missing }
}
