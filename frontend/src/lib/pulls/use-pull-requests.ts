import { useEffect } from "react"
import { ProjectService } from "@/lib/rpc"
import type { PullRequestSummary } from "@/lib/api-types"
import { onPullRequestInvalidated } from "@/lib/pulls/pull-request-lookup"
import type { PullsState } from "@/lib/pulls/pull-request-list"
import { useRemoteResource } from "@/lib/use-remote-resource"

const NO_PULL_REQUESTS: PullRequestSummary[] = []

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
  const { data, loading, error, refresh } = useRemoteResource(
    path && `${path} ${state}`,
    () => ProjectService.ListPullRequests(path, state).then((list) => list ?? NO_PULL_REQUESTS),
    {
      empty: NO_PULL_REQUESTS,
      refetchOnFocus: true,
      // The state rides the cache key because it is what was asked for; the
      // path, because two repositories are two lists.
      cache: `pulls.list ${path} ${state}`,
      // Rows of the state being left have to go before the new ones arrive: the
      // column labels itself from the query, so keeping them would caption open
      // pull requests as merged for as long as gh takes to answer.
      resetOn: state,
    },
  )

  useEffect(() => onPullRequestInvalidated(refresh), [refresh])

  return { list: data, loading, error, refresh }
}
