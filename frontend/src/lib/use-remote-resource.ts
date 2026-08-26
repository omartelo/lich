import { useCallback, useEffect, useRef, useState } from "react"
import { readRemoteCache, writeRemoteCache } from "@/lib/remote-cache"
import { errorText } from "@/lib/utils"

// The read side of a backend lookup: what came back, whether a call is in
// flight, why the last one failed, and the way to run it again.
export interface RemoteResource<T> {
  data: T
  loading: boolean
  error: string | null
  refresh: () => void
}

export interface RemoteResourceOptions<T> {
  /** What `data` holds before the first answer, and after a failed or skipped
   * lookup. Pass a module-level constant for arrays and objects — a fresh one
   * per render would notify subscribers on every failure. */
  empty: T
  /** Re-read when the window regains focus. On for anything the user can
   * change outside lich (a branch pushed, a review left); off where re-reading
   * is expensive enough that an alt-tab must not pay for it. */
  refetchOnFocus?: boolean
  /** Blank `data` back to `empty` the moment this value changes, ahead of the
   * answer. For a readout that captions itself from the request — the pull
   * request column labels its rows with the state it asked for — where holding
   * the old rows would mislabel them for as long as the call takes. Off by
   * default: a plain refetch keeps what it has, so a moved HEAD refreshes a
   * screen in place instead of flashing it back to a skeleton. A filed answer
   * for the new request wins over blanking: that one is captioned correctly. */
  resetOn?: string
  /** File each answer under this key, so the next mount of the same request
   * paints from it and revalidates underneath instead of showing a skeleton
   * (remote-cache). Off by default, and pointless for anything that never
   * unmounts.
   *
   * Deliberately not `key`. `key` is what makes an answer *stale*, and on the
   * pull request screen that includes the checkout's HEAD — which a fresh mount
   * does not have until its git poll has answered, so a cache keyed by it would
   * miss on the one frame this exists for. This is what the answer is *about*:
   * the caller's own name and the request, and nothing that merely dates it.
   * Two callers sharing one of these serve each other's answers, so the name is
   * not optional. */
  cache?: string
}

// useRemoteResource is the one lookup skeleton behind the screens' backend
// reads: a sequence-guarded call whose late answer can never land on a newer
// request or on an unmounted component, plus the loading/error pair every
// caller was otherwise rebuilding by hand.
//
// `key` is the contract. It stands for the whole request — every path, number
// and revision the call closes over — because `load` itself is read through a
// ref and is deliberately not a dependency: callers pass an inline closure
// rather than memoising one, and a key that leaves out an argument is a hook
// that answers with the previous request's data.
//
// An empty key is "nothing to look up", not a failure: the lookup is skipped,
// `data` holds `empty` and `loading` is false. That is what an absent path
// means everywhere in the app, and it keeps a screen with no repository from
// spending a round-trip.
export function useRemoteResource<T>(
  key: string,
  load: () => Promise<T>,
  options: RemoteResourceOptions<T>,
): RemoteResource<T> {
  const { empty, refetchOnFocus = false, resetOn, cache } = options
  // The answer this request already has, if it was asked for before and the
  // caller keeps them. Read every render: the first paint and every move to
  // another request both have to see it.
  const held = cache && key ? readRemoteCache<T>(cache) : undefined
  const [data, setData] = useState<T>(held ?? empty)
  // Starts true so the first paint of a real request reads as loading rather
  // than as an answered "there is nothing here" — unless the answer is already
  // in hand, which is the whole point of filing it.
  const [loading, setLoading] = useState(key !== "" && held === undefined)
  const [error, setError] = useState<string | null>(null)
  // Sequence number of the newest request; a reply carrying an older one is
  // dropped. The cleanup bumps it, which is also how unmount invalidates.
  const seq = useRef(0)
  const loadRef = useRef(load)
  loadRef.current = load
  const emptyRef = useRef(empty)
  emptyRef.current = empty

  const refresh = useCallback(() => {
    if (!key) {
      seq.current++
      setData(emptyRef.current)
      setError(null)
      setLoading(false)
      return
    }
    const mine = ++seq.current
    // A request whose last answer is in hand revalidates underneath: the screen
    // keeps what it is showing rather than flipping back to a skeleton, as true
    // of a manual reload after a merge as of a return to the screen. `data` is
    // moved during render, not here — by the time an effect ran, the frame this
    // exists to prevent would already have been painted.
    if (!cache || readRemoteCache(cache) === undefined) {
      setLoading(true)
    }
    loadRef
      .current()
      .then((result) => {
        // Filed before the sequence check: the key is this closure's own, so a
        // reply that arrived too late for the screen is still the right answer
        // to the request it was made for, and the next mount should have it.
        if (cache) {
          writeRemoteCache(cache, result)
        }
        if (mine !== seq.current) return
        setData(result)
        setError(null)
      })
      .catch((err: unknown) => {
        if (mine !== seq.current) return
        // The filed answer is left standing: a lookup that failed says nothing
        // about the last one that worked, and the next success replaces it.
        setData(emptyRef.current)
        setError(errorText(err))
      })
      .finally(() => {
        if (mine === seq.current) setLoading(false)
      })
  }, [key, cache])

  // Moving the data during render, not in an effect: whatever belongs to the
  // abandoned request must never reach the screen, and an effect would paint it
  // one frame first.
  //
  // What has already been accounted for is held in state, never in a ref. React
  // may discard a render that updates state during it and replay it, and a ref
  // written by the discarded pass makes the replay skip the very update it was
  // guarding — the filed answer is seeded and then silently lost, which is how
  // this screen came back blank instead of painted.
  const [seenCache, setSeenCache] = useState(cache)
  const [seenReset, setSeenReset] = useState(resetOn)
  if (cache !== seenCache || resetOn !== seenReset) {
    const resetting = resetOn !== seenReset
    setSeenCache(cache)
    setSeenReset(resetOn)
    if (held !== undefined) {
      // The filed answer is this request's own, so it is captioned correctly
      // even where resetOn was about to blank the screen. Nothing is being
      // waited for either: the refetch the effect is about to run replaces what
      // is already there.
      setData(held)
      setLoading(false)
    } else if (resetting) {
      setData(empty)
    }
  }

  useEffect(() => {
    refresh()
    if (refetchOnFocus) {
      window.addEventListener("focus", refresh)
    }
    return () => {
      // Invalidate any in-flight call so a late answer cannot land on the next
      // request or after unmount.
      seq.current++
      window.removeEventListener("focus", refresh)
    }
  }, [refresh, refetchOnFocus])

  return { data, loading, error, refresh }
}
