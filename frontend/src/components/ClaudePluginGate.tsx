import {useEffect, useRef, useState} from "react"
import {toast} from "sonner"
import {Button} from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  decidePluginAction,
  DISMISSED_FLAG,
  INSTALL_DISMISSED_KEY,
  UPDATE_DISMISSED_KEY,
} from "@/lib/plugin-gate"
import {ClaudePlugin} from "@/lib/rpc"
import {errorText} from "@/lib/utils"
import {runWithToast} from "@/lib/toast-async"

const RESTART_HINT = "restart your Claude sessions to apply."

// ClaudePluginGate checks on startup whether the lich Claude Code plugin is
// installed and current. Missing → a one-click install modal; a newer release →
// a non-blocking, actionable update toast. Plugin hooks only load in new Claude
// sessions, so both actions remind the user to restart. Any failure is silent —
// it must never block or break startup.
export function ClaudePluginGate() {
  const [installOpen, setInstallOpen] = useState(false)
  const [installing, setInstalling] = useState(false)
  // Guard React strict-mode's double effect: the check runs once per start.
  const checked = useRef(false)

  useEffect(() => {
    if (checked.current) return
    checked.current = true
    void check()
  }, [])

  const check = async () => {
    let action
    try {
      const status = await ClaudePlugin.Status()
      action = decidePluginAction(
        status,
        localStorage.getItem(INSTALL_DISMISSED_KEY) === DISMISSED_FLAG,
        localStorage.getItem(UPDATE_DISMISSED_KEY),
      )
    } catch {
      return
    }
    if (action.kind === "install") {
      setInstallOpen(true)
    } else if (action.kind === "update") {
      promptUpdate(action.version)
    }
  }

  const promptUpdate = (version: string) => {
    toast(`lich plugin ${version} is available`, {
      duration: Infinity,
      action: {label: "Update", onClick: () => void runUpdate()},
      cancel: {
        label: "Later",
        onClick: () => localStorage.setItem(UPDATE_DISMISSED_KEY, version),
      },
    })
  }

  const runUpdate = () =>
    runWithToast(
      "Updating lich plugin…",
      () => ClaudePlugin.Update(),
      `Plugin updated — ${RESTART_HINT}`,
      "Update failed",
    )

  const runInstall = async () => {
    setInstalling(true)
    try {
      await ClaudePlugin.Install()
      setInstallOpen(false)
      toast.success(`Plugin installed — ${RESTART_HINT}`)
    } catch (error) {
      toast.error(`Install failed: ${errorText(error)}`)
    } finally {
      setInstalling(false)
    }
  }

  const dismissForever = () => {
    localStorage.setItem(INSTALL_DISMISSED_KEY, DISMISSED_FLAG)
    setInstallOpen(false)
  }

  return (
    <Dialog open={installOpen} onOpenChange={(open) => !open && !installing && setInstallOpen(false)}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Enable the Claude Code integration</DialogTitle>
          <DialogDescription>
            Install the lich plugin for Claude Code to get the most out of lich.
            It deepens the integration between your sessions and the app, and gains
            new capabilities with each release.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={dismissForever} disabled={installing}>
            Don't ask again
          </Button>
          <Button variant="ghost" onClick={() => setInstallOpen(false)} disabled={installing}>
            Not now
          </Button>
          <Button onClick={() => void runInstall()} disabled={installing}>
            {installing ? "Installing…" : "Install"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
