import { useEffect, useState } from "react"
import { Service as ProjectService } from "../../bindings/github.com/skipodotdev/skipo/internal/project"

const GIT_POLL_MS = 5_000

export interface GitStatus {
  branch: string
  files: number
  added: number
  deleted: number
}

// useGitStatus polls branch + diff stats for a directory while the tab is
// visible, refreshing immediately on path change and on tab re-focus. Returns
// null until the first successful fetch (or after a failed one), so callers can
// hide the segments instead of rendering misleading zeros.
export function useGitStatus(path: string): GitStatus | null {
  const [status, setStatus] = useState<GitStatus | null>(null)
  useEffect(() => {
    if (!path) {
      setStatus(null)
      return
    }
    let alive = true
    const refresh = async () => {
      if (document.hidden) {
        return
      }
      try {
        const [branch, diff] = await Promise.all([
          ProjectService.Branch(path),
          ProjectService.Diff(path),
        ])
        if (alive) {
          setStatus({ branch, ...diff })
        }
      } catch {
        if (alive) {
          setStatus(null)
        }
      }
    }
    void refresh()
    const timer = setInterval(() => void refresh(), GIT_POLL_MS)
    const onVisible = () => void refresh()
    document.addEventListener("visibilitychange", onVisible)
    return () => {
      alive = false
      clearInterval(timer)
      document.removeEventListener("visibilitychange", onVisible)
    }
  }, [path])
  return status
}
