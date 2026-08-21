import { useState } from "react"
import { ExternalLink } from "lucide-react"
import { failed } from "@/lib/binary-layers"
import { System } from "@/lib/rpc"
import { useBinaryCheck } from "@/lib/use-binary-check"
import { GIT, RESTART_HINT } from "@/lib/vcs-tools"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

// GitMissingGate says the one thing a machine without git is never told: git is
// what every branch, diff and worktree is read through, and none of them will
// ever fill in. The pollers swallow their failures by design — a status read
// that logged would file a warning a second — so without this the app is simply
// blank in the places git answers, with nothing on screen that says why.
//
// git is the only tool that earns a dialog. gh has a screen of its own to be
// missing on (Pulls), while git has none: it is behind the sidebar, the footer
// and the dock at once.
//
// Nothing is remembered across launches. The dismissal is this run's, because
// the condition is not a preference to be honoured — it is a machine that
// cannot do half of what lich offers, and it stops asking the moment git exists.
export function GitMissingGate() {
  const check = useBinaryCheck(GIT.bin)
  const [dismissed, setDismissed] = useState(false)

  if (!failed(check) || dismissed) {
    return null
  }

  return (
    <Dialog open onOpenChange={() => setDismissed(true)}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>git is not installed</DialogTitle>
          <DialogDescription>
            lich reads every branch, diff and worktree through git. Sessions still run without it —
            the version control surfaces stay empty.
          </DialogDescription>
        </DialogHeader>
        <DialogDescription>{RESTART_HINT}</DialogDescription>
        <DialogFooter>
          <Button variant="ghost" onClick={() => setDismissed(true)}>
            Not now
          </Button>
          <Button
            onClick={() => {
              void System.OpenExternal(GIT.url)
              setDismissed(true)
            }}
          >
            Install git
            <ExternalLink />
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
