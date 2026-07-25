import { useCallback, useEffect, useRef, useState } from "react"
import { ProjectService } from "./rpc"
import { parseDiff, type DiffFile } from "./diff"
import { errorText } from "./utils"

export interface PullRequestDiffState {
  files: DiffFile[] | null
  error: string | null
}

// usePullRequestDiff fetches and parses the PR's unified diff (gh pr diff) for
// the Files changed tab. Fetched on demand — the diff is a gh round-trip and can
// be large — and refetched when the checkout's HEAD moves or on window focus,
// like the PR detail. files is null while loading or on error; an empty array
// is a PR with no file changes.
export function usePullRequestDiff(path: string, head: string): PullRequestDiffState {
  const [files, setFiles] = useState<DiffFile[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const seq = useRef(0)

  const refresh = useCallback(() => {
    if (!path) {
      setFiles(null)
      setError(null)
      return
    }
    const mine = ++seq.current
    ProjectService.PullRequestDiff(path)
      .then((text) => {
        if (mine !== seq.current) return
        setFiles(parseDiff(text))
        setError(null)
      })
      .catch((err: unknown) => {
        if (mine !== seq.current) return
        setFiles(null)
        setError(errorText(err))
      })
  }, [path])

  useEffect(() => {
    refresh()
    window.addEventListener("focus", refresh)
    return () => {
      seq.current++
      window.removeEventListener("focus", refresh)
    }
  }, [refresh, head])

  return { files, error }
}
