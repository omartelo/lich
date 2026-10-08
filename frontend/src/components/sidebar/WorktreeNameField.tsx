import type { ReactNode } from "react"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { WorktreeName } from "./useWorktreeName"

interface WorktreeNameFieldProps {
  field: WorktreeName
  placeholder: string
  /** Said instead of the name's lines when the name does not apply. */
  disabledNote?: string
  /** What the name creates, under the issue lines: a branch, or a race's set. */
  creates: ReactNode
}

// WorktreeNameField draws useWorktreeName: the field, the issue a "#N" resolved
// to or why it did not, and the line naming what will be created.
export function WorktreeNameField({
  field,
  placeholder,
  disabledNote,
  creates,
}: WorktreeNameFieldProps) {
  const { name, setName, issue, issueError, issueRef, lookingUp } = field
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor="worktree-name" className="text-xs uppercase tracking-wide">
        Worktree name
      </Label>
      <Input
        id="worktree-name"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder={placeholder}
        disabled={disabledNote !== undefined}
        autoFocus
      />
      <div className="flex flex-col font-mono text-xs text-muted-foreground">
        {disabledNote !== undefined ? (
          <span>{disabledNote}</span>
        ) : lookingUp ? (
          <span>Looking up #{issueRef}…</span>
        ) : (
          <>
            {issue && (
              <span className="text-foreground">
                #{issue.number} {issue.title}
              </span>
            )}
            {issueError && <span className="font-sans text-destructive">{issueError}</span>}
            {creates}
          </>
        )}
      </div>
    </div>
  )
}
