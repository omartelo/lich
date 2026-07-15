import {useCallback, useSyncExternalStore} from "react"
import {ProjectService} from "./rpc"
import {createGitStatusStore, type GitStatus} from "./git-status-store"

export type {GitStatus}

const GIT_POLL_MS = 3_000

async function fetchGitStatus(path: string): Promise<GitStatus | null> {
  try {
    const [branch, diff] = await Promise.all([
      ProjectService.Branch(path),
      ProjectService.Diff(path),
    ])
    return {branch, ...diff}
  } catch {
    return null
  }
}

const store = createGitStatusStore(fetchGitStatus, GIT_POLL_MS)

// refreshGitStatus fetches a path's git status immediately, ahead of its poll
// tick — used by the session-touched hook signal to cut the up-to-3s lag on the
// diff badge after Claude edits files. No-op when no card watches the path.
export function refreshGitStatus(path: string): void {
  store.refresh(path)
}

// useGitStatus subscribes to the shared per-path poller (see git-status-store):
// all components watching the same directory share one fetch cycle, and an
// unchanged status never re-renders them. Returns null until the first
// successful fetch (or after a failed one), so callers can hide the segments
// instead of rendering misleading zeros.
export function useGitStatus(path: string): GitStatus | null {
  const subscribe = useCallback(
    (onChange: () => void) =>
      path ? store.subscribe(path, onChange) : () => {},
    [path],
  )
  return useSyncExternalStore(subscribe, () =>
    path ? store.get(path) : null,
  )
}
