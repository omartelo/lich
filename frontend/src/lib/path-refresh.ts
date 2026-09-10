// The one recovery for a tool installed while lich was open. lich resolves the
// login shell's $PATH once, at launch, and pins it into its own process
// (terminal.PinPath) — so a scan repeated against that pin can only ever report
// what was installed then. This re-reads the shell in the backend and replaces
// the pin (providers.RefreshPath), which is what every check below it resolves
// through.
import { Providers } from "./rpc"

// What a surface says when the login shell did not answer inside its bound. The
// pin is left exactly as it was, so a re-check now would report the same
// absence, and a relaunch is the way through.
export const PATH_REREAD_FAILED = "PATH could not be re-read; relaunch lich."

// version counts successful re-reads. Every binary check is answered from the
// pin, so each one has to run again when the pin moves — subscribing to this is
// how they do it without every caller knowing about every check.
let version = 0
const listeners = new Set<() => void>()

export function subscribePathRefresh(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function pathVersion(): number {
  return version
}

// refreshPath re-reads the machine's $PATH and re-pins it, then wakes every
// check drawn from it. It throws PATH_REREAD_FAILED rather than resolving
// quietly: a re-scan of the old pin is not a fresh answer, and the surface has
// to be able to say which one it is showing.
export async function refreshPath(): Promise<void> {
  try {
    await Providers.RefreshPath()
  } catch {
    throw new Error(PATH_REREAD_FAILED)
  }
  version++
  for (const listener of listeners) {
    listener()
  }
}
