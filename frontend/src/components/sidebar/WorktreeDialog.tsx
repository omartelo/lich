import { useEffect, useState } from "react"
import { WorktreeSandboxRow } from "@/components/sidebar/WorktreeSandboxRow"
import { WorktreeScriptRows } from "@/components/sidebar/WorktreeScriptRows"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { issueBrief } from "@/lib/issue"
import { useSandboxChoice, type SandboxAnswer } from "@/lib/use-sandbox-choice"
import { cn, errorText } from "@/lib/utils"
import { BaseBranchPicker } from "./BaseBranchPicker"
import { useBaseBranch } from "./useBaseBranch"
import { WorktreeNameField } from "./WorktreeNameField"
import { useWorktreeName } from "./useWorktreeName"

interface WorktreeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectPath: string
  /** The project the worktree belongs to, and the provider its session will
   * run: together they scope the sandbox rung the confinement row starts on. */
  projectId: string
  providerId: string
  /** The repo's checked-out branch, preselected as the base. */
  currentBranch: string
  /** Create the worktree and open its session; rejections show in the dialog.
   * sandbox is the confinement answer for that session ("on"/"off", "" when the
   * machine cannot confine and nothing was asked). prompt is what that session
   * is handed once it comes up — the issue this worktree is for, or "" when the
   * name was not one. carryFrom is the checkout whose uncommitted work the new
   * one starts with, or "" when it starts at the base's last commit. */
  onCreate: (
    name: string,
    base: string,
    baseIsRemote: boolean,
    sandbox: SandboxAnswer,
    prompt: string,
    carryFrom: string,
  ) => Promise<void>
  /** Reopen a session on an already-existing worktree. */
  onResume: (wt: { name: string; path: string }) => void
  /** The session whose conversation this worktree's session will carry, when
   * the dialog was opened by a fork rather than by the + button: its label for
   * the sentence, and its checkout, which is both the base the fork opens on
   * and the source of the working-tree row. The + button's row reads the
   * project's own checkout instead. */
  forkOf?: { label: string; path: string } | null
}

// WorktreeDialog collects a worktree name (blank = random adjective-noun, plain
// words = a slug of them) and a base picked from a searchable list — existing
// worktrees to resume, then local and remote branches (remote bases are fetched
// and tracked). It stays open on failure so git's error is readable in place.
export function WorktreeDialog({
  open,
  onOpenChange,
  projectPath,
  projectId,
  providerId,
  currentBranch,
  onCreate,
  onResume,
  forkOf = null,
}: WorktreeDialogProps) {
  const [submitError, setSubmitError] = useState("")
  const [submitting, setSubmitting] = useState(false)
  // A worktree is always the linked checkout, so the rung is read on that side.
  const sandbox = useSandboxChoice(providerId, projectId, true, open)
  const naming = useWorktreeName(projectPath, open)
  const picker = useBaseBranch({
    open,
    projectPath,
    currentBranch,
    forkOf,
    // A fork carries a conversation into a new checkout, so reopening one that
    // exists is not what it can do.
    offerResume: !forkOf,
    onEnter: () => {
      if (picker.base && !submitting) {
        void submit()
      }
    },
  })

  useEffect(() => {
    if (open) {
      setSubmitError("")
      setSubmitting(false)
    }
  }, [open])

  const submit = async () => {
    const choice = picker.choice()
    if (!choice) {
      return
    }
    if (choice.resume) {
      onResume({ name: choice.resume.name, path: choice.resume.path })
      return
    }
    setSubmitting(true)
    setSubmitError("")
    try {
      await onCreate(
        naming.newBranch,
        choice.branch,
        choice.remote,
        sandbox.answer,
        naming.issue ? issueBrief(naming.issue) : "",
        choice.carryFrom,
      )
    } catch (err) {
      setSubmitError(errorText(err))
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* Fixed-height dialog: the base-branch section takes the leftover row
        (minmax(0,1fr)) so its list scrolls instead of growing the modal. */}
      <DialogContent
        className={cn(
          "h-[85vh] sm:max-w-2xl",
          forkOf
            ? "grid-rows-[auto_auto_auto_minmax(0,1fr)_auto_auto_auto]"
            : "grid-rows-[auto_auto_minmax(0,1fr)_auto_auto_auto]",
        )}
      >
        <DialogHeader>
          <DialogTitle>
            {forkOf ? `Fork ${forkOf.label} to a new worktree` : "New worktree"}
          </DialogTitle>
          <DialogDescription>
            Pick a base branch and (optionally) a name for the new worktree.
          </DialogDescription>
        </DialogHeader>

        {forkOf && (
          <div className="border-l-2 border-primary/60 pl-3">
            <div className="text-sm font-medium">Carries the conversation</div>
            <span className="text-xs text-muted-foreground">
              The new session opens on a copy of {forkOf.label}&rsquo;s history and keeps going from
              there. The original card is untouched.
            </span>
          </div>
        )}

        <WorktreeNameField
          field={naming}
          placeholder="A branch name, an issue (#128), or the task in plain words"
          disabledNote={picker.isResume ? "Opens the selected worktree" : undefined}
          creates={<span>Branch: {naming.newBranch || "<auto-generated>"}</span>}
        />

        <BaseBranchPicker picker={picker} sourceNoun={forkOf ? "session" : "checkout"} />

        <WorktreeScriptRows projectPath={projectPath} />

        <WorktreeSandboxRow choice={sandbox} />

        {(picker.loadError || submitError) && (
          <span className="text-xs break-words text-destructive">
            {picker.loadError || submitError}
          </span>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="ghost" />}>Cancel</DialogClose>
          <Button onClick={() => void submit()} disabled={!picker.base || submitting}>
            {submitting
              ? "Creating…"
              : picker.isResume
                ? "Open worktree"
                : forkOf
                  ? "Fork"
                  : "Create worktree"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
