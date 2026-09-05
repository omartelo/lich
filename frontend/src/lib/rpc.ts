// RPC client for the Go services over the loopback listener (internal/rpc).
// The page is served by that listener, so the endpoint rides the page URL:
// ?token=... (auth) and, under the Vite dev server only, &backend=<port> —
// dev splits the page origin from the RPC listener (see `task dev`).
//
// Each facade mirrors its Go service's method names and signatures; shapes
// live in lib/api-types.ts.

import type {
  AppUpdateStatus,
  BaseStatus,
  BinaryCheck,
  BranchRules,
  ClosedSession,
  LastSaid,
  LastTurn,
  Branches,
  CommitIdentity,
  DetectedProvider,
  Diagnostics as DiagnosticsData,
  DiffStats,
  DraftReviewComment,
  Attachment,
  DropItem,
  MergeMethod,
  PatchNotes as PatchNotesData,
  PluginStatus,
  Project,
  PullRequest,
  PullRequestConversation,
  PullRequestDetail,
  PullRequestSummary,
  QuotaPlan,
  RecentProject,
  ReviewCandidate,
  ReviewEvent,
  StoredProject,
  StoredSession,
  ThemeDefinition,
  ThemeGitInstallResult,
  ThemeImportResult,
  TranscriptMatch,
  Worktree,
  WorktreeSetup,
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
   * false once the provider has dropped it, which is the resume the prompt must
   * not offer. cwd is the session's working directory: Crush keeps its
   * conversations per checkout, so the same id answers differently in another. */
  ResumeAvailable: (kind: string, providerSessionID: string, cwd: string) =>
    call<boolean>("terminal.ResumeAvailable", [kind, providerSessionID, cwd]),
  /** resume: a provider session id to reopen (--resume); "" starts fresh.
   * name: what the session answers to in its provider's peer roster (lib/session/peer-name);
   * only Claude Code has one, every other kind ignores it.
   * setup: run the project's worktree setup script ahead of the provider —
   * passed once, by the first Start after the worktree is created. */
  Start: (
    id: string,
    projectID: string,
    cwd: string,
    kind: string,
    resume: string,
    name: string,
    setup: boolean,
    cols: number,
    rows: number,
  ) => call<null>("terminal.Start", [id, projectID, cwd, kind, resume, name, setup, cols, rows]),
  Write: (id: string, data: string) => call<null>("terminal.Write", [id, data]),
  /** Whether this session can be given work: its provider is the program
   * reading the PTY — not the checkout's setup script, not a TUI still taking
   * the tty over — and the prompt is not half-filled by the person at it. False
   * for a session that is not running, including one whose card exists but
   * whose terminal has never been opened (lib/terminal/write-at-prompt). */
  Ready: (id: string) => call<boolean>("terminal.Ready", [id]),
  Resize: (id: string, cols: number, rows: number) =>
    call<null>("terminal.Resize", [id, cols, rows]),
  SetVisible: (id: string, visible: boolean) => call<null>("terminal.SetVisible", [id, visible]),
  // Base64 tail of a session's output, to reseed scrollback after a reload.
  Replay: (id: string) => call<string>("terminal.Replay", [id]),
  /** How long a session has been worked on, in whole seconds — typed at,
   * reporting, or producing output for an open turn, minus every silence longer
   * than the idle gap. 0 for a session nothing has been counted for yet, and up
   * to one flush behind what has been measured (internal/terminal.handsOn). */
  HandsOn: (id: string) => call<number>("terminal.HandsOn", [id]),
  /** Which of these sessions have talked about the query, and what they said.
   * Empty for a query under three characters, and for a session whose current
   * conversation the backend cannot read. */
  SearchTranscripts: (ids: string[], query: string) =>
    call<TranscriptMatch[] | null>("terminal.SearchTranscripts", [ids, query]),
  Close: (id: string) => call<null>("terminal.Close", [id]),
  /** What changed on disk while this session's last finished turn ran — a
   * window of time, not an attribution (see internal/terminal.LastTurnDiff). */
  LastTurnDiff: (id: string) => call<LastTurn>("terminal.LastTurnDiff", [id]),
  /** The last thing the agent said in this session's conversation — its own
   * closing words, not a summary of them (see internal/terminal.LastTurnSaid).
   * `text` is absent whenever there are none to show. */
  LastTurnSaid: (id: string) => call<LastSaid>("terminal.LastTurnSaid", [id]),
}

export const DropService = {
  /** Absolute paths for the dropped items found under root, in order; "" where
   * the tree holds no single match and the caller must upload a copy instead.
   * `confined` is whether the session runs in the sandbox: its home is an empty
   * private one, so the home is not searched for it and anything outside the
   * checkout falls through to the copy. The upload is not here — its body is
   * the file, so it has its own endpoint (see lib/terminal/drop-files.ts). */
  Resolve: (root: string, items: DropItem[], confined: boolean) =>
    call<string[]>("drop.Resolve", [root, items, confined]),
  /** Open the native file picker and answer with a path the session can open —
   * the file's own, or a copy's when a confined session cannot reach it. The
   * picker runs on the backend rather than here: see internal/drop.Attach. */
  Attach: (sessionId: string, root: string, confined: boolean) =>
    call<Attachment>("drop.Attach", [sessionId, root, confined]),
}

export const ProjectService = {
  Open: () => call<Project | null>("project.Open", []),
  /** A project rooted at the user's home dir, no picker (the update flow's
   * install terminal when no project is in view). */
  Home: () => call<Project>("project.Home", []),
  /** Whether a stored project's directory is still there, before reopening it. */
  Exists: (path: string) => call<boolean>("project.Exists", [path]),
  /** Which of these project directories are gone, so the lists that offer them
   * can mark the rows that need relocating first. */
  Missing: (paths: string[]) => call<string[]>("project.Missing", [paths]),
  /** Point a stored project at a new directory, keeping id — hence its sessions
   * and its worktrees. null when the user cancels the picker. */
  Relocate: (id: string) => call<Project | null>("project.Relocate", [id]),
  PickFile: (title: string) => call<string>("project.PickFile", [title]),
  /** Native save dialog seeded with defaultName. "" when the user cancels. */
  PickSaveFile: (title: string, defaultName: string) =>
    call<string>("project.PickSaveFile", [title, defaultName]),
  Branch: (path: string) => call<string>("project.Branch", [path]),
  /** Branch for several checkouts at once, keyed by path; a checkout that names
   * none is left out. For the lists that want each row's branch once as it is
   * drawn, rather than the per-path poll a live card subscribes to. */
  BranchesOf: (paths: string[]) => call<Record<string, string>>("project.BranchesOf", [paths]),
  Diff: (path: string) => call<DiffStats>("project.Diff", [path]),
  /** How far the checkout's base branch has moved and what a merge would
   * collide on. null when the repository has no origin to measure against. */
  BaseStatus: (path: string) => call<BaseStatus | null>("project.BaseStatus", [path]),
  DiffText: (path: string) => call<string>("project.DiffText", [path]),
  /** Tracked files, repo-relative and slash-separated, sorted (git ls-files). */
  Tree: (path: string) => call<string[] | null>("project.Tree", [path]),
  ReadFile: (path: string, rel: string) => call<string>("project.ReadFile", [path, rel]),
  /** Lines from..to (1-based, inclusive) of one file for the diff's context
   * expander. ref is the revision the diff's new side stands at: "" for the
   * working tree, otherwise a git oid — a local object when the checkout has
   * it, GitHub's copy when it does not (a pull request's head). The backend
   * caps one answer, so a caller wanting more asks again from where it ended. */
  FileLines: (path: string, rel: string, ref: string, from: number, to: number) =>
    call<string[] | null>("project.FileLines", [path, rel, ref, from, to]),
  DiscardFile: (path: string, rel: string) => call<null>("project.DiscardFile", [path, rel]),
  ListBranches: (path: string) => call<Branches>("project.ListBranches", [path]),
  /** The setup script a new worktree of this project will run (or the
   * suggestion to offer when the repo ships none), and the run command its
   * checkouts open a Run card on. */
  WorktreeSetup: (path: string) => call<WorktreeSetup>("project.WorktreeSetup", [path]),
  /** Write .lich/setup-worktree.sh in the project checkout ("" removes it). */
  SaveWorktreeSetup: (path: string, script: string) =>
    call<null>("project.SaveWorktreeSetup", [path, script]),
  /** Write .lich/run-worktree.sh in the project checkout ("" removes it, which
   * is how a project stops offering a Run card). */
  SaveWorktreeRun: (path: string, script: string) =>
    call<null>("project.SaveWorktreeRun", [path, script]),
  /** The logins gh is authenticated as, for the project's account picker;
   * errors when gh is missing or logged out. */
  GitHubAccounts: () => call<string[] | null>("project.GitHubAccounts", []),
  /** Who the next commit in this checkout would be authored as — the fact the
   * account picker does not govern. Empty fields = no identity configured. */
  CommitIdentity: (path: string) => call<CommitIdentity>("project.CommitIdentity", [path]),
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
  /** Which files a conflicting PR collides with its base on — GitHub answers
   * that a pull request conflicts and never where. A network round trip (it
   * fetches both commits), so it is asked only once mergeable says CONFLICTING.
   * `url` is the PR's own: it names the repository the PR lives on, which is not
   * origin in a clone of a fork. */
  PullRequestConflicts: (path: string, number: number, base: string, url: string) =>
    call<string[] | null>("project.PullRequestConflicts", [path, number, base, url]),
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
  /** Replace the PR's description. An empty body clears it. */
  EditPullRequestBody: (path: string, number: number, body: string) =>
    call<null>("project.EditPullRequestBody", [path, number, body]),
  /** Ask a login for a review, or withdraw the request. GitHub refuses one
   * addressed to the PR's own author. */
  RequestReview: (path: string, number: number, login: string, requested: boolean) =>
    call<null>("project.RequestReview", [path, number, login, requested]),
  /** Who this repository allows on a review — the picker's list; capped at 100. */
  AssignableReviewers: (path: string) =>
    call<ReviewCandidate[] | null>("project.AssignableReviewers", [path]),
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
  WorktreeAdopted: (wtPath: string) => call<boolean>("project.WorktreeAdopted", [wtPath]),
}

export const Store = {
  LoadState: () => call<StoredProject[] | null>("store.LoadState", []),
  AddProject: (id: string, name: string, path: string) =>
    call<null>("store.AddProject", [id, name, path]),
  CloseProject: (id: string) => call<null>("store.CloseProject", [id]),
  /** The closed projects offered for reopening, newest first (capped backend-side). */
  RecentProjects: () => call<RecentProject[] | null>("store.RecentProjects", []),
  /** sandbox is whether this session runs confined ("on"/"off", "" to follow the
   * provider's rung). It rides the insert because the PTY reads it on the first
   * spawn — a second call would race the card this one puts on screen. */
  AddSession: (
    projectID: string,
    sessionID: string,
    label: string,
    kind: string,
    path: string,
    nextSeq: number,
    sandbox = "",
  ) => call<null>("store.AddSession", [projectID, sessionID, label, kind, path, nextSeq, sandbox]),
  /**
   * AddSession for a session that was opened by delegation: originID is the
   * session that asked for it and originLabel what that one was called then.
   * The window only ever uses it to re-create a row it already had — an origin
   * is minted backend-side, in internal/spawn.
   */
  AddSessionFrom: (
    projectID: string,
    sessionID: string,
    label: string,
    kind: string,
    path: string,
    nextSeq: number,
    originID: string,
    originLabel: string,
  ) =>
    call<null>("store.AddSessionFrom", [
      projectID,
      sessionID,
      label,
      kind,
      path,
      nextSeq,
      originID,
      originLabel,
    ]),
  DeleteSession: (projectID: string, sessionID: string, activeID: string) =>
    call<null>("store.DeleteSession", [projectID, sessionID, activeID]),
  /** Park a session (keep its row for a later resume) instead of deleting it. */
  CloseSession: (projectID: string, sessionID: string, activeID: string) =>
    call<null>("store.CloseSession", [projectID, sessionID, activeID]),
  /** Resume a parked worktree session under a fresh id, or null when none. */
  ReopenWorktreeSession: (projectID: string, path: string, newSessionID: string) =>
    call<StoredSession | null>("store.ReopenWorktreeSession", [projectID, path, newSessionID]),
  /** The parked sessions, last closed first — the history the palette browses.
   * Capped store-side, so this is also how far back a search can reach. */
  ClosedSessions: () => call<ClosedSession[] | null>("store.ClosedSessions", []),
  /** Resume one parked session by its own id, or null when it is no longer
   * parked — another window resumed it, or its worktree was removed. */
  ReopenSession: (sessionID: string, newSessionID: string) =>
    call<StoredSession | null>("store.ReopenSession", [sessionID, newSessionID]),
  /** Delete one parked session for good. Refused on a session that is open: a
   * card on screen is closed, never forgotten. */
  ForgetSession: (sessionID: string) => call<null>("store.ForgetSession", [sessionID]),
  /** Drop every session row for a worktree path when the worktree is removed. */
  PurgeWorktreeSessions: (projectID: string, path: string) =>
    call<null>("store.PurgeWorktreeSessions", [projectID, path]),
  RenameSession: (sessionID: string, label: string) =>
    call<null>("store.RenameSession", [sessionID, label]),
  /** The command a terminal session opens into; "" clears it back to a plain
   * shell. The store refuses it on anything but a shell session. */
  SetSessionEntrypoint: (sessionID: string, entrypoint: string) =>
    call<null>("store.SetSessionEntrypoint", [sessionID, entrypoint]),
  /** Park a prompt to be typed at a session later, at unix second `at`. A 0
   * time, or an empty prompt, clears whatever was parked — a session holds one
   * scheduled prompt at a time, and scheduling again replaces it. */
  SetSessionSchedule: (sessionID: string, at: number, prompt: string) =>
    call<null>("store.SetSessionSchedule", [sessionID, at, prompt]),
  /** Whether this one session runs confined, overriding the provider's rung for
   * it alone: "on", "off", or "" to follow the setting. Read on every later
   * spawn, so a reload and a resume keep the answer the session opened with. */
  SetSessionSandbox: (sessionID: string, sandbox: string) =>
    call<null>("store.SetSessionSandbox", [sessionID, sandbox]),
  /** Name a session from an automatic source — the provider's ai-title, or a
   * terminal's own entrypoint. A no-op once the user has renamed the card;
   * answers whether the label actually moved. */
  SetSessionTitle: (sessionID: string, title: string) =>
    call<boolean>("store.SetSessionTitle", [sessionID, title]),
  /** The provider conversation id recorded for a session, "" when none. */
  ProviderSession: (sessionID: string) => call<string>("store.ProviderSession", [sessionID]),
  /** Re-attach a provider conversation id to a session row. */
  SetProviderSession: (sessionID: string, providerSessionID: string) =>
    call<null>("store.SetProviderSession", [sessionID, providerSessionID]),
  /** Pin (or unpin) a session: it sorts to the head of its project's list and
   * refuses to close until unpinned. */
  SetSessionPinned: (sessionID: string, pinned: boolean) =>
    call<null>("store.SetSessionPinned", [sessionID, pinned]),
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

export const AgentPlugin = {
  /** One entry per provider that can run the plugin, in display order. */
  Status: () => call<PluginStatus[]>("agentplugin.Status", []),
  Install: (provider: string) => call<null>("agentplugin.Install", [provider]),
  Update: (provider: string) => call<null>("agentplugin.Update", [provider]),
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
  /** Open a session's checkout — the folder itself — in $VISUAL/$EDITOR. Same
   * two answers as OpenInEditor: "" for a detached GUI launch, or a shell
   * command line to run in a terminal session. */
  OpenFolderInEditor: (dir: string) => call<string>("system.OpenFolderInEditor", [dir]),
  /** Show a session's checkout in the platform's file manager. Rejects a path
   * that is no longer a folder, so a card outliving its worktree says so. */
  OpenFolder: (dir: string) => call<null>("system.OpenFolder", [dir]),
  /** Version, platform and log path — the page cannot derive any of the three. */
  Diagnostics: () => call<DiagnosticsData>("system.Diagnostics", []),
  /** Open the log's folder in the platform's file manager, for attaching it. */
  RevealLog: () => call<null>("system.RevealLog", []),
  /** Whether the run before this one ended without closing its window. Reading
   * it clears it, so only the first caller of a launch is told. */
  TakeUncleanExit: () => call<boolean>("system.TakeUncleanExit", []),
  /** Raise a desktop notification: a headline and an optional second line.
   * The caller decides it is warranted — the backend only delivers. */
  Notify: (summary: string, detail: string) => call<null>("system.Notify", [summary, detail]),
  /** What confines a session on this machine — "bubblewrap", "sandbox-exec" —
   * or "" when nothing can. A fact about the machine, not about a provider.
   * The name rather than a yes: the two backends have different holes, and the
   * Sandbox pane says which one is in play. */
  SandboxBackend: () => call<string>("system.SandboxBackend", []),
  /** The identities loaded in the user's ssh agent, one readable line each.
   * Shown beside the setting that hands the agent to a confined session: that
   * setting reads as "let it push with my GitHub key" and actually covers every
   * key in the list. Empty for no agent, no ssh-add, or an agent holding
   * nothing — all three mean the same thing to whoever is deciding, and all
   * three answer null rather than []: Go marshals a nil slice that way, so the
   * type says so and the caller folds it into a list before reading a length
   * off it. */
  SSHAgentKeys: () => call<string[] | null>("system.SSHAgentKeys", []),
}

export const Providers = {
  /** Every known provider with its install state (binary found on PATH). */
  Detect: () => call<DetectedProvider[]>("providers.Detect", []),
  /** Resolve a configured binary the way the spawn does, and report whether it
   * can be run. "" answers `empty` — the layer below is what will be used. */
  Verify: (bin: string) => call<BinaryCheck>("providers.Verify", [bin]),
}

export const Quota = {
  /** Plan usage for every provider that meters a subscription, for the account
   * a session spends — an empty id reads lich's own login, the machine-wide
   * question. Served from a five-minute cache per account, so calling it often
   * is cheap and asks nothing extra of endpoints that rate-limit. */
  Plans: (sessionId: string) => call<QuotaPlan[]>("quota.Plans", [sessionId]),
}

export const Spawn = {
  /** Open the project's Run card in the checkout at cwd: a terminal session
   * whose entrypoint is .lich/run-worktree.sh, its PTY started by the backend
   * so the app is up whether or not the card is ever looked at. Rejects a
   * project that ships no run script. The card arrives through session-opened,
   * like every other session opened outside the window. */
  Run: (projectId: string, cwd: string) => call<null>("spawn.Run", [projectId, cwd]),
}

export const Themes = {
  /** Bundled themes plus user-imported themes from the config dir. */
  List: () => call<ThemeDefinition[] | null>("themes.List", []),
  /** Import a picked theme file, reporting an existing id before replacement. */
  Import: (path: string, overwrite: boolean) =>
    call<ThemeImportResult>("themes.Import", [path, overwrite]),
  /** Clone a theme repository and install every theme its manifest versions. */
  InstallFromGit: (url: string, overwrite: boolean) =>
    call<ThemeGitInstallResult>("themes.InstallFromGit", [url, overwrite]),
  /** Re-clone the repository a theme came from, installing a newer manifest. */
  UpdateFromGit: (id: string) => call<ThemeGitInstallResult>("themes.UpdateFromGit", [id]),
  /** Remove a user-imported theme. Bundled themes are rejected by the backend. */
  Remove: (id: string) => call<null>("themes.Remove", [id]),
  /** Write the bundled starter theme to a destination the user picked. */
  SaveTemplate: (path: string) => call<null>("themes.SaveTemplate", [path]),
}
