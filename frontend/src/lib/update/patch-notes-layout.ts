// How the What's new dialog lays a release out. Kept pure (no React) so the
// splits are testable; PatchNotesDialog renders what these return.

import type { PatchNotesHighlight } from "@/lib/api-types"

// A changelog item split into the bold lead-in it opens with and the paragraph
// behind it. Every entry since 0.40 opens with a bold sentence, so the lead-in
// is the row and the body opens on click; an item without one is its own row.
export interface ItemParts {
  headline: string
  body: string
}

export function splitLeadIn(item: string): ItemParts {
  if (!item.startsWith("**")) return { headline: item, body: "" }
  const close = item.indexOf("**", 2)
  if (close === -1) return { headline: item, body: "" }
  return {
    headline: item.slice(2, close).trim(),
    body: item.slice(close + 2).trim(),
  }
}

// The dialog's opening: the first "important" block is the headline region,
// everything else (a second important included) is a callout under it. Two
// headlines would fight; the second demoting to a callout is the author's
// signal to fold it into the groups.
export interface HighlightLayout {
  lead: PatchNotesHighlight | null
  callouts: PatchNotesHighlight[]
}

export function layoutHighlights(highlights: PatchNotesHighlight[] | null): HighlightLayout {
  const all = highlights ?? []
  const leadIndex = all.findIndex((h) => h.kind === "important")
  if (leadIndex === -1) return { lead: null, callouts: all }
  return {
    lead: all[leadIndex],
    callouts: all.filter((_, i) => i !== leadIndex),
  }
}
