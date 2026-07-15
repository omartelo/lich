// RPC client for the Go services over the loopback listener (internal/rpc),
// replacing the Wails binding bridge so the same bundle runs in the Wails
// webview and the Chromium --app shell (docs/chromium-shell.md).
//
// Endpoint discovery is the only shell-specific part:
// - Chromium shell: the page is served BY the listener — same origin, token
//   arrives in the page URL (?token=...).
// - Wails webview: one binding call (terminal Service.Transport) bootstraps
//   port + token; every later call goes over HTTP.
//
// Each facade mirrors its binding's method names and signatures, so a call
// site swaps only the import. Types come from the generated binding models
// (type-only imports, erased at build).

import { Service as TerminalBinding } from "../../bindings/github.com/omartelo/lich/internal/terminal"
import type {
  Branches,
  DiffStats,
  Project,
  PullRequest,
  Worktree,
} from "../../bindings/github.com/omartelo/lich/internal/project/models"
import type { Project as StoredProject } from "../../bindings/github.com/omartelo/lich/internal/store/models"
import type { Status as PluginStatus } from "../../bindings/github.com/omartelo/lich/internal/claudeplugin/models"

export interface Endpoint {
  base: string
  token: string
}

let cached: Promise<Endpoint> | null = null

// endpointFromLocation reads the Chromium-shell coordinates off the page URL.
// Exported for tests; production callers use endpoint().
export function endpointFromLocation(href: string): Endpoint | null {
  try {
    const url = new URL(href)
    const token = url.searchParams.get("token")
    if (!token || !url.host) {
      return null
    }
    return { base: `${url.protocol}//${url.host}`, token }
  } catch {
    return null
  }
}

export function endpoint(): Promise<Endpoint> {
  return (cached ??= (async () => {
    const fromUrl = endpointFromLocation(window.location.href)
    if (fromUrl) {
      return fromUrl
    }
    const info = await TerminalBinding.Transport()
    return { base: `http://127.0.0.1:${info.port}`, token: info.token }
  })())
}

async function call<T>(method: string, args: unknown[]): Promise<T> {
  const { base, token } = await endpoint()
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

export const Terminal = {
  Start: (id: string, projectID: string, cwd: string, kind: string, cols: number, rows: number) =>
    call<null>("terminal.Start", [id, projectID, cwd, kind, cols, rows]),
  Write: (id: string, data: string) => call<null>("terminal.Write", [id, data]),
  Resize: (id: string, cols: number, rows: number) =>
    call<null>("terminal.Resize", [id, cols, rows]),
  SetVisible: (id: string, visible: boolean) =>
    call<null>("terminal.SetVisible", [id, visible]),
  Close: (id: string) => call<null>("terminal.Close", [id]),
}

export const ProjectService = {
  Open: () => call<Project | null>("project.Open", []),
  PickFile: () => call<string>("project.PickFile", []),
  Branch: (path: string) => call<string>("project.Branch", [path]),
  Diff: (path: string) => call<DiffStats>("project.Diff", [path]),
  DiffText: (path: string) => call<string>("project.DiffText", [path]),
  DiscardFile: (path: string, rel: string) => call<null>("project.DiscardFile", [path, rel]),
  ListBranches: (path: string) => call<Branches>("project.ListBranches", [path]),
  PullRequest: (path: string) => call<PullRequest | null>("project.PullRequest", [path]),
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
  RenameSession: (sessionID: string, label: string) =>
    call<null>("store.RenameSession", [sessionID, label]),
  SetActiveSession: (projectID: string, sessionID: string) =>
    call<null>("store.SetActiveSession", [projectID, sessionID]),
  ReorderProjects: (ids: string[]) => call<null>("store.ReorderProjects", [ids]),
  ReorderSessions: (projectID: string, ids: string[]) =>
    call<null>("store.ReorderSessions", [projectID, ids]),
  GetSetting: (key: string, projectID: string) => call<string>("store.GetSetting", [key, projectID]),
  SetSetting: (key: string, projectID: string, value: string) =>
    call<null>("store.SetSetting", [key, projectID, value]),
  ClaudeBin: (projectID: string) => call<string>("store.ClaudeBin", [projectID]),
}

export const Fonts = {
  List: () => call<string[] | null>("fonts.List", []),
}

export const ClaudePlugin = {
  Status: () => call<PluginStatus>("claudeplugin.Status", []),
  Install: () => call<null>("claudeplugin.Install", []),
  Update: () => call<null>("claudeplugin.Update", []),
}

export const System = {
  OpenExternal: (url: string) => call<null>("system.OpenExternal", [url]),
}
