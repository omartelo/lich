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
