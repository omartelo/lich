// Startup decision for lich's own update: prompt for the newer release or stay
// silent. Kept pure (no bindings, no storage) so it is trivially testable; the
// component wires it to Status() and the toast. Mirrors plugin-gate.ts.

import type {AppUpdateStatus} from "./api-types"

// Stores the version the update prompt was dismissed for, so a newer release
// re-prompts (unlike a permanent dismissal).
export const UPDATE_DISMISSED_KEY = "lich.appUpdateDismissed"

// UpdateAction is what the gate should do: prompt for the update (with the
// target version and how to apply it) or nothing.
export type UpdateAction =
  | {kind: "none"}
  | {kind: "update"; version: string; canSelfApply: boolean; releaseUrl: string; installCommand: string}

// decideUpdateAction resolves the startup prompt. A newer release not yet
// dismissed for that exact version → update; otherwise nothing.
export function decideUpdateAction(
  status: AppUpdateStatus,
  dismissedVersion: string | null,
): UpdateAction {
  if (status.updateAvailable && dismissedVersion !== status.latestVersion) {
    return {
      kind: "update",
      version: status.latestVersion,
      canSelfApply: status.canSelfApply,
      releaseUrl: status.releaseUrl,
      installCommand: status.installCommand,
    }
  }
  return {kind: "none"}
}
