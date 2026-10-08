import { useState } from "react"
import { toast } from "sonner"
import { folderCheckouts, raceTask } from "@/lib/session/agent-race"
import type { ProviderKind, Session } from "@/lib/session/sessions"
import type { SandboxAnswer } from "@/lib/use-sandbox-choice"
import { useProjects } from "@/providers/projects"
import { type RaceBase, useAgentRace } from "./useAgentRace"

/** The slice of usePanes a race needs: building its wall. */
interface RaceWall {
  groupWith: (sessionId: string, delegateIds: readonly string[], name?: string) => number
}

export interface RaceFlow {
  raceOpen: boolean
  setRaceOpen: (open: boolean) => void
  /** The folder being consolidated, or null while that dialog is closed. */
  consolidating: string | null
  setConsolidating: (folder: string | null) => void
  /** The race's worktrees the consolidation prompt lists. */
  consolidatePaths: string[]
  consolidateTask: string
  start: (
    name: string,
    base: RaceBase,
    sandbox: SandboxAnswer,
    task: string,
    agents: ProviderKind[],
  ) => Promise<void>
  consolidate: (
    name: string,
    base: RaceBase,
    sandbox: SandboxAnswer,
    prompt: string,
    agent: ProviderKind,
  ) => Promise<void>
}

// useRaceFlow is the sidebar's half of a race: which of its two dialogs is
// open, and what each one's button does once the worktrees exist. A started
// race goes on a wall of its own, named after its folder, so every agent is on
// screen at once; a consolidation opens alone, the race's wall one click away.
export function useRaceFlow(
  projectId: string,
  projectPath: string,
  sessions: Session[],
  wall: RaceWall,
): RaceFlow {
  const { activateSession } = useProjects()
  const race = useAgentRace(projectId, projectPath)
  const [raceOpen, setRaceOpen] = useState(false)
  const [consolidating, setConsolidating] = useState<string | null>(null)
  const consolidatePaths = consolidating
    ? folderCheckouts(sessions, consolidating).map((checkout) => checkout.path)
    : []

  return {
    raceOpen,
    setRaceOpen,
    consolidating,
    setConsolidating,
    consolidatePaths,
    consolidateTask: consolidating ? raceTask(projectId, consolidating) : "",

    async start(name, base, sandbox, task, agents) {
      const started = await race.start(name, base, sandbox, task, agents)
      setRaceOpen(false)
      const [first, ...rest] = started.sessionIds
      // The stage refuses panes it cannot draw readably (pane-grid.fits); the
      // agents left out run in the folder all the same, and the toast says so.
      const skipped = wall.groupWith(first, rest, started.folder)
      if (skipped > 0) {
        toast(
          skipped === 1
            ? "1 agent could not be added to the split"
            : `${skipped} agents could not be added to the split`,
        )
      }
    },

    async consolidate(name, base, sandbox, prompt, agent) {
      if (!consolidating) {
        return
      }
      const opened = await race.consolidate(consolidating, name, base, sandbox, prompt, agent)
      setConsolidating(null)
      activateSession(projectId, opened)
    },
  }
}
