import { createKeyedStore } from "@/lib/keyed-store"
import { useKeyedStore } from "@/lib/use-keyed-store"

// Which sessions have text waiting for a prompt that is not free yet
// (write-at-prompt.ts). A handoff is written into a session and left there for
// the user to read and send — so when the prompt is busy, nothing appears and
// nothing failed: the click reads as having done nothing at all, which is how
// the pull request handoff was first reported.
//
// Not fed by an event like the other per-session stores: the wait happens in
// this page, in the loop that polls for the prompt, so the only thing that
// knows about it is the caller.
export const handoffHolds = createKeyedStore<boolean>(false)

// useHandoffHeld is whether this session has a handoff waiting for its prompt.
// False for nearly every card, nearly always.
export function useHandoffHeld(id: string): boolean {
  return useKeyedStore(handoffHolds, id)
}
