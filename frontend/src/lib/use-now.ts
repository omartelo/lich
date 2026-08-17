import { useEffect, useState } from "react"

const MINUTE_MS = 60_000

// useNow is the wall clock, re-read on the minute rather than on a fixed
// interval: every readout built on it is minute-resolution (the footer's HH:MM,
// a quota window's time to reset), so a 10s tick spent five of every six renders
// redrawing the same string, and still showed a minute that could be ten seconds
// stale.
export function useNow(): Date {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    let timer = 0
    const tick = () => {
      const at = new Date()
      setNow(at)
      timer = window.setTimeout(tick, MINUTE_MS - (at.getTime() % MINUTE_MS))
    }
    timer = window.setTimeout(tick, MINUTE_MS - (Date.now() % MINUTE_MS))
    return () => window.clearTimeout(timer)
  }, [])
  return now
}
