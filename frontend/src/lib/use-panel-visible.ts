import { useState } from "react"
import { readPref, writePref } from "@/lib/prefs"

// What a hidden panel stores. Not a boolean through writePref: these keys are
// named "…hidden" and already hold "1" in every install, so the encoding is
// kept rather than migrated for a cosmetic gain.
const HIDDEN = "1"
const SHOWN = "0"

// A navigator panel the user can collapse — the pull request list, the changed
// files tree. Both are a way into the thing being reviewed rather than the
// review itself, so hiding one hands its width to the diff, and the choice
// survives leaving the screen (localStorage, like the panel widths beside it).
//
// Shown is the default: a key nobody has written yet means a user who has
// never hidden the panel.
export function usePanelVisible(storageKey: string): [boolean, () => void] {
  const [visible, setVisible] = useState(() => readPref(storageKey) !== HIDDEN)
  const toggle = () => {
    setVisible((wasVisible) => {
      writePref(storageKey, wasVisible ? HIDDEN : SHOWN)
      return !wasVisible
    })
  }
  return [visible, toggle]
}
