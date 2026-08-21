import type { BinaryCheck } from "./api-types"

// Where a provider's binary can be set, in the order lich resolves it: the
// project's own override, the global one, then whatever $PATH answers. Mirrors
// store.ProviderBin (Go), which is what the spawn actually calls.
export type BinaryScope = "project" | "global" | "path"

// A configurable layer as the screen holds it: the path written into it, and
// whether its switch is on. The two are separate so a path can be parked rather
// than deleted — putting it back is a switch, not retyping it.
export type BinaryLayer = { bin: string; off: boolean }

// winningScope is the layer a session would spawn from — the first switched-on
// one with a path. Values are compared trimmed, because a field holding only
// spaces is a field nobody filled in.
export function winningScope(project: BinaryLayer, global: BinaryLayer): BinaryScope {
  if (resolves(project)) {
    return "project"
  }
  return resolves(global) ? "global" : "path"
}

// resolves reports a layer a session could spawn from. A parked layer never is,
// whatever it holds — which is what keeps a broken path switched off from
// failing a check nobody is being asked to fix.
export function resolves(layer: BinaryLayer): boolean {
  return !layer.off && layer.bin.trim() !== ""
}

// parked is the mirror of `resolves`: a layer holding a path with its switch
// off, so it would spawn if it were switched back on. A layer that is off and
// empty is not parked — there is nothing there to put back.
function parked(layer: BinaryLayer): boolean {
  return layer.off && layer.bin.trim() !== ""
}

// parkedLabel names the overrides that are set but switched off — the fact the
// closed state would otherwise hide, since the switches themselves are not on
// screen. A layer is named only when switching it back on would change what
// spawns: an override the winner already shadows is not news, so a parked global
// under a winning project override stays quiet while both parked under $PATH are
// both named.
export function parkedLabel(
  scope: BinaryScope,
  project: BinaryLayer,
  global: BinaryLayer,
  projectName: string | undefined,
): string {
  const names: string[] = []
  if (scope !== "project" && parked(project)) {
    names.push(`${projectName ?? "project"} override off`)
  }
  if (scope === "path" && parked(global)) {
    names.push("global override off")
  }
  return names.map((name) => ` · ${name}`).join("")
}

// checkLabel is the verdict beside a path, in the width a row has. Empty when
// there is nothing to say — an unset layer is not a failure.
//
// A missing binary reads differently depending on what was asked for: a bare
// name was looked up on $PATH and a path was looked for where it was written.
export function checkLabel(check: BinaryCheck | null, bin: string): string {
  switch (check?.status) {
    case "ok":
      return "executable"
    case "not-found":
      return bin.includes("/") || bin.includes("\\") ? "no such file" : "not on $PATH"
    case "not-executable":
      return "not executable"
    case "home-shortcut":
      return "~ is not expanded"
    case "relative":
      return "relative path"
    default:
      return ""
  }
}

// checkDetail is the sentence under a failing binary: what will happen, and what
// to do about it. Empty for a check that passed or has not arrived.
export function checkDetail(check: BinaryCheck | null): string {
  switch (check?.status) {
    case "home-shortcut":
      return "lich spawns the binary directly, so ~ is taken literally rather than expanded. Use the full path."
    case "relative":
      return "A relative path is resolved against each session's own working directory, so it names a different binary per session. Use a full path."
    case "not-found":
    case "not-executable":
      return "Sessions will not start until this is fixed. An override can also be switched off or cleared, falling back to the layer below."
    default:
      return ""
  }
}

// failed reports a check that must be shown as a problem. A check still in
// flight is not one: the row would flash a failure between keystrokes.
export function failed(check: BinaryCheck | null): boolean {
  return check !== null && check.status !== "ok" && check.status !== "empty"
}
