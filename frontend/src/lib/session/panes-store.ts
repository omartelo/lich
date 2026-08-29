import { useSyncExternalStore } from "react"
import { parseNumberPref, readPref, removePref, writePref } from "@/lib/prefs"
import { type Beside, clampRatio, formatBeside, paneKey, RATIO_DEFAULT, RATIO_KEY } from "./panes"

// Where the split lives between mounts. localStorage is the store itself, not a
// cache in front of one: both values are a string and a number, reads are a map
// lookup, and a second copy in module state would be one more thing that can
// disagree with the pref it was written from.
//
// UI preference, so the page's localStorage rather than the workspace database
// (root CLAUDE.md) — the split is a property of this window, the way the
// sidebar's width and the dock's tab are.
//
// One listener set rather than one per project: the subscribers are the
// terminal host, the sidebar and the layout's shortcuts, and a fan-out keyed by
// project would be machinery for three of them.
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

/** The stored beside id, unreconciled — pass it through resolveBeside with the
 * project's live sessions before showing anything. */
export function useStoredBeside(projectId: string): string {
  return useSyncExternalStore(subscribe, () =>
    projectId ? (readPref(paneKey(projectId)) ?? "") : "",
  )
}

export function usePaneRatio(): number {
  return useSyncExternalStore(subscribe, () =>
    parseNumberPref(readPref(RATIO_KEY), RATIO_DEFAULT, clampRatio),
  )
}

export function openBeside(projectId: string, beside: Beside): void {
  if (!projectId || !beside.id) {
    return
  }
  writePref(paneKey(projectId), formatBeside(beside))
  notify()
}

export function closeBeside(projectId: string): void {
  if (!projectId) {
    return
  }
  removePref(paneKey(projectId))
  notify()
}

export function setPaneRatio(ratio: number): void {
  writePref(RATIO_KEY, clampRatio(ratio))
  notify()
}
