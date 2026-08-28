import type { PullRequestDetail } from "@/lib/api-types"
import { bracketedPaste } from "@/lib/terminal/bracketed-paste"

// How many failed checks the prompt names before it stops and says how many it
// left out. A rollup can carry dozens of red jobs on a matrix build, and the
// prompt is the agent's starting point, not the CI page's index — a hundred
// URLs pasted into a terminal is the log by another route.
const NAMED_CHECKS = 8

/** One thing wrong with a pull request, and the prompt that hands it over. */
export interface PullRequestHandoff {
  label: string
  /** Ready to write into a PTY: the prompt as one bracketed paste. */
  prompt: string
}

// pullRequestHandoff names the pull request's current problem, worst first: a
// conflict before red CI, because a branch that will not merge is the thing to
// fix and its checks are about to be re-run over the resolution anyway. Null
// when nothing is wrong — the Merge button already owns that state.
//
// It only writes a prompt. Nothing here runs, re-runs or watches anything: what
// the agent does with it, and whether it was worth asking, stays the reader's.
export function pullRequestHandoff(detail: PullRequestDetail): PullRequestHandoff | null {
  if (detail.state !== "OPEN") {
    return null
  }
  // Both fields, for the reason mergeBlockedReason reads both: mergeable is
  // what an older gh reports, mergeStateStatus what a current one does.
  if (detail.mergeStateStatus === "DIRTY" || detail.mergeable === "CONFLICTING") {
    return {
      label: "Resolve conflicts",
      prompt: bracketedPaste(
        `Pull request #${detail.number} (${detail.headRefName}) has merge conflicts with ${detail.baseRefName}. Resolve them.`,
      ),
    }
  }
  if (detail.checks.failed > 0) {
    return { label: "Fix CI errors", prompt: bracketedPaste(checksPrompt(detail)) }
  }
  return null
}

function checksPrompt(detail: PullRequestDetail): string {
  const head = `CI is failing on pull request #${detail.number} (${detail.headRefName}).`
  const failed = (detail.checkRuns ?? []).filter((run) => run.state === "failed")
  // gh reports the counts and the runs from the same rollup, but the runs are
  // the half that can come back empty — a status context with no name, an older
  // gh. The count is what the header showed, so it is what the prompt owes.
  if (failed.length === 0) {
    return `${head} ${detail.checks.failed} ${detail.checks.failed === 1 ? "check is" : "checks are"} red; find out which and fix them.`
  }
  const named = failed.slice(0, NAMED_CHECKS)
  const lines = named.map((run) => `- ${run.name}${run.url ? ` — ${run.url}` : ""}`)
  const omitted = failed.length - named.length
  const tail =
    omitted > 0
      ? `\n\n…and ${omitted} more failing ${omitted === 1 ? "check" : "checks"} not listed.`
      : ""
  return `${head} Fix these checks:\n\n${lines.join("\n")}${tail}`
}
