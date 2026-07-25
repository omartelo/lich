import { useCallback, useEffect, useRef, useState } from "react"
import { ProjectService } from "@/lib/rpc"
import type { PullRequestDetail } from "@/lib/api-types"
import { errorText } from "@/lib/utils"

export type { PullRequestDetail }

// How often the detail is re-read while a check is still running. CI is the one
// thing here that changes with nobody touching the repository, so it is the one
// thing polled — and only while something is actually in flight.
const RUNNING_CHECKS_POLL_MS = 10_000

export interface PullRequestState {
  detail: PullRequestDetail | null
  loading: boolean
  error: string | null
  refresh: () => void
}

// usePullRequestDetail resolves the active branch's open PR in full for the
// Pulls screen. Like the footer badge it is not polled — each lookup is a gh
// network round-trip — but it refetches whenever the checkout's HEAD moves (a
// commit from the session next door lands in the checks and the diff), on
// window focus, and through refresh() so an in-app merge or a manual reload
// updates the screen at once. detail is null with no error when the branch
// simply has no open PR: the screen's empty state.
export function usePullRequestDetail(path: string, branch: string, head: string): PullRequestState {
  const [detail, setDetail] = useState<PullRequestDetail | null>(null)
  // Starts true so the first paint — before the mount effect fires the lookup —
  // reads as loading, not as "this branch has no pull request".
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const seq = useRef(0)

  const refresh = useCallback(() => {
    if (!path) {
      setDetail(null)
      setError(null)
      setLoading(false)
      return
    }
    const mine = ++seq.current
    setLoading(true)
    ProjectService.PullRequestDetail(path)
      .then((result) => {
        if (mine !== seq.current) return
        setDetail(result)
        setError(null)
      })
      .catch((err: unknown) => {
        if (mine !== seq.current) return
        setDetail(null)
        setError(errorText(err))
      })
      .finally(() => {
        if (mine === seq.current) setLoading(false)
      })
  }, [path])

  useEffect(() => {
    refresh()
    window.addEventListener("focus", refresh)
    return () => {
      // Invalidate any in-flight lookup so a late response can't land on the
      // next branch/path or after unmount.
      seq.current++
      window.removeEventListener("focus", refresh)
    }
  }, [refresh, branch, head])

  // Depends on the count, not the detail object: while it stays at 3 the
  // interval keeps running instead of being torn down and rebuilt each read,
  // and it stops on its own the moment the last check settles.
  const pending = detail?.checks.pending ?? 0
  useEffect(() => {
    if (pending === 0) {
      return
    }
    const timer = setInterval(refresh, RUNNING_CHECKS_POLL_MS)
    return () => clearInterval(timer)
  }, [pending, refresh])

  return { detail, loading, error, refresh }
}
