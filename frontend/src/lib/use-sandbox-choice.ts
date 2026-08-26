import { useEffect, useState } from "react"
import { sandboxDefaultFor, sandboxKey, sandboxLevel } from "@/lib/providers-store"
import { Store, System } from "@/lib/rpc"

// The answer a session about to open carries: "on", "off", or "" when nothing
// was asked. The stored spelling, so it can go straight into the insert.
export type SandboxAnswer = "on" | "off" | ""

export interface SandboxChoice {
  /** Whether this machine can confine anything at all. */
  available: boolean
  /** Whether the session opens confined. */
  confined: boolean
  setConfined: (confined: boolean) => void
  /** What to record on the session row. */
  answer: SandboxAnswer
}

// useSandboxChoice is the confinement decision for one session about to open. It
// starts on the rung the spawn would reach by itself (store.SandboxDefault in
// Go, mirrored by sandboxDefaultFor), so a box the user never touches means
// exactly what it shows.
//
// worktree says which side of the rung this session lands on: a linked checkout,
// or the project's own directory. open is whether the dialog is on screen, and it
// is what the rung is read on: the dialog lives inside the sidebar, which stays
// mounted while Settings takes the screen (App.tsx), so a read done at mount
// would show — and then record — the rung as it was before the user changed it.
//
// The answer is recorded either way, never left empty once the machine can
// confine: an untouched box is still this session's answer, and freezing it on
// the row is what keeps a later change to the rung from quietly confining — or
// releasing — a session the user already opened.
export function useSandboxChoice(
  providerId: string,
  projectId: string,
  worktree: boolean,
  open: boolean,
) {
  const [available, setAvailable] = useState(false)
  const [confined, setChoice] = useState(false)

  useEffect(() => {
    if (!open) {
      return
    }
    let stale = false
    const load = async () => {
      const canConfine = (await System.SandboxBackend()) !== ""
      // The project's rung wins over the global one, which is the scope
      // store.SandboxLevel reads in the same order.
      const [scoped, global] = await Promise.all([
        projectId ? Store.GetSetting(sandboxKey(providerId), projectId) : Promise.resolve(""),
        Store.GetSetting(sandboxKey(providerId), ""),
      ])
      if (stale) {
        return
      }
      setAvailable(canConfine)
      setChoice(sandboxDefaultFor(sandboxLevel(scoped || global), worktree))
    }
    // A settings read that fails leaves the row absent and the session
    // unconfined, which is what lich did before the sandbox existed.
    void load().catch(() => undefined)
    return () => {
      stale = true
    }
  }, [providerId, projectId, worktree, open])

  const answer: SandboxAnswer = available ? (confined ? "on" : "off") : ""
  return { available, confined, setConfined: setChoice, answer }
}
