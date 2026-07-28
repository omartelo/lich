import { useCallback, useEffect, useRef, useState } from "react"
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
   * screen in place instead of flashing it back to a skeleton. */
  resetOn?: string
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
  const { empty, refetchOnFocus = false, resetOn } = options
  const [data, setData] = useState<T>(empty)
  // Starts true so the first paint of a real request reads as loading rather
  // than as an answered "there is nothing here".
  const [loading, setLoading] = useState(key !== "")
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
    setLoading(true)
    loadRef
      .current()
      .then((result) => {
        if (mine !== seq.current) return
        setData(result)
        setError(null)
      })
      .catch((err: unknown) => {
        if (mine !== seq.current) return
        setData(emptyRef.current)
        setError(errorText(err))
      })
      .finally(() => {
        if (mine === seq.current) setLoading(false)
      })
  }, [key])

  // Blanking during render, not in an effect: the old data must never reach
  // the screen once the request behind it has been abandoned, and an effect
  // would paint it one frame first.
  const lastReset = useRef(resetOn)
  if (resetOn !== undefined && resetOn !== lastReset.current) {
    lastReset.current = resetOn
    setData(emptyRef.current)
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
