import {useEffect, useRef} from "react"
import {toast} from "sonner"
import {useMatch, useNavigate} from "react-router-dom"
import {Button} from "@/components/ui/button"
import {decideUpdateAction, UPDATE_DISMISSED_KEY, type UpdateAction} from "@/lib/app-update-gate"
import {AppUpdate, System} from "@/lib/rpc"
import {useProjects} from "@/lib/projects"
import {queuePaste} from "@/lib/paste-queue"
import {runWithToast} from "@/lib/toast-async"

const RESTART_HINT = "restart lich to apply."

// How often to re-check for a release after startup, so a long-running session
// eventually notices one. Hourly is plenty — releases are rare and the
// unauthenticated GitHub API allows only 60 requests/hour per IP.
const POLL_INTERVAL_MS = 60 * 60 * 1000

// The one-liner from install.sh / the README. Pasted into a shell for the user
// to run — never executed automatically.
const INSTALL_CMD = "curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh"

// AppUpdateGate checks on startup, then hourly, whether a newer lich release
// exists. Where the binary is writable (Windows/macOS) it offers a one-click
// self-update; on Linux the binary is package-manager owned, so it offers to
// paste the install command into a terminal (the user runs it) or open the
// release page. Any failure is silent — it must never block or break startup.
export function AppUpdateGate() {
  const {newSession, ensureHomeProject} = useProjects()
  const navigate = useNavigate()
  const activeProjectId = useMatch("/projects/:projectId")?.params.projectId ?? null

  // Ref so the toast handler reads the latest active project without re-running.
  const activeRef = useRef(activeProjectId)
  activeRef.current = activeProjectId
  // The version last toasted this session, so the hourly poll (and strict-mode's
  // double effect) never stacks a second toast for the same release; a genuinely
  // newer release still gets one.
  const promptedVersion = useRef<string | null>(null)

  useEffect(() => {
    void check()
    const id = setInterval(() => void check(), POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [])

  const check = async () => {
    let action: UpdateAction
    try {
      const status = await AppUpdate.Status()
      action = decideUpdateAction(status, localStorage.getItem(UPDATE_DISMISSED_KEY))
    } catch {
      return
    }
    if (action.kind !== "update") return
    if (promptedVersion.current === action.version) return
    promptedVersion.current = action.version
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

  const runApply = () =>
    runWithToast(
      "Downloading lich update…",
      () => AppUpdate.Apply(),
      `lich updated — ${RESTART_HINT}`,
      "Update failed",
    )

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
