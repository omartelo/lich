import { useCallback, useState } from "react"
import { Store } from "@/lib/rpc"
import { useRemoteResource } from "@/lib/use-remote-resource"

// useStoredSetting is one stored settings string in one scope: read when the
// pane opens, written through on every change, and remembered across the
// screen's own unmount so the second visit paints the value instead of the
// fallback.
//
// The read is the sequence-guarded one rather than a bare effect, and that is
// not a preference. Settings renders every provider's section at the same
// position, so React keeps one instance and the key changes underneath it — two
// lookups in flight can answer out of order, and a stale answer here is not a
// stale readout but the previous provider's path written into this provider's
// key by the next keystroke. The scope is part of the request for the same
// reason.
//
// `fallback` is what the value reads as until the store answers, and what it
// keeps reading as for a key nobody has set. It is always the answer lich has
// behaved like all along: this hook backs the switches that confine a session
// and hand an agent a credential, and the unknown state must never be drawn as
// the permissive one.
//
// An undefined scope is a layer this pane does not have — the hub has no
// project — and reads as the fallback without touching the store.
export function useStoredSetting(key: string, scope: string | undefined, fallback = "") {
  const request = scope === undefined ? "" : `${key}\n${scope}`
  const { data } = useRemoteResource(request, () => Store.GetSetting(key, scope ?? ""), {
    empty: fallback,
    resetOn: request,
    // The request itself, which is the whole of what this answer is about: one
    // key in one scope, and nothing that merely dates it. Two panes reading the
    // same key in the same scope *are* asking the same question.
    cache: `settings.value ${request}`,
  })
  // What was typed, tagged with the request it was typed against, so it is
  // dropped by the same change that blanks the value it was overriding.
  const [draft, setDraft] = useState<{ request: string; value: string } | null>(null)

  // Stable across renders, so a caller can depend on it from an effect without
  // that effect re-running on every render. It answers with the write, for the
  // one caller that has to act only once the value has landed — naming a GitHub
  // account makes every pull request badge look up again, and a re-read that
  // overtakes the write replays the old account's answer.
  const persist = useCallback(
    (next: string): Promise<unknown> => {
      setDraft({ request, value: next })
      if (scope === undefined) {
        return Promise.resolve()
      }
      return Store.SetSetting(key, scope, next.trim())
    },
    [key, scope, request],
  )
  return [draft?.request === request ? draft.value : data || fallback, persist] as const
}

/** The same value as the boolean it reads as everywhere else: the store holds
 * strings, so one place knows that "true" is the only one that means on. */
export function useStoredFlag(key: string, scope: string | undefined) {
  const [value, persist] = useStoredSetting(key, scope)
  return [value === "true", (on: boolean) => void persist(on ? "true" : "false")] as const
}
