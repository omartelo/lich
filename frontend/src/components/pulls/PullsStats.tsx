import type { ReactNode } from "react"
import {
  Check,
  CircleDashed,
  Clock,
  GitMerge,
  GitPullRequestArrow,
  X,
  type LucideIcon,
} from "lucide-react"
import type { ChecksRollup } from "@/lib/api-types"
import { cn } from "@/lib/utils"

// The pull request's status line, one reading per item: where it stands, what
// CI said, whether it would merge, where the review is. Each returns null when
// it has nothing to report, so the row carries no dead labels.

type Tone = "pass" | "fail" | "pending" | "muted"

const toneClass: Record<Tone, string> = {
  pass: "text-emerald-500",
  fail: "text-destructive",
  pending: "text-amber-500",
  muted: "text-muted-foreground",
}

function Stat({
  icon: Icon,
  tone,
  children,
}: {
  icon: LucideIcon
  tone: Tone
  children: ReactNode
}) {
  return (
    <span className={cn("flex items-center gap-1.5 font-medium", toneClass[tone])}>
      <Icon className="size-3.5" />
      {children}
    </span>
  )
}

export function ChecksStat({ checks }: { checks: ChecksRollup }) {
  const { passed, failed, pending, total } = checks
  if (total === 0) {
    return null
  }
  if (failed > 0) {
    return (
      <Stat icon={X} tone="fail">
        {failed} of {total} checks failing
      </Stat>
    )
  }
  if (pending > 0) {
    return (
      <Stat icon={Clock} tone="pending">
        {pending} of {total} checks running
      </Stat>
    )
  }
  return (
    <Stat icon={Check} tone="pass">
      {passed === 1 ? "1 check passed" : `${passed} checks passed`}
    </Stat>
  )
}

// Merged and closed reach this screen only through the list, which can be asked
// for them by number; the branch lookup still yields open pull requests alone.
// They read as muted rather than as another colour — there is nothing left to
// act on, so nothing for the eye to catch.
export function StateStat({ state, isDraft }: { state: string; isDraft: boolean }) {
  if (state === "MERGED") {
    return (
      <Stat icon={GitMerge} tone="muted">
        Merged
      </Stat>
    )
  }
  if (state === "CLOSED") {
    return (
      <Stat icon={X} tone="muted">
        Closed
      </Stat>
    )
  }
  return (
    <Stat icon={GitPullRequestArrow} tone={isDraft ? "pending" : "pass"}>
      {isDraft ? "Draft" : "Open"}
    </Stat>
  )
}

// gh's aggregate verdict, in the header's words. A repository that requires no
// review reports "" and gets no entry: there is no review to be waiting on.
const REVIEW_STAT: Record<string, { icon: LucideIcon; tone: Tone; label: string }> = {
  APPROVED: { icon: Check, tone: "pass", label: "Approved" },
  CHANGES_REQUESTED: { icon: X, tone: "fail", label: "Changes requested" },
  REVIEW_REQUIRED: { icon: CircleDashed, tone: "muted", label: "Review required" },
}

export function ReviewStat({ decision }: { decision: string }) {
  const stat = REVIEW_STAT[decision]
  if (!stat) {
    return null
  }
  return (
    <Stat icon={stat.icon} tone={stat.tone}>
      {stat.label}
    </Stat>
  )
}

export function MergeableStat({
  mergeable,
  base,
  state,
}: {
  mergeable: string
  base: string
  state: string
}) {
  // gh reports UNKNOWN for a pull request that is over, and "Checking
  // mergeability…" against something already merged reads as a stuck screen.
  if (state !== "OPEN") {
    return null
  }
  if (mergeable === "CONFLICTING") {
    return (
      <Stat icon={X} tone="fail">
        Conflicts with {base}
      </Stat>
    )
  }
  if (mergeable === "MERGEABLE") {
    return (
      <Stat icon={GitMerge} tone="pass">
        Mergeable
      </Stat>
    )
  }
  return (
    <Stat icon={CircleDashed} tone="muted">
      Checking mergeability…
    </Stat>
  )
}
