// AppUpdateGate registers its forced check here; the Settings "Check for
// updates" button runs it, so the gate stays the single owner of the update UX.

import type {AppUpdateStatus} from "./api-types"

export type UpdateChecker = () => Promise<AppUpdateStatus>

let checker: UpdateChecker | null = null

export function registerUpdateChecker(fn: UpdateChecker | null): void {
  checker = fn
}

export function runUpdateCheck(): Promise<AppUpdateStatus> {
  if (!checker) {
    return Promise.reject(new Error("update checker not ready"))
  }
  return checker()
}
