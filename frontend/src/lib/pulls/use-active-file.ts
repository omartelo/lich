import { useState } from "react"
import { readActiveFile, writeActiveFile } from "./pulls-prefs"

// The changed-files tree's selection, remembered across the tab switch that
// destroys it. Every other tab of the pull request screen replaces the Files
// tab outright, so walking to Conversation and back used to forget which file
// the review had reached, on a screen whose whole job is to be walked away from.
//
// The pull request is a prop, not a constant: the Files tab is not remounted
// when the list column moves to another pull request, so the seed has to be
// re-read when it does. That is done during render and held in state — a ref
// would be written by a render React discarded and replayed, and the replay
// would skip the re-read (the trap `use-remote-resource.ts` documents).
export function useActiveFile(pullRequest: string): [string | null, (path: string) => void] {
  const [active, setActive] = useState(() => readActiveFile(pullRequest))
  const [seen, setSeen] = useState(pullRequest)
  if (seen !== pullRequest) {
    setSeen(pullRequest)
    setActive(readActiveFile(pullRequest))
  }
  // The write sits beside the setter rather than inside it: React treats an
  // updater as pure and calls it twice under StrictMode.
  const select = (path: string) => {
    writeActiveFile(pullRequest, path)
    setActive(path)
  }
  // "" is how the store spells "nothing marked"; the tree spells it null.
  return [active || null, select]
}
