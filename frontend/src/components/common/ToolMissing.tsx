import type { LucideIcon } from "lucide-react"
import { ExternalLink } from "lucide-react"
import { Button } from "@/components/ui/button"
import { System } from "@/lib/rpc"
import { RESTART_HINT, type VcsTool } from "@/lib/vcs-tools"

interface ToolMissingProps {
  tool: VcsTool
  icon: LucideIcon
}

// ToolMissing is what a screen shows in place of itself when the command-line
// tool it runs on is not installed: the fact, what it costs, and the page that
// fixes it. Not a dialog — a screen that cannot do anything without the tool is
// its own notice, and a box over it would only cover the explanation.
//
// It fills the area it is handed rather than overlaying one (see EmptyScreen),
// so a panel and a whole screen can both show it.
export function ToolMissing({ tool, icon: Icon }: ToolMissingProps) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
      <Icon className="size-8 text-muted-foreground" />
      <div className="flex flex-col gap-1">
        <p className="text-sm text-foreground">{tool.label} is not installed</p>
        <p className="max-w-sm text-sm text-muted-foreground">{tool.without}</p>
      </div>
      <Button size="sm" onClick={() => void System.OpenExternal(tool.url)}>
        Install {tool.bin}
        <ExternalLink />
      </Button>
      <p className="text-xs text-muted-foreground">{RESTART_HINT}</p>
    </div>
  )
}
