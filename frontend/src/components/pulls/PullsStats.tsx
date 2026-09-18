import type { ReactNode } from "react"
import {
  Check,
  CheckCheck,
  CircleDashed,
  Clock,
  GitMerge,
  GitPullRequestArrow,
  X,
  type LucideIcon,
} from "lucide-react"
import type { ChecksRollup, PullRequestDetail } from "@/lib/api-types"
import type { ThreadTally } from "@/lib/pulls/conversation-timeline"
import { conflictsWithBase } from "@/lib/pulls/merge-gate"
import { cn } from "@/lib/utils"

// The pull request's status line, one reading per item: where it stands, what
// CI said, whether it would merge, where the review is. Each returns null when
// it has nothing to report, so the row carries no dead labels.

type Tone = "pass" | "fail" | "pending" | "muted"

// Tokens, not palette steps. A value that reads on the dark card is thin on the
// light one (emerald-500 measured 2.33:1 against it, amber-500 2.02:1, where
// 12px text wants 4.5:1), so each tone is defined per theme in index.css and
// carried by the theme file, the way --destructive always was.
const toneClass: Record<Tone, string> = {
  pass: "text-tone-pass",
  fail: "text-destructive",
  pending: "text-tone-wait",
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

// Whether the chip carries a thread count, and so has somewhere to lead. The
// header reads this to decide whether to make the chip a way into the
// Conversation tab: a count on screen with no way to reach what it counts, or a
// link on a chip that counts nothing, are the two ways this drifts apart.
export function reviewStatCountsThreads(decision: string, threads: ThreadTally): boolean {
  return decision === "CHANGES_REQUESTED" && threads.total > 0
}

// A requested change that has run out of open threads is not a failure any more,
// it is a wait, and what it waits on is the reviewer, not the branch. That is
// what the row already means by the pending tone, next to the checks running.
//
// Only CHANGES_REQUESTED is annotated: everywhere else the verdict is not a
// question the threads can answer.
export function ReviewStat({
  decision,
  threads,
}: {
  decision: string
  /** The review threads, as threadTally counts them. */
  threads: ThreadTally
}) {
  const stat = REVIEW_STAT[decision]
  if (!stat) {
    return null
  }
  if (!reviewStatCountsThreads(decision, threads)) {
    return (
      <Stat icon={stat.icon} tone={stat.tone}>
        {stat.label}
      </Stat>
    )
  }
  if (threads.open > 0) {
    return (
      <Stat icon={stat.icon} tone={stat.tone}>
        {stat.label} · {threads.open} of {threads.total} threads unresolved
      </Stat>
    )
  }
  return (
    <Stat icon={CheckCheck} tone="pending">
      {stat.label} · all threads resolved
    </Stat>
  )
}

export function MergeableStat({ detail }: { detail: PullRequestDetail }) {
  // gh reports UNKNOWN for a pull request that is over, and "Checking
  // mergeability…" against something already merged reads as a stuck screen.
  if (detail.state !== "OPEN") {
    return null
  }
  // The pair, the way every other reader of a conflict reads it (merge-gate):
  // the chip is what the conflicting-file list below it captions itself against,
  // and a chip still checking mergeability over a list of colliding files is the
  // screen disagreeing with itself.
  if (conflictsWithBase(detail)) {
    return (
      <Stat icon={X} tone="fail">
        Conflicts with {detail.baseRefName}
      </Stat>
    )
  }
  if (detail.mergeable === "MERGEABLE") {
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
