import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { Button } from "@/components/ui/button"
import { System } from "@/lib/rpc"
import { useHotkey } from "@/lib/use-hotkey"
import { runningSessions } from "@/lib/session/use-session-status"
import { sessionLabels } from "@/lib/session/sessions"
import { useT } from "@/lib/i18n/i18n"
import { errorText } from "@/lib/utils"
import { useProjects } from "@/providers/projects-context"

// QuitDialog is the window's way to end lich, which closing the window no longer
// does. Mounted once at the app root and opened by the "quit" action, which has
// no chord by default: the palette's ">" mode is where it is found. The sessions
// mid-turn are read when it opens, since they are what quitting costs.
export function QuitDialog() {
  const t = useT()
  const { sessions } = useProjects()
  const [working, setWorking] = useState<string[] | null>(null)

  useHotkey("quit", () => {
    const ids = Object.values(sessions).flatMap((project) => project.sessions.map((s) => s.id))
    setWorking(sessionLabels(sessions, runningSessions(ids)))
  })

  const quit = () => {
    setWorking(null)
    System.Quit().catch((error: unknown) => {
      toast.error(`${t("shell.quitDialog.failed")}: ${errorText(error)}`)
    })
  }

  return (
    <ConfirmDialog
      open={working !== null}
      onCancel={() => setWorking(null)}
      title={t("shell.quitDialog.title")}
      description={
        <>
          {t("shell.quitDialog.description")}
          {working && working.length > 0 && (
            <>
              <span className="mt-2 block">
                {t("shell.quitDialog.working", { count: working.length })}
              </span>
              <span className="mt-1 block font-mono text-xs">{working.join(", ")}</span>
            </>
          )}
        </>
      }
    >
      <Button variant="destructive" onClick={quit}>
        {t("shell.quitDialog.confirm")}
      </Button>
    </ConfirmDialog>
  )
}
