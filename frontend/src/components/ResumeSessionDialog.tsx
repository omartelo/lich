import { useState } from "react"
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
import { useProviders, type RestoreChoice } from "@/lib/providers-store"
import type { Session } from "@/lib/session/sessions"
import { Trans } from "@/components/common/Trans"
import { useT } from "@/lib/i18n/i18n"

interface ResumeSessionDialogProps {
  /** The restored session about to spawn, or null when the dialog is hidden. */
  session: Session | null
  /** Spawn the provider fresh, leaving the previous conversation behind. */
  onStartNew: () => void
  /** Spawn the provider on its resume invocation, continuing the conversation. */
  onResume: () => void
  /** Store the answer as the provider's default, so its next restored card is not asked. */
  onRemember: (kind: string, choice: Exclude<RestoreChoice, "ask">) => void
}

// ResumeSessionDialog asks, the first time a restored card is opened, whether
// its terminal should continue the provider conversation it ran before the
// restart or start a new one. Dismissing is the same answer as "Start new": the
// card has to end up with a terminal either way, and the spawn is waiting on
// this. Only the two buttons remember: a dismiss is not a choice anyone made.
export function ResumeSessionDialog({
  session,
  onStartNew,
  onResume,
  onRemember,
}: ResumeSessionDialogProps) {
  const t = useT()
  const providers = useProviders()
  // Tagged with the session it was ticked for, so the next prompt opens unticked.
  const [rememberFor, setRememberFor] = useState<string | null>(null)
  const remember = session !== null && rememberFor === session.id
  const providerName =
    providers.find((p) => p.id === session?.kind)?.name ??
    t("shell.resumeSessionDialog.thisProvider")

  const answer = (choice: Exclude<RestoreChoice, "ask">, spawn: () => void) => {
    if (remember && session) {
      onRemember(session.kind, choice)
    }
    spawn()
  }

  return (
    <Dialog open={session !== null} onOpenChange={(next) => !next && onStartNew()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("shell.resumeSessionDialog.title")}</DialogTitle>
          <DialogDescription className="break-words">
            <Trans
              k="shell.resumeSessionDialog.description"
              params={{
                label: <span className="font-medium">{session?.label}</span>,
                id: (
                  <span className="break-all font-mono select-text">
                    {session?.providerSessionId}
                  </span>
                ),
              }}
            />
          </DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2">
          <Checkbox
            id="resume-remember"
            checked={remember}
            onCheckedChange={(checked) => setRememberFor(checked && session ? session.id : null)}
          />
          <Label htmlFor="resume-remember" className="text-sm font-normal">
            {t("shell.resumeSessionDialog.alwaysFor", { provider: providerName })}
            <span className="text-muted-foreground">
              {t("shell.resumeSessionDialog.changeInSettings")}
            </span>
          </Label>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => answer("fresh", onStartNew)}>
            {t("shell.resumeSessionDialog.startNew")}
          </Button>
          <Button onClick={() => answer("resume", onResume)}>
            {t("shell.resumeSessionDialog.resume")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
