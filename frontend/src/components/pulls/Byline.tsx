import type { ReactNode } from "react"
import { updatedAgo } from "@/lib/pulls/pull-request-list"

interface BylineProps {
  author: string
  /** gh's ISO timestamp, rendered as the age everything else on this screen
   * reads in. */
  date: string
  /** What sits between the name and the age — a review's verdict. */
  children?: ReactNode
}

// Who said it and when: the line above every comment, thread message and review
// on the Pulls screen. GitHub leaves the author null for an account that has
// been deleted since, which arrives here as an empty login.
export function Byline({ author, date, children }: BylineProps) {
  return (
    <div className="flex items-baseline gap-2 text-xs text-muted-foreground">
      <span className="font-semibold text-foreground">{author || "someone"}</span>
      {children}
      <span>{updatedAgo(date)}</span>
    </div>
  )
}
