import { useSyncExternalStore } from "react"
import { readPref, writePref } from "@/lib/prefs"
import { formatGroups, groupsKey, type PaneGroup, parseGroups } from "./panes"

// Where the split groups live between mounts. localStorage is the store itself,
// not a cache in front of one: a second copy in module state would be one more
// thing that can disagree with the pref it was written from.
//
// UI preference, so the page's localStorage rather than the workspace database
// (root CLAUDE.md) — a wall is a property of this window, the way the sidebar's
// width and the dock's tab are.
//
// One listener set rather than one per project: the subscribers are the terminal
// host, the sidebar and the layout's shortcuts, and a fan-out keyed by project
// would be machinery for three of them.
const listeners = new Set<() => void>()

function notify(): void {
  for (const listener of listeners) {
    listener()
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

// Snapshots must be stable across calls or useSyncExternalStore re-renders
// forever, and every read here parses a string into fresh objects. So the last
// parse is kept beside the raw string it came from and handed back until that
// string changes — the cache is for React's identity check, never for the value,
// which still comes from the pref on every read.
const cache = new Map<string, { raw: string | null; value: PaneGroup[] }>()

/** The stored groups, unreconciled — put them through resolveGroups with the
 * project's live sessions before drawing anything. Outside React too, for the
 * neighbour walk, which orders cards the way the sidebar draws them. A component
 * takes useStoredGroups instead: this read holds no listener, so what it returns
 * goes stale the moment a pane moves. */
export function storedGroups(projectId: string): PaneGroup[] {
  if (!projectId) {
    return EMPTY
  }
  const key = groupsKey(projectId)
  const raw = readPref(key)
  const last = cache.get(key)
  if (last && last.raw === raw) {
    return last.value
  }
  const value = parseGroups(raw)
  cache.set(key, { raw, value })
  return value
}

// One frozen array for every project with nothing stored, so the snapshot of a
// project with no walls is identity-stable too.
const EMPTY: PaneGroup[] = []

export function useStoredGroups(projectId: string): PaneGroup[] {
  return useSyncExternalStore(subscribe, () => storedGroups(projectId))
}

export function writeGroups(projectId: string, groups: readonly PaneGroup[]): void {
  if (!projectId) {
    return
  }
  writePref(groupsKey(projectId), formatGroups(groups))
  notify()
}

// The stage's measured size, kept here rather than passed around: the element is
// measured in the terminal host and the guard that needs it — would one more
// pane still be readable — is asked from the layout's shortcuts, two subtrees
// away. Not reactive, and deliberately: nothing renders from it, one caller asks
// it a yes-or-no question at the moment a key is pressed.
let stage = { width: 0, height: 0 }

export function setStageSize(width: number, height: number): void {
  stage = { width, height }
}

export function stageSize(): { width: number; height: number } {
  return stage
}
