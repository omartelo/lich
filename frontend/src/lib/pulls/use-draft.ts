import { useCallback } from "react"
import { useKeyedStore } from "@/lib/use-keyed-store"
import { draftStore, type DraftKind, draftKey } from "./draft-store"

/** One box of unsent prose, read and written like `useState` but owned outside
 * the tree (draft-store), so a tab switch, a collapsed file or a refetched diff
 * cannot take what was typed. null is "no draft": for the description and the
 * reply box, that is also what closes them. */
export function useDraft(
  kind: DraftKind,
  id: string,
): [string | null, (next: string | null) => void] {
  const key = id ? draftKey(kind, id) : ""
  const draft = useKeyedStore(draftStore, key)
  const set = useCallback(
    (next: string | null) => {
      if (key) {
        draftStore.set(key, next)
      }
    },
    [key],
  )
  return [draft, set]
}
