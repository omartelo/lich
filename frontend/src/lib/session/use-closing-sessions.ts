import { useEffect, useState } from "react"
import type { Session } from "./sessions"

// How long a closed card stays mounted, animating out. The card's exit
// transition is timed from this same constant, so the two cannot drift: a
// shorter transition would leave a finished card sitting in the list, a longer
// one would cut its animation off mid-way.
export const EXIT_MS = 180

/** A session that has left the list, and the slot it left behind. */
export interface Closing {
  session: Session
  index: number
}

/** The sessions of `previous` that `live` no longer holds, with their slots. */
export function departed(previous: Session[], live: Session[]): Closing[] {
  return previous
    .map((session, index) => ({ session, index }))
    .filter(({ session }) => !live.some((other) => other.id === session.id))
}

/** `live` with each closing session put back where it was. */
export function withClosing(live: Session[], closing: Closing[]): Session[] {
  const shown = [...live]
  for (const { session, index } of [...closing].sort((a, b) => a.index - b.index)) {
    shown.splice(Math.min(index, shown.length), 0, session)
  }
  return shown
}

// useClosingSessions keeps a session on screen for the length of its exit
// animation after it has left the list. Closing a session drops it from state
// in the same frame its card would animate, and no CSS can animate an element
// that is no longer rendered — holding it here is what buys those frames, and
// it buys them for every way a session can leave: the card's menu, the hotkey,
// the MCP tool, a session that died on its own.
//
// The departure is caught while rendering rather than from an effect, which is
// what makes the card animate at all: an effect commits one paint with the card
// already gone, and the card React then puts back is a new element that starts
// collapsed instead of collapsing. Caught here, the very render that drops the
// session is the render that marks its card closing, so the transition starts
// from the card as it stands.
export function useClosingSessions(sessions: Session[]): {
  shown: Session[]
  closing: Set<string>
} {
  const [held, setHeld] = useState<Closing[]>([])
  const [tracked, setTracked] = useState(() => ({ ids: idsOf(sessions), list: sessions }))

  const ids = idsOf(sessions)
  if (tracked.ids !== ids) {
    const gone = departed(tracked.list, sessions)
    setTracked({ ids, list: sessions })
    if (gone.length > 0) {
      setHeld((current) => [...current, ...gone])
    }
  }

  // Timed from the frame the browser starts the transition on rather than from
  // the commit that asked for it: the two are a paint apart, and a card removed
  // on the earlier clock is one that pops out of its own last frames.
  useEffect(() => {
    if (held.length === 0) {
      return
    }
    let timer = 0
    const frame = requestAnimationFrame(() => {
      timer = window.setTimeout(() => setHeld([]), EXIT_MS)
    })
    return () => {
      cancelAnimationFrame(frame)
      clearTimeout(timer)
    }
  }, [held])

  // A session can come back while its card is still going (a close undone), and
  // then the live list is the one that counts.
  const closing = held.filter(({ session }) => !sessions.some((live) => live.id === session.id))
  if (closing.length === 0) {
    return { shown: sessions, closing: NONE }
  }
  return {
    shown: withClosing(sessions, closing),
    closing: new Set(closing.map(({ session }) => session.id)),
  }
}

function idsOf(sessions: Session[]): string {
  return sessions.map((session) => session.id).join("\u0000")
}

const NONE: Set<string> = new Set()
