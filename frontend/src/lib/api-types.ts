// Shapes shared with the Go services over the loopback RPC (internal/rpc).
// Hand-owned, no codegen: field names mirror the Go structs' JSON tags — keep
// them in sync when a service struct changes.

import type { ProviderKind } from "@/lib/session/sessions"

/** internal/project.Project — an opened project directory's identity. */
export interface Project {
  id: string
  name: string
  path: string
}

/** internal/store.Recent — a closed project offered for reopening. Identity
 * only, hence the same shape as an opened one. */
export type RecentProject = Project

/** internal/project.DiffStats — uncommitted-changes summary of a work tree. */
export interface DiffStats {
  files: number
  added: number
  deleted: number
  /** The HEAD commit the counts sit on; "" in a repository without commits. */
  head: string
}

/** internal/project.BaseStatus — where a checkout stands against the branch it
 * will merge into. null from the service when there is nothing to answer from:
 * no origin, or no origin/HEAD to name a base with. */
export interface BaseStatus {
  /** The branch merged into, in "origin/main" form. */
  base: string
  /** Commits the base has that this checkout does not. */
  behind: number
  /** Repo-relative paths a merge would collide on; empty when it is clean. */
  conflicts: string[] | null
}

/** internal/project.PullRequest — the branch's open GitHub PR (gh CLI). */
export interface PullRequest {
  number: number
  url: string
  state: string
}

/** internal/project.ChecksRollup — gh statusCheckRollup collapsed to counts. */
export interface ChecksRollup {
  passed: number
  failed: number
  pending: number
  total: number
}

/** How a pull request is merged — the gh flag the backend allow-lists. */
export type MergeMethod = "squash" | "merge" | "rebase"

/** internal/project.BranchRules — what a repository's rulesets say about
 * merging into one branch. */
export interface BranchRules {
  /** The methods the branch accepts, in GitHub's spelling. Empty is "no rule
   * about it", which is also what an unreadable answer reports — so an empty
   * list must always widen the menu, never empty it. */
  allowedMergeMethods: string[] | null
  /** The rules that read a commit's own text. Empty means "no rule of this
   * kind" — never "nothing blocks this merge". */
  commitPatterns: CommitPattern[] | null
  /** This account administers the repository, and so may merge past the rules
   * above. The one field here that fails closed: anything short of a clear yes
   * is false, so the bypass is hidden rather than offered and refused. */
  viewerCanBypass: boolean
}

/** internal/project.CommitPattern — one ruleset rule testing the text of a
 * commit rather than the state of a pull request. The one class of rule
 * nothing on a pull request hints at, and so the one worth naming on screen. */
export interface CommitPattern {
  /** What the rule reads: "message" | "author email" | "committer email". */
  target: string
  /** GitHub's spelling: regex | starts_with | ends_with | contains. */
  operator: string
  pattern: string
  /** The pattern is what the text must *not* be. */
  negate: boolean
}

/** internal/project.CheckItem — one check of the PR's rollup, for the Checks tab. */
export interface CheckItem {
  name: string
  state: "passed" | "failed" | "pending"
  /** Workflow name, or the status context's own description. */
  description: string
  /** Where to read the run; "" when gh reports none. */
  url: string
  /** gh's ISO timestamps; "" while a check has not started or finished. */
  startedAt: string
  completedAt: string
}

/** internal/project.PRCommit — one commit of the PR, for the Commits tab. */
export interface PullRequestCommit {
  oid: string
  /** The message's subject line. */
  headline: string
  /** The rest of the message; "" for a one-line commit. */
  body: string
  /** The first author's login, or their name; "" when gh reports none. */
  author: string
  /** gh's ISO committedDate. */
  date: string
}

/** internal/project.PRSummary — one row of the repository's open pull requests. */
export interface PullRequestSummary {
  number: number
  title: string
  /** Login, or the display name when gh reports no login. */
  author: string
  /** gh: OPEN | CLOSED | MERGED */
  state: string
  isDraft: boolean
  /** gh: APPROVED | CHANGES_REQUESTED | REVIEW_REQUIRED; "" when none is required. */
  reviewDecision: string
  headRefName: string
  /** The head branch lives on a fork: it can be read, but not pushed back to. */
  isCrossRepository: boolean
  /** gh's ISO timestamp. */
  updatedAt: string
  checks: ChecksRollup
}

/** internal/project.PRDetail — the branch's open PR in full, for the Pulls panel. */
export interface PullRequestDetail {
  number: number
  url: string
  title: string
  body: string
  /** Who opened it: a login, or a display name when gh reports no login. */
  author: string
  /** gh: OPEN | CLOSED | MERGED. Only a number-addressed lookup returns a
   * non-OPEN one; the branch lookup still hides them. */
  state: string
  isDraft: boolean
  /** gh: MERGEABLE | CONFLICTING | UNKNOWN */
  mergeable: string
  /** GitHub's own "can this merge right now": CLEAN | UNSTABLE | BLOCKED |
   * BEHIND | DIRTY | DRAFT | HAS_HOOKS | UNKNOWN. See mergeBlockedReason. */
  mergeStateStatus: string
  /** gh's aggregate verdict: APPROVED | CHANGES_REQUESTED | REVIEW_REQUIRED;
   * "" where the repository requires no review. */
  reviewDecision: string
  /** Who was asked for a review and who answered, in one list; null when
   * nobody is on it. */
  reviewers: PullRequestReviewer[] | null
  baseRefName: string
  headRefName: string
  changedFiles: number
  /** The head branch lives on a fork. */
  isCrossRepository: boolean
  /** GitHub's "allow edits by maintainers" on a fork PR: what decides whether a
   * session can be opened on it. False on a same-repository PR, which needs no
   * such permission. */
  maintainerCanModify: boolean
  checks: ChecksRollup
  /** The rollup itself, worst state first; null when the PR reports no checks. */
  checkRuns: CheckItem[] | null
  /** Every commit the PR would land, oldest first; null when gh reports none. */
  commits: PullRequestCommit[] | null
}

/** internal/project.PRReviewer — one entry of the review roster: somebody asked
 * and still owing a review, or somebody who has answered. */
export interface PullRequestReviewer {
  /** A login, or a team's slug. */
  login: string
  /** A team, which gh addresses as org/slug — so it is shown, never edited. */
  isTeam: boolean
  /** "" while the review is still owed, else gh's APPROVED | CHANGES_REQUESTED
   * | COMMENTED | DISMISSED. */
  state: string
}

/** internal/project.PRCandidate — one account the reviewer picker may offer. */
export interface ReviewCandidate {
  login: string
  /** The display name; "" when the account has none. */
  name: string
}

/** internal/project.PRReview — one submitted review and its verdict. */
export interface PullRequestReview {
  author: string
  /** gh: APPROVED | CHANGES_REQUESTED | COMMENTED | DISMISSED */
  state: string
  body: string
  date: string
}

/** internal/project.PRComment — a comment on the pull request itself. */
export interface PullRequestComment {
  author: string
  body: string
  date: string
}

/** internal/project.PRThreadComment — one message inside a review thread. */
export interface ReviewThreadComment {
  /** REST database id — the address a reply is posted to. */
  id: number
  author: string
  body: string
  date: string
  /** GitHub's own snippet of the lines the thread is about; what an outdated
   * thread has left once the diff no longer holds them. */
  diffHunk: string
}

/** internal/project.PRReviewThread — one conversation anchored to a diff line. */
export interface ReviewThread {
  /** GraphQL node id — what resolving takes, never what a reply takes. */
  id: string
  path: string
  /** Anchor line in the file `side` names; a thread that lost its anchor keeps
   * the original line and is marked outdated. */
  line: number
  /** First line of a multi-line thread; 0 when it covers one line. */
  startLine: number
  /** RIGHT (the new file) | LEFT (a deleted line). */
  side: string
  isResolved: boolean
  isOutdated: boolean
  comments: ReviewThreadComment[] | null
}

/** internal/project.PRConversation — everything said about a pull request. */
export interface PullRequestConversation {
  /** The commit new review comments are filed against. */
  headRefOid: string
  reviews: PullRequestReview[] | null
  comments: PullRequestComment[] | null
  threads: ReviewThread[] | null
}

/** internal/project.ReviewComment — one line comment of a review being sent. */
export interface DraftReviewComment {
  path: string
  /** Last line the comment covers. */
  line: number
  /** First line of a range; 0 for a single line. */
  startLine: number
  /** RIGHT | LEFT; "" is RIGHT. */
  side: string
  body: string
}

/** The verdict a submitted review carries. */
export type ReviewEvent = "approve" | "comment" | "request_changes"

/** internal/project.Worktree — a git worktree checkout: branch and path. */
export interface Worktree {
  name: string
  path: string
}

/** internal/project.Branches — everything the base-branch picker offers. */
export interface Branches {
  local: string[] | null
  /** "origin/main" form */
  remote: string[] | null
  worktrees: Worktree[] | null
}

/** internal/project.WorktreeSetup — what the New-worktree dialog shows about
 * the project's setup: the .lich/setup-worktree.sh content, or a suggestion
 * detected from the repository root when the file is absent. */
export interface WorktreeSetup {
  script: string
  suggestion?: string
  /** The files behind suggestion, e.g. "pnpm-lock.yaml". */
  detected?: string
}

/** internal/store.Session — a persisted terminal session (metadata only). */
export interface StoredSession {
  id: string
  label: string
  kind: string
  path: string
  providerSessionId: string
  pinned: boolean
  /** The session that asked for this one, "" for a session nobody delegated. */
  originSessionId: string
  /** What that session was called at the time; all that survives its close. */
  originLabel: string
}

/** internal/terminal.TranscriptMatch — a session whose conversation mentions a
 * search query, with its newest matching message. */
export interface TranscriptMatch {
  id: string
  snippet: string
  count: number
}

/** internal/store.Project — a persisted project with its session state. */
export interface StoredProject {
  id: string
  name: string
  path: string
  nextSeq: number
  activeSessionId: string
  sessions: StoredSession[] | null
}

/** internal/agentplugin.Status — one provider's plugin install/update state. */
export interface PluginStatus {
  /** Provider id (mirrors internal/providers) whose CLI this is about. */
  provider: ProviderKind
  /** Display name, for the rows the user reads. */
  name: string
  /** Whether that CLI is on PATH at all — false means nothing to install into. */
  available: boolean
  installed: boolean
  installedVersion: string
  latestVersion: string
  updateAvailable: boolean
}

/** internal/appupdate.Status — lich's own release/update state. */
export interface AppUpdateStatus {
  currentVersion: string
  latestVersion: string
  updateAvailable: boolean
  /** true where lich can swap its own binary (Windows/macOS); false on Linux. */
  canSelfApply: boolean
  releaseUrl: string
  /** shell command the UI pastes to update a package-manager install; "" where canSelfApply. */
  installCommand: string
}

/**
 * internal/drop.Item — one entry of a terminal file drop, described by the
 * only thing Chromium tells the page about a local file. mtime is
 * File.lastModified (milliseconds); size is 0 for a directory, whose reported
 * one is a placeholder.
 */
export interface DropItem {
  name: string
  size: number
  mtime: number
  dir: boolean
}

/** internal/themes.Theme — a color theme for the UI tokens and xterm. */
export interface ThemeDefinition {
  id: string
  name: string
  scheme: "light" | "dark"
  origin: "bundled" | "custom"
  /** Set only for a theme installed from a repository; a picked file has none. */
  source?: ThemeSource
  app: Record<string, string>
  terminal: Record<string, string>
}

/** internal/themes.Source — the repository a theme was installed from. */
export interface ThemeSource {
  url: string
  version: string
}

/** internal/themes.ImportResult — import outcome before an optional overwrite. */
export interface ThemeImportResult {
  theme: ThemeDefinition
  needsOverwrite: boolean
}

/**
 * internal/themes.GitInstallResult — a repository install. A non-empty
 * conflicts names the ids that need confirmation: the scan before the write
 * refuses the whole pack, while an id that appeared in between is the only one
 * left alone and themes says what did go in.
 */
export interface ThemeGitInstallResult {
  pack: string
  version: string
  themes: ThemeDefinition[] | null
  conflicts: string[] | null
  upToDate: boolean
}

/** internal/patchnotes.Group — one "### Added/Changed/Fixed" block of a release. */
export interface PatchNotesGroup {
  label: string
  /** Item text with markdown bold/code markers intact, rendered by the gate. */
  items: string[]
}

/** internal/patchnotes.Notes — the running build's changelog section. */
export interface PatchNotes {
  version: string
  /** null when no section matches (a dev build, or a version not in the changelog). */
  groups: PatchNotesGroup[] | null
}

/** internal/system.Diagnostics — what a bug report opens with. */
export interface Diagnostics {
  /** "dev" outside a release build. */
  version: string
  /** GOOS/GOARCH, e.g. "linux/amd64". */
  platform: string
  /** "" when the log file could not be opened and lich logs to stderr only. */
  logPath: string
}

/** internal/providers.Detected — a known provider and whether it is on PATH. */
export interface DetectedProvider {
  id: string
  name: string
  installed: boolean
  path: string
}
