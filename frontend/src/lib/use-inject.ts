import { useCallback } from "react"
import { Terminal as TerminalService } from "./rpc"

// useInject writes text straight into a session's PTY — the file and line
// references the review panel, the file browser and the pull request screen all
// hand to the agent working there. A session-less surface gets a no-op rather
// than a guard at every call site, and the false it answers with is for the
// callers whose text would otherwise be lost (a batch of review comments).
export function useInject(sessionId: string): (text: string) => boolean {
  return useCallback(
    (text: string) => {
      if (!sessionId) {
        return false
      }
      void TerminalService.Write(sessionId, text)
      return true
    },
    [sessionId],
  )
}
