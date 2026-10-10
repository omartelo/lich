// Startup decision for the lich plugin: which provider CLIs to offer an install
// for, which to offer an update for, or stay silent. Kept pure (no bindings, no
// storage) so it is trivially testable; the component wires it to Status(), the
// dialog, and the toast.

import type { PluginStatus as Status } from "@/lib/api-types"
import { t } from "@/lib/i18n/i18n"
export type { Status }

export const INSTALL_DISMISSED_KEY = "lich.pluginInstallDismissed"
export const UPDATE_DISMISSED_KEY = "lich.pluginUpdateDismissed"

// Value stored under INSTALL_DISMISSED_KEY: the install prompt is dismissed for
// good, unlike the update one which stores the version it was dismissed for.
export const DISMISSED_FLAG = "1"

// PLUGIN_INCOMPATIBLE_EVENT is raised when a session's hooks report from a
// plugin release outside the range this lich speaks (internal/terminal
// pluginEventName). The gate answers it by asking for the status again.
export const PLUGIN_INCOMPATIBLE_EVENT = "plugin-incompatible"

// PluginAction is what the gate should do: an incompatible prompt (with the
// compatible release to install, empty when none is known), an install prompt
// listing the providers it can install into, an update prompt (with the target
// version and the providers it covers), or nothing.
export type PluginAction =
  | { kind: "incompatible"; version: string; providers: Status[] }
  | { kind: "install"; providers: Status[] }
  | { kind: "update"; version: string; providers: Status[] }
  | { kind: "none" }

// decidePluginAction resolves the startup prompt. A provider whose CLI is not on
// the machine is never part of either: there is nothing to install into, and its
// absence is not news at startup.
//
// An incompatible install wins over both and ignores every dismissal: its reports
// may be refused, so the cards go quiet in a way nothing else explains. Cursor
// is left out because its version is Claude Code's, and the fix is offered on
// that row.
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
  const incompatible = available.filter(
    (s) => s.installed && !s.compatible && s.provider !== "cursor",
  )
  if (incompatible.length > 0) {
    return {
      kind: "incompatible",
      version: incompatible[0]?.latestVersion ?? "",
      providers: incompatible,
    }
  }
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

// incompatibleMessage is the incompatible prompt's text: which installs are out
// of range, and what closes the gap.
export function incompatibleMessage(version: string, providers: Status[]): string {
  const installs = providers.map((p) => `${p.name} (v${p.installedVersion})`).join(", ")
  return version
    ? t("update.plugin.incompatibleInstall", { installs, version })
    : t("update.plugin.incompatibleUpdate", { installs })
}
