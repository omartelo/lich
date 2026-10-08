import { useEffect, useState } from "react"
import type { ExternalSession } from "@/lib/api-types"
import { Store } from "@/lib/rpc"

// useExternalSessions lists the conversations started outside lich while
// enabled, which the palette sets only on the History tab. The list is asked for
// once per enabling and filtered in the window as the user types: it costs a
// walk of every provider's store and a git call per project (about 300 ms on a
// real workspace), which is a price for opening a tab, not for a keystroke.
export function useExternalSessions(enabled: boolean): ExternalSession[] {
  const [rows, setRows] = useState<ExternalSession[]>([])

  useEffect(() => {
    if (!enabled) {
      setRows([])
      return
    }
    let live = true
    void Store.ExternalSessions().then((external) => {
      if (live) {
        setRows(external ?? [])
      }
    })
    return () => {
      live = false
    }
  }, [enabled])

  return rows
}
