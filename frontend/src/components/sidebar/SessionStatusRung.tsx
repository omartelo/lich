import {
  ArrowLeft,
  ArrowRight,
  CircleQuestionMark,
  Clock,
  CornerDownLeft,
  FoldVertical,
  Hourglass,
  Inbox,
  ListChecks,
} from "lucide-react"
import type { Session } from "@/lib/session/sessions"
import type { SessionStatus } from "@/lib/session/session-events"
import { toolGlyph } from "@/lib/session/tool-glyph"
import { toolLine } from "@/lib/session/tool-label"
import { useSessionInbox } from "@/lib/session/use-session-inbox"
import { useSessionLimit } from "@/lib/session/use-session-limit"
import { useSessionRelay } from "@/lib/session/use-session-relay"
import { useSessionWaitingReason } from "@/lib/session/use-session-status"
import { useSessionTodo } from "@/lib/session/use-session-todo"
import { useSessionTool } from "@/lib/session/use-session-tool"
import { useHandoffHeld } from "@/lib/terminal/handoff-store"
import { Trans } from "@/components/common/Trans"
import { useT } from "@/lib/i18n/i18n"
import { SessionLimitRung } from "./SessionLimitRung"

// The line under a card's label that says what the session is doing or waiting
// on. Its own component because each rung reads a store of its own, and the
// card only needs the line, not what decides it.
export function SessionStatusRung({
  session,
  status,
  origin,
  scheduledIn,
}: {
  session: Session
  status: SessionStatus | null
  origin: string
  scheduledIn: string | null
}) {
  const t = useT()
  // What the session is blocked on, when its provider's event had words for it:
  // the line the user reads to decide whether this is the card to open. "" from
  // a provider that reports the block and nothing about it.
  const waitingReason = useSessionWaitingReason(session.id)
  // The tool the turn is running right now, reported by the provider's pre-tool
  // hook: null outside a tool call, which is what keeps the card its usual size
  // whenever nothing is happening in it.
  const tool = useSessionTool(session.id)
  // The request this session has open with another, reported by the relay when
  // a message lands in a PTY and cleared when it is answered. null the rest of
  // the time, which is nearly always.
  const relay = useSessionRelay(session.id)
  // Text handed to this session — a pull request's conflict, the issue a
  // worktree was named after — still waiting for a prompt free enough to take
  // it. The wait can be minutes, and until it ends the card looks exactly like
  // one nothing was handed to (write-at-prompt.ts).
  const handoffHeld = useHandoffHeld(session.id)
  // How many results this session has waiting in the relay's inbox: results of
  // tasks it delegated, uncollected. Zero — the usual case — draws nothing.
  const inbox = useSessionInbox(session.id)
  const limit = useSessionLimit(session.id)
  // How far the agent got through the task list it wrote for itself. null for
  // most sessions: no list, a finished one, or a provider whose list lich
  // cannot read (terminal.todoReaderFor).
  const todo = useSessionTodo(session.id)
  const ToolGlyph = tool && toolGlyph(tool.name)
  // The two halves of the line, which are not always the two fields the report
  // sent: on Antigravity the tool's identity arrives in the detail, and drawing
  // it takes the whole line (tool-label.ts).
  const line = tool && toolLine(tool, session.mcpServers)
  const scheduledAt = session.scheduledAt ?? 0
  return (
    <>
      {/* One line, ten rungs: an open request, then a session blocked
          on the user, then a conversation being compacted, then a handoff waiting for this prompt, then a
          usage limit the turn stopped on, then results waiting to be collected, then how far
          a quiet card got through its task list, then the
          tool, then a prompt scheduled for later, then where the session
          came from. A request in flight
          explains the whole turn — a card working because another
          session asked it to, or one stalled waiting on a card
          elsewhere in the list. A block outranks the rest for the
          reason it needs words at all: the amber ring differs from the
          emerald one by hue alone, so nothing else on the card says the
          session wants an answer. A held handoff sits under the block
          and over the rest: text was handed to this session and is not
          at its prompt, which without a rung is indistinguishable from a
          click that did nothing. A usage limit sits there too, and for
          the same reason: the card is stopped on something, and only
          the rung says what and until when. The inbox sits under those and over
          the tool: mid-turn the live tool is the news, and the count
          takes the rung when the card goes quiet — the same rule the
          relay's own nudge follows. Task-list progress answers to that
          rule for a reason of its own: the tool changes at every step
          and is how a card proves it is moving, while a count can sit
          still for minutes, so it waits for the turn to end and then
          answers the question a finished card leaves open, which is how
          much the agent stopped short of. The origin is last precisely
          because it is never news: it says something that has been true
          since the card was created, so it surfaces only once the card
          is quiet, which is when somebody scanning the sidebar is
          working out where a card came from. A scheduled prompt sits
          just above it for half that reason: it is not what this session
          is doing, so it never takes the line from the turn — but it is
          the one rung about something that has not happened yet, and a
          quiet card is exactly where that is worth reading. Only one rung ever draws,
          so the card grows by one row at most. */}
      {relay ? (
        <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
          {relay.direction === "out" ? (
            <ArrowRight className="size-3 shrink-0" />
          ) : (
            <ArrowLeft className="size-3 shrink-0" />
          )}
          {relay.peer ? (
            <span className="truncate font-medium text-foreground">{relay.peer}</span>
          ) : (
            // Not a session, so not a label: the other end is the `lich`
            // command run from a script or a shell (docs/cli.md).
            <span className="truncate italic">{t("sidebar.sessionStatusRung.commandLine")}</span>
          )}
        </span>
      ) : status === "waiting" ? (
        <span className="flex w-full min-w-0 items-center gap-1 text-xs">
          <CircleQuestionMark className="size-3 shrink-0 text-tone-wait" />
          {/* The question takes the whole line when there is one: the
              amber glyph and the ring around the icon already say the
              session is waiting, so spending the width on saying it
              again would cost the card the only words on it the user
              cannot already see. Not every provider has them (see
              docs/hooks/session-state.md), and the generic line is what
              those fall back to. */}
          <span className="truncate font-medium text-tone-wait">
            {waitingReason || t("sidebar.sessionStatusRung.waitingOnYou")}
          </span>
        </span>
      ) : status === "compacting" ? (
        // Its own rung because the spinner beside it reads as a turn running,
        // and a compaction is several seconds of no tool line on a card that is not
        // stuck: the conversation is being folded into a summary.
        <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
          <FoldVertical className="size-3 shrink-0" />
          <span className="truncate font-medium text-foreground">
            {t("sidebar.sessionStatusRung.compacting")}
          </span>
        </span>
      ) : handoffHeld ? (
        <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
          <Hourglass className="size-3 shrink-0" />
          <span className="truncate">{t("sidebar.sessionStatusRung.handoffHeld")}</span>
        </span>
      ) : limit ? (
        <SessionLimitRung limit={limit} scheduledAt={session.scheduledAt ?? 0} />
      ) : status !== "busy" && inbox > 0 ? (
        <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
          <Inbox className="size-3 shrink-0" />
          <span className="truncate font-medium text-foreground">
            {t("sidebar.sessionStatusRung.inbox", { count: inbox })}
          </span>
        </span>
      ) : status !== "busy" && todo ? (
        <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
          <ListChecks className="size-3 shrink-0" />
          <span className="truncate">
            <Trans
              k="sidebar.sessionStatusRung.todo"
              params={{
                progress: (
                  <span className="font-medium tabular-nums text-foreground">
                    {t("sidebar.sessionStatusRung.todoProgress", {
                      done: todo.done,
                      total: todo.total,
                    })}
                  </span>
                ),
              }}
            />
          </span>
        </span>
      ) : tool ? (
        <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
          {ToolGlyph && <ToolGlyph className="size-3 shrink-0" />}
          {/* The detail gives its width up first and the name only once
              the detail has none left to give — which is what the lopsided
              shrink factor buys. Both still shrink, so neither can push the
              row past the card the way a name that refused to shrink did.
              The separator travels inside the detail so it leaves with it,
              instead of dangling after a truncated name. */}
          <span className="min-w-0 truncate font-medium text-foreground">{line?.label}</span>
          {line?.detail && (
            <span className="min-w-0 shrink-[9999] truncate font-mono">
              <span className="opacity-50">·</span> {line.detail}
            </span>
          )}
        </span>
      ) : scheduledAt ? (
        <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
          <Clock className="size-3 shrink-0" />
          {scheduledIn ? (
            <span className="truncate">
              <Trans
                k="sidebar.sessionStatusRung.scheduled"
                params={{
                  when: <span className="font-medium text-foreground">{scheduledIn}</span>,
                }}
              />
            </span>
          ) : (
            // Due and still here: the prompt is waiting on somewhere to
            // be typed — a setup script still running, a half-written
            // line at that prompt, a card whose terminal was never
            // opened — and it goes in the moment there is one.
            <span className="truncate">{t("sidebar.sessionStatusRung.waitingForPrompt")}</span>
          )}
        </span>
      ) : (
        // Quieter than the rungs above it, on purpose: muted throughout
        // and at normal weight, where an open request puts its peer in
        // text-foreground. The word "from" earns its place — an arrow
        // and a name alone read as traffic happening now, which is the
        // confusion this rung exists to end. Not a link either: the
        // parent may be closed, and a dead link is worse than a
        // sentence.
        origin && (
          <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
            <CornerDownLeft className="size-3 shrink-0" />
            <span className="truncate">{t("sidebar.sessionStatusRung.origin", { origin })}</span>
          </span>
        )
      )}
    </>
  )
}
