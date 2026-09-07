import { useCallback, useEffect, useMemo, useState } from "react"
import type { ClosedSession } from "@/lib/api-types"
import { ProjectService, Store } from "@/lib/rpc"
import { historyRows, type PaletteHistory } from "./command-palette"

// A history search runs a query over every session ever parked and then reads
// git for the rows it returns, so it waits for the typing to settle rather than
// firing on the keystroke — the same bargain the transcript search makes.
const DEBOUNCE_MS = 200

// useHistorySearch asks the store for the parked sessions matching `query` and
// joins them to what git and the filesystem say about their checkouts. Idle —
// no rows, no calls — while the palette is closed.
//
// The term goes to the backend rather than filtering rows already in hand: the
// store answers with one page, so a session parked further back than that page
// is only reachable if the match happens in the query. The palette's own filter
// still runs over what comes back, which is what narrows by branch — the one
// part of a row that is not in the database.
//
// An empty query skips the debounce: it is the list the palette opens with, and
// nothing was typed to wait out. Every effect run supersedes the one before it,
// so a reply that lands after the query moved on is dropped rather than painted
// over newer rows.
export function useHistorySearch(
  query: string,
  enabled: boolean,
): { rows: PaletteHistory[]; forget: (sessionID: string) => void } {
  const [parked, setParked] = useState<readonly ClosedSession[]>([])
  const [branches, setBranches] = useState<Readonly<Record<string, string>>>({})
  const [missing, setMissing] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    if (!enabled) {
      setParked([])
      return
    }
    let live = true
    const timer = window.setTimeout(
      () => {
        void Store.ClosedSessions(query).then((parkedRows) => {
          const history = parkedRows ?? []
          if (!live) {
            return
          }
          setParked(history)
          // Asked for after the rows are up: what git and the filesystem say is
          // what a row says about itself, and a failed check must not cost the
          // palette its entries.
          //
          // The branches come in one batch rather than by subscribing each row
          // to the git poller a live card uses: that poll is three calls per
          // path per second, and this list is long and on screen for as long as
          // it takes to type.
          const paths = history.map((row) => row.path).filter(Boolean)
          void ProjectService.Missing(paths).then((gone) => {
            if (live) {
              setMissing(new Set(gone ?? []))
            }
          })
          void ProjectService.BranchesOf(paths).then((named) => {
            if (live) {
              setBranches(named ?? {})
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

  // Forgetting drops the row in place rather than refetching: the list stays up
  // while several stale rows are cleared, which is how they are usually left.
  const forget = useCallback((sessionID: string) => {
    setParked((rows) => rows.filter((row) => row.id !== sessionID))
  }, [])

  // Memoised because the rows are a dependency of the palette's own filter:
  // a fresh array every render would re-filter every group on every keystroke.
  const rows = useMemo(() => historyRows(parked, branches, missing), [parked, branches, missing])
  return { rows, forget }
}
