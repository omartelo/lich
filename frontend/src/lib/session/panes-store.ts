import { useSyncExternalStore } from "react"
import { readPref, removePref, writePref } from "@/lib/prefs"
import { COLS_KEY, ROWS_KEY, stageKey } from "./panes"

// Where the stage lives between mounts. localStorage is the store itself, not a
// cache in front of one: the values are a list of ids, an index and two vectors
// of fractions, reads are a map lookup, and a second copy in module state would
// be one more thing that can disagree with the pref it was written from.
//
// UI preference, so the page's localStorage rather than the workspace database
// (root CLAUDE.md) — the stage is a property of this window, the way the
// sidebar's width and the dock's tab are.
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
// forever, and every read here parses a string into a fresh array. So each key's
// last parse is kept beside the raw string it came from and handed back until
// that string changes — the cache is for React's identity check, never for the
// value, which still comes from the pref on every read.
function parsed<T>(parse: (raw: string | null) => T): (key: string) => T {
  const seen = new Map<string, { raw: string | null; value: T }>()
  return (key: string) => {
    const raw = readPref(key)
    const last = seen.get(key)
    if (last && last.raw === raw) {
      return last.value
    }
    const value = parse(raw)
    seen.set(key, { raw, value })
    return value
  }
}

const readList = parsed<string[]>((raw) => (raw ? raw.split(",").filter((id) => id !== "") : []))

const readNumbers = parsed<number[]>((raw) =>
  raw ? raw.split(",").map(Number).filter(Number.isFinite) : [],
)

/** The stored cells outside React, for the two readers that are not components:
 * the collapsed rail and the neighbour walk, both of which have to order cards
 * the way the sidebar draws them. */
export function storedStage(projectId: string): string[] {
  return projectId ? readList(stageKey(projectId)) : EMPTY
}

/** The stored cells, unreconciled — put them through resolveStage with the
 * project's live sessions before drawing anything. */
export function useStoredStage(projectId: string): string[] {
  return useSyncExternalStore(subscribe, () => (projectId ? readList(stageKey(projectId)) : EMPTY))
}

// One frozen array for every project with nothing stored, so the snapshot of an
// empty stage is identity-stable too.
const EMPTY: string[] = []

export function useStoredCols(): number[] {
  return useSyncExternalStore(subscribe, () => readNumbers(COLS_KEY))
}

export function useStoredRows(): number[] {
  return useSyncExternalStore(subscribe, () => readNumbers(ROWS_KEY))
}

// The stage's measured size, kept here rather than passed around: the element
// is measured in the terminal host and the guard that needs it — would one more
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

export function writeStage(projectId: string, cells: readonly string[]): void {
  if (!projectId) {
    return
  }
  if (cells.length <= 1) {
    // One pane is no wall at all: leaving a single id stored would keep bringing
    // that arrangement back for a session that is simply the active one.
    removePref(stageKey(projectId))
  } else {
    writePref(stageKey(projectId), cells.join(","))
  }
  notify()
}

export function writeCols(cols: readonly number[]): void {
  writePref(COLS_KEY, cols.join(","))
  notify()
}

export function writeRows(rows: readonly number[]): void {
  writePref(ROWS_KEY, rows.join(","))
  notify()
}
