# Contract: mod answer

Lets a subagent worker (`lich open --subagent`) answer the task it was handed
the way a native subagent does: its last message is its result. The worker's
Claude Code **mod** (the same one as [mod-control.md](mod-control.md), shipped
by the companion plugin) posts the final text of a turn that ended with nothing
left running, and lich answers the worker's open subagent errand with it, by
the same path as `lich reply` and `reply_to_session`.

A worker whose mod does this is handed its task without the ticket and the
reply instructions a relayed task otherwise carries: just the task, under a
line naming the session that asked.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/mod/answer?token=${LICH_TOKEN}
Content-Type: application/json
X-Lich-Plugin: <plugin release>

{"session_id": "<LICH_SESSION_ID>", "text": "Rewrote docs/cli.md; the tests pass."}
```

- `session_id`: required, `LICH_SESSION_ID`.
- `text`: the turn's final visible text, never blank. The mod cuts it at
  16,000 UTF-16 units and marks the cut `[truncated]`, as it does an `ask`'s
  answer.
- `unanswered`: why a turn with nothing left running ended without an answer
  (see Unanswered turns below): `blank`, `error` or `refusal`.

A body carries exactly one of `text` and `unanswered`. Any other value of
`unanswered` is refused, so a mod newer than its lich hears `400` rather than
reporting into nothing.

```
{"session_id": "<LICH_SESSION_ID>", "unanswered": "refusal"}
```

The body is held to 64 KiB, as an ack is.

Responses: `204` taken · `401` invalid token · `400` invalid body · `405` not a
POST.

A `204` does not say the report went anywhere. lich answers the errand only
when the session has exactly one open subagent errand, and ignores the report
otherwise: an ordinary session, a worker whose errand was already answered
(the first answer wins, whether it came from here or from the worker calling
`reply_to_session` itself), and one holding two subagent errands, where nothing
says which one the text belongs to.

It also ignores the report of a worker still awaiting an outcome of its own:
an errand it sent to another session (a subagent card of its own, or a
`send_to_session` that handed back a ticket) is still open, or a result for it
has not been read by a turn of its yet. None of those is in Claude Code's
`background_tasks`, so the turn that handed the work off ends with the list
empty and reports "I handed it off". The outcome reaches the worker's prompt
and starts the turn whose report answers the errand.

Both sides test against the payloads in
[`fixtures/mod-answer.jsonl`](fixtures/mod-answer.jsonl).

## Event → report mapping

| Claude Code mod                                   | Codex hook           | Antigravity hook     | opencode event       | oh-my-pi event       | Crush hook           | Cursor CLI hook      | Kiro CLI hook        |
|---------------------------------------------------|----------------------|----------------------|----------------------|----------------------|----------------------|----------------------|----------------------|
| `classic.Stop` on the main loop, then `turn.complete` | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) |

The mod posts once per main-loop turn, and only from a worker: a session
whose `LICH_SUBAGENT_DEPTH` is above 0 (README, Shared transport), which every
`--subagent` session is. A mod from 0.19.0 under a lich that sets no depth, and
every mod before 0.19.0, takes `LICH_SUBAGENT_CARDS=off` for that instead, which
lich then sets on every worker. A turn is reported when all of these hold, measured on
Claude Code 2.1.289:

- `classic.Stop` fired without an `agent_id` (the main loop, not a subagent or
  an engine fork: `classic.SubagentStop` fires for those, and a background
  subagent lists itself as still running there).
- Its `background_tasks` is empty. A shell, subagent, monitor or workflow still
  running means the turn handed work to the background; Claude Code resumes in
  a new turn when it finishes (its prompt's origin is `task-notification`), and
  that turn's `Stop` is the one with nothing listed.
- Its `last_assistant_message` is not blank. It equals the `answer` of the
  `turn.complete` that follows.
- That `turn.complete` is not aborted and its `reason` is not `aborted`,
  `error` or `refusal`.

## Unanswered turns

A worker's turn that ended with nothing left running and no answer posts
`unanswered` instead, so the caller hears `unanswered` rather than waiting on
an errand that will never be answered. The mod posts it once per main-loop
turn, from a worker only, on the same conditions as an answer except the last
two:

| `unanswered` | The main-loop turn ended | `background_tasks` read from |
|--------------|--------------------------|------------------------------|
| `blank`      | `classic.Stop` with a blank `last_assistant_message`, and the `turn.complete` after it not aborted | that `classic.Stop` |
| `refusal`    | `turn.complete` with `reason` `refusal` | the main loop's `classic.Stop` of the same turn |
| `error`      | an API error: Claude Code's `StopFailure`, or `turn.complete` with `reason` `error` | see the open point below |

The rule all three share: **post only when the main-loop turn ended with
`background_tasks` empty.** A turn that handed work to the background is
resumed by Claude Code when that work finishes, and the resumed turn is the one
that answers or posts `unanswered`. Posting for the turn that handed it off
would tell the caller the errand failed while the worker is still on it.

An aborted turn (Esc in the worker's card) posts nothing. Stopping a worker's
turn is its user taking it over, and the turn they start next is the one that
answers.

**Open point for the plugin: where `background_tasks` is for an API error.**
Not measured on a live failure. Read off the Claude Code 2.1.296 bundle, the
`Stop` hook input is built with `background_tasks`, while the `StopFailure` one
carries `error`, `error_details` and `last_assistant_message` and no
`background_tasks`; and a turn an API error ended fires `StopFailure` instead
of `Stop` (docs/hooks/session-state.md). The plugin side measures where a
failed main-loop turn's background list can be read, if anywhere, before it
posts `error`. Until it can tell the list was empty, it posts no `error`: a
failed turn reports nothing, as before.

lich ignores `unanswered` exactly where it ignores an answer (one open subagent
errand, nothing of the worker's own still out), and also when the errand has
already ended, and `error` while lich has parked the worker's continuation
after a usage limit, since that continuation is the turn that answers.

## lich server side

- **Endpoint** (`internal/terminal/modanswer.go`, `transport.modAnswer`):
  validates the token and body (`parseModAnswer`), read up to
  `modAckBodyLimit`, and hands the text to the relay (`relay.WorkerAnswered`,
  wired with `terminal.SetWorkerAnswer`).
- **Unanswered** (`relay.WorkerUnanswered`, wired with the answer): ends the
  one open subagent errand as `unanswered`, with a line naming the reason as
  its text, the way a turn ending without a reply ends an ordinary errand: a
  caller holding the line hears it at once, and one that is not finds it in
  its inbox. The errand stays answerable by its ticket, so a worker its user
  steers to an answer afterwards still reaches the caller. The worker is not
  closed: its card holds the turn that failed.
- **Answer** (`relay.WorkerAnswered`): finds the one open subagent errand at the
  session, delivered or lapsed, and answers it through `Reply`, so the caller
  gets the report the way it gets one sent with `reply_to_session`, unless the
  worker still awaits an outcome of its own (`relay.awaitsOutcomeLocked`). A
  result counts as read once a turn of the worker's starts after the worker was
  told about it (`inboxEntry.seen`). A worker in its caller's checkout is
  closed once the answer is in, its turn has ended and nothing it sent is still
  out, whichever lands last.
- **A worker's own worker closed** (`relay.SessionClosed`): the stop is filed in
  the inbox of a caller that is itself a worker somebody waits on, so it
  resumes and answers instead of waiting on an errand that ended in silence.
  Any other caller has it filed too unless it closed the worker itself.
- **The task** (`relay.handOff`): a subagent errand handed to a worker whose
  mod polls from a plugin release that posts here (`agentplugin.ModAnswerRelease`)
  is composed without the ticket and the reply instructions, and is marked as
  answered by the mod: a turn that ends without an answer does not end that
  errand, since the turn handed work to the background and the answer comes
  when it resumes. Any other worker is handed today's text and answers through
  the ticket.

## Known ceilings

- **Claude Code only.** The other seven harnesses have no mod system; their
  workers are handed the ticket and answer with `reply_to_session` or
  `lich reply`.
- **An aborted turn answers nothing.** The errand stays open while the worker
  runs, and the caller sees the worker's card, not a report, until a later turn
  answers.
- **An API error is reported only once the plugin can read its background
  list** (Unanswered turns, open point). Until then the errand of a worker
  whose turn failed stays open while the worker runs.
- **A usage limit's `error` can land before lich parks the continuation.**
  The limit is read off the transcript on the turn's `done`, and nothing orders
  that against the mod's post. The caller then hears `unanswered` first and the
  report the continuation writes after it.
- **AskUserQuestion holds the worker without a Stop.** A worker that asks its
  user a question with that tool blocks until someone answers in its card, and
  no turn ends, so nothing is reported until then.
- **The task is composed when it is handed over.** A worker whose mod had not
  polled yet when its task went in is handed the ticket and the reply
  instructions; its mod still reports, and whichever answer lands first wins.
- **A mod older than 0.17.0 does not post here.** Its worker is handed the
  ticket, as before.
- **An errand the worker sent that ages out holds its answer.** An ordinary
  errand with nobody holding the line is dropped after an hour without
  activity (`ticketTTL`) and nothing reaches the worker's prompt, so its report
  waits for the next turn something else starts there.
