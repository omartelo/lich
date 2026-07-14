import {useEffect, useState} from "react"
import {Service as ProjectService} from "../../bindings/github.com/omartelo/lich/internal/project"
import type {PullRequest} from "../../bindings/github.com/omartelo/lich/internal/project/models"

export type {PullRequest}

// usePullRequest resolves the open GitHub PR for a path's current branch via the
// gh CLI. Unlike git status it is not polled: a PR is opened once and rarely
// changes, and each lookup is a network round-trip. It refetches when the path
// or branch changes, and on window focus — so opening or merging a PR in the
// browser is reflected the moment the user returns to lich, without a branch
// switch. Returns null while loading, on any error, or when the branch has no
// open PR (a merged or closed one is filtered server-side), so the caller hides
// the badge.
export function usePullRequest(path: string, branch: string): PullRequest | null {
  const [pr, setPr] = useState<PullRequest | null>(null)
  useEffect(() => {
    if (!path) {
      setPr(null)
      return
    }
    let alive = true
    const load = () => {
      ProjectService.PullRequest(path)
        .then((result) => {
          if (alive) setPr(result)
        })
        .catch(() => {
          if (alive) setPr(null)
        })
    }
    load()
    window.addEventListener("focus", load)
    return () => {
      alive = false
      window.removeEventListener("focus", load)
    }
  }, [path, branch])
  return pr
}
