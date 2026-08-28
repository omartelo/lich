// The screen each project was last showing, so its tab returns to it instead of
// to the project's terminals: switching to another project and back is a glance
// away, not a decision to leave Settings — or a pull request — behind.
//
// Keyed by project id and never persisted, like the parked cards these screens
// belong to (card-store): a restart starts on the terminals.
const routes = new Map<string, string>()

export function rememberProjectRoute(projectId: string, path: string): void {
  routes.set(projectId, path)
}

// The route a project's tab opens: the screen it was last on, falling back to
// the project itself — whichever session it has active.
export function projectRoute(projectId: string): string {
  return routes.get(projectId) ?? `/projects/${projectId}`
}
