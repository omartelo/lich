import { useEffect, useState } from "react"
import { Terminal } from "@/lib/rpc"
import { MESSAGE_MIN_QUERY, paletteMessages, type PaletteMessage } from "./command-palette"
import type { PaletteSession } from "./command-palette"

// A search reads a slice of every session's transcript off disk, so it waits
// for the typing to settle rather than firing on the keystroke. Long enough to
// skip the letters of a word, short enough that the rows land while the word is
// still being read.
const DEBOUNCE_MS = 200

// useTranscriptSearch asks the backend which of `sessions` have talked about
// `query` and returns them as palette rows. Idle — no rows, no call — while the
// palette is closed or the query is too short to search for.
//
// Every effect run supersedes the one before it: the timer is cleared, and a
// reply that lands after the query moved on is dropped rather than painted over
// newer rows. A failed call reads as no rows; the palette's other two groups are
// unaffected, which is the right degradation for a search that only ever adds.
export function useTranscriptSearch(
  query: string,
  sessions: readonly PaletteSession[],
  enabled: boolean,
): PaletteMessage[] {
  const [rows, setRows] = useState<PaletteMessage[]>([])

  useEffect(() => {
    if (!enabled || query.trim().length < MESSAGE_MIN_QUERY) {
      setRows([])
      return
    }
    let cancelled = false
    const timer = window.setTimeout(() => {
      Terminal.SearchTranscripts(
        sessions.map((session) => session.sessionId),
        query,
      )
        .then((matches) => {
          if (!cancelled) {
            setRows(paletteMessages(matches, sessions))
          }
        })
        .catch(() => {
          if (!cancelled) {
            setRows([])
          }
        })
    }, DEBOUNCE_MS)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [query, sessions, enabled])

  return rows
}
