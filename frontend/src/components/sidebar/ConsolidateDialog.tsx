import { useEffect, useState } from "react"
import { WorktreeSandboxRow } from "@/components/sidebar/WorktreeSandboxRow"
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
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import type { ProviderState } from "@/lib/providers-store"
import { ProjectService } from "@/lib/rpc"
import { canRace, consolidationPrompt, type ConsolidationSource } from "@/lib/session/agent-race"
import type { ProviderKind } from "@/lib/session/sessions"
import { useSandboxChoice, type SandboxAnswer } from "@/lib/use-sandbox-choice"
import { errorText } from "@/lib/utils"
import { AgentPicker } from "./AgentPicker"
import { BaseBranchPicker } from "./BaseBranchPicker"
import type { RaceBase } from "./useAgentRace"
import { useBaseBranch } from "./useBaseBranch"
import { WorktreeNameField } from "./WorktreeNameField"
import { useWorktreeName } from "./useWorktreeName"

interface ConsolidateDialogProps {
  /** The race's folder, or null while the dialog is closed. */
  folder: string | null
  onClose: () => void
  projectPath: string
  projectId: string
  /** The race's worktrees, which the prompt lists. */
  paths: string[]
  /** The race's task when this window remembers it, "" when it does not. */
  task: string
  providers: ProviderState[]
  defaultProvider: ProviderKind
  currentBranch: string
  onConsolidate: (
    name: string,
    base: RaceBase,
    sandbox: SandboxAnswer,
    prompt: string,
    agent: ProviderKind,
  ) => Promise<void>
}

// ConsolidateDialog is "Consolidate…" on a race's folder: one agent in a new
// worktree, handed a prompt that lists every raced branch and asks for the best
// of each in one implementation. The prompt is written out here and editable;
// until it is edited it follows the base picked below, which it names.
export function ConsolidateDialog({
  folder,
  onClose,
  projectPath,
  projectId,
  paths,
  task,
  providers,
  defaultProvider,
  currentBranch,
  onConsolidate,
}: ConsolidateDialogProps) {
  const open = folder !== null
  const [agent, setAgent] = useState<ProviderKind | null>(null)
  const [sources, setSources] = useState<ConsolidationSource[]>([])
  const [edited, setEdited] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const naming = useWorktreeName(projectPath, open)
  const sandbox = useSandboxChoice(agent ?? defaultProvider, projectId, true, open)
  const picker = useBaseBranch({
    open,
    projectPath,
    currentBranch,
    forkOf: null,
    offerResume: false,
    onEnter: () => void submit(),
  })
  const choice = picker.choice()
  const baseName = choice && !choice.resume ? choice.branch : currentBranch
  const prompt = edited ?? consolidationPrompt(baseName, sources, task)
  const { setName } = naming

  useEffect(() => {
    if (!open) {
      return
    }
    setAgent(canRace(defaultProvider) ? defaultProvider : null)
    setName(`${folder}-consolidated`)
    setEdited(null)
    setSubmitError("")
    setSubmitting(false)
  }, [open, folder, defaultProvider, setName])

  // The branch each raced worktree is on, asked of git rather than read off the
  // card: a card's label can be renamed, a checkout's branch is what the diff
  // the prompt asks for is taken against.
  useEffect(() => {
    if (!open) {
      return
    }
    let stale = false
    ProjectService.BranchesOf(paths)
      .then((byPath) => {
        if (!stale) {
          setSources(paths.map((path) => ({ branch: byPath[path] ?? path, path })))
        }
      })
      .catch((err: unknown) => {
        if (!stale) {
          setSubmitError(errorText(err))
        }
      })
    return () => {
      stale = true
    }
  }, [open, paths])

  const submit = async () => {
    if (!choice || choice.resume || !agent || !naming.newBranch || submitting) {
      return
    }
    setSubmitting(true)
    setSubmitError("")
    try {
      await onConsolidate(
        naming.newBranch,
        { branch: choice.branch, remote: choice.remote, carryFrom: choice.carryFrom },
        sandbox.answer,
        prompt,
        agent,
      )
    } catch (err) {
      setSubmitError(errorText(err))
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="h-[85vh] grid-rows-[auto_auto_minmax(0,1fr)_auto_auto_auto] sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Consolidate {folder}</DialogTitle>
          <DialogDescription>
            A new worktree with an agent that reads all {paths.length} and combines the best of
            each.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <AgentPicker
            label="Agent"
            providers={providers}
            picked={agent ? [agent] : []}
            onToggle={setAgent}
          />
          <WorktreeNameField
            field={naming}
            placeholder="A branch name for the consolidated work"
            creates={<span>Branch: {naming.newBranch || "<name it>"}</span>}
          />
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="consolidate-prompt" className="text-xs uppercase tracking-wide">
              Prompt
            </Label>
            <Textarea
              id="consolidate-prompt"
              value={prompt}
              onChange={(e) => setEdited(e.target.value)}
              className="max-h-40 font-mono text-xs"
            />
            <span className="text-xs text-muted-foreground">
              Sent to the agent once it is at its own prompt.
            </span>
          </div>
        </div>

        <BaseBranchPicker picker={picker} sourceNoun="checkout" />

        <WorktreeSandboxRow choice={sandbox} />

        {(picker.loadError || submitError) && (
          <span className="text-xs break-words text-destructive">
            {picker.loadError || submitError}
          </span>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="ghost" />}>Cancel</DialogClose>
          <Button
            onClick={() => void submit()}
            disabled={!picker.base || !agent || !naming.newBranch || submitting}
          >
            {submitting ? "Starting…" : "Consolidate"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
