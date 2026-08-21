import type { LucideIcon } from "lucide-react"
import { ExternalLink, GitBranch, GitPullRequestArrow, Wrench } from "lucide-react"
import type { BinaryCheck } from "@/lib/api-types"
import { failed } from "@/lib/binary-layers"
import { System } from "@/lib/rpc"
import { useBinaryCheck } from "@/lib/use-binary-check"
import { GH, GIT, RESTART_HINT, type VcsTool } from "@/lib/vcs-tools"
import { Button } from "@/components/ui/button"
import { SettingBlock } from "./SettingBlock"

// VcsToolsSetting is the first thing the Version Control screen answers: whether
// the machine has the two tools every surface below it shells out to, and where
// to get the one it does not. Without it, git or gh missing is silent — the
// pollers swallow the failure and the screens simply stay empty.
//
// It reports the resolved path rather than a tick, because "is git installed" is
// rarely the real question: lich pins the login shell's $PATH at launch, so
// *which* git it found is what a machine with two of them needs to see.
export function VcsToolsSetting() {
  const git = useBinaryCheck(GIT.bin)
  const gh = useBinaryCheck(GH.bin)
  return (
    <SettingBlock
      icon={<Wrench className="size-4" />}
      title="Command-line tools"
      description="lich drives git and the GitHub CLI. Everything on this screen runs through them."
    >
      <div className="flex w-full flex-col gap-1">
        <ToolRow tool={GIT} icon={GitBranch} check={git} />
        <ToolRow tool={GH} icon={GitPullRequestArrow} check={gh} />
        {(failed(git) || failed(gh)) && (
          <p className="pt-1 text-xs text-muted-foreground">{RESTART_HINT}</p>
        )}
      </div>
    </SettingBlock>
  )
}

interface ToolRowProps {
  tool: VcsTool
  icon: LucideIcon
  /** Null until the check answers — the row shows neither verdict until it does. */
  check: BinaryCheck | null
}

function ToolRow({ tool, icon: Icon, check }: ToolRowProps) {
  const gone = failed(check)
  return (
    <div>
      <div className="flex items-center gap-2.5 py-1 text-sm">
        <Icon className="size-4 text-muted-foreground" />
        <span>{tool.label}</span>
        <span className="ml-auto text-xs text-muted-foreground">
          {gone ? (
            <span className="text-destructive">Not on $PATH</span>
          ) : (
            <span className="font-mono">{check?.path}</span>
          )}
        </span>
        {gone && (
          <Button size="sm" onClick={() => void System.OpenExternal(tool.url)}>
            Install
            <ExternalLink />
          </Button>
        )}
      </div>
      {gone && <p className="ml-6.5 text-xs text-muted-foreground">{tool.without}</p>}
    </div>
  )
}
