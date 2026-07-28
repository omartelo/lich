import { ProjectService } from "@/lib/rpc"
import { parseDiff, type DiffFile } from "@/lib/git/diff"
import { useRemoteResource } from "@/lib/use-remote-resource"

const NO_FILES: DiffFile[] | null = null

export interface PullRequestDiffState {
  files: DiffFile[] | null
  error: string | null
}

// usePullRequestDiff fetches and parses the PR's unified diff (gh pr diff) for
// the Files changed tab. Fetched on demand — the diff is a gh round-trip and can
// be large — and refetched when the checkout's HEAD moves. Unlike the PR detail
// next door it does not refresh on window focus: re-rendering a wide diff is far
// too expensive to spend on every alt-tab back into the app. files is null while
// loading or on error; an empty array is a PR with no file changes.
export function usePullRequestDiff(
  path: string,
  head: string,
  number: number,
): PullRequestDiffState {
  const { data, error } = useRemoteResource(
    path && `${path} ${number} ${head}`,
    () => ProjectService.PullRequestDiff(path, number).then(parseDiff),
    { empty: NO_FILES },
  )
  return { files: data, error }
}
