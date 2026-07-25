import { useEffect, useState } from "react"
import { lookupPullRequest } from "./pull-request-lookup"
import type { PullRequest } from "@/lib/api-types"

export type { PullRequest }

// usePullRequest resolves the open GitHub PR for a path's current branch via the
// gh CLI. Unlike git status it is not polled — each lookup is a network
// round-trip — but it refetches whenever the checkout's HEAD moves, so a PR the
// session just opened (or a merge that closed one) reaches the badge without
// waiting for a window focus. It also refetches on focus, for a PR opened or
// merged in the browser. Callers asking about the same checkout share one gh
// call (pull-request-lookup). Returns null while loading, on any error, or when
// the branch has no open PR (a merged or closed one is filtered server-side),
// so the caller hides the badge.
export function usePullRequest(path: string, branch: string, head: string): PullRequest | null {
  const [pr, setPr] = useState<PullRequest | null>(null)

  // A different checkout is a different PR: drop the old badge at once rather
  // than showing it against the new branch. A new commit on the same branch
  // keeps it, so the badge does not blink on every commit.
  useEffect(() => {
    setPr(null)
  }, [path, branch])

  useEffect(() => {
    if (!path) {
      return
    }
    let alive = true
    const load = () => {
      void lookupPullRequest(path, branch, head).then((result) => {
        if (alive) setPr(result)
      })
    }
    load()
    window.addEventListener("focus", load)
    return () => {
      alive = false
      window.removeEventListener("focus", load)
    }
  }, [path, branch, head])

  return pr
}
