import { ProjectService } from "@/lib/rpc"
import type { PullRequestConversation } from "@/lib/api-types"
import { useRemoteResource } from "@/lib/use-remote-resource"

export type { PullRequestConversation }

const NO_CONVERSATION: PullRequestConversation | null = null

export interface ConversationState {
  conversation: PullRequestConversation | null
  loading: boolean
  error: string | null
  refresh: () => void
}

// usePullRequestConversation reads what has been said about one pull request:
// the reviews, its own comments, and the threads the diff renders inline.
//
// Deliberately unpolled. A review arrives when a person writes it, not on a
// cadence, so this rides what the screen already does — the Refresh button, and
// the window regaining focus — rather than adding a third timer beside the
// checks poll. A moved HEAD refetches because it can outdate a thread.
//
// A zero number is "nothing to look up": the conversation is addressed by pull
// request, and the screen has one only once the detail has resolved.
export function usePullRequestConversation(
  path: string,
  number: number,
  head: string,
): ConversationState {
  const { data, loading, error, refresh } = useRemoteResource(
    path && number > 0 ? `${path} ${number} ${head}` : "",
    () => ProjectService.PullRequestConversation(path, number),
    { empty: NO_CONVERSATION, refetchOnFocus: true, cache: `pulls.conversation ${path} ${number}` },
  )
  return { conversation: data, loading, error, refresh }
}
