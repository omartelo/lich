import type { PullRequestCommit } from "@/lib/api-types"

// How many addresses or accounts the line names before it folds the rest into a
// count. Three is enough to recognise the mistake; a branch that made it under
// four different identities has a bigger problem than a list can render.
const NAMED = 3

const NO_ACCOUNT = "linked to no GitHub account"

// Whether gh reported an author for this commit at all. A web-flow merge commit
// carries none, and calling that one unattributed would be the false alarm this
// whole reading exists to avoid — nobody's identity is wrong, there is nobody.
function hasAuthor(commit: PullRequestCommit): boolean {
  return commit.email !== "" || commit.name !== ""
}

function unattributed(commit: PullRequestCommit): boolean {
  return commit.login === "" && hasAuthor(commit)
}

/** The line under a commit's subject: who landed it, when, and — when GitHub
 * resolved the author's email to no account — that nobody owns it. A login is
 * GitHub's own answer and is shown as one; without the mark, a commit belonging
 * to nobody prints the git name and reads exactly like one that does. */
export function commitMetaLine(commit: PullRequestCommit): string {
  const at = new Date(commit.date)
  const date = Number.isNaN(at.getTime()) ? "" : at.toLocaleDateString()
  const who = commit.login ? `@${commit.login}` : commit.name
  const landed =
    who && date ? `${who} committed ${date}` : who || (date && `Committed ${date}`) || ""
  if (!unattributed(commit)) {
    return landed
  }
  return landed ? `${landed} · ${NO_ACCOUNT}` : `Committed by an address ${NO_ACCOUNT}`
}

/** The one line above the commit list, or "" when there is nothing to say.
 *
 * Nothing here guesses. GitHub resolved every author email against the verified
 * addresses of every account before answering, so a login that differs is a
 * different account and an empty one is an email no account owns — the two
 * readings a noreply form, a vanity domain or an org alias used to make
 * unsayable. What the line cannot see is a commit that has not been pushed:
 * before that, nobody has resolved anything. */
export function commitAuthorNotice(
  commits: PullRequestCommit[] | null,
  authorLogin: string,
): string {
  if (!commits || commits.length === 0) {
    return ""
  }
  const strays = commits.filter((c) => c.login !== "" && c.login !== authorLogin)
  const orphans = commits.filter(unattributed)
  return [
    // Only when the pull request's own author is a login: gh reports none for a
    // deleted account, and comparing against "" would call every commit a stray.
    authorLogin && strays.length > 0
      ? `${count(strays.length, "commit")} landed under ${list(distinct(strays.map((c) => `@${c.login}`)))}, not @${authorLogin}.`
      : "",
    orphans.length > 0
      ? `${count(orphans.length, "commit")} ${orphans.length === 1 ? "is" : "are"} ${NO_ACCOUNT}${emails(orphans)}.`
      : "",
  ]
    .filter(Boolean)
    .join(" ")
}

function emails(commits: PullRequestCommit[]): string {
  const addresses = distinct(commits.map((c) => c.email).filter(Boolean))
  return addresses.length > 0 ? `: ${list(addresses)}` : ""
}

function distinct(values: string[]): string[] {
  return Array.from(new Set(values))
}

// Up to NAMED of them, then how many were held back — so a long list stays one
// line and still says it is not the whole list.
function list(values: string[]): string {
  const hidden = values.length - NAMED
  const shown = values.slice(0, NAMED).join(", ")
  return hidden > 0 ? `${shown} and ${hidden} more` : shown
}

function count(n: number, noun: string): string {
  return `${n} ${noun}${n === 1 ? "" : "s"}`
}
