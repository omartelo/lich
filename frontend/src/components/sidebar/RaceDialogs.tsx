import type { ProviderState } from "@/lib/providers-store"
import type { ProviderKind } from "@/lib/session/sessions"
import { ConsolidateDialog } from "./ConsolidateDialog"
import { RaceAgentsDialog } from "./RaceAgentsDialog"
import type { RaceFlow } from "./useRaceFlow"

interface RaceDialogsProps {
  flow: RaceFlow
  projectId: string
  projectPath: string
  providers: ProviderState[]
  defaultProvider: ProviderKind
  currentBranch: string
}

// RaceDialogs mounts the two questions a race asks: how to start one, and how
// to consolidate it. Both are open only while useRaceFlow says so.
export function RaceDialogs({
  flow,
  projectId,
  projectPath,
  providers,
  defaultProvider,
  currentBranch,
}: RaceDialogsProps) {
  return (
    <>
      <RaceAgentsDialog
        open={flow.raceOpen}
        onOpenChange={flow.setRaceOpen}
        projectPath={projectPath}
        projectId={projectId}
        providers={providers}
        defaultProvider={defaultProvider}
        currentBranch={currentBranch}
        onStart={flow.start}
      />
      <ConsolidateDialog
        folder={flow.consolidating}
        onClose={() => flow.setConsolidating(null)}
        projectPath={projectPath}
        projectId={projectId}
        paths={flow.consolidatePaths}
        task={flow.consolidateTask}
        providers={providers}
        defaultProvider={defaultProvider}
        currentBranch={currentBranch}
        onConsolidate={flow.consolidate}
      />
    </>
  )
}
