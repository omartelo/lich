// RPC client for the Go services over the loopback listener (internal/rpc).
// The page is served by that listener, so the endpoint rides the page URL:
// ?token=... (auth) and, under the Vite dev server only, &backend=<port> —
// dev splits the page origin from the RPC listener (see `task dev`).
//
// Each facade mirrors its Go service's method names and signatures; shapes
// live in lib/api-types.ts.

import type {
  Branches,
  DiffStats,
  PluginStatus,
  Project,
  PullRequest,
  StoredProject,
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
      throw new Error("no backend endpoint in page URL (missing ?token=) — launch through the lich binary")
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
