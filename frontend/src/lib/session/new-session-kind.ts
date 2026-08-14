import type { ProviderKind, SessionKind } from "./sessions"

// resolveNewSessionKind keeps explicit terminal/provider choices intact and
// applies the project's already-resolved default only to implicit creation.
export function resolveNewSessionKind(
  requestedKind: SessionKind | undefined,
  projectDefault: ProviderKind,
): SessionKind {
  return requestedKind ?? projectDefault
}

export function resolveNewWorktreeSessionKind(projectDefault: ProviderKind): ProviderKind {
  return projectDefault
}
