import { ProjectService } from "@/lib/rpc"
import type { BranchRules } from "@/lib/api-types"
import { useRemoteResource } from "@/lib/use-remote-resource"

export type { BranchRules }

const NO_RULES: BranchRules | null = null

// useBranchRules reads what a repository's rulesets say about merging into one
// branch. Keyed by the branch rather than by the pull request, because that is
// what the answer is about — every pull request onto the same base gets the
// same one.
//
// Deliberately unpolled and not refetched on focus: a ruleset is a repository
// setting somebody changes once a quarter, and the only thing this narrows is a
// menu. A stale answer offers a method GitHub then refuses, which is the same
// place the screen was before this existed.
export function useBranchRules(path: string, branch: string): BranchRules | null {
  const { data } = useRemoteResource(
    path && branch ? `${path} ${branch}` : "",
    () => ProjectService.BranchRules(path, branch),
    { empty: NO_RULES, cache: `pulls.rules ${path} ${branch}` },
  )
  return data
}
