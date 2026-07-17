import {useEffect, useRef} from "react"
import {toast} from "sonner"
import {useMatch, useNavigate} from "react-router-dom"
import {Button} from "@/components/ui/button"
import {decideUpdateAction, UPDATE_DISMISSED_KEY} from "@/lib/app-update-gate"
import {AppUpdate, System} from "@/lib/rpc"
import {useProjects} from "@/lib/projects"
import {queuePaste} from "@/lib/paste-queue"
import {errorText} from "@/lib/utils"

const RESTART_HINT = "restart lich to apply."

// The one-liner from install.sh / the README. Pasted into a shell for the user
// to run — never executed automatically.
const INSTALL_CMD = "curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh"

// AppUpdateGate checks on startup whether a newer lich release exists. Where the
// binary is writable (Windows/macOS) it offers a one-click self-update; on Linux
// the binary is package-manager owned, so it offers to paste the install command
// into a terminal (the user runs it) or open the release page. Any failure is
// silent — it must never block or break startup.
export function AppUpdateGate() {
  const {newSession, ensureHomeProject} = useProjects()
  const navigate = useNavigate()
  const activeProjectId = useMatch("/projects/:projectId")?.params.projectId ?? null

  // Ref so the toast handler reads the latest active project without re-running.
  const activeRef = useRef(activeProjectId)
  activeRef.current = activeProjectId
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
      const status = await AppUpdate.Status()
      action = decideUpdateAction(status, localStorage.getItem(UPDATE_DISMISSED_KEY))
    } catch {
      return
    }
    if (action.kind !== "update") return
    if (action.canSelfApply) {
      promptSelfApply(action.version)
    } else {
      promptInstall(action.version, action.releaseUrl)
    }
  }

  const dismiss = (version: string) => localStorage.setItem(UPDATE_DISMISSED_KEY, version)

  // Windows/macOS: swap the binary in place, then ask for a restart.
  const promptSelfApply = (version: string) => {
    toast(`lich ${version} is available`, {
      duration: Infinity,
      action: {label: "Update & install", onClick: () => void runApply()},
      cancel: {label: "Later", onClick: () => dismiss(version)},
    })
  }

  const runApply = async () => {
    const pending = toast.loading("Downloading lich update…")
    try {
      await AppUpdate.Apply()
      toast.success(`lich updated — ${RESTART_HINT}`, {id: pending})
    } catch (error) {
      toast.error(`Update failed: ${errorText(error)}`, {id: pending})
    }
  }

  // Linux: three choices — paste the install command into a terminal, open the
  // release page, or dismiss for this version. sonner's default toast has only
  // two buttons, so this is a custom one styled with the popover tokens.
  const promptInstall = (version: string, releaseUrl: string) => {
    toast.custom(
      (id) => (
        <div className="flex flex-col gap-3 rounded-md border bg-popover p-4 text-sm text-popover-foreground shadow-lg">
          <span>lich {version} is available</span>
          <div className="flex gap-2">
            <Button
              size="sm"
              onClick={() => {
                toast.dismiss(id)
                void openInstall(releaseUrl)
              }}
            >
              Install
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                toast.dismiss(id)
                void System.OpenExternal(releaseUrl)
              }}
            >
              View release
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                toast.dismiss(id)
                dismiss(version)
              }}
            >
              Later
            </Button>
          </div>
        </div>
      ),
      {duration: Infinity},
    )
  }

  // Open a shell and paste the install command without running it. Rooted at the
  // project in view; with none in view (the Home screen, e.g. right after
  // launch) a $HOME-rooted project is opened so Install never dead-ends. Falls
  // back to the release page only if even that fails (home dir unresolvable).
  const openInstall = async (releaseUrl: string) => {
    let projectId = activeRef.current
    if (!projectId) {
      try {
        projectId = await ensureHomeProject()
      } catch {
        void System.OpenExternal(releaseUrl)
        return
      }
    }
    const sessionId = newSession(projectId, "shell")
    queuePaste(sessionId, INSTALL_CMD)
    navigate(`/projects/${projectId}`)
  }

  return null
}
