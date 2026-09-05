import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"
import { clockAt, scheduleChoices, scheduledFor } from "@/lib/session/schedule"

interface SchedulePromptDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The card this lands in, named so the dialog can say where it will be typed. */
  label: string
  /** The prompt already parked on this session, "" when nothing is. */
  prompt: string
  /** When that one is due, in unix seconds; 0 when nothing is parked. */
  at: number
  onSchedule: (at: number, prompt: string) => void
}

// SchedulePromptDialog parks one prompt on a session for later. The time is the
// submit: picking one schedules and closes, which is why there is no Save
// button to reach past four choices that already say what they do.
//
// Each choice carries the wall clock it resolves to, so "Tomorrow" never has to
// be taken on trust — and the times are resolved once per opening rather than
// on a timer: they are a minute stale at worst, against shortcuts a quarter of
// an hour apart at their closest.
export function SchedulePromptDialog({
  open,
  onOpenChange,
  label,
  prompt,
  at,
  onSchedule,
}: SchedulePromptDialogProps) {
  const [value, setValue] = useState(prompt)
  const [now, setNow] = useState(() => new Date())

  // Reseeded on every open, like the entrypoint dialog: the component outlives
  // one visit, and a session reopened after a cancel would otherwise hold the
  // abandoned text and yesterday's clock.
  useEffect(() => {
    if (open) {
      setValue(prompt)
      setNow(new Date())
    }
  }, [open, prompt])

  const commit = (when: number) => {
    const next = value.trim()
    if (!next) {
      return
    }
    onOpenChange(false)
    onSchedule(when, next)
  }

  const clear = () => {
    onOpenChange(false)
    onSchedule(0, "")
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Schedule a prompt</DialogTitle>
          <DialogDescription>
            Typed at <span className="font-medium text-foreground">{label}</span> when the time
            comes, as if you had typed it yourself.
          </DialogDescription>
        </DialogHeader>
        <Textarea
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder="run the release checklist"
          autoFocus
          rows={3}
          className="max-h-40"
        />
        <div className="grid grid-cols-4 gap-1.5">
          {scheduleChoices(now).map((choice) => (
            <Button
              key={choice.label}
              variant="secondary"
              disabled={!value.trim()}
              onClick={() => commit(choice.at)}
              className="h-auto flex-col gap-0 py-1.5"
            >
              <span className="text-sm font-medium">{choice.label}</span>
              <span className="text-xs font-normal tabular-nums text-muted-foreground">
                {clockAt(choice.at)}
              </span>
            </Button>
          ))}
        </div>
        {at > 0 && (
          // Only the second visit draws this: it is both what is already parked
          // and the only way to take it off, so a session with nothing waiting
          // has no cancel to misread.
          <DialogFooter className="items-center justify-between sm:justify-between">
            <span className="text-xs text-muted-foreground">
              Scheduled for {scheduledFor(at, now)}
            </span>
            <Button variant="ghost" size="sm" onClick={clear}>
              Cancel it
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
