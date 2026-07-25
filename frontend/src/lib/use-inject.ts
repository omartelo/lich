import { useCallback } from "react"
import { Terminal as TerminalService } from "./rpc"

// useInject writes text straight into a session's PTY — the file and line
// references the review panel, the file browser and the pull request screen all
// hand to the agent working there. A session-less surface gets a no-op rather
// than a guard at every call site.
export function useInject(sessionId: string): (text: string) => void {
  return useCallback(
    (text: string) => {
      if (sessionId) {
        void TerminalService.Write(sessionId, text)
      }
    },
    [sessionId],
  )
}
