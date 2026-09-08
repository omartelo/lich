import { ProjectService } from "@/lib/rpc"
import type { Worktree } from "@/lib/api-types"
import { useRemoteResource } from "@/lib/use-remote-resource"

const NO_CHECKOUTS: Worktree[] = []

// useCheckouts lists every checkout of the project that holds a branch — its
// own directory as much as its linked worktrees. It answers the question the
// Pulls screen asks of each pull request: is this head branch already checked
// out somewhere? git refuses to check one branch out twice, and it refuses just
// as hard when the branch is held by the project's own directory, which is
// where a branch usually is.
//
// A local git call, so it re-reads on window focus like the git status poller
// — a worktree can be created or a branch switched from a terminal — and
// through refresh() right after this screen creates one itself. An error yields
// an empty list: the worst it costs is a checkout attempt that gh refuses with
// its own message, which is why the error itself is dropped here.
//
// refresh answers with the list it read, and that is what an action must use.
// The filed answer paints the button, so it may be a round trip behind a
// checkout removed from a terminal; deciding what the click *does* on a fresh
// read is what keeps "Go to session" from pointing at a directory that is gone.
export function useCheckouts(projectPath: string): {
  checkouts: Worktree[]
  refresh: () => Promise<Worktree[]>
} {
  const { data, refresh } = useRemoteResource(
    projectPath,
    () => ProjectService.ListCheckouts(projectPath).then((list) => list ?? NO_CHECKOUTS),
    { empty: NO_CHECKOUTS, refetchOnFocus: true, cache: `git.checkouts ${projectPath}` },
  )
  return { checkouts: data, refresh }
}
