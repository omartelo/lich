import { ProjectService } from "@/lib/rpc"
import { useRemoteResource } from "@/lib/use-remote-resource"

// Module-level, as useRemoteResource asks: a fresh array per render would be a
// new answer on every failed lookup.
const NO_FILES: string[] = []

export interface PullRequestConflicts {
  files: string[]
  loading: boolean
  error: string | null
}

// usePullRequestConflicts names the files a pull request collides with its base
// on. Asked only while `conflicting` holds — GitHub has already said the merge
// conflicts — because the answer is computed by fetching both commits, which is
// a network round trip a clean pull request has no reason to pay for.
//
// The base is in the cache key as well as the request key: a retargeted pull
// request asks a different question, and one entry answering both would paint
// the old base's files under the new base's name with no loading state to
// disown them.
//
// Keyed by the pull request and its base alone: what makes the answer stale is
// a push to either side, and both of those arrive as a re-read of the detail
// that flips `conflicting` or moves the base. Not refetched on focus for the
// same reason — the fetch is the expensive kind an alt-tab must not trigger.
export function usePullRequestConflicts(
  path: string,
  number: number,
  base: string,
  url: string,
  conflicting: boolean,
): PullRequestConflicts {
  const { data, loading, error } = useRemoteResource(
    conflicting && path && number > 0 && base ? `${path} ${number} ${base} ${url}` : "",
    () =>
      ProjectService.PullRequestConflicts(path, number, base, url).then(
        (files) => files ?? NO_FILES,
      ),
    { empty: NO_FILES, cache: `pulls.conflicts ${path} #${number} ${base}` },
  )
  return { files: data, loading, error }
}
