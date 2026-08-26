import { createKeyedStore } from "@/lib/keyed-store"
import { useKeyedStore } from "@/lib/use-keyed-store"

// Where the Code tab's browse was left: the filter box, the folders opened to
// reach a file, the row that reads as current, and the file the preview is
// covering the tree with.
//
// It lives out here because the panel does not. RightDock swaps one component
// type for the other between its tabs, so flipping over to read a diff — and
// closing the dock at all — unmounts the browser and everything it held: the
// filter emptied, the tree collapsed back to the root, the preview gone.
//
// Keyed by the checkout path, which is the rule the panel already had: a
// worktree switch shows that checkout's own browse and never the previous
// one's. Coming back to the first checkout is the part that is new — it used to
// clear, and now it is where it was left.
//
// In memory only, like the comment batch next door: every entry here is a
// position in a tree that is re-read on each mount, and outliving a reload
// would mean pointing at files that have since moved.
export interface FileBrowse {
  /** The filter box, verbatim — it is free text, so anything held is valid. */
  query: string
  /** The file the preview covers the tree with, or "" for the tree itself. */
  open: string
  /** The row that reads as current. It outlives the preview: after Back the
   * tree still marks the file just read, which is where the eye left off. */
  selected: string
  /** The folders FileTree has toggled away from its default (see FileTree). */
  toggled: ReadonlySet<string>
}

const NO_BROWSE: FileBrowse = { query: "", open: "", selected: "", toggled: new Set() }

const store = createKeyedStore<FileBrowse>(NO_BROWSE)

/** Move part of a checkout's browse, leaving the rest of it where it is. A
 * panel with no checkout yet writes nothing rather than filing everyone's
 * browse under one empty key. */
export function updateFileBrowse(path: string, patch: Partial<FileBrowse>): void {
  if (!path) {
    return
  }
  store.set(path, { ...store.get(path), ...patch })
}

/** How this checkout was left browsing, all defaults for one never browsed. */
export function fileBrowse(path: string): FileBrowse {
  return store.get(path)
}

/** The same, subscribed: the panel re-renders when this checkout's browse moves
 * and stays put while any other one does. */
export function useFileBrowse(path: string): FileBrowse {
  return useKeyedStore(store, path)
}
