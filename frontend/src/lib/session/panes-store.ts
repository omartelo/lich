import { useSyncExternalStore } from "react"
import { prefKeys, readPref, removePref, writePref } from "@/lib/prefs"
import {
  formatGroups,
  GROUPS_KEY,
  LEGACY_GROUPS_PREFIX,
  mergeGroups,
  type PaneGroup,
  parseGroups,
} from "./panes"

// Where the split groups live between mounts. localStorage is the store itself,
// not a cache in front of one: a second copy in module state would be one more
// thing that can disagree with the pref it was written from.
//
// UI preference, so the page's localStorage rather than the workspace database
// (root CLAUDE.md) — a wall is a property of this window, the way the sidebar's
// width and the dock's tab are.
//
// One key for the whole window: a wall can hold sessions of several projects,
// so it belongs to none of them.
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
let cache: { raw: string | null; value: PaneGroup[] } | null = null

// Walls used to be stored one key per project. The first read of a window that
// still has those carries them into the one key and drops the old ones, so no
// wall is lost to the upgrade. It runs inside a read because every path to the
// walls starts with one, and it is idempotent: once the old keys are gone there
// is nothing left for a second call to do. The flag keeps every later snapshot
// from walking the whole of localStorage to find that out.
let migrated = false

function migrateLegacyGroups(): void {
  if (migrated) {
    return
  }
  migrated = true
  const legacy = prefKeys(LEGACY_GROUPS_PREFIX)
  if (legacy.length === 0) {
    return
  }
  const carried = legacy.map((key) => parseGroups(readPref(key)))
  writePref(GROUPS_KEY, formatGroups(mergeGroups([parseGroups(readPref(GROUPS_KEY)), ...carried])))
  for (const key of legacy) {
    removePref(key)
  }
}

/** The stored groups, unreconciled — put them through resolveGroups with the
 * workspace's live sessions before drawing anything. A component takes
 * useStoredGroups instead: this read holds no listener, so what it returns goes
 * stale the moment a pane moves. */
export function storedGroups(): PaneGroup[] {
  migrateLegacyGroups()
  const raw = readPref(GROUPS_KEY)
  if (cache && cache.raw === raw) {
    return cache.value
  }
  cache = { raw, value: parseGroups(raw) }
  return cache.value
}

export function useStoredGroups(): PaneGroup[] {
  return useSyncExternalStore(subscribe, storedGroups)
}

export function writeGroups(groups: readonly PaneGroup[]): void {
  writePref(GROUPS_KEY, formatGroups(groups))
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
