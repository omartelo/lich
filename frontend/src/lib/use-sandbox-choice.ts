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
  /** Whether the rung — rather than the user — is what set the box. */
  fromRung: boolean
  /** What to record on the session row. */
  answer: SandboxAnswer
}

// useSandboxChoice is the confinement decision for one session about to open. It
// starts on the rung the spawn would reach by itself (store.SandboxDefault in
// Go, mirrored by sandboxDefaultFor), so a box the user never touches means
// exactly what it shows.
//
// worktree says which side of the rung this session lands on: a linked checkout,
// or the project's own directory.
//
// The answer is recorded either way, never left empty once the machine can
// confine: an untouched box is still this session's answer, and freezing it on
// the row is what keeps a later change to the rung from quietly confining — or
// releasing — a session the user already opened.
export function useSandboxChoice(providerId: string, projectId: string, worktree: boolean) {
  const [available, setAvailable] = useState(false)
  const [confined, setChoice] = useState(false)
  const [fromRung, setFromRung] = useState(true)

  useEffect(() => {
    let stale = false
    const load = async () => {
      const canConfine = await System.SandboxAvailable()
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
      setFromRung(true)
    }
    // A settings read that fails leaves the row absent and the session
    // unconfined, which is what lich did before the sandbox existed.
    void load().catch(() => undefined)
    return () => {
      stale = true
    }
  }, [providerId, projectId, worktree])

  const setConfined = (next: boolean) => {
    setChoice(next)
    setFromRung(false)
  }

  const answer: SandboxAnswer = available ? (confined ? "on" : "off") : ""
  return { available, confined, setConfined, fromRung, answer }
}
