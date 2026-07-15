// Shapes shared with the Go services over the loopback RPC (internal/rpc).
// Formerly generated Wails bindings; hand-owned since the Wails shell was
// deleted (docs/chromium-shell.md phase 5). Field names mirror the Go structs'
// JSON tags — keep them in sync when a service struct changes.

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
}

/** internal/project.PullRequest — the branch's open GitHub PR (gh CLI). */
export interface PullRequest {
  number: number
  url: string
  state: string
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
  claudeSessionId: string
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
