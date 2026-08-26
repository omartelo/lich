import { useEffect } from "react"
import { ProjectService } from "@/lib/rpc"
import type { PullRequestDetail } from "@/lib/api-types"
import { useRemoteResource } from "@/lib/use-remote-resource"

export type { PullRequestDetail }

// How often the detail is re-read while a check is still running. CI is the one
// thing here that changes with nobody touching the repository, so it is the one
// thing polled — and only while something is actually in flight.
const RUNNING_CHECKS_POLL_MS = 10_000

const NO_DETAIL: PullRequestDetail | null = null

export interface PullRequestState {
  detail: PullRequestDetail | null
  loading: boolean
  error: string | null
  refresh: () => void
}

// usePullRequestDetail resolves one open PR in full for the Pulls screen:
// number when the list has selected one, the checkout's own branch when it is
// zero. Like the footer badge it is not polled — each lookup is a gh network
// round-trip — but it refetches whenever the selection or the checkout's HEAD
// moves (a commit from the session next door lands in the checks and the diff),
// on window focus, and through refresh() so an in-app merge or a manual reload
// updates the screen at once. detail is null with no error when there is no
// open PR: the screen's empty state.
export function usePullRequestDetail(
  path: string,
  branch: string,
  head: string,
  number = 0,
): PullRequestState {
  // branch and head ride the key without being arguments of the call: they are
  // not what is asked for, they are what makes the last answer stale.
  const { data, loading, error, refresh } = useRemoteResource(
    path && `${path} ${number} ${branch} ${head}`,
    () => ProjectService.PullRequestDetail(path, number),
    {
      empty: NO_DETAIL,
      refetchOnFocus: true,
      // Two different requests, and the cache key says which: a number
      // addresses one pull request outright, and a zero asks for whichever the
      // branch has open. Neither is dated by head — it makes the answer stale,
      // it does not say what was asked for, and a cache key carrying it could
      // not be read until a fresh mount's git poll had answered, which is the
      // one frame this exists for.
      cache: number > 0 ? `pulls.detail ${path} #${number}` : `pulls.detail ${path} @${branch}`,
    },
  )

  // Depends on the count, not the detail object: while it stays at 3 the
  // interval keeps running instead of being torn down and rebuilt each read,
  // and it stops on its own the moment the last check settles.
  const pending = data?.checks.pending ?? 0
  useEffect(() => {
    if (pending === 0) {
      return
    }
    const timer = setInterval(refresh, RUNNING_CHECKS_POLL_MS)
    return () => clearInterval(timer)
  }, [pending, refresh])

  return { detail: data, loading, error, refresh }
}
