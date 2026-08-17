import type { BinaryCheck } from "./api-types"

// Where a provider's binary can be set, in the order lich resolves it: the
// project's own override, the global one, then whatever $PATH answers. Mirrors
// store.ProviderBin (Go), which is what the spawn actually calls.
export type BinaryScope = "project" | "global" | "path"

// winningScope is the layer a session would spawn from — the first one with a
// path. Values are compared trimmed, because a field holding only spaces is a
// field nobody filled in.
export function winningScope(projectBin: string, globalBin: string): BinaryScope {
  if (projectBin.trim()) {
    return "project"
  }
  return globalBin.trim() ? "global" : "path"
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
      return "Sessions will not start until this is fixed or cleared. Clearing it falls back to the layer below."
    default:
      return ""
  }
}

// failed reports a check that must be shown as a problem. A check still in
// flight is not one: the row would flash a failure between keystrokes.
export function failed(check: BinaryCheck | null): boolean {
  return check !== null && check.status !== "ok" && check.status !== "empty"
}
