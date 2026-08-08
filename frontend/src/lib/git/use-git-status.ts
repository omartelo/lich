import { createGitStatusStore, type GitStatus, type PollCadence } from "./git-status-store"
import { ProjectService } from "@/lib/rpc"
import { useKeyedStore } from "@/lib/use-keyed-store"

export type { GitStatus }

// A checkout being worked in is read every second, so an edit or a commit shows
// up while the agent is still typing; after five quiet reads it falls back to
// the old 3s, which is what an idle project costs today. The plugin's
// session-touched hook still nudges an immediate read on top of this.
const GIT_POLL: PollCadence = { fastMs: 1_000, slowMs: 3_000, idleTicks: 5 }

async function fetchGitStatus(path: string): Promise<GitStatus | null> {
  try {
    const [branch, diff, base] = await Promise.all([
      ProjectService.Branch(path),
      ProjectService.Diff(path),
      ProjectService.BaseStatus(path),
    ])
    return { branch, ...diff, base }
  } catch {
    return null
  }
}

const store = createGitStatusStore(fetchGitStatus, GIT_POLL)

// refreshGitStatus fetches a path's git status immediately, ahead of its poll
// tick — used by the session-touched hook signal to cut the lag on the diff
// badge after Claude edits files. No-op when no card watches the path.
export function refreshGitStatus(path: string): void {
  store.refresh(path)
}

// useGitStatus subscribes to the shared per-path poller (see git-status-store):
// all components watching the same directory share one fetch cycle, and an
// unchanged status never re-renders them. Returns null until the first
// successful fetch (or after a failed one), so callers can hide the segments
// instead of rendering misleading zeros. An empty path subscribes to nothing —
// useKeyedStore's guard is what keeps a poll off "".
export function useGitStatus(path: string): GitStatus | null {
  return useKeyedStore(store, path)
}
