import { toast } from "sonner"
import type { Worktree } from "@/lib/api-types"
import { carryInto } from "@/lib/git/carry"
import { ProjectService } from "@/lib/rpc"
import { planRace, rememberConsolidator, rememberRaceTask } from "@/lib/session/agent-race"
import type { ProviderKind } from "@/lib/session/sessions"
import { queueSetup } from "@/lib/terminal/setup-queue"
import { sendWhenStarted } from "@/lib/terminal/write-at-prompt"
import type { SandboxAnswer } from "@/lib/use-sandbox-choice"
import { errorText } from "@/lib/utils"
import { useProjects } from "@/providers/projects"

/** Where a race or a consolidation starts: the base every worktree branches off. */
export interface RaceBase {
  branch: string
  remote: boolean
  carryFrom: string
}

export interface StartedRace {
  folder: string
  /** The sessions opened, in the order the agents were picked. */
  sessionIds: string[]
}

export interface AgentRace {
  start: (
    name: string,
    base: RaceBase,
    sandbox: SandboxAnswer,
    task: string,
    agents: ProviderKind[],
  ) => Promise<StartedRace>
  consolidate: (
    folder: string,
    name: string,
    base: RaceBase,
    sandbox: SandboxAnswer,
    prompt: string,
    agent: ProviderKind,
  ) => Promise<string>
}

// useAgentRace opens the worktrees of a race and of its consolidation: the
// new-worktree dialog's create, once per agent, with the task sent to each once
// that agent reports it is at its own prompt (sendWhenStarted, canRace).
//
// A race's worktrees are made one at a time. git takes a lock on the repository
// for each, and a refusal halfway is easier to report as "this one, after
// those" than as a set of parallel failures. The ones made before a refusal are
// left running: each is a whole worktree with its agent at work, and taking them
// back would throw that away to tidy up a failure that was not theirs.
export function useAgentRace(projectId: string, projectPath: string): AgentRace {
  const { newWorktreeSession } = useProjects()

  const open = async (
    branch: string,
    base: RaceBase,
    sandbox: SandboxAnswer,
    kind: ProviderKind,
    folder: string,
    text: string,
  ): Promise<string> => {
    const wt: Worktree | null = await ProjectService.CreateWorktree(
      projectPath,
      projectId,
      branch,
      base.branch,
      base.remote,
    )
    if (!wt) {
      throw new Error(`git made no worktree for ${branch}`)
    }
    await carryInto(base.carryFrom, wt)
    const opened = newWorktreeSession(projectId, wt, sandbox, null, kind, folder)
    queueSetup(opened)
    // Not awaited, like the issue handoff: the wait is the setup script, the
    // provider's boot and any trust question the user has to answer on the
    // card, and the dialog closes on worktrees that exist.
    void sendWhenStarted(opened, text).catch((err: unknown) => {
      toast.error(`Couldn’t send the task to ${branch}: ${errorText(err)}`)
    })
    return opened
  }

  return {
    async start(name, base, sandbox, task, agents) {
      const race = planRace(name, task, agents)
      rememberRaceTask(projectId, race.folder, task)
      const sessionIds: string[] = []
      for (const checkout of race.checkouts) {
        try {
          sessionIds.push(
            await open(checkout.branch, base, sandbox, checkout.kind, race.folder, task),
          )
        } catch (err) {
          throw new Error(
            `Started ${sessionIds.length} of ${race.checkouts.length} agents; ${checkout.branch} failed: ${errorText(err)}`,
          )
        }
      }
      return { folder: race.folder, sessionIds }
    },

    async consolidate(folder, name, base, sandbox, prompt, agent) {
      const opened = await open(name, base, sandbox, agent, folder, prompt)
      rememberConsolidator(projectId, folder, opened)
      return opened
    },
  }
}
