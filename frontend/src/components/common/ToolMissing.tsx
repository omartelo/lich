import type { LucideIcon } from "lucide-react"
import { ExternalLink } from "lucide-react"
import { Button } from "@/components/ui/button"
import { System } from "@/lib/rpc"
import type { VcsTool } from "@/lib/vcs-tools"
import { CheckAgainButton } from "./CheckAgainButton"
import { useT } from "@/lib/i18n/i18n"

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
  const t = useT()
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
      <Icon className="size-8 text-muted-foreground" />
      <div className="flex flex-col gap-1">
        <p className="text-sm text-foreground">
          {t("common.toolMissing.notInstalled", { label: tool.label })}
        </p>
        <p className="max-w-sm text-sm text-muted-foreground">{tool.without}</p>
      </div>
      <Button size="sm" onClick={() => void System.OpenExternal(tool.url)}>
        {t("common.toolMissing.install", { bin: tool.bin })}
        <ExternalLink />
      </Button>
      <CheckAgainButton />
    </div>
  )
}
