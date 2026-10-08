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
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { issueText } from "@/lib/issue"
import type { ProviderState } from "@/lib/providers-store"
import { canRace, planRace, toggleAgent } from "@/lib/session/agent-race"
import type { ProviderKind } from "@/lib/session/sessions"
import { useSandboxChoice, type SandboxAnswer } from "@/lib/use-sandbox-choice"
import { errorText } from "@/lib/utils"
import { AgentPicker } from "./AgentPicker"
import { BaseBranchPicker } from "./BaseBranchPicker"
import { useBaseBranch } from "./useBaseBranch"
import type { RaceBase } from "./useAgentRace"
import { WorktreeNameField } from "./WorktreeNameField"
import { useWorktreeName } from "./useWorktreeName"

// The fewest agents that make a race: one is a plain worktree, which has a
// dialog of its own.
const MIN_AGENTS = 2

interface RaceAgentsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectPath: string
  projectId: string
  /** The providers New session offers; the project's default starts ticked. */
  providers: ProviderState[]
  defaultProvider: ProviderKind
  currentBranch: string
  /** Start the race; a rejection shows in the dialog, which stays open. */
  onStart: (
    name: string,
    base: RaceBase,
    sandbox: SandboxAnswer,
    task: string,
    agents: ProviderKind[],
  ) => Promise<void>
}

// RaceAgentsDialog is "Race agents…": one task, a worktree per agent off one
// base. Everything but the agents and the task is the new-worktree dialog's own
// parts, so the name, base, setup and sandbox rows behave the same in both.
export function RaceAgentsDialog({
  open,
  onOpenChange,
  projectPath,
  projectId,
  providers,
  defaultProvider,
  currentBranch,
  onStart,
}: RaceAgentsDialogProps) {
  const [agents, setAgents] = useState<ProviderKind[]>([])
  const [task, setTask] = useState("")
  const [submitError, setSubmitError] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const naming = useWorktreeName(projectPath, open)
  const race = planRace(naming.newBranch, task, agents)
  const ready = agents.length >= MIN_AGENTS && task.trim() !== ""
  // Read for the first agent picked: the box is one answer for all of them.
  const sandbox = useSandboxChoice(agents[0] ?? defaultProvider, projectId, true, open)
  const picker = useBaseBranch({
    open,
    projectPath,
    currentBranch,
    forkOf: null,
    offerResume: false,
    onEnter: () => void submit(),
  })

  useEffect(() => {
    if (!open) {
      return
    }
    setAgents(canRace(defaultProvider) ? [defaultProvider] : [])
    setTask("")
    setSubmitError("")
    setSubmitting(false)
  }, [open, defaultProvider])

  // A race on an issue is a race on its text, unless a task was already
  // written: that one is the user's and is kept.
  useEffect(() => {
    if (naming.issue) {
      const found = naming.issue
      setTask((written) => written || issueText(found))
    }
  }, [naming.issue])

  const submit = async () => {
    const choice = picker.choice()
    if (!choice || choice.resume || !ready || submitting) {
      return
    }
    setSubmitting(true)
    setSubmitError("")
    try {
      await onStart(
        race.folder,
        { branch: choice.branch, remote: choice.remote, carryFrom: choice.carryFrom },
        sandbox.answer,
        task.trim(),
        agents,
      )
    } catch (err) {
      setSubmitError(errorText(err))
      setSubmitting(false)
    }
  }

  const offered = providers.map((provider) => provider.id)
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="h-[85vh] grid-rows-[auto_auto_minmax(0,1fr)_auto_auto_auto] sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Race agents</DialogTitle>
          <DialogDescription>
            One task, a worktree per agent, every agent side by side.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <AgentPicker
              label="Agents"
              providers={providers}
              picked={agents}
              onToggle={(kind) => setAgents(toggleAgent(offered, agents, kind))}
            />
            <span
              className={
                agents.length >= MIN_AGENTS
                  ? "text-xs text-tone-wait"
                  : "text-xs text-muted-foreground"
              }
            >
              {agents.length >= MIN_AGENTS
                ? `${agents.length} agents work the same task at once; each one spends its own plan.`
                : "Pick at least two agents."}
            </span>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="race-task" className="text-xs uppercase tracking-wide">
              Task
            </Label>
            <Textarea
              id="race-task"
              value={task}
              onChange={(e) => setTask(e.target.value)}
              placeholder="What every agent should do"
              className="max-h-32"
            />
            <span className="text-xs text-muted-foreground">
              Sent to each agent once it is at its own prompt.
            </span>
          </div>

          <WorktreeNameField
            field={naming}
            placeholder="A name, an issue (#128), or blank to name it after the task"
            creates={
              <>
                <span>Folder: {race.folder || "<named after the task>"}</span>
                {race.folder && (
                  <span className="break-all">
                    Branches: {race.checkouts.map((checkout) => checkout.branch).join(", ")}
                  </span>
                )}
              </>
            }
          />
        </div>

        <BaseBranchPicker picker={picker} sourceNoun="checkout" />

        <WorktreeScriptRows projectPath={projectPath} />

        <WorktreeSandboxRow choice={sandbox} />

        {(picker.loadError || submitError) && (
          <span className="text-xs break-words text-destructive">
            {picker.loadError || submitError}
          </span>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="ghost" />}>Cancel</DialogClose>
          <Button onClick={() => void submit()} disabled={!picker.base || !ready || submitting}>
            {submitting ? "Starting…" : `Start ${Math.max(agents.length, MIN_AGENTS)} agents`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
