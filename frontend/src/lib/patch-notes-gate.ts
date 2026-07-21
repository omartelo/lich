// Startup decision for the "what's new" popup: show this build's changelog
// section, silently remember it, or do nothing. Kept pure (no bindings, no
// storage) so it is trivially testable; the component wires it to Current(),
// localStorage and the dialog. Mirrors app-update-gate.ts.

import type {PatchNotes} from "./api-types"

// Stores the version whose notes were last shown (or silently recorded on first
// run), so a given release's popup fires exactly once.
export const PATCH_NOTES_SEEN_KEY = "lich.patchNotesSeen"

// PatchNotesAction is what the gate should do:
// - show: open the popup for this version's notes.
// - record: remember this version without a popup — the first run that ever
//   sees the feature (or a fresh install), so users are not greeted on launch.
// - none: nothing new to show.
export type PatchNotesAction =
  | {kind: "none"}
  | {kind: "record"; version: string}
  | {kind: "show"; version: string; notes: PatchNotes}

// decidePatchNotes resolves the startup popup. No section (dev build, or a
// version absent from the changelog) → none. Never recorded before → record
// silently. Same version already seen → none. A different version with notes →
// show.
export function decidePatchNotes(
  notes: PatchNotes,
  lastSeenVersion: string | null,
): PatchNotesAction {
  if (!notes.version || !notes.groups || notes.groups.length === 0) {
    return {kind: "none"}
  }
  if (lastSeenVersion === null) {
    return {kind: "record", version: notes.version}
  }
  if (lastSeenVersion === notes.version) {
    return {kind: "none"}
  }
  return {kind: "show", version: notes.version, notes}
}
