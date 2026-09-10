import { useCallback, useEffect, useMemo, useState } from "react"
import type { ClosedSession } from "@/lib/api-types"
import { ProjectService, Store } from "@/lib/rpc"
import { historyRows, type PaletteHistory } from "./command-palette"

// A history search runs a query over every session ever parked and then reads
// git for the rows it returns, so it waits for the typing to settle rather than
// firing on the keystroke — the same bargain the transcript search makes.
const DEBOUNCE_MS = 200

// How long the list waits before asking again while the store is still indexing
// parked conversations. The backfill reads one session every tenth of a second,
// so a second between polls is a handful of new rows per ask: often enough that
// the header counts down while somebody reads the list, rare enough that it is
// not a query per keystroke of thinking time.
const BACKFILL_POLL_MS = 1000

// useHistorySearch asks the store for the parked sessions matching `query` and
// joins them to what git and the filesystem say about their checkouts. Idle —
// no rows, no calls — while the palette is closed.
//
// `total` is how many sessions matched in all, which is not how many came back:
// the store answers with one page, and the list needs the number to say it was
// cut rather than present the page as the whole answer.
//
// `indexing` is how many parked sessions have no searchable conversation yet, on
// a workspace that predates the index. Asking with a term is what starts that
// backfill, so this hook polls itself while the number is above zero. Otherwise
// the header would report a count that only moves when somebody types.
//
// The term goes to the backend rather than filtering rows already in hand, for
// that same page: a session parked further back than it is only reachable if the
// match happens in the query. The palette's own filter still runs over what
// comes back.
//
// An empty query skips the debounce: it is the list the palette opens with, and
// nothing was typed to wait out. Every effect run supersedes the one before it,
// so a reply that lands after the query moved on is dropped rather than painted
// over newer rows.
export function useHistorySearch(
  query: string,
  enabled: boolean,
): {
  rows: PaletteHistory[]
  total: number
  indexing: number
  forget: (sessionID: string) => void
} {
  const [parked, setParked] = useState<readonly ClosedSession[]>([])
  const [total, setTotal] = useState(0)
  const [indexing, setIndexing] = useState(0)
  const [poll, setPoll] = useState(0)
  const [branches, setBranches] = useState<Readonly<Record<string, string>>>({})
  const [missing, setMissing] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    if (!enabled) {
      setParked([])
      setTotal(0)
      setIndexing(0)
      return
    }
    let live = true
    const timer = window.setTimeout(
      () => {
        void Store.ClosedSessions(query).then((answer) => {
          const history = answer?.sessions ?? []
          if (!live) {
            return
          }
          setParked(history)
          setTotal(answer?.total ?? 0)
          setIndexing(answer?.indexing ?? 0)
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
  }, [query, enabled, poll])

  // Ask again while the backfill is working. The tick is state rather than an
  // interval so it chains off the answer that reported the count: a slow reply
  // never stacks a second request behind the first.
  //
  // Only behind a term: the store starts its backfill on a term and on nothing
  // else, so with the palette open on the plain list the count never moves, and
  // polling it is a query and a git read per row every second for as long as
  // the palette stays up, reporting nothing.
  useEffect(() => {
    if (!enabled || indexing === 0 || query.trim() === "") {
      return
    }
    const timer = window.setTimeout(() => setPoll((n) => n + 1), BACKFILL_POLL_MS)
    return () => window.clearTimeout(timer)
  }, [enabled, indexing, poll, query])

  // Forgetting drops the row in place rather than refetching: the list stays up
  // while several stale rows are cleared, which is how they are usually left.
  // The match count comes down with it, or a page that was never cut would start
  // claiming it was after a few rows were dropped from it.
  const forget = useCallback((sessionID: string) => {
    setParked((rows) => rows.filter((row) => row.id !== sessionID))
    setTotal((matched) => Math.max(0, matched - 1))
  }, [])

  // Memoised because the rows are a dependency of the palette's own filter:
  // a fresh array every render would re-filter every group on every keystroke.
  const rows = useMemo(() => historyRows(parked, branches, missing), [parked, branches, missing])
  return { rows, total, indexing, forget }
}
