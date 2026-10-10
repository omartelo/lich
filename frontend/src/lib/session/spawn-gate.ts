import type { RestoreChoice } from "@/lib/providers-store"
import type { Session } from "./sessions"
import { t, tIn } from "@/lib/i18n/i18n"

// What has to be settled before a session's terminal spawns. Both questions are
// about state that outlived the row naming it — the directory the PTY would
// start in, and the provider conversation the card would resume — so both are
// asked of the backend, and neither can be answered from the workspace alone.

export type SpawnDecision =
  /** Nothing in the way. */
  | { verdict: "spawn" }
  /** A conversation is still there: raise the resume prompt. */
  | { verdict: "ask" }
  /** A conversation is still there and the provider is set to resume it. */
  | { verdict: "resume" }
  /** The conversation is gone: spawn without it, and say so. */
  | { verdict: "fresh"; notice: string }
  /** The checkout is gone: there is nothing to spawn into. */
  | { verdict: "park"; notice: string }

/** The backend reads, injected so the decision is testable without the RPC. */
export interface SpawnProbe {
  workdirMissing: (cwd: string) => Promise<boolean>
  resumeAvailable: (kind: string, providerSessionID: string, cwd: string) => Promise<boolean>
  restoreChoice: (kind: string) => Promise<RestoreChoice>
}

// The English wording, for what pins it; the notice a session shows is read
// through t() at the moment of the decision.
export const CHECKOUT_GONE = tIn("en", "session.spawnGate.checkoutGone")

export const CONVERSATION_GONE = tIn("en", "session.spawnGate.conversationGone")

// spawnDecision resolves what a session's first view should do. Order matters:
// a session with no directory to run in has nothing to resume either, so the
// checkout is settled first.
//
// Both checks fail safe, in opposite directions, because their mistakes cost
// different things. A workdir check that errors spawns anyway — the PTY reports
// its own failure, and closing a session on a check that never answered would be
// the worse trade. A resume check that errors still asks — losing a live
// conversation to an answer nobody gave costs more than the error the check
// removes.
export async function spawnDecision(
  cwd: string,
  resumable: Session | null,
  probe: SpawnProbe,
): Promise<SpawnDecision> {
  const missing = await probe.workdirMissing(cwd).catch(() => false)
  if (missing) {
    return { verdict: "park", notice: t("session.spawnGate.checkoutGone") }
  }
  if (!resumable) {
    return { verdict: "spawn" }
  }
  const available = await probe
    .resumeAvailable(resumable.kind, resumable.providerSessionId ?? "", cwd)
    .catch(() => true)
  if (!available) {
    return { verdict: "fresh", notice: t("session.spawnGate.conversationGone") }
  }
  // A failed read asks, for the same reason a failed resume check does.
  const choice = await probe.restoreChoice(resumable.kind).catch(() => "ask" as const)
  if (choice === "resume") {
    return { verdict: "resume" }
  }
  return choice === "fresh" ? { verdict: "spawn" } : { verdict: "ask" }
}
