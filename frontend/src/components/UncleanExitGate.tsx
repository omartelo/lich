import { useEffect } from "react"
import { toast } from "sonner"
import { System } from "@/lib/rpc"
import { runWithToast } from "@/lib/toast-async"

// UncleanExitGate says, once per launch, that the run before this one ended
// without closing its window — a crash, a kill, a machine that went down. It is
// the only place the user can learn it: the workspace comes back looking exactly
// as it does after a deliberate close, so a turn an agent was mid-way through is
// gone with nothing on screen to say so. The backend consumes the flag, which is
// what keeps a page reload from reading as a second crash.
export function UncleanExitGate() {
  useEffect(() => {
    void System.TakeUncleanExit()
      .then((unclean) => {
        if (unclean) prompt()
      })
      .catch(() => {})
  }, [])

  return null
}

// Persistent on purpose: this is the one telling, and a toast that expires while
// the user is reading a session is a toast they never saw.
const prompt = () => {
  toast.warning("lich's previous run ended unexpectedly", {
    description:
      "Your sessions were restored, but whatever they were running stopped with it. That run's log is in the log folder.",
    duration: Infinity,
    action: {
      label: "Open log folder",
      onClick: () =>
        void runWithToast(
          "Opening log folder…",
          System.RevealLog,
          "Log folder opened.",
          "Could not open the log folder",
        ),
    },
    // sonner closes the toast itself; the handler is only what its type asks for.
    cancel: { label: "Dismiss", onClick: () => {} },
  })
}
