import { parseBoolPref, readPref, removePref, writePref } from "@/lib/prefs"

// Which of the sidebar's session blocks are folded. A fold is not a view state:
// somebody working across several worktrees folds the ones they are not in, and
// a fold that unfolds itself on the next project switch — or on collapsing the
// sidebar to its rail, which unmounts the whole list — is a fold typed again.
//
// UI preferences, so the page's localStorage rather than the workspace database
// (see the root CLAUDE.md). Parsing is the pure half prefs.ts owns; reading and
// writing the key stays the two-line wrapper it describes.
//
// Scoped by project *and* group key. The key alone would collide: a worktree
// block is keyed by its absolute checkout path and no two projects can share
// one, but ROOT_GROUP_KEY and PINNED_GROUP_KEY are stand-ins every project has,
// so folding one project's pinned block would fold every project's. The project
// id is hex (internal/project.projectID), so the separator is unambiguous.
const COLLAPSED_PREFIX = "lich.sidebar.collapsed."

export function readGroupCollapsed(projectId: string, groupKey: string): boolean {
  return parseBoolPref(readPref(`${COLLAPSED_PREFIX}${projectId}:${groupKey}`), false)
}

export function writeGroupCollapsed(projectId: string, groupKey: string, collapsed: boolean): void {
  const key = `${COLLAPSED_PREFIX}${projectId}:${groupKey}`
  // Unfolded is the default, so it is stored by not being stored: what is left
  // behind names exactly the blocks that are folded, and a worktree removed
  // while unfolded leaves nothing behind at all.
  if (collapsed) {
    writePref(key, true)
  } else {
    removePref(key)
  }
}
