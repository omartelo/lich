// Startup decision for the lich plugin: which provider CLIs to offer an install
// for, which to offer an update for, or stay silent. Kept pure (no bindings, no
// storage) so it is trivially testable; the component wires it to Status(), the
// dialog, and the toast.

import type { PluginStatus as Status } from "@/lib/api-types"
export type { Status }

export const INSTALL_DISMISSED_KEY = "lich.pluginInstallDismissed"
export const UPDATE_DISMISSED_KEY = "lich.pluginUpdateDismissed"

// Value stored under INSTALL_DISMISSED_KEY: the install prompt is dismissed for
// good, unlike the update one which stores the version it was dismissed for.
export const DISMISSED_FLAG = "1"

// Plugin hooks are read when a provider session starts, so nothing an install or
// an update changes reaches a session already running.
export const RESTART_HINT = "restart your sessions to apply."

// Codex refuses to run a plugin's hooks until they are reviewed once, and says
// nothing when it skips them — so an install there is only half done without
// this, and the user would see a plugin that reports nothing.
export const CODEX_TRUST_HINT = "run /hooks in a Codex session to trust the plugin's hooks."

// PluginAction is what the gate should do: an install prompt listing the
// providers it can install into, an update prompt (with the target version and
// the providers it covers), or nothing.
export type PluginAction =
  | { kind: "install"; providers: Status[] }
  | { kind: "update"; version: string; providers: Status[] }
  | { kind: "none" }

// decidePluginAction resolves the startup prompt. A provider whose CLI is not on
// the machine is never part of either: there is nothing to install into, and its
// absence is not news at startup.
//
// The install prompt wins over the update one when both apply — a CLI still
// without the plugin is the bigger gap, and the install dialog is where the
// update can be picked up on the next start anyway. Its dismissal is one flag
// for the whole prompt, and the update's one version for all providers, because
// every provider installs the same release from the same repository.
export function decidePluginAction(
  statuses: Status[],
  installDismissed: boolean,
  updateDismissedVersion: string | null,
): PluginAction {
  const available = statuses.filter((s) => s.available)
  const missing = available.filter((s) => !s.installed)
  if (missing.length > 0 && !installDismissed) {
    return { kind: "install", providers: missing }
  }
  const outdated = available.filter((s) => s.installed && s.updateAvailable)
  const version = outdated[0]?.latestVersion ?? ""
  if (outdated.length > 0 && updateDismissedVersion !== version) {
    return { kind: "update", version, providers: outdated }
  }
  return { kind: "none" }
}
