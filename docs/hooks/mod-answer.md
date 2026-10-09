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
- `text`: required, the turn's final visible text, never blank. The mod cuts it
  at 16,000 UTF-16 units and marks the cut `[truncated]`, as it does an
  `ask`'s answer.

The body is held to 64 KiB, as an ack is.

Responses: `204` taken · `401` invalid token · `400` invalid body · `405` not a
POST.

A `204` does not say the answer went anywhere. lich answers the errand only
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
spawned with `LICH_SUBAGENT_CARDS=off` (README, Shared transport), which every
`--subagent` session is. A turn is reported when all of these hold, measured on
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

## lich server side

- **Endpoint** (`internal/terminal/modanswer.go`, `transport.modAnswer`):
  validates the token and body (`parseModAnswer`), read up to
  `modAckBodyLimit`, and hands the text to the relay (`relay.WorkerAnswered`,
  wired with `terminal.SetWorkerAnswer`).
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
- **A turn with nothing to say answers nothing.** A blank final message, an
  aborted turn, an API error and a refusal post nothing, and the errand stays
  open while the worker runs: the caller sees the worker's card, not a report.
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
