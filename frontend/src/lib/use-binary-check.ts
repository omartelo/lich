import { useEffect, useState, useSyncExternalStore } from "react"
import type { BinaryCheck } from "./api-types"
import { pathVersion, subscribePathRefresh } from "./path-refresh"
import { Providers } from "./rpc"

// How long a field rests before its value is verified. Every keystroke of a path
// is a state the path has never been in, so checking on each one would spend a
// round trip per character to report a file nobody has finished naming yet.
const SETTLE_MS = 400

// The settle for a `bin` that is a constant: there is no next keystroke to wait
// for, and every millisecond of it is one the caller spends not knowing whether
// the tool it is about to shell out to exists.
export const NO_SETTLE = 0

// The last verdict for each input, filed the way remote-cache.ts files a backend
// answer: module-level, never persisted, and never a source of truth. It is what
// a surface paints while its own check is in flight, so a settings pane reopened
// or a provider block remounted comes back with the verdict it had instead of
// blinking through "unknown".
//
// The key carries the whole input, the $PATH pin included: a verdict is resolved
// through the pin (path-refresh), so the same value asked before and after a
// re-read are two different questions, and a key of the value alone would paint
// the pre-move answer as if it still held.
const verdicts = new Map<string, BinaryCheck>()

const verdictKey = (bin: string, path: number): string => `${path} ${bin}`

// useBinaryCheck resolves a configured binary through the backend, re-checking
// whenever the value settles. Null for an input never checked, so the callers
// draw nothing rather than flashing a failure between keystrokes.
//
// It re-checks on a re-read of the machine's $PATH too (lib/path-refresh): the
// answer is resolved through the pin, so a moved pin makes every verdict on
// screen stale at once, wherever the re-check was pressed.
export function useBinaryCheck(bin: string, settleMs: number = SETTLE_MS): BinaryCheck | null {
  const path = useSyncExternalStore(subscribePathRefresh, pathVersion)
  const key = verdictKey(bin, path)
  const [check, setCheck] = useState<BinaryCheck | null>(() => verdicts.get(key) ?? null)

  useEffect(() => {
    let live = true
    setCheck(verdicts.get(key) ?? null)
    const timer = setTimeout(() => {
      Providers.Verify(bin)
        .then((next) => {
          verdicts.set(key, next)
          if (live) {
            setCheck(next)
          }
        })
        // A failed call is not a failed path: leaving the verdict absent says
        // "unknown", which is the truth when nothing answered.
        .catch(() => {})
    }, settleMs)
    return () => {
      live = false
      clearTimeout(timer)
    }
  }, [bin, settleMs, key])

  return check
}
