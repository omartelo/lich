// Shapes shared with the Go services over the loopback RPC (internal/rpc).
// Hand-owned, no codegen: field names mirror the Go structs' JSON tags — keep
// them in sync when a service struct changes.

/** internal/project.Project — an opened project directory's identity. */
export interface Project {
  id: string
  name: string
  path: string
}

/** internal/project.DiffStats — uncommitted-changes summary of a work tree. */
export interface DiffStats {
  files: number
  added: number
  deleted: number
  /** The HEAD commit the counts sit on; "" in a repository without commits. */
  head: string
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
  isDraft: boolean
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
  isDraft: boolean
  /** gh: MERGEABLE | CONFLICTING | UNKNOWN */
  mergeable: string
  baseRefName: string
  headRefName: string
  changedFiles: number
  /** The head branch lives on a fork: no session can be opened on it. */
  isCrossRepository: boolean
  checks: ChecksRollup
  /** The rollup itself, worst state first; null when the PR reports no checks. */
  checkRuns: CheckItem[] | null
  /** Every commit the PR would land, oldest first; null when gh reports none. */
  commits: PullRequestCommit[] | null
}

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

/** internal/store.Session — a persisted terminal session (metadata only). */
export interface StoredSession {
  id: string
  label: string
  kind: string
  path: string
  providerSessionId: string
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

/** internal/claudeplugin.Status — the plugin's install/update state. */
export interface PluginStatus {
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

/** internal/providers.Detected — a known provider and whether it is on PATH. */
export interface DetectedProvider {
  id: string
  name: string
  installed: boolean
  path: string
}
