import { Check, Clock, ExternalLink, X } from "lucide-react"
import { Notice } from "@/components/common/Notice"
import type { CheckItem } from "@/lib/api-types"
import { checkDuration } from "@/lib/pulls/check-duration"
import { System } from "@/lib/rpc"
import { cn } from "@/lib/utils"

const stateIcon = {
  passed: { icon: Check, className: "text-emerald-500" },
  failed: { icon: X, className: "text-destructive" },
  pending: { icon: Clock, className: "text-amber-500" },
} as const

// PullsChecks is the "Checks" tab of the Pulls screen: every check the rollup
// reported, worst state first, so a red CI names the job that broke instead of
// just counting it. A row opens its run in the browser — reading the log is
// GitHub's job, finding out which log to read is this tab's.
export function PullsChecks({ checks }: { checks: CheckItem[] | null }) {
  if (!checks || checks.length === 0) {
    return <Notice className="px-6 py-5 text-sm">No checks reported.</Notice>
  }
  return (
    <div className="flex flex-col py-1">
      {checks.map((check) => (
        <CheckRow key={`${check.name}\n${check.url}`} check={check} />
      ))}
    </div>
  )
}

function CheckRow({ check }: { check: CheckItem }) {
  const { icon: Icon, className } = stateIcon[check.state]
  const duration = checkDuration(check.startedAt, check.completedAt)
  const open = () => {
    if (check.url) {
      void System.OpenExternal(check.url)
    }
  }
  return (
    <button
      type="button"
      onClick={open}
      disabled={!check.url}
      title={check.url || undefined}
      className="group flex w-full items-center gap-3 px-6 py-2 text-left transition-colors hover:bg-accent/50 disabled:cursor-default disabled:hover:bg-transparent"
    >
      <Icon className={cn("size-4 shrink-0", className)} />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm">{check.name}</span>
        {check.description && (
          <span className="block truncate text-xs text-muted-foreground">{check.description}</span>
        )}
      </span>
      {duration && (
        <span className="shrink-0 tabular-nums text-xs text-muted-foreground">{duration}</span>
      )}
      {check.url && (
        <ExternalLink className="size-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
      )}
    </button>
  )
}
