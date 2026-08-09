// The first-run decision: which harnesses a new user is offered, and which one
// new sessions spawn. lich used to answer both by hardcoding Claude — the only
// provider its author ran — so a machine with just Codex met a first session
// that died on "claude: command not found". The dialog this gates asks instead.

import type { ProviderState } from "./providers-store"
import type { ProviderKind } from "./session/sessions"

export type ProviderSetupDecision =
  | { kind: "wait" }
  | { kind: "skip" }
  | { kind: "show"; preselect: ProviderKind | null }

// decideProviderSetup gates on the stored default provider, the one setting only
// a deliberate choice ever writes: empty means the user has never chosen, so
// existing installs that never opened Settings meet the dialog once too.
//
// An empty provider list is detection not having answered yet, never a machine
// with no harness — Detect returns the whole registry with install flags. That
// case is "wait", so the dialog cannot flash its no-agents shape on startup.
//
// preselect is the first installed provider in registry order, or null when none
// is installed. The caller turns it on before opening, so the common case (one
// harness, or one obvious first) is a single click.
export function decideProviderSetup(
  providers: ProviderState[],
  storedDefault: string,
): ProviderSetupDecision {
  if (providers.length === 0) {
    return { kind: "wait" }
  }
  if (storedDefault !== "") {
    return { kind: "skip" }
  }
  return { kind: "show", preselect: providers.find((p) => p.installed)?.id ?? null }
}
