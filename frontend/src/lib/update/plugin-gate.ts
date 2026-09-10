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

// Crush has one hook event, so two of the four reports have nowhere to ride.
// Said here because the gap is invisible otherwise: a card that never shows a
// spinner reads as a broken install rather than as the harness's own limit.
export const CRUSH_SCOPE_HINT =
  "reports its session id and refreshes git status. It has no end-of-turn event, so its cards show no status and keep their own name."

// oh-my-pi and Antigravity report everything except the one state a user is
// waiting to see, and neither raises an approval event any run was measured
// emitting. Said here for the same reason as Crush's: a bell that never rings
// reads as a broken install rather than as the harness's own gap. One line for
// both, because the gap and its consequence are the same.
export const NO_APPROVAL_EVENT_HINT =
  "reports its status, name and git changes. It has no observed approval event, so a session waiting on your permission shows a spinner rather than a bell."

// Cursor is the one provider lich ships no hooks to: it runs Claude Code's
// installed plugin itself, so its reports and its version are that install's,
// and what lich writes here is only the MCP registration Cursor takes from
// nowhere else. Said here because the row reads as broken otherwise — its
// version tracks another row's, and it offers no update of its own.
export const CURSOR_SHARED_PLUGIN_HINT =
  "runs the plugin installed in Claude Code, so it reports what that version reports and updates with it. lich adds only its own tools here. Like the two above, it raises no approval event, so a session waiting on your permission shows a spinner rather than a bell."

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
