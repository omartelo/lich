import { useCallback, useEffect, useRef, useState } from "react"
import { ProjectService } from "@/lib/rpc"
import type { Worktree } from "@/lib/api-types"

// useCheckouts lists every checkout of the project that holds a branch — its
// own directory as much as its linked worktrees. It answers the question the
// Pulls screen asks of each pull request: is this head branch already checked
// out somewhere? git refuses to check one branch out twice, and it refuses just
// as hard when the branch is held by the project's own directory, which is
// where a branch usually is.
//
// A local git call, so it re-reads on window focus like the git status poller
// — a worktree can be created or a branch switched from a terminal — and
// through refresh() right after this screen creates one itself. An error yields
// an empty list: the worst it costs is a checkout attempt that gh refuses with
// its own message.
export function useCheckouts(projectPath: string): {
  checkouts: Worktree[]
  refresh: () => void
} {
  const [checkouts, setCheckouts] = useState<Worktree[]>([])
  const seq = useRef(0)

  const refresh = useCallback(() => {
    if (!projectPath) {
      setCheckouts([])
      return
    }
    const mine = ++seq.current
    ProjectService.ListCheckouts(projectPath)
      .then((list) => {
        if (mine === seq.current) setCheckouts(list ?? [])
      })
      .catch(() => {
        if (mine === seq.current) setCheckouts([])
      })
  }, [projectPath])

  useEffect(() => {
    refresh()
    window.addEventListener("focus", refresh)
    return () => {
      seq.current++
      window.removeEventListener("focus", refresh)
    }
  }, [refresh])

  return { checkouts, refresh }
}
