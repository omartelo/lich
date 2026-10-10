import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import {
  CLOSE_ACTION_SETTING_KEY,
  CLOSE_REQUESTED_EVENT,
  closeActionOf,
  closeOutcome,
  holdsClose,
  inLichWindow,
  type CloseAction,
} from "@/lib/close-request"
import { System, Terminal } from "@/lib/rpc"
import { useT } from "@/lib/i18n/i18n"
import { errorText } from "@/lib/utils"
import { useStoredSetting } from "@/lib/use-stored-setting"

const GLOBAL_SCOPE = ""

// CloseDialog asks, when the user closes lich's window, whether lich keeps its
// sessions running in the background or quits. Mounted once at the app root;
// it renders nothing until a close is held (lib/close-request.ts).
export function CloseDialog() {
  const t = useT()
  const [stored, persist] = useStoredSetting(CLOSE_ACTION_SETTING_KEY, GLOBAL_SCOPE)
  // Read inside the once-only listeners, which must not be torn down and
  // rebuilt by a change of the setting.
  const action = useRef<CloseAction>("ask")
  action.current = closeActionOf(stored)
  const [running, setRunning] = useState<number | null>(null)
  const [remember, setRemember] = useState(false)

  useEffect(() => {
    if (!inLichWindow(window.location.href)) {
      return
    }
    const hold = (event: BeforeUnloadEvent) => {
      if (holdsClose(action.current)) {
        event.preventDefault()
      }
    }
    const requested = () => {
      void Terminal.LiveCount().then((live) => {
        if (closeOutcome(action.current, live) === "quit") {
          quit()
          return
        }
        setRemember(false)
        setRunning(live)
      })
    }
    window.addEventListener("beforeunload", hold)
    window.addEventListener(CLOSE_REQUESTED_EVENT, requested)
    return () => {
      window.removeEventListener("beforeunload", hold)
      window.removeEventListener(CLOSE_REQUESTED_EVENT, requested)
    }
  }, [])

  const answer = (choice: CloseAction, act: () => void) => {
    setRunning(null)
    if (remember) {
      void persist(choice)
    }
    act()
  }

  const keepRunning = () => {
    System.CloseWindow().catch((error: unknown) => {
      toast.error(`${t("shell.closeDialog.failed")}: ${errorText(error)}`)
    })
  }

  return (
    <Dialog open={running !== null} onOpenChange={(next) => !next && setRunning(null)}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("shell.closeDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("shell.closeDialog.running", { count: running ?? 0 })}
          </DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2">
          <Checkbox
            id="close-remember"
            checked={remember}
            onCheckedChange={(checked) => setRemember(checked)}
          />
          <Label htmlFor="close-remember" className="text-sm font-normal">
            {t("shell.closeDialog.dontAsk")}{" "}
            <span className="text-muted-foreground">{t("shell.closeDialog.changeInSettings")}</span>
          </Label>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => answer("quit", quit)}>
            {t("shell.closeDialog.quit")}
          </Button>
          <Button onClick={() => answer("background", keepRunning)}>
            {t("shell.closeDialog.keep")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function quit() {
  System.Quit().catch((error: unknown) => {
    toast.error(errorText(error))
  })
}
