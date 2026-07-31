// RPC client for the Go services over the loopback listener (internal/rpc).
// The page is served by that listener, so the endpoint rides the page URL:
// ?token=... (auth) and, under the Vite dev server only, &backend=<port> —
// dev splits the page origin from the RPC listener (see `task dev`).
//
// Each facade mirrors its Go service's method names and signatures; shapes
// live in lib/api-types.ts.

import type {
  AppUpdateStatus,
  BranchRules,
  Branches,
  DetectedProvider,
  DiffStats,
  DraftReviewComment,
  MergeMethod,
  PatchNotes as PatchNotesData,
  PluginStatus,
  Project,
  PullRequest,
  PullRequestConversation,
  PullRequestDetail,
  PullRequestSummary,
  ReviewEvent,
  StoredProject,
  StoredSession,
  Worktree,
} from "./api-types"

export interface Endpoint {
  base: string
  token: string
}

let cached: Endpoint | null = null

// endpointFromLocation reads the backend coordinates off the page URL.
// Exported for tests; production callers use endpoint().
export function endpointFromLocation(href: string): Endpoint | null {
  try {
    const url = new URL(href)
    const token = url.searchParams.get("token")
    if (!token || !url.host) {
      return null
    }
    const backend = url.searchParams.get("backend")
    const base = backend ? `http://127.0.0.1:${backend}` : `${url.protocol}//${url.host}`
    return { base, token }
  } catch {
    return null
  }
}

export function endpoint(): Endpoint {
  if (!cached) {
    const fromUrl = endpointFromLocation(window.location.href)
    if (!fromUrl) {
      throw new Error(
        "no backend endpoint in page URL (missing ?token=) — launch through the lich binary",
      )
    }
    cached = fromUrl
  }
  return cached
}

async function call<T>(method: string, args: unknown[]): Promise<T> {
  const { base, token } = endpoint()
  const response = await fetch(`${base}/rpc/${method}?token=${token}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(args),
  })
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `rpc ${method}: HTTP ${response.status}`)
  }
  return (await response.json()) as T
}

// post hits a bespoke (non-/rpc/) endpoint with the connect token. Only
// /restart needs this today; every service facade goes through call().
async function post(path: string): Promise<void> {
  const { base, token } = endpoint()
  const response = await fetch(`${base}${path}?token=${token}`, { method: "POST" })
  if (!response.ok) {
    throw new Error(`${path}: HTTP ${response.status}`)
  }
}

export const Terminal = {
  /** Whether a session cannot start in cwd because the directory is gone — a
   * worktree removed outside lich. False for every uncertainty, so only a
   * provable absence closes a session. */
  WorkdirMissing: (cwd: string) => call<boolean>("terminal.WorkdirMissing", [cwd]),
  /** Whether the conversation providerSessionID names can still be reopened —
   * false once the provider has pruned its transcript, which is the resume the
   * prompt must not offer. */
  ResumeAvailable: (kind: string, providerSessionID: string) =>
    call<boolean>("terminal.ResumeAvailable", [kind, providerSessionID]),
  /** resume: a provider session id to reopen (--resume); "" starts fresh.
   * setup: run the project's worktree setup script ahead of the provider —
   * passed once, by the first Start after the worktree is created. */
  Start: (
    id: string,
    projectID: string,
    cwd: string,
    kind: string,
    resume: string,
    setup: boolean,
    cols: number,
    rows: number,
  ) => call<null>("terminal.Start", [id, projectID, cwd, kind, resume, setup, cols, rows]),
  Write: (id: string, data: string) => call<null>("terminal.Write", [id, data]),
  Resize: (id: string, cols: number, rows: number) =>
    call<null>("terminal.Resize", [id, cols, rows]),
  SetVisible: (id: string, visible: boolean) => call<null>("terminal.SetVisible", [id, visible]),
  // Base64 tail of a session's output, to reseed scrollback after a reload.
  Replay: (id: string) => call<string>("terminal.Replay", [id]),
  Close: (id: string) => call<null>("terminal.Close", [id]),
}

export const ProjectService = {
  Open: () => call<Project | null>("project.Open", []),
  /** A project rooted at the user's home dir, no picker (the update flow's
   * install terminal when no project is in view). */
  Home: () => call<Project>("project.Home", []),
  PickFile: () => call<string>("project.PickFile", []),
  Branch: (path: string) => call<string>("project.Branch", [path]),
  Diff: (path: string) => call<DiffStats>("project.Diff", [path]),
  DiffText: (path: string) => call<string>("project.DiffText", [path]),
  /** Tracked files, repo-relative and slash-separated, sorted (git ls-files). */
  Tree: (path: string) => call<string[] | null>("project.Tree", [path]),
  ReadFile: (path: string, rel: string) => call<string>("project.ReadFile", [path, rel]),
  DiscardFile: (path: string, rel: string) => call<null>("project.DiscardFile", [path, rel]),
  ListBranches: (path: string) => call<Branches>("project.ListBranches", [path]),
  /** The logins gh is authenticated as, for the project's account picker;
   * errors when gh is missing or logged out. */
  GitHubAccounts: () => call<string[] | null>("project.GitHubAccounts", []),
  /** Every checkout holding a branch, the project's own directory included —
   * which ListBranches omits, since it cannot be resumed as a worktree. */
  ListCheckouts: (path: string) => call<Worktree[] | null>("project.ListCheckouts", [path]),
  PullRequest: (path: string) => call<PullRequest | null>("project.PullRequest", [path]),
  /** The repository's PRs in one state (open|closed|merged|all) for the Pulls
   * list column; null when it has none. The backend allow-lists the state. */
  ListPullRequests: (path: string, state: string) =>
    call<PullRequestSummary[] | null>("project.ListPullRequests", [path, state]),
  /** One open PR in full (title, body, checks): the given number, or 0 for the checkout's own branch. */
  PullRequestDetail: (path: string, number: number) =>
    call<PullRequestDetail | null>("project.PullRequestDetail", [path, number]),
  /** Merge a PR on GitHub (0 = the checkout's branch). The backend allow-lists
   * the method. `admin` merges with administrator privileges — GitHub's own
   * bypass of the rules on the base branch, and the only way gh calls GitHub at
   * all once a rule holds the merge back. */
  MergePullRequest: (
    path: string,
    number: number,
    method: MergeMethod,
    subject: string,
    body: string,
    admin: boolean,
  ) => call<null>("project.MergePullRequest", [path, number, method, subject, body, admin]),
  /** A PR's reviews, its own comments and its review threads, in one call. The
   * number is required — the query addresses a PR, never "the branch's". */
  PullRequestConversation: (path: string, number: number) =>
    call<PullRequestConversation | null>("project.PullRequestConversation", [path, number]),
  /** File one review: the verdict, the summary body, and every line comment the
   * screen has been holding. `head` is the commit they were written against.
   * Approve is this with no body and no comments; GitHub refuses a PR opened by
   * the same account. */
  SubmitReview: (
    path: string,
    number: number,
    event: ReviewEvent,
    body: string,
    head: string,
    comments: DraftReviewComment[],
  ) => call<null>("project.SubmitReview", [path, number, event, body, head, comments]),
  /** Answer an existing thread; commentID is the database id of a comment in
   * it. Posts immediately — a reply is never part of the pending review. */
  ReplyToReviewThread: (path: string, number: number, commentID: number, body: string) =>
    call<null>("project.ReplyToReviewThread", [path, number, commentID, body]),
  /** Resolve a thread, or reopen it. threadID is the GraphQL node id. */
  ResolveReviewThread: (path: string, threadID: string, resolved: boolean) =>
    call<null>("project.ResolveReviewThread", [path, threadID, resolved]),
  /** Comment on the pull request itself — no file, no line. */
  CommentOnPullRequest: (path: string, number: number, body: string) =>
    call<null>("project.CommentOnPullRequest", [path, number, body]),
  /** What a repository's rulesets say about merging into one branch. Answers
   * per branch, not per pull request — every PR onto the same base shares it. */
  BranchRules: (path: string, branch: string) =>
    call<BranchRules | null>("project.BranchRules", [path, branch]),
  /** Open GitHub's "new pull request" page in the browser (gh pr create --web). */
  CreatePullRequest: (path: string) => call<null>("project.CreatePullRequest", [path]),
  /** A PR's unified diff (gh pr diff) for the Files changed tab; 0 = the checkout's branch. */
  PullRequestDiff: (path: string, number: number) =>
    call<string>("project.PullRequestDiff", [path, number]),
  CreateWorktree: (
    projectPath: string,
    projectID: string,
    name: string,
    base: string,
    baseIsRemote: boolean,
  ) =>
    call<Worktree | null>("project.CreateWorktree", [
      projectPath,
      projectID,
      name,
      base,
      baseIsRemote,
    ]),
  /** Check a pull request's head branch out into its own worktree; rejects a fork PR. */
  CreateWorktreeFromPR: (projectPath: string, projectID: string, number: number) =>
    call<Worktree | null>("project.CreateWorktreeFromPR", [projectPath, projectID, number]),
  RemoveWorktree: (projectPath: string, wtPath: string, force: boolean) =>
    call<null>("project.RemoveWorktree", [projectPath, wtPath, force]),
  WorktreeDirty: (wtPath: string) => call<boolean>("project.WorktreeDirty", [wtPath]),
}

export const Store = {
  LoadState: () => call<StoredProject[] | null>("store.LoadState", []),
  AddProject: (id: string, name: string, path: string) =>
    call<null>("store.AddProject", [id, name, path]),
  CloseProject: (id: string) => call<null>("store.CloseProject", [id]),
  AddSession: (
    projectID: string,
    sessionID: string,
    label: string,
    kind: string,
    path: string,
    nextSeq: number,
  ) => call<null>("store.AddSession", [projectID, sessionID, label, kind, path, nextSeq]),
  DeleteSession: (projectID: string, sessionID: string, activeID: string) =>
    call<null>("store.DeleteSession", [projectID, sessionID, activeID]),
  /** Park a session (keep its row for a later resume) instead of deleting it. */
  CloseSession: (projectID: string, sessionID: string, activeID: string) =>
    call<null>("store.CloseSession", [projectID, sessionID, activeID]),
  /** Resume a parked worktree session under a fresh id, or null when none. */
  ReopenWorktreeSession: (projectID: string, path: string, newSessionID: string) =>
    call<StoredSession | null>("store.ReopenWorktreeSession", [projectID, path, newSessionID]),
  /** Drop every session row for a worktree path when the worktree is removed. */
  PurgeWorktreeSessions: (projectID: string, path: string) =>
    call<null>("store.PurgeWorktreeSessions", [projectID, path]),
  RenameSession: (sessionID: string, label: string) =>
    call<null>("store.RenameSession", [sessionID, label]),
  SetActiveSession: (projectID: string, sessionID: string) =>
    call<null>("store.SetActiveSession", [projectID, sessionID]),
  ReorderProjects: (ids: string[]) => call<null>("store.ReorderProjects", [ids]),
  ReorderSessions: (projectID: string, ids: string[]) =>
    call<null>("store.ReorderSessions", [projectID, ids]),
  GetSetting: (key: string, projectID: string) =>
    call<string>("store.GetSetting", [key, projectID]),
  SetSetting: (key: string, projectID: string, value: string) =>
    call<null>("store.SetSetting", [key, projectID, value]),
}

export const Fonts = {
  List: () => call<string[] | null>("fonts.List", []),
}

export const ClaudePlugin = {
  Status: () => call<PluginStatus>("claudeplugin.Status", []),
  Install: () => call<null>("claudeplugin.Install", []),
  Update: () => call<null>("claudeplugin.Update", []),
}

export const AppUpdate = {
  Status: () => call<AppUpdateStatus>("appupdate.Status", []),
  /** Download, verify and swap the binary. Only valid where canSelfApply. */
  Apply: () => call<null>("appupdate.Apply", []),
  /** In-place relaunch (POST /restart, not an RPC method): spawns a successor
   * and closes this window. Resolves on the 204 sent before teardown. */
  Restart: () => post("/restart"),
}

export const PatchNotes = {
  /** The running build's changelog section, for the "what's new" popup. */
  Current: () => call<PatchNotesData>("patchnotes.Current", []),
}

export const System = {
  OpenExternal: (url: string) => call<null>("system.OpenExternal", [url]),
  /** Open a work-tree file (repo-relative rel under dir) in $VISUAL/$EDITOR.
   * Returns "" when it launched a GUI editor detached, or a shell command line
   * to run in a terminal session when the editor is a terminal editor. */
  OpenInEditor: (dir: string, rel: string) => call<string>("system.OpenInEditor", [dir, rel]),
}

export const Providers = {
  /** Every known provider with its install state (binary found on PATH). */
  Detect: () => call<DetectedProvider[]>("providers.Detect", []),
}
