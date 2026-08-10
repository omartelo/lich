# Contract: the `lich` CLI

An agent running in a lich session has a shell and nothing else — no window, no
RPC client, no knowledge of this app. The `lich` command is the surface it uses
to reach the sessions beside it: list them, hand one a task, answer one that
handed it a task, open a new one.

It is not only for agents. The same commands run from any shell on the machine —
a script, a scheduled job, the user's own terminal — which is what makes lich
automatable without a provider in the loop. `--json` is there for exactly that.

Agents mostly do not run it as a command. lich registers the same operations as
**MCP tools** for the providers that can be told at spawn, so they are in the
agent's own tool list without anybody being told anything — see
[Registration](#registration). The command line stays because a tool list only
reaches an agent: a script, a scheduled job or a plain terminal has no tool
list, and the reply path has to work even where no server was registered.

This document is **contract-first**, exactly as [`hooks/`](hooks/README.md) is.
A flag, a tool name or an output line that moves here can break the companion
plugin ([`omartelo/lich-plugin`](https://github.com/omartelo/lich-plugin)) and
whatever the user automated on top, neither of which this repo can see. Move the
contract first.

## Why an agent writes its own answer

lich delivers a message by typing it at the target's prompt, exactly as the user
would. It never reads the answer off the terminal: a TUI's output is boxes,
spinners and ANSI, and parsing it would mean a parser per provider, each hostage
to that provider's next release.

So the message lich types **asks the agent to report back** — with the
`reply_to_session` tool where lich registered one, and always by running
`lich reply <ticket>`, which needs nothing registered at all. The answer is
written by the agent that produced it. That is the whole reason this works on
every provider lich runs, rather than only on the ones that ship a messaging
channel of their own: anything that can run a shell command can answer.

## Transport

Every command reads the coordinates lich already injects into each PTY (see the
hooks [README](hooks/README.md)) and posts to the same loopback listener the
window uses:

| Var               | Purpose                                          |
|-------------------|--------------------------------------------------|
| `LICH_PORT`       | endpoint port (loopback)                         |
| `LICH_TOKEN`      | auth token (`?token=`)                           |
| `LICH_SESSION_ID` | the card this command is running in — the sender |
| `LICH_BIN`        | the path of the lich this session belongs to     |

```
POST http://127.0.0.1:${LICH_PORT}/rpc/relay.<Method>?token=${LICH_TOKEN}
POST http://127.0.0.1:${LICH_PORT}/rpc/spawn.<Method>?token=${LICH_TOKEN}
```

**Outside a session** those are absent, and the command finds the running lich
on its own: it reads the coordinates from `runtime.json` (`internal/singleton`,
mode 0600), the same file `install.sh` reads to reach a running lich for
`/restart`. `LICH_DEV` selects the dev instance's file, as everywhere else. With
no lich running at all, every command exits 1 with `lich: no lich is running`.

The environment wins over the runtime file, and that order is load-bearing: a
session belongs to the lich that spawned it, and on a machine running a daily
driver beside a `task dev` build the runtime file names only one of them.

Such a caller has **no session of its own**, so `LICH_SESSION_ID` is empty. That
is passed on rather than rejected: the relayed message is then attributed to the
command line instead of to a session (see below).

**Always call it as `"$LICH_BIN"`, not as `lich`.** An installed lich is on
`$PATH` and a `task dev` build is not; on a machine running both, the bare name
resolves to whichever comes first rather than to the lich the session belongs
to. `LICH_BIN` is the one that is right by construction, which is why the
relayed message spells the reply command that way.

## Commands

Every command prints its result on stdout and its failure as one `lich: …` line
on stderr, exiting 0 or 1. Anything that is not a subcommand below — including
`lich -- <chromium flags>` — opens the app instead.

`--json` on `sessions`, `send`, `wait` and `open` replaces the prose with one
JSON line: the peer array, the result object and the session object exactly as
this document describes them. An empty roster is `[]`, never `null` — a script
should not have to tell those apart.

### `lich sessions [--json]`

Lists the live sessions the caller can address, tab-separated with a header:

```
session	project	provider
docs	lich	codex
api	revu	crush
```

`No other live sessions.` when there are none. A session is listed only while a
process is running in it: a card whose terminal was never opened has nothing to
type at.

### `lich send [--project <name>] [--timeout <seconds>] <session> <prompt>`

Types `<prompt>` at `<session>`'s prompt, submits it, and waits.

- `<session>` is the label on the card, **or** the name that session answers to
  in Claude Code's peer roster (`myrepo-a1b2`, the one lich passes as `--name`
  and a mention writes at a prompt). Both name the same session and both are
  accepted, because an agent holding one of them should never have to know which
  door it is standing at — offering one name and accepting the other is what
  once made an agent treat a single session as two and use both channels at
  once. The label wins a tie.
- Labels are unique within a project, not across them: a label two live sessions
  answer to is an error naming both, and `--project` is what narrows it.
  Guessing which session a prompt lands in is the one mistake this must not
  make. A name that matches nothing comes back with every live session's two
  names, so a miss is one step from a hit.
- `--timeout` bounds the wait in seconds. Default 100 — under the 120s an
  agent's shell tool typically allows a command, so the wait ends in an answer
  this side controls rather than a killed process. Capped at 30 minutes.
- Answered: prints the answer alone, exit 0.
- Not answered in time: the message was still delivered, so it says so and hands
  back a ticket, exit 0. **The answer is not lost by giving up on it** — see
  below.
- **Answered somewhere else**: the target worked through the request and ended
  its turn without replying here — it answered over its provider's own channel,
  or out loud to whoever was watching. The wait ends there rather than running
  its clock out, and says the answer is in that session and has to be read
  there. The window raises a toast that opens the card.

```
docs is still working. The message was delivered; its answer will be typed at
the sending session's prompt when it arrives. To hold the line for it instead:
  lich wait a1b2c3d4
```

A prompt is capped at 8192 characters and stripped of control characters — the
text is typed into a terminal, and an escape sequence inside it would stop being
text.

### `lich wait [--timeout <seconds>] <ticket>`

Waits again on a ticket a previous `send` handed back. Same output as `send`.
A ticket that was answered, or that nobody answered within an hour, is gone and
waiting on it is an error.

### `lich reply <ticket> <answer>`

Hands `<answer>` to the session waiting on `<ticket>`; prints `Answer sent.`
This is what a relayed message asks the receiving agent to run. An answer is
capped at 64 KiB. Replying twice to one ticket is an error — the first answer
already went home.

### `lich open [--project <name>] [--kind <provider>] [--worktree <branch>] [--base <branch>]`

Opens a new session, starts it, and prints the two names it is addressed by:

```
Opened session "auth-fix" (claude) in project "lich", in worktree /home/you/.local/share/lich/worktrees/1a2b/auth-fix.
It answers to "auth-fix" and to "auth-fix-9f8e". It is still starting up — give it a moment before you send it work.
```

- `--project` names the project it lands in; the default is the caller's own.
  **Outside a session there is no default** and the project must be named — the
  error lists what is open.
- `--kind` is what the session runs: any provider id (`claude`, `codex`,
  `opencode`, `omp`, `crush`) or `shell`. The default is the caller's own
  provider, so an agent opening a worker gets another of itself; a caller that
  is not a session at all gets Claude Code.
- `--worktree` is the **branch name** of a new git worktree, created off
  `--base` (the project's current branch by default) under the app data dir. The
  session is rooted there and labelled after it, and the project's worktree
  setup script (Settings › Project) runs in its terminal before the provider,
  exactly as when the window creates one. Without it the session opens in the
  project's own directory, beside the caller's.
- `--base` is checked against the repository's branches — local, remote
  (`origin/…`, fetched and tracked), or one another worktree already holds. A
  base that is not a branch is refused rather than resolved: git would happily
  branch off a typo that names a revision, leaving a checkout nobody asked for.
- A session without a worktree is labelled from the project's own counter
  (`Session 4`), continuing the numbering the window uses.

The card appears in the sidebar **without taking focus** — nobody in front of
the window asked for this session — and its PTY is started by lich itself rather
than by the first person to look at the card. That is what makes it addressable:
`lich sessions` lists only sessions with a process in them, so a session nobody
opened would be one nobody could send to.

Failure is one line on stderr and exit 1. A worktree that could not be created
leaves nothing behind — no row, no card. A session whose *terminal* failed to
start keeps both, and says so, because the card is the only place its error can
be read.

### `lich mcp`

Serves the commands above as MCP tools over stdio: one JSON-RPC 2.0 message per
line, `initialize` / `tools/list` / `tools/call` / `ping`. stdout carries
protocol and nothing else.

You rarely run this by hand — lich registers it for the providers that can be
told at spawn (see below). Run it yourself only to point some other MCP client
at lich.

| Tool | What it does |
|------|--------------|
| `list_sessions` | The live sessions that can be given work, as JSON. |
| `send_to_session` | `session`, `prompt`, optional `project` and `timeout_seconds`. |
| `wait_for_answer` | `ticket`, optional `timeout_seconds`. |
| `reply_to_session` | `ticket`, `answer` — what a relayed message asks for. |
| `open_session` | optional `project`, `kind`, `worktree`, `base` — `lich open`. |

A tool that fails answers with `isError` and the reason as text, not a JSON-RPC
error: the agent should read what went wrong and act on it, not lose the turn.

## Registration

An MCP tool is in the agent's own tool list from the first turn. That is the
point of it — a command line only helps an agent that has been told the command
exists, and the person who would have to be told is the one who does not read
release notes.

So lich registers the server itself, per session, at spawn
(`internal/terminal`, `mcpArgs`), for the providers that can be told on their
own command line (`providers.AcceptsMCPServer`):

| Provider | How | Registered |
|---|---|---|
| Claude Code | `--mcp-config` with a JSON string, no file on disk | yes |
| Codex | `-c mcp_servers.lich.command=…` and `…args=["mcp"]` | yes |
| opencode · oh-my-pi · Crush | no flag for it — Crush's whole flag list is cwd, data-dir, session and debug | no |

The three without a flag would need lich to write a config file it does not own,
so it does not: their sessions use the command line above.

Two rules the registration must keep:

- **Never `--strict-mcp-config`.** It would make Claude Code ignore every other
  MCP configuration — the user's own servers dropped for the sake of lich's.
- **Order is not free.** `--mcp-config` is variadic and reads whatever follows
  it as another config path, so for Claude Code it goes last. Codex spells
  resume as a subcommand and its global options must precede it, so for Codex it
  goes first. `providerArgs` owns both.

**Why stdio and not HTTP.** lich already serves HTTP on the loopback listener,
and registering a URL would have been less code. But the registration lands in
the provider's **argv**, and `/proc/<pid>/cmdline` is readable by any user on
the machine while `/proc/<pid>/environ` is not. A URL registration means
`?token=` in argv. The stdio one names a binary and a subcommand and carries no
secret at all — the server inherits the coordinates from the PTY's environment,
where they already were.

## The answer comes back the way the request went out

A relayed task can take minutes; a tool call cannot sit still that long (Claude
Code detaches one that runs past 120 seconds). So waiting is optional here.

When the answer lands and **nobody is still holding the line**, lich types it at
the sending session's own prompt and submits it — the same bracketed paste, the
same delay, the same submit that carried the request to the target:

```
[lich] Answer from session "docs", to the request you sent it:

3 failures in foo_test
```

That is what makes a long errand work without polling: the asker carries on, and
the answer arrives when it exists. A caller that *is* still waiting carries the
answer out itself and nothing is typed — delivering both would deliver twice.

A sender that is not a session — the `lich` command from a script — has no
prompt to answer at. Such a caller waits, or does without.

## The message a target receives

```
[lich] Message from session "<sender label>", not from your own prompt.

<prompt>

When you have an answer, send it back by running:
  "$LICH_BIN" reply <ticket> "<your answer>"
Whoever asked is blocked waiting on that command.
```

A caller with no session of its own — the CLI run from a script or a plain
shell — opens with `Message relayed by the lich command line` instead. The
distinction is the point: the receiving agent must not read either as its user
speaking, and the two are not the same kind of "not your user".

A target whose provider has the registered server is offered the tool first:

```
When you have an answer, send it back with the lich tool `reply_to_session`
(ticket <ticket>), or by running:
  "$LICH_BIN" reply <ticket> "<your answer>"
```

The command is named either way. A session whose shell is locked down would
otherwise have no way to answer at all, and the reply path exists for the
receiving agent only because this text describes it.

## lich server side

- **Spawn** — `internal/spawn`: opens a session for a caller that is not the
  window. It is the one place that does all four things a session is at once —
  the worktree (`internal/project`), the row (`internal/store`), the PTY
  (`internal/terminal`) and the card (the `session-opened` event) — because no
  existing package owns more than one of them. The window does the same four in
  `frontend/src/providers/projects.tsx`, in its own order.
- **The card** — `session-opened` carries the whole session
  (`{id, projectId, project, label, name, kind, path, nextSeq}`) rather than an
  id to look up: the row is already written and the PTY is already running, so
  the window has nothing to fetch and nothing to spawn. `adoptSession`
  (`frontend/src/lib/session/sessions.ts`) appends it **without focusing it** —
  an agent opening three workers must not drag the view along three times.
- **Relay** — `internal/relay`: resolves a label to a live session, composes the
  message above, types it through the terminal service, and holds the ticket the
  answer comes back on. Tickets live in memory: one exists for as long as its
  errand does, and a lich that restarted has no PTY left to answer into.
- **UI push** — the relay emits the global app event `session-relay`
  (`{id, peer, direction}`) for **both** ends when a message lands in a PTY, and
  again with an empty direction for both when the errand closes. It is raised
  after the write, never before: a mark that outlived a delivery which never
  happened would be a card claiming something untrue. A caller with no session
  of its own gets no mark — there is no card to put one on — and the target's
  `peer` is then empty, which the card words as the command line.
- **Store and card** — `frontend/src/lib/session/session-relay-store.ts`, keyed
  by session id, read by `SessionCard`. The row outranks the tool line while a
  request is open: it explains the whole turn, and the tool comes back when the
  errand closes. The store also clears on `idle` (SessionEnd), which the relay's
  own clear cannot reach — a session that ended can answer nothing, and its
  ticket would otherwise hold the mark for an hour.
- **Delegating from the window** — `delegate-targets.ts` lists every other
  session, `delegate-prompt.ts` decides what is typed: a plain request where the
  sender has lich's tools, the spelled-out command where it has none. That split
  mirrors `providers.AcceptsMCPServer` and has to keep mirroring it.
- **Delivery** — `internal/terminal`, `Service.Write`: the same PTY write the
  window's keyboard input takes. The message is wrapped in bracketed paste
  (`\x1b[200~`…`\x1b[201~`) and followed by a carriage return, so a multi-line
  message arrives as one prompt instead of one submission per line.
- **Liveness** — `internal/terminal`, `Service.Live`: whether a card has a PTY
  behind it right now.
- **CLI** — `internal/cli`: argument parsing and the RPC client. It answers
  before `main` opens the database or takes the log file, so a command never
  races the lich it is talking to. `dispatch` is the seam its tests use: a
  client built by `Run` resolves the runtime file of the lich actually running,
  and a test that reached it would deliver into a real session.
- **Finding a lich from outside** — `internal/singleton`, `Read`: the runtime
  file the CLI falls back to when the environment carries no coordinates.
- **MCP server** — `internal/cli/mcp.go`: the stdio transport, the tool table
  and the handshake. Adding a tool is one entry in `mcpTools`, which is what
  registering once buys — every tool lich grows later reaches every provider
  that took the registration, with no further per-provider work.
- **Registration** — `internal/terminal`, `providerArgs` / `mcpArgs`, beside
  `nameArgs` and `resumeArgs`; which providers accept one is
  `providers.AcceptsMCPServer`. The names it registers under
  (`relay.MCPServerName`, `relay.MCPSubcommand`, `relay.ToolReply`) live in the
  relay because both this and the composed message need them, and only that
  direction closes no import cycle.
- **Env** — `internal/terminal`, `Service.sessionEnv`: exports `LICH_BIN`
  alongside the hook coordinates.

## Two channels, one address space

A Claude Code session can reach another one twice over: through its own peer
messaging, and through lich. That is not a conflict to be settled by a setting —
it became one only because the two named the same sessions differently, so an
agent handed a name could not tell which door it was standing at. On the first
real run one hedged and used both at once: it delivered over its own channel and
then waited on a lich ticket nobody would ever close.

Three things keep that from repeating, and none of them asks the user to choose:

1. **One address space.** The relay answers to the card label *and* to the peer
   roster name (`internal/relay/rostername.go`, the Go half of
   `frontend/src/lib/session/peer-name.ts` — a divergence there delivers a
   message to the wrong terminal). A mention that slips into lich's tool works
   instead of erroring.
2. **Both names travel.** `list_sessions` and every miss report the label and
   the roster name together, so an agent sees one session with two names.
3. **One route home.** The relayed message says the ticket is the only way back
   and that an answer sent any other way is lost; `reply_to_session` says the
   same.

When all three fail anyway, the fourth catches it: the target's own turn ending
closes the wait and points the user at that session. That path is the exception,
not a supported mode — an answer that goes there is one lich cannot hand to
whoever asked.

## Known ceilings

- **A relayed prompt carries the sender's reach, not the user's.** It is typed
  at the target's prompt and submitted, so the receiving agent acts on it under
  its own permissions — in a session running without permission prompts, that is
  every tool it has. This does not widen lich's trust boundary (`LICH_TOKEN` is
  already in every PTY, and any process in one can already write to any
  session), but it is the first feature that uses it, and there is no switch.
- **The tools cost context in every session, used or not.** Five tool
  definitions are in the prompt of every Claude Code and Codex session lich
  spawns, whether or not that session ever talks to another one. The command
  line costs nothing until it is called; the tools are what buy discovery, and
  this is the price. Anything added to `mcpTools` raises it for everyone.
- **Registration is silent and there is no switch.** A session lich spawns is
  handed the server without being asked. This does not widen the trust boundary
  — `LICH_TOKEN` was already in every PTY — but it does mean a user who never
  wanted this has no way to say so short of the provider's own MCP settings.
- **The card shows what is in flight, never what happened.** Marks live with
  their ticket and the ticket lives in memory, so a finished request leaves no
  trace and a reload starts blank — the same ceiling the status readout has.
- **Only live sessions are addressable.** A parked or never-opened card is
  invisible to `sessions` and unreachable by `send` — the window spawns a
  session's terminal on its first view and nothing here changes that. `open` is
  the exception, and only for the session it creates: it starts that PTY itself,
  which is why the card it leaves behind is one you can send to unseen.
- **A session is addressable before its agent can read.** `open` returns when
  the PTY exists, not when the provider inside it has finished starting — there
  is no signal for the latter that every provider gives. A message sent in that
  window is written into a TUI that may not be reading yet, and lich cannot tell
  that apart from a delivered one (delivery is proven, receipt is not). Hence
  the line the command prints; opening and sending are two steps on purpose.
- **`open` starts the PTY at 80×24.** Nothing is watching it, so there is no
  terminal to measure. The window resizes it the first time the card is viewed,
  and a TUI that drew itself into the smaller grid redraws — but output produced
  before that view was wrapped for 80 columns, and the replayed scrollback keeps
  those wraps.
- **An opened session takes the project's active slot.** The row is written the
  way the window writes one, so the project's `active_session_id` moves to it —
  the card is not focused now, but a reload lands on it. Two of them and the
  last one wins.
- **Delivery is proven, receipt is not.** `send` knows the bytes reached the
  PTY. Whether the target's TUI queued them, and whether its agent ever runs the
  reply command, is what the wait and the ticket are for. A provider that does
  not honour bracketed paste would read the message's newlines as submissions.
- **A busy target is written to anyway.** Every provider lich spawns queues text
  typed mid-turn and submits it when the turn ends, so the message waits its
  turn rather than interrupting one.
- **The answer is the agent's summary, not the transcript.** Nothing reads the
  target's output, so what comes back is exactly what that agent chose to type
  into `lich reply` — and an agent that never runs it is indistinguishable from
  one still working.
