// Tracks which projects have their Settings card open in the sidebar. This is a
// pure UI concern that is never persisted: the settings screen itself is
// route-driven, this only remembers that a project's Settings card should stay
// parked in its session list (inactive) while the user works in a terminal, so
// it can be clicked back to. A restart starts with no cards open — settings are
// reopened on demand, the stored workspace is untouched.
//
// Module-level store + a subscribe/getSnapshot pair for useSyncExternalStore,
// matching the app's no-state-library pattern.
const openIds = new Set<string>()
const listeners = new Set<() => void>()

function emit(): void {
  for (const listener of listeners) {
    listener()
  }
}

/** Open a project's Settings card. No-op (and no notification) if already open. */
export function openSettings(projectId: string): void {
  if (openIds.has(projectId)) {
    return
  }
  openIds.add(projectId)
  emit()
}

/** Close a project's Settings card. No-op (and no notification) if not open. */
export function closeSettings(projectId: string): void {
  if (!openIds.delete(projectId)) {
    return
  }
  emit()
}

export function isSettingsOpen(projectId: string): boolean {
  return openIds.has(projectId)
}

export function subscribeSettingsCard(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}
