import { useCallback, useEffect, useRef, useState } from "react"
import { ProjectService } from "@/lib/rpc"
import type { PullRequestSummary } from "@/lib/api-types"
import { onPullRequestInvalidated } from "@/lib/pulls/pull-request-lookup"
import type { PullsState } from "@/lib/pulls/pull-request-list"
import { errorText } from "@/lib/utils"

export interface PullRequestListState {
  list: PullRequestSummary[]
  loading: boolean
  error: string | null
  refresh: () => void
}

// usePullRequests resolves the repository's open pull requests for the list
// column. Like the detail beside it this is a gh round-trip, so it is not
// polled: it re-reads on window focus, and whenever something lich itself did
// retires the badges (a merge, a PR opened from the terminal). An empty list
// with no error is a repository whose pull requests are all closed.
export function usePullRequests(path: string, state: PullsState): PullRequestListState {
  const [list, setList] = useState<PullRequestSummary[]>([])
  // Starts true so the first paint reads as loading rather than as "no pull
  // requests", the same contract as the detail hook.
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const seq = useRef(0)
  const loaded = useRef(state)

  const refresh = useCallback(() => {
    if (!path) {
      setList([])
      setError(null)
      setLoading(false)
      return
    }
    const mine = ++seq.current
    setLoading(true)
    // Rows of the state being left have to go before the new ones arrive: the
    // column labels itself from the query, so keeping them would caption open
    // pull requests as merged for as long as gh takes to answer.
    if (loaded.current !== state) {
      loaded.current = state
      setList([])
    }
    ProjectService.ListPullRequests(path, state)
      .then((result) => {
        if (mine !== seq.current) return
        setList(result ?? [])
        setError(null)
      })
      .catch((err: unknown) => {
        if (mine !== seq.current) return
        setList([])
        setError(errorText(err))
      })
      .finally(() => {
        if (mine === seq.current) setLoading(false)
      })
  }, [path, state])

  useEffect(() => {
    refresh()
    window.addEventListener("focus", refresh)
    const unsubscribe = onPullRequestInvalidated(refresh)
    return () => {
      // Invalidate any in-flight call so a late response cannot land on the
      // next path or after unmount.
      seq.current++
      window.removeEventListener("focus", refresh)
      unsubscribe()
    }
  }, [refresh])

  return { list, loading, error, refresh }
}
