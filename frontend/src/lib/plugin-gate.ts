// Startup decision for the lich Claude Code plugin: whether to prompt an
// install, an update, or stay silent. Kept pure (no bindings, no storage) so it
// is trivially testable; the component wires it to Status(), the dialog, and the
// toast.

import type {Status} from "../../bindings/github.com/omartelo/lich/internal/claudeplugin"

export const INSTALL_DISMISSED_KEY = "lich.pluginInstallDismissed"
export const UPDATE_DISMISSED_KEY = "lich.pluginUpdateDismissed"

// Value stored under INSTALL_DISMISSED_KEY: the install prompt is dismissed for
// good, unlike the update one which stores the version it was dismissed for.
export const DISMISSED_FLAG = "1"

// PluginAction is what the gate should do: install prompt, update prompt (with
// the target version), or nothing.
export type PluginAction = {kind: "install"} | {kind: "update"; version: string} | {kind: "none"}

// decidePluginAction resolves the startup prompt. Not installed and not
// permanently dismissed → install. Installed with a newer release not yet
// dismissed for that exact version → update. Otherwise nothing.
export function decidePluginAction(
  status: Status,
  installDismissed: boolean,
  updateDismissedVersion: string | null,
): PluginAction {
  if (!status.installed) {
    return installDismissed ? {kind: "none"} : {kind: "install"}
  }
  if (status.updateAvailable && updateDismissedVersion !== status.latestVersion) {
    return {kind: "update", version: status.latestVersion}
  }
  return {kind: "none"}
}
