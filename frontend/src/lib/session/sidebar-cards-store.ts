// The cards the mounted sidebar draws, for the session shortcuts, which live in
// the projects provider and cannot see what the sidebar hides: the filter is
// the open sidebar's own state, and a fold is read per block.
//
// A reader rather than a published list: folding a block re-renders that block
// alone, so a list published on the sidebar's render would go stale on every
// fold. The sidebar registers how to read its order, and a press reads it then.

import type { SidebarCard } from "./sidebar-groups"

let read: () => SidebarCard[] = () => []

// registerSidebarCards makes `reader` the order the shortcuts walk, and returns
// the unregister for an effect's cleanup. Only one sidebar is mounted at a time
// (the open list or its rail), so the latest registration is the one on screen.
export function registerSidebarCards(reader: () => SidebarCard[]): () => void {
  read = reader
  return () => {
    if (read === reader) {
      read = () => []
    }
  }
}

export function readSidebarCards(): SidebarCard[] {
  return read()
}
