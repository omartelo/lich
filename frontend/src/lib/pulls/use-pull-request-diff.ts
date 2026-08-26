import { ProjectService } from "@/lib/rpc"
import { parseDiff, type DiffFile } from "@/lib/git/diff"
import { useRemoteResource } from "@/lib/use-remote-resource"

const NO_FILES: DiffFile[] | null = null

export interface PullRequestDiffState {
  files: DiffFile[] | null
  error: string | null
}

// usePullRequestDiff fetches and parses the PR's unified diff (gh pr diff) for
// the Files changed tab, and is refetched when the checkout's HEAD moves. Unlike
// the PR detail next door it does not refresh on window focus: re-rendering a
// wide diff is far too expensive to spend on every alt-tab back into the app.
// files is null while loading or on error; an empty array is a PR with no file
// changes.
//
// Fetched when the tab is open, which is no longer the same as on demand: the
// tab is remembered (pulls-prefs), so a reviewer who lives in Files changed
// opens every pull request straight into a diff round-trip. That is the trade
// the remembered tab makes — landing where the work is, at the cost of a call
// the Overview would not have made — and the answer is filed, so the second
// visit to the same diff pays nothing.
export function usePullRequestDiff(
  path: string,
  head: string,
  number: number,
): PullRequestDiffState {
  const { data, error } = useRemoteResource(
    path && `${path} ${number} ${head}`,
    () => ProjectService.PullRequestDiff(path, number).then(parseDiff),
    // head is not in the cache key: `gh pr diff` is asked for a number and
    // answers with GitHub's head, so the local HEAD only dates the answer.
    { empty: NO_FILES, cache: `pulls.diff ${path} ${number}` },
  )
  return { files: data, error }
}
