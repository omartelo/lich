import { useCallback } from "react"
import { useKeyedStore } from "@/lib/use-keyed-store"
import { type DraftKind, type DraftScope, draftKey, draftStore, setDraft } from "./draft-store"

/** One box of unsent prose, read and written like `useState` but owned outside
 * the tree (draft-store), so a tab switch, a collapsed file, a refetched diff —
 * or a reload — cannot take what was typed. null is "no draft": for the
 * description and the reply box, that is also what closes them.
 *
 * A scope that has not resolved yet, or a reply with no thread behind it, is
 * filed under no key at all rather than under one ending in nothing. */
export function useDraft(
  scope: DraftScope,
  kind: DraftKind,
  id = "",
): [string | null, (next: string | null) => void] {
  const addressed = scope.projectId !== "" && scope.number > 0 && (kind !== "reply" || id !== "")
  const key = addressed ? draftKey(scope, kind, id) : ""
  const draft = useKeyedStore(draftStore, key)
  const set = useCallback(
    (next: string | null) => {
      if (key) {
        setDraft(key, next)
      }
    },
    [key],
  )
  return [draft, set]
}
