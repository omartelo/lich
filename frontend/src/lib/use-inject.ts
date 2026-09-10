import { useCallback } from "react"
import { Terminal as TerminalService } from "./rpc"
import { safeToWrite } from "./terminal/bracketed-paste"

// useInject writes text straight into a session's PTY — the file and line
// references the review panel, the file browser and the pull request screen all
// hand to the agent working there. A session-less surface gets a no-op rather
// than a guard at every call site, and the false it answers with is for the
// callers whose text would otherwise be lost (a batch of review comments).
//
// Straight into the PTY is why safeToWrite is here rather than at the call
// sites: two of the three inject a path out of a repository tree, and nothing
// wraps this write, so a file committed with a newline in its name would send
// the prompt on the agent's behalf.
export function useInject(sessionId: string): (text: string) => boolean {
  return useCallback(
    (text: string) => {
      if (!sessionId) {
        return false
      }
      void TerminalService.Write(sessionId, safeToWrite(text))
      return true
    },
    [sessionId],
  )
}
