import type { SessionKind } from "./sessions"

// resolveNewSessionKind keeps explicit terminal/provider choices intact and
// applies the project's already-resolved default only to implicit creation. That
// default is a SessionKind rather than a ProviderKind because a machine with no
// agent installed resolves it to the shell (providers-store).
export function resolveNewSessionKind(
  requestedKind: SessionKind | undefined,
  projectDefault: SessionKind,
): SessionKind {
  return requestedKind ?? projectDefault
}
