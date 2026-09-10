import type { Issue } from "@/lib/api-types"
import { bracketedPaste } from "@/lib/terminal/bracketed-paste"

// GitHub's own spelling of an issue reference, and the URL you get from copying
// one out of the browser. Both are what a person already has in hand when they
// decide to work on an issue — nothing here is a syntax lich invented.
//
// A bare number is deliberately not a reference: the field it is typed into is
// a branch name first, and git takes "128" as one. The "#" is the marker, so
// there is one field with one meaning rather than a mode to be in.
const ISSUE_HASH = /^#(\d+)$/
const ISSUE_URL = /^https?:\/\/[^/\s]+\/[^/\s]+\/[^/\s]+\/issues\/(\d+)(?:[/?#].*)?$/

/**
 * The issue number a typed worktree name refers to, or null when it names no
 * issue — which is every ordinary branch name and every task typed in words.
 */
export function parseIssueRef(input: string): number | null {
  const trimmed = input.trim()
  const match = ISSUE_HASH.exec(trimmed) ?? ISSUE_URL.exec(trimmed)
  if (!match) {
    return null
  }
  const number = Number(match[1])
  return Number.isSafeInteger(number) && number > 0 ? number : null
}

/**
 * The name an issue's worktree is proposed under: the number first, so the
 * branch sorts and greps by it, then the title. toBranchName does the rest —
 * this only decides what it is given.
 */
export function issueName(issue: Issue): string {
  return `${issue.number} ${issue.title}`
}

/**
 * What the issue's session finds at its prompt, ready to write into a PTY.
 *
 * One bracketed paste, so the newlines in a body are text rather than a run of
 * Enters — and so the whole thing lands unsent, which is the rule the pull
 * request handoff follows too: lich writes the prompt, the user sends it.
 */
export function issueBrief(issue: Issue): string {
  const head = `GitHub issue #${issue.number} — ${issue.title}\n${issue.url}`
  const body = issue.body.trim()
  return bracketedPaste(body ? `${head}\n\n${body}` : head)
}
