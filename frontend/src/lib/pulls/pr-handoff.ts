import type { BaseStatus, PullRequestDetail } from "@/lib/api-types"
import { prompt } from "@/lib/i18n/prompt"
import { conflictsWithBase } from "@/lib/pulls/merge-gate"
import { bracketedPaste } from "@/lib/terminal/bracketed-paste"

// How many failed checks the prompt names before it stops and says how many it
// left out. A rollup can carry dozens of red jobs on a matrix build, and the
// prompt is the agent's starting point, not the CI page's index — a hundred
// URLs pasted into a terminal is the log by another route.
const NAMED_CHECKS = 8

/** One thing wrong with a pull request, and the prompt that hands it over. */
export interface PullRequestHandoff {
  label: string
  /** Ready to write into a PTY: the prompt as one bracketed paste. A getter, so
   * the prompt language is read when the prompt is used, not when the card
   * rendered. */
  readonly prompt: string
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
  if (conflictsWithBase(detail)) {
    return {
      label: "Resolve conflicts",
      get prompt() {
        return bracketedPaste(
          prompt("prompts.pullRequest.conflicts", {
            number: detail.number,
            branch: detail.headRefName,
            base: detail.baseRefName,
          }),
        )
      },
    }
  }
  if (detail.checks.failed > 0) {
    return {
      label: "Fix CI errors",
      get prompt() {
        return bracketedPaste(checksPrompt(detail))
      },
    }
  }
  return null
}

function checksPrompt(detail: PullRequestDetail): string {
  const params = { number: detail.number, branch: detail.headRefName }
  const failed = (detail.checkRuns ?? []).filter((run) => run.state === "failed")
  // gh reports the counts and the runs from the same rollup, but the runs are
  // the half that can come back empty — a status context with no name, an older
  // gh. The count is what the header showed, so it is what the prompt owes.
  if (failed.length === 0) {
    return prompt("prompts.pullRequest.checksUnnamed", { ...params, count: detail.checks.failed })
  }
  const named = failed.slice(0, NAMED_CHECKS)
  const lines = named.map((run) => `- ${run.name}${run.url ? ` — ${run.url}` : ""}`)
  const omitted = failed.length - named.length
  const tail = omitted > 0 ? prompt("prompts.pullRequest.checksOmitted", { count: omitted }) : ""
  return prompt("prompts.pullRequest.checksListed", { ...params, checks: lines.join("\n") }) + tail
}

// createPullRequestPrompt hands the writing of a branch's first pull request to
// the agent that made the commits, the way the problems above are handed over.
// The base, draft or not, and the template are left to it and to gh: the text
// sits at the prompt unsent, so a reader who wants otherwise edits it there.
export function createPullRequestPrompt(branch: string): string {
  return bracketedPaste(prompt("prompts.pullRequest.create", { branch }))
}

// offersPullRequest says whether a branch with no open pull request is one a
// pull request could be opened from: on a branch, with an origin to push to, and
// not the base itself. Commits ahead of the base are not counted (BaseStatus
// carries only the behind side), so a branch with nothing to propose still gets
// the offer, and the agent is the one that says so.
export function offersPullRequest(branch: string, base: BaseStatus | null): boolean {
  return branch !== "" && base !== null && `origin/${branch}` !== base.base
}
