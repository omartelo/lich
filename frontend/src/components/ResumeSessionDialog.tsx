import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import type { Session } from "@/lib/sessions"

interface ResumeSessionDialogProps {
  /** The restored session about to spawn, or null when the dialog is hidden. */
  session: Session | null
  /** Spawn Claude Code fresh, leaving the previous conversation behind. */
  onStartNew: () => void
  /** Spawn Claude Code with --resume, continuing the previous conversation. */
  onResume: () => void
}

// ResumeSessionDialog asks, the first time a restored card is opened, whether
// its terminal should continue the Claude session it ran before the restart
// (`--resume`) or start a new one. Dismissing is the same answer as "Start
// new": the card has to end up with a terminal either way, and the spawn is
// waiting on this.
export function ResumeSessionDialog({
  session,
  onStartNew,
  onResume,
}: ResumeSessionDialogProps) {
  return (
    <ConfirmDialog
      open={session !== null}
      onCancel={onStartNew}
      cancelLabel="Start new"
      title="Resume previous session?"
      description={
        <>
          <span className="font-medium">{session?.label}</span> left a Claude
          Code session behind (
          <span className="break-all font-mono">{session?.providerSessionId}</span>
          ). Resume it to pick the conversation up where it stopped, or start new
          for an empty one.
        </>
      }
    >
      <Button onClick={onResume}>Resume</Button>
    </ConfirmDialog>
  )
}
