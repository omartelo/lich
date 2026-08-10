import { useEffect, useState } from "react"
import { PatchNotesDialog } from "./PatchNotesDialog"
import {
  decidePatchNotes,
  LEGACY_PATCH_NOTES_SEEN_KEY,
  PATCH_NOTES_SEEN_KEY,
} from "@/lib/update/patch-notes-gate"
import { PatchNotes, Store } from "@/lib/rpc"
import { readPref } from "@/lib/prefs"
import type { PatchNotes as PatchNotesData } from "@/lib/api-types"

// The setting is the app's, not a project's — it answers for this install.
const GLOBAL_SCOPE = ""

// PatchNotesGate shows lich's "what's new" popup once per release, right after
// an update. On startup it fetches this build's changelog section and decides:
// show the popup, silently record the version (first run — no popup on a fresh
// install), or nothing. Any failure is silent — it must never block startup.
export function PatchNotesGate() {
  const [notes, setNotes] = useState<PatchNotesData | null>(null)

  useEffect(() => {
    let alive = true
    void Promise.all([PatchNotes.Current(), Store.GetSetting(PATCH_NOTES_SEEN_KEY, GLOBAL_SCOPE)])
      .then(([current, seen]) => {
        if (!alive) return
        // A setting that was never written reads as "": the gate wants null for
        // it, so a first run records rather than shows.
        const action = decidePatchNotes(current, seen || readPref(LEGACY_PATCH_NOTES_SEEN_KEY))
        if (action.kind === "record") {
          record(action.version)
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
    record(notes.version)
    setNotes(null)
  }

  return <PatchNotesDialog notes={notes} onClose={close} />
}

// record remembers the version whose notes have been seen. A failed write only
// costs the popup one extra appearance, so it stays silent like the read.
function record(version: string): void {
  void Store.SetSetting(PATCH_NOTES_SEEN_KEY, GLOBAL_SCOPE, version).catch(() => {})
}
