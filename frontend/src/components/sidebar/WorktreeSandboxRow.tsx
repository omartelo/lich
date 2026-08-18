import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import type { SandboxChoice } from "@/lib/use-sandbox-choice"

// WorktreeSandboxRow is the New-worktree dialog's confinement line: whether the
// session about to open runs inside the sandbox (internal/sandbox). It arrives
// showing what the provider's rung would do on its own, and whatever it shows
// when Create is pressed is recorded on that session alone — a later change to
// the rung never moves a session already open.
//
// Absent on a machine with no backend for it, WorktreeSetupRow's rule: a control
// that cannot change anything is worse than no control.
export function WorktreeSandboxRow({ choice }: { choice: SandboxChoice }) {
  if (!choice.available) {
    return null
  }
  return (
    <div className="flex items-start gap-2.5">
      <Checkbox
        id="worktree-sandbox"
        checked={choice.confined}
        onCheckedChange={choice.setConfined}
        className="mt-0.5"
      />
      <div className="flex flex-col gap-0.5">
        <Label htmlFor="worktree-sandbox" className="text-sm font-medium">
          Run confined
        </Label>
        <span className="text-xs text-muted-foreground">
          An empty home holding only the agent's own state, the machine read-only, and writes only
          inside this worktree. The network stays on.
        </span>
      </div>
    </div>
  )
}
