import { t } from "@/lib/i18n/i18n"
// The three reasons sandbox.Backend() answers "", and the only place the pane is
// told them apart. Only Linux's is about bubblewrap: macOS confines with
// sandbox-exec, and Windows has no backend at all — telling either of them to go
// install a Linux program is an error message about somebody else's machine.
export type SandboxPlatform = "linux" | "mac" | "windows"

export interface CannotConfineCopy {
  // What the status strip says after the em dash: why this machine cannot.
  reason: string
  // The line under it: what, if anything, the user can do about it.
  advice: string
}

export function cannotConfineCopy(platform: SandboxPlatform): CannotConfineCopy {
  if (platform === "windows") {
    return {
      reason: t("env.sandbox.windowsReason"),
      advice: t("env.sandbox.windowsAdvice"),
    }
  }
  if (platform === "mac") {
    return {
      reason: t("env.sandbox.macReason"),
      advice: t("env.sandbox.macAdvice"),
    }
  }
  return {
    reason: t("env.sandbox.linuxReason"),
    advice: t("env.sandbox.linuxAdvice"),
  }
}

// What confinement costs a session, in the one wording every surface that puts
// the question uses: the New worktree dialog's row and the New session menu's
// item. "Its checkout" rather than "this worktree" because the menu also opens
// sessions in the project's own directory.
export function confinedMeans(): string {
  return t("env.sandbox.confinedMeans")
}
