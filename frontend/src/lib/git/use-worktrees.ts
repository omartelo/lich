import { useCallback, useEffect, useRef, useState } from "react"
import { ProjectService } from "@/lib/rpc"
import type { Worktree } from "@/lib/api-types"

// useWorktrees lists the project's linked worktree checkouts (branch + path).
// It answers the question the Pulls screen asks of every pull request: is this
// head branch already checked out somewhere? git refuses to check one branch
// out twice, so knowing beforehand is the difference between a button that
// opens the session and one that fails on click.
//
// A local git call, so it re-reads on window focus like the git status poller
// — a worktree can be created from a terminal — and through refresh() right
// after this screen creates one itself. An error yields an empty list: the
// worst it costs is a checkout attempt that gh refuses with its own message.
export function useWorktrees(projectPath: string): {
  worktrees: Worktree[]
  refresh: () => void
} {
  const [worktrees, setWorktrees] = useState<Worktree[]>([])
  const seq = useRef(0)

  const refresh = useCallback(() => {
    if (!projectPath) {
      setWorktrees([])
      return
    }
    const mine = ++seq.current
    ProjectService.ListBranches(projectPath)
      .then((branches) => {
        if (mine === seq.current) setWorktrees(branches.worktrees ?? [])
      })
      .catch(() => {
        if (mine === seq.current) setWorktrees([])
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

  return { worktrees, refresh }
}
