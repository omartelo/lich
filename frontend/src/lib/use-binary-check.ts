import { useEffect, useState } from "react"
import type { BinaryCheck } from "./api-types"
import { Providers } from "./rpc"

// How long a field rests before its value is verified. Every keystroke of a path
// is a state the path has never been in, so checking on each one would spend a
// round trip per character to report a file nobody has finished naming yet.
const SETTLE_MS = 400

// useBinaryCheck resolves a configured binary through the backend, re-checking
// whenever the value settles. Null while the first answer for a value is in
// flight — the callers draw nothing rather than flashing a failure between
// keystrokes.
export function useBinaryCheck(bin: string): BinaryCheck | null {
  const [check, setCheck] = useState<BinaryCheck | null>(null)

  useEffect(() => {
    let live = true
    setCheck(null)
    const timer = setTimeout(() => {
      Providers.Verify(bin)
        .then((next) => {
          if (live) {
            setCheck(next)
          }
        })
        // A failed call is not a failed path: leaving the verdict absent says
        // "unknown", which is the truth when nothing answered.
        .catch(() => {})
    }, SETTLE_MS)
    return () => {
      live = false
      clearTimeout(timer)
    }
  }, [bin])

  return check
}
