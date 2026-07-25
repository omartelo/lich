import { ProjectService } from "@/lib/rpc"
import type { PullRequest } from "@/lib/api-types"

// How long an answer is reused. Every session card of a worktree, the footer
// and the sidebar's pull request card ask about the same checkout, and each ask
// is a gh subprocess — so a window focus used to fan out into four of them.
// Long enough to collapse that burst into one call, short enough that the next
// focus still reaches gh.
const SHARE_MS = 2000

interface Shared {
  at: number
  pending: boolean
  result: Promise<PullRequest | null>
}

const shared = new Map<string, Shared>()

const listeners = new Set<() => void>()

// onPullRequestInvalidated registers a badge's re-read, returning its
// unsubscribe. Every badge of every checkout is called, not just the one that
// changed: a merge is a rare, deliberate action, and the callers of one
// checkout still collapse into a single gh call. Key the listeners by path if
// that ever stops being cheap enough.
export function onPullRequestInvalidated(reload: () => void): () => void {
  listeners.add(reload)
  return () => {
    listeners.delete(reload)
  }
}

// invalidatePullRequests drops every shared answer and asks the badges to look
// up again — for a change only lich itself can announce. Merging does not move
// the checkout's HEAD (the commit lands on the base branch, on the remote), so
// without this the footer, the session cards and the sidebar's pull request
// entry keep reading "Open" until the window loses focus and gets it back.
// Clearing the map matters as much as the notify: a re-read inside the share
// window would otherwise replay the pre-merge answer.
export function invalidatePullRequests(): void {
  shared.clear()
  for (const reload of listeners) {
    reload()
  }
}

// lookupPullRequest resolves a checkout's open PR, sharing one gh call across
// every caller asking about the same checkout+branch+commit at the same time.
// It never rejects: a failed lookup (no gh, not a GitHub repo) reads as "no
// PR", the caller's cue to hide the badge.
export function lookupPullRequest(
  path: string,
  branch: string,
  head: string,
): Promise<PullRequest | null> {
  const key = `${path}\n${branch}\n${head}`
  const now = Date.now()
  const hit = shared.get(key)
  // An in-flight call is always shared, however long gh takes.
  if (hit && (hit.pending || now - hit.at < SHARE_MS)) {
    return hit.result
  }
  for (const [staleKey, entry] of shared) {
    if (!entry.pending && now - entry.at >= SHARE_MS) {
      shared.delete(staleKey)
    }
  }
  const entry: Shared = {
    at: now,
    pending: true,
    result: ProjectService.PullRequest(path)
      .catch(() => null)
      .finally(() => {
        entry.pending = false
      }),
  }
  shared.set(key, entry)
  return entry.result
}
