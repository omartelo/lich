import { useEffect, useState } from "react"
import { PatchNotesDialog } from "./PatchNotesDialog"
import { decidePatchNotes, PATCH_NOTES_SEEN_KEY } from "@/lib/update/patch-notes-gate"
import { PatchNotes } from "@/lib/rpc"
import { readPref, writePref } from "@/lib/prefs"
import type { PatchNotes as PatchNotesData } from "@/lib/api-types"

// PatchNotesGate shows lich's "what's new" popup once per release, right after
// an update. On startup it fetches this build's changelog section and decides:
// show the popup, silently record the version (first run — no popup on a fresh
// install), or nothing. Any failure is silent — it must never block startup.
export function PatchNotesGate() {
  const [notes, setNotes] = useState<PatchNotesData | null>(null)

  useEffect(() => {
    let alive = true
    void PatchNotes.Current()
      .then((current) => {
        if (!alive) return
        const action = decidePatchNotes(current, readPref(PATCH_NOTES_SEEN_KEY))
        if (action.kind === "record") {
          writePref(PATCH_NOTES_SEEN_KEY, action.version)
        } else if (action.kind === "show") {
          setNotes(action.notes)
        }
      })
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [])

  if (!notes) return null

  const close = () => {
    writePref(PATCH_NOTES_SEEN_KEY, notes.version)
    setNotes(null)
  }

  return <PatchNotesDialog notes={notes} onClose={close} />
}
