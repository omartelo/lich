import { useState } from "react"
import { toast } from "sonner"
import {
  Check,
  ChevronDown,
  CircleDashed,
  MessageSquare,
  Pencil,
  UserPlus,
  Users,
  X,
  type LucideIcon,
} from "lucide-react"
import type { PullRequestDetail, PullRequestReviewer, ReviewCandidate } from "@/lib/api-types"
import { pendingReviewerLogins, pickableReviewers } from "@/lib/pulls/reviewers"
import { ProjectService } from "@/lib/rpc"
import { cn, errorText } from "@/lib/utils"
import { Markdown } from "@/components/Markdown"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { commentFieldClass } from "./CommentBox"

interface PullsOverviewProps {
  path: string
  detail: PullRequestDetail
  /** Re-read the pull request: an edit here lands on GitHub, and GitHub's
   * answer — not a local copy of it — is what the screen then shows. */
  onRefresh: () => void
}

// PullsOverview is the "Overview" tab: who opened the pull request, who is
// reviewing it, and what it says. The description and the reviewers are both
// editable here — they were the last two things about a pull request that sent
// the reader out to github.com.
export function PullsOverview({ path, detail, onRefresh }: PullsOverviewProps) {
  // null is "not editing"; an empty string is a description being cleared, which
  // is a legitimate edit and must not read as the same thing.
  const [draft, setDraft] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const save = async () => {
    if (draft === null) {
      return
    }
    setSaving(true)
    try {
      await ProjectService.EditPullRequestBody(path, detail.number, draft)
      setDraft(null)
      onRefresh()
    } catch (err: unknown) {
      toast.error(`Couldn’t save the description: ${errorText(err)}`)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex max-w-3xl flex-col gap-5 px-6 py-5">
      <div className="flex flex-col gap-2 text-xs">
        {detail.author !== "" && (
          <span className="text-muted-foreground">
            Opened by <span className="text-foreground">{detail.author}</span>
          </span>
        )}
        <Reviewers path={path} detail={detail} onRefresh={onRefresh} />
      </div>

      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <span className="text-xs text-muted-foreground">Description</span>
          {draft === null && (
            <Button variant="ghost" size="sm" onClick={() => setDraft(detail.body)}>
              <Pencil />
              Edit
            </Button>
          )}
        </div>
        {draft === null ? (
          detail.body.trim() !== "" ? (
            <Markdown>{detail.body}</Markdown>
          ) : (
            <p className="text-sm text-muted-foreground">No description.</p>
          )
        ) : (
          <div className="flex flex-col gap-2">
            <textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Escape") {
                  e.preventDefault()
                  setDraft(null)
                }
              }}
              placeholder="Describe the pull request"
              // biome-ignore lint/a11y/noAutofocus: the box only exists because Edit was just clicked — typing is the next thing that happens.
              autoFocus
              // The box is sized by the description rather than by a row count:
              // a pull request body runs from one line to a page, and a fixed
              // height either wastes the screen or hides most of the text. The
              // bounds are the viewport's, so it grows with the window and
              // still leaves Save on screen.
              className={cn(commentFieldClass, "field-sizing-content min-h-[40vh] max-h-[70vh]")}
            />
            <div className="flex items-center gap-2">
              <Button size="sm" onClick={() => void save()} disabled={saving}>
                {saving ? "Saving…" : "Save"}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setDraft(null)} disabled={saving}>
                Cancel
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// The roster, and the one control that changes it. The picker is the only way a
// reviewer is added or dropped — a second affordance on the chips would be two
// ways to say the same thing, and the tick already reads as "asked".
function Reviewers({ path, detail, onRefresh }: PullsOverviewProps) {
  // Read on first open rather than with the pull request: most readings of a
  // pull request never touch the roster, and this is a gh round-trip.
  const [candidates, setCandidates] = useState<ReviewCandidate[] | null>(null)
  const [busy, setBusy] = useState(false)
  const roster = detail.reviewers ?? []
  const pending = pendingReviewerLogins(detail.reviewers)
  const pickable = pickableReviewers(candidates, detail.author)

  const load = async () => {
    if (candidates !== null) {
      return
    }
    try {
      setCandidates((await ProjectService.AssignableReviewers(path)) ?? [])
    } catch (err: unknown) {
      setCandidates([])
      toast.error(`Couldn’t read who can review: ${errorText(err)}`)
    }
  }

  const toggle = async (login: string, requested: boolean) => {
    setBusy(true)
    try {
      await ProjectService.RequestReview(path, detail.number, login, requested)
      onRefresh()
    } catch (err: unknown) {
      const asked = requested ? "request a review from" : "withdraw the review request for"
      toast.error(`Couldn’t ${asked} ${login}: ${errorText(err)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
      <span className="text-muted-foreground">Reviewers</span>
      {roster.length === 0 && <span className="text-muted-foreground">Nobody yet</span>}
      {roster.map((reviewer) => (
        <ReviewerChip key={reviewer.login} reviewer={reviewer} />
      ))}
      {/* A closed or merged pull request takes no new reviewer: GitHub refuses
          the request, so the control goes rather than failing on click. */}
      {detail.state === "OPEN" && (
        <DropdownMenu
          onOpenChange={(open) => {
            if (open) {
              void load()
            }
          }}
        >
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="sm">
                <UserPlus />
                Request review
                <ChevronDown />
              </Button>
            }
          />
          <DropdownMenuContent align="start" className="max-h-80 overflow-y-auto">
            {candidates === null ? (
              <DropdownMenuItem disabled>Reading who can review…</DropdownMenuItem>
            ) : pickable.length === 0 ? (
              <DropdownMenuItem disabled>Nobody else can review this</DropdownMenuItem>
            ) : (
              pickable.map((candidate) => (
                <DropdownMenuCheckboxItem
                  key={candidate.login}
                  checked={pending.has(candidate.login)}
                  disabled={busy}
                  onCheckedChange={(next) => void toggle(candidate.login, next)}
                >
                  {candidate.login}
                  {candidate.name !== "" && (
                    <span className="text-muted-foreground">{candidate.name}</span>
                  )}
                </DropdownMenuCheckboxItem>
              ))
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}

// What each verdict reads as on a chip. A reviewer with no verdict yet is the
// default below: the review is still owed.
const VERDICTS: Record<string, { icon: LucideIcon; tone: string; label: string }> = {
  APPROVED: { icon: Check, tone: "text-emerald-500", label: "approved" },
  CHANGES_REQUESTED: { icon: X, tone: "text-destructive", label: "requested changes" },
  COMMENTED: { icon: MessageSquare, tone: "text-muted-foreground", label: "commented" },
  DISMISSED: { icon: CircleDashed, tone: "text-muted-foreground", label: "review dismissed" },
}

const AWAITING = { icon: CircleDashed, tone: "text-muted-foreground", label: "review requested" }

function ReviewerChip({ reviewer }: { reviewer: PullRequestReviewer }) {
  const verdict = VERDICTS[reviewer.state] ?? AWAITING
  // A team only ever arrives on the roster as a pending request — a verdict
  // belongs to the person who left it — so its own glyph hides nothing.
  const Icon = reviewer.isTeam ? Users : verdict.icon
  return (
    <span className={cn("flex items-center gap-1 font-medium", verdict.tone)} title={verdict.label}>
      <Icon className="size-3.5" />
      {reviewer.login}
    </span>
  )
}
