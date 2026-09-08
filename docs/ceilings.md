# Known Ceilings

Deliberate limits and shortcuts, and the traps they set. A bullet earns its place by naming something that breaks
work when nobody knows it and that the call site never shows. The mechanism and the history stay in the code and
`CHANGELOG.md`; this file is the trap alone.

- **Session cwd is polled** from the terminal's foreground process group (`internal/terminal/cwd.go`): a shell
  hosted elsewhere — tmux, ssh, a container — is beyond every reader, and the readout goes on naming a real
  local directory that is not where the user is, with nothing on screen saying so.
- **A project's gh account governs gh, not git**: `vcs.account` (`internal/project/ghaccount.go`) puts one
  account's token in `GH_TOKEN` for every gh call lich makes for that project. A push still rides the remote's
  ssh key and signs with the global `user.email`, so a PR can be *read* by one account and its commits *land*
  under another, with no error anywhere. The Version Control settings print both identities and never compare
  them: noreply forms, vanity domains and org aliases make a mismatch warning a false-positive farm. lich never
  writes `user.email`.
- **`LICH_WORKTREE_PORT` is reserved, never held** (`internal/terminal/worktreeport.go`): the number is a name the
  checkout owns, nothing binds it, and anything on the machine can take the port before the dev server starts. A
  Run card shortens that window rather than closing it — the process it starts is what binds the port, and
  whether the script even mentions the variable is the project's own business.
- **A Run card is a terminal, not a supervisor** (`internal/spawn/run.go`, `.lich/run-worktree.sh`): lich starts
  the command and stops caring. Nothing restarts it, nothing reports memory or CPU, and nothing dedupes — asking
  twice opens two cards, and the second one's own `address already in use` is the whole of the report. When the
  command exits the entrypoint wrapper leaves the user's shell in the same card with the error still on screen
  (`internal/terminal/entrypoint.go`), which is the retry: ↑, Enter. The script is one command, so a project with
  a web server and a worker beside it opens two cards or writes a script that backgrounds one of them.
- **The Run card is never started for you, and never on Windows** (`frontend/src/components/sidebar/SessionSidebar.tsx`):
  a fresh worktree's setup script is still installing dependencies in the agent's card when the checkout appears,
  and lich has no "setup finished" signal to hang an automatic start on — `terminal.Ready` answers a different
  question, going false again for every turn the agent takes. So the card is one gesture, which is also what
  keeps eight worktrees from meaning eight dev servers. On Windows the menu item is absent for the setup script's
  reason: `.lich/run-worktree.sh` holds sh, and a session there runs PowerShell, where `$LICH_WORKTREE_PORT`
  expands to nothing in silence.
- **The cost readout bills per `(session, transcript)`** (`internal/pricing`, `internal/terminal/usage_cost.go`): a
  conversation forked inside the PTY bills its copied history twice — lich's own resume continues the same
  transcript and is unaffected — and each sub-agent's own transcript is counted in, so one unreadable or
  unpriceable sub-agent withholds the whole session's number. A withheld number is marked `$—` on the footer
  only when the reason is standing (`costMiss.spoken` in `internal/terminal/usage_cost.go`): a transcript that
  merely could not be read this turn says nothing and keeps the last figure, so a reader cannot tell that
  absence from a session still on its first turn.
- **The session readout sits on three rungs, and only two providers reach the top one**
  (`internal/terminal/usage.go`, `usageSourceFor`). Which rung a provider is on is decided by what it writes
  down, not by what lich chose to read:

  | Provider | Rung | Why |
  | --- | --- | --- |
  | Claude Code | full readout | per-turn token counts, and a model whose window is named (`windowForModel`) |
  | Codex | full readout | the effective `model_context_window`, and a running `total_token_usage` |
  | oh-my-pi | cost only | a USD total on every assistant turn; no line carries a context window |
  | opencode | cost only | `session.cost` per conversation; no window recorded with it |
  | Crush | cost only | `sessions.cost` per conversation; no window recorded with it |
  | Kiro CLI | context only | a window and a per-request percentage; spend is metered in credits, not dollars |
  | Antigravity | nothing | conversation filed as SQLite lich has no reader for |
  | Cursor CLI | nothing | chat filed as SQLite lich has no reader for, search included |

  **Kiro CLI is the mirror image of the cost-only rung, and the only provider on its own one.** It records
  `context_usage_percentage` against a `context_window_tokens` (`internal/terminal/usage_kiro.go`), so the ring
  is real — but it files *every* token count as zero even on a turn that spent them, so the tooltip's
  "n / window tokens" is **derived from the percentage**, not read. It is Kiro's own percentage against Kiro's
  own window, and it is what keeps the tooltip agreeing with the ring; it is not a token count anybody measured.
  Its spend never appears at all: `metering_usage` is denominated in **credits**, whose dollar value depends on
  the account's plan, and lich's readout is dollars — so a Kiro session shows a context ring and no cost, and the
  CLI's own footer is the only place that credit figure can be read.

  The cost-only rung's footer shows the figure and nothing else: its usage event carries a zero window, and a
  zero window is what tells `FooterSession` to drop the ring and `SessionModel` to render nothing rather than a
  provider glyph beside a model nobody reported. **Those three figures are the providers' own arithmetic, never
  re-priced here** — they bill models `internal/pricing` has never heard of, so a second opinion would only be a
  second, disagreeing number — and each therefore inherits what its own accounting leaves out. oh-my-pi's total
  is the sum of its assistant turns, the same walk its own status line makes, so a `task` sub-agent's spend is
  missing from lich's figure exactly as it is from omp's. opencode files each sub-agent as a session of its own
  and does *not* roll it into the parent, so the read walks the `parent_id` chain; Crush does roll it in
  (`updateParentSessionCost`), so summing its children would bill them twice. **Both of those are measured
  behaviour of another tool's schema, not a promise it made** — the day either changes its mind about the
  rollup, the footer is silently wrong in one direction or the other, with nothing on screen saying so. A zero
  from any of the three is an answer and not a gap: it is what they record for a free model and for a
  subscription — while an oh-my-pi turn written with no total at all withholds the session's whole number and
  borrows the `unpriced-model` marker, whose tooltip then blames a network that would not have helped. That
  branch has never been seen in the wild and gets no reason of its own until it is. The two bottom-rung
  providers get no bullet beyond their row — a reader for either means a reader for its own SQLite schema, and
  neither schema is a contract anybody promised to keep.
  The Codex window in that table is 95% of the rollout's default or configured `model_context_window`, and
  Codex is the one rung lich prices itself. `internal/pricing/prices.json` bakes a fixed slice of OpenAI rates
  beside the Claude ones, copied by hand from the same LiteLLM table the refresh reads, so an offline Codex
  session shows a cost at the rate that shipped — **stale by however long it has been since the release**,
  until the refresh (`internal/pricing/pricing.go`) lands and overrides it. Nothing regenerates that slice, so
  what is left
  with no price at all is a model newer than the build, or one LiteLLM never priced — and with no network
  the refresh that would settle it never runs. A Codex conversation that ran `/model` has no cost from that
  turn on (`codexCostScan.mixed`): the one running total spans both models and the rollout never splits it,
  so no rate prices it — absent, the way an unpriced line is absent, rather than a number billed at
  whichever model happened to go last. **Those two absences are spoken; the staleness is not.** A session
  lich cannot price draws `$—` with a tooltip naming which of the two it is (`usageEvent.CostMiss`,
  `COST_MISS_REASON` in `frontend/src/lib/session/session-cost.ts`), while a cost billed at a rate the
  release froze reads as an ordinary figure with nothing beside it. The marker also names the reason and
  never the model or the turn, so a session whose sub-agent alone is unpriced looks like one whose every
  turn is.
- **Footer choices cannot add readings a provider never reports** (`frontend/src/components/FooterSession.tsx`):
  Appearance's global toggles preserve the gaps above — Claude Code and Codex full usage,
  oh-my-pi/opencode/Crush cost only, Kiro context only, and Antigravity/Cursor no transcript usage.
  Selecting a missing reading renders nothing, never a zero. The two ordered sides live in localStorage and
  disappear with a cleared browser profile. Cost still uses the backend setting because it controls whether
  transcripts are priced: when another profile disables it, an old layout cannot enable pricing just by
  moving an unrelated item. The editor waits for that setting before migrating old visibility choices.
  Hiding a reading hides its warning too. An item without data can occupy an editor slot while drawing
  nothing in the live footer — a PR on a branch without one, or an unsupported provider reading.
- **Hands-on time is read off three signals, and one of them is not universal**
  (`internal/terminal/handson.go`, `noteOutput`, `closableState`): the figure beside the cost
  counts the gap between consecutive signs of life in a session — any hook report naming it, a
  keystroke at its PTY, or its own output while a turn is open — and drops any gap longer than
  `handsOnIdleGap`. The output signal is the one that carries an unattended turn, and it is
  gated on the provider having reported `busy`, because a `tail -f`, a dev server or a TUI
  repainting would otherwise bill hours nobody worked. **Which turns get counted therefore
  depends on what a provider's hooks report at all**, and there are two rungs. On the top one —
  Claude Code, Codex, Antigravity, opencode, oh-my-pi — the turn opens, so a turn nobody
  touches is counted from its own output. On the lower one — **Crush and Cursor CLI** — no turn
  ever opens (`docs/hooks/session-state.md`), and the turn is counted through the reports its
  tool calls fire: Cursor's `PreToolUse`/`PostToolUse` state reports, which beat even though
  `closableState` refuses to publish them, and Crush's `/session-start`, which its only hook
  event (`PreToolUse`) fires once per tool call — measured 2026-09-03 against Crush 0.88.0, three
  tool calls in one turn, three POSTs. **What that rung cannot count is a turn that calls no tool
  at all**: a long answer written straight out, with no keystroke and no report between the
  prompt and the reply, is worth only the gap the prompt itself closed. A plain shell session is
  keystrokes-only by design and not a gap: there is no agent in it whose work could be missed.
  The trap is reading the number as exactly comparable across cards — a toolless turn is time on
  a Claude Code card and nothing on a Crush one. **The tooltip under the figure says which rung
  the session is on** (`handsOnDetail`, `frontend/src/lib/session/hands-on.ts`), in the same two
  sentences on both, by naming what the clock listened to: a turn, or a tool call. What it does
  not say is how much the rung cost this figure — nothing can know that without inventing the
  turns it never heard.
- **A split stage puts every pane on the visible cadence** (`internal/terminal/coalescer.go`,
  `frontend/src/components/TerminalHost.tsx`): the coalescer batches a *visible* session's output every
  8ms and a hidden one's every 250ms, and until the stage could divide, exactly one session per window
  was ever visible — `Service.SetVisible` had one true at a time. A wall of eight makes eight,
  deliberately: a pane nobody demoted is the entire point of opening it. So the window's hot path — the
  event bridge, its base64 and the WASM parse behind it — carries as many streams as there are panes,
  and anything reasoning about that cadence from the constants alone will read a number that was true
  for one terminal. The budget suite pins that adding a pane mounts one terminal and remounts none
  (`frontend/src/components/render-budget.test.tsx`); it cannot measure the cadence, because jsdom has
  no canvas to paint.
- **A dropped file has no path, so lich guesses it** (`internal/drop`): a file under neither the session directory
  nor home is *copied*, so an agent told to edit it edits the copy — and that copy is deleted 3 days on, so a path
  pasted into a prompt eventually stops resolving.
- **An interrupted turn is read off the keystrokes, not from the provider** (`internal/terminal/draft.go`,
  `hookstate.go`, `Service.noteInterrupt`): Claude Code, Codex and oh-my-pi all skip the hook that ends a turn
  when the user stops one, so lich publishes `interrupted` itself when a lone Ctrl+C or Escape reaches a session
  it knows is mid-turn. It is a guess made from bytes, and it has three edges. A provider session running a tool
  that owns the terminal — an editor opened through a shell command — takes Escape as the interrupt and clears
  the ring while the turn is still running; the next report from the provider puts it back. opencode does report
  its own abort, and reports it as the turn *finishing* (`session.status idle`), so an interrupted opencode card
  wears the same solid ring a completed turn does — nothing in the event says which happened, and the provider's
  own word outranks the keystroke. Crush reports no state at all, so it has no turn to end and the fallback never
  fires there. And an errand the relay delivered survives an interrupt on purpose: stopping a turn is not
  answering the request, so the sender keeps waiting for the target's next turn rather than being told the work
  is over.
- **A diff's context expander reads GitHub, one round-trip per gap** (`internal/project/filelines.go`):
  `FileLines` tries the local object first and asks the contents API when the clone does not have it, which
  on the Pulls screen is the normal case — the branch under review is usually not one this clone ever
  fetched. The call site is a promise from an RPC, so nothing there says a click costs a network round-trip
  and a rate-limit unit against the project's gh token. Anything that multiplies the clicks multiplies
  that: prefetching a file's gaps on mount, incremental ±20 stepping, or an "expand everything" control
  would each turn one reader's file into dozens of calls. The per-request line cap is the only bound, and
  a gap wider than it is several calls already. Two things follow from the same shape. The expander needs
  the revision the diff's new side stands at, so a source that cannot name one has no expander at all
  rather than a wrong one — a last-turn record from before `LastTurn.After` existed, or a pull request
  whose detail carries no commits. And there is no expanding *past the last hunk*: a unified diff carries
  no file length, so nothing here knows whether anything follows it, and an affordance drawn there would
  be a no-op on every file whose change reaches the end.
- **The Review panel's "Last turn" is a window of wall-clock time**
  (`internal/terminal/turnsnap.go`, `internal/project/turnsnap.go`): the panel brackets a turn with two
  `git write-tree` snapshots taken against an index of lich's own, so what it shows is everything that
  touched the checkout between the `busy` and the `done` — a formatter, an editor open beside lich, the
  user's own hands. Nothing in it can attribute a line, which is why the copy names the window and never
  the agent. Four traps follow. `add -A` obeys `.gitignore` (deliberately, so this and `DiffText` never
  disagree about which files exist), so a turn that only touched ignored files reports itself as having
  changed nothing. Every snapshot in the app runs on one FIFO worker, because git refuses a second `add`
  against an index another holds — so one session's first snapshot of a large checkout delays the next
  session's, and a queue past `snapQueueDepth` drops a job, costing that turn its record with only the log
  saying so. A checkout whose *first* snapshot fails is dropped outright and never asked again — the
  ordinary reason is a session opened outside a repository, but a transient failure at spawn reads the same
  and leaves that card with no last turn until it respawns. A pair read back at launch names loose objects
  no ref reaches, so a `git gc --prune` in that checkout between one run and the next leaves the panel
  reporting a failure rather than an absent turn. And the boundary is the session-state contract, so
  **Crush and Cursor CLI have no last turn at all**: neither reports a state
  (`docs/hooks/session-state.md`), so nothing ever opens or closes a window there and the switch is never
  drawn — a rule read off the session's own reports, not a list of providers, so it corrects itself the day
  either one starts reporting.
- **The recap beside that diff answers to a different clock, and to a different set of providers**
  (`internal/terminal/said.go`): the band reads the last thing the agent *said* out of the provider's own
  transcript, where the diff beside it brackets the window a turn ran in. The two agree once a turn has
  finished — which is the only time a diff is offered — but mid-turn the band still shows the previous
  turn's words with no date on them, beside a diff that reads "unavailable"; nothing in the panel says
  which turn is speaking, and only the card's spinner does. The read is a bounded tail for the reason the
  transcript search's is (`searchTailBytes`), so a turn whose closing words sit behind more than 4 MB of
  tool output shows none — the band simply does not appear, which is also what a turn that ended on a tool
  call looks like. And its provider list is *not* the one the switch above it is drawn from: seven of the
  eight are read, and the one that is not — Cursor CLI — is also one of the two with no last turn to begin
  with, so that gap is invisible today. Crush is the other, and its gap is not: its words are read and never
  drawn, because nothing above it ever opens a window to draw them in. Both would surface the moment either
  started reporting a state. Kiro CLI was on that list and silent in practice until the palette's search
  reused the same reader: its block payloads are typed per block kind, and declaring one as a string failed
  every line where the agent thought before it spoke — which is nearly all of them.
- **A finished turn is unread until its own card is watched** (`frontend/src/lib/session/session-status-store.ts`,
  `frontend/src/providers/projects.tsx`): the solid emerald ring means "back from the agent, not read yet", and it
  fades only for the session whose terminal is on screen **while the window has focus**. A card left focused in a
  background window keeps its ring solid until the window is touched again, which is the point, but it also means
  a browser that reports focus oddly never fades one.
- **A session close is a hang-up on Unix and a kill on Windows** (`internal/terminal/pty_unix.go`,
  `pty_windows.go`): closing a card signals the agent and gives it `closeGrace` to leave, so its exit path runs —
  hooks, transcripts, whatever it writes on the way out. A ConPTY has no signal to deliver, so the same close on
  Windows is still abrupt: an agent that saves state on exit loses it there, and nothing on screen says so.
- **The shell-env pty read is bounded by silence, not by the child's exit, and Windows never gets one**
  (`internal/terminal/shellenv_unix.go`, `runShellDump`): resolving PATH and friends runs the login shell on a
  pty rather than a pipe so an rc guarded on `[ -t 0 ]`/`tty -s` (nvm's and fnm's own init, among others) loads —
  but neither closing that pty from another goroutine nor `SetReadDeadline` interrupts a read already blocked in
  it (both measured directly against a blocked read: Close returns without error and the read stays parked in
  the kernel regardless, and the deadline is never enforced on this fd). So the read is driven from a goroutine
  free to outlive the call, and the result is decided by `shellDumpQuiet`: once real output has started, 300ms
  of silence is taken as "done", which is what lets an rc that backgrounds a job (an `ssh-agent`/`gpg-agent`
  eval, a prompt tool) after printing still hand back what it printed instead of paying the full
  `shellEnvTimeout` for nothing. Three edges follow. A background job that keeps printing on its own schedule
  (a spinner, a periodic notice) keeps resetting that timer, so a shell like that is bounded by the 5s ceiling
  instead of the 300ms one — the same outcome as a genuinely hung shell, and nothing distinguishes the two. A
  quiet-window or ctx timeout leaves the reader goroutine running for whatever still holds the pty, and it is
  never collected: the fd, the goroutine and the zombie child persist until that holder exits on its own or
  lich itself does, whichever comes first — one leak per lich launch that hits this edge, not a recurring one.
  And **Windows gets none of this**: `SHELL` is normally unset there, so `ResolveShellEnv` returns before
  `shellenv_windows.go`'s pipe-based `runShellDump` ever runs — but on a machine where the user sets it anyway
  (Git Bash, a POSIX-ish shell reached through PATH), that path still runs over a pipe, so an rc guarded the
  same way is skipped there exactly as it was everywhere before this fix, with no ConPTY wired in to close the
  gap.
- **A terminal entrypoint reaches shell sessions only, and reads a different rc on each OS**
  (`internal/terminal/entrypoint.go`): the menu item is absent on a provider card. On Linux and macOS the command
  runs through the shell's `-c`, which loads no interactive rc: an alias defined in `.zshrc` is not a command that
  can be an entrypoint, though `$PATH` is intact (`internal/terminal/shellenv.go`). On Windows it runs through
  PowerShell's `-EncodedCommand`, which *does* load `$PROFILE` first — so the same alias works there, and an
  entrypoint one user shares is not necessarily one the next can run.
- **The worktree setup script answers to the main checkout, never the new branch, and never runs on Windows**
  (`internal/project/setup.go`, `internal/terminal/setup.go`): improve `.lich/setup-worktree.sh` on a feature
  branch and fresh worktrees keep running the old one until the change reaches the checkout the project points
  at. And `.lich/setup-worktree.sh` is one file, versioned and shared by every checkout, holding sh — so a
  Windows session skips it rather than feeding it to PowerShell, which would run the leading words of every line
  as commands. A worktree opens there with its setup silently not done.
- **opencode files a fork under the parent's directory, not the new checkout's** (measured on 1.18.23): the
  copied session's `directory` column is the one the original ran in, and its `parent_id` is left empty, so
  opencode's own session list places a fork in the checkout it came from and records no lineage. lich's own
  card is right — it carries the worktree it was opened in, and `origin_session_id` names the parent — but the
  two disagree, and only lich's side is visible in lich.
- **git status is polled** — one shared poller per repository path (`frontend/src/lib/git/git-status-store.ts`); the
  lich plugin's `session-touched` hook nudges an immediate refresh.
- **The status badge has a single source** (`internal/project/status.go`): the branch, the HEAD commit and the
  dirty count all come out of one `git status --porcelain=v2 --branch` parse. A git release that changes those
  records breaks all three together rather than one at a time, and there is no second call left to disagree with
  the first — `Branch` still asks `symbolic-ref`, but nothing on the polled path calls it.
- **lich fetches on its own** (`internal/project/basestatus.go`, `internal/project/prconflicts.go`) — the only
  git writes lich makes outside the worktree flows: the base-branch readout moves remote refs in the user's own
  repository, unannounced, for as long as a card is on screen. Naming a conflicting pull request's files is the
  second, made when that pull request is opened on the Pulls screen: it fetches GitHub's `refs/pull/<n>/head`
  and the base branch. The remote fetched from is the one whose URL matches the pull request's own, origin when
  none does — lich reads gh's base repository off that URL rather than asking gh again, so a clone whose remotes
  all name a different repository than the pull request lives on falls back to origin and fails there.
- **A project opened from outside the window is matched by the spelling of its path**
  (`internal/project/project.go`, `Identify`; `internal/spawn/projects.go`): `lich open --project <dir>` normalizes
  what it is handed — `~` expanded, cleaned, refused unless absolute — and then matches that string against the
  open projects and the workspace's history. It stops there. A symlink to a directory, or a bind mount of it, is a
  different string and opens a second project on the same real directory: two rows, two ids, two tabs, and every
  path-addressed lookup lich makes (which project a checkout belongs to, which gh account its calls run as)
  answering with whichever the query reached first. The window's picker has the same hole — it takes the path
  zenity hands back — so resolving symlinks here would make the two surfaces disagree about which project a
  directory *is*, which is worse than the duplicate. The trap is a workspace where `/home/you/work` and
  `/home/you/src/work` are the same checkout: nothing on screen says the two tabs are one directory.
  It also moves a boundary that used to be the window's: until this, an agent could reach only the projects
  somebody had opened in front of it. Any directory on the machine is now one `open_session` away from a tab, with
  no confirmation on the way. It is not a new privilege — the agent already runs as you, in a shell that can read
  the same disk — but it is new visibility, and a card it opens there is a card with a PTY in it.

- **lich's own window offers the page every primary-modifier chord before Chromium runs it**
  (`shell/src/main.rs`): a CEF keyboard handler marks each Ctrl chord (Cmd on macOS) a keyboard shortcut, which
  is the only way a page can claim one of Chromium's *reserved* accelerators. Ctrl+T, Ctrl+W, Ctrl+Shift+T and
  the tab selectors otherwise run in the browser before the renderer is given the key, and no command handler
  sees them either: that is how Ctrl+Shift+T came to reopen a closed tab in a window with no tabs. What the page
  consumes is now gone from the browser, and a focused session consumes a lot, since xterm.js claims every
  Ctrl+letter: while you type in a session, Ctrl+W no longer closes the window and Ctrl+T no longer opens a tab.
  It also holds in the bundled window alone. Opened in a system browser (`--no-window`, or the fallback when the
  window fails), the browser keeps its accelerators and a chord it reserves never reaches lich at all.
- **Hidden sessions are serialized and destroyed**: 2MB replay rings on both sides
  (`frontend/src/lib/terminal/replay-buffer.ts` page-side, `internal/terminal/replay.go` backend-side — the latter
  survives a full page reload). Scrollback past the ring is gone, not paged. The snapshot carries only the modes
  xterm's SerializeAddon reads off `term.modes`; the ones an app relies on and it does not record are restored by
  hand in `frontend/src/lib/terminal/term-modes.ts`.
- **One socket carries every session's output** (`internal/terminal/writequeue.go`): the per-session outbox
  decouples the *producers*, never the wire. A window that stops reading stalls the connection's single writer,
  so after `wsWriteTimeout` (5s) every session's output switches to the `/events` bridge at once. That is a
  second socket, and the page reads the two independently: a frame that fell back can land ahead of one still
  sitting in the stalled connection's buffer, so output is never dropped but its order across that switch is not
  guaranteed. Until then the queue holds `writeQueueDepth` frames for the whole app, and two things still wait on
  it across sessions: a push that finds it full, and the flush a session runs before its exit banner so the banner
  cannot overtake its own last bytes.
- **Single instance via the pinned port**: the bind is the lock (`internal/singleton`); a duplicate launch hands
  its URL to the running window through Chromium's profile lock (`chromium.Focus`) and exits 0. With the
  bundled window the hand-off is a second `lich-shell` on the same profile: CEF forwards the command line to
  the running one, which raises the window it has (the kurogane fork's relaunch hook; without it CEF opened a
  second browser that kept lich alive after the window closed, #470), reports the forward as a failed
  initialise, the duplicate exits 1, and `focusRunning` logs one Warn per duplicate launch. Raising is
  best-effort: a Wayland compositor may only mark the window urgent. Focus never climbs the ladder — a
  fallback there would open lich a second time, in a system browser, on the profile the window owns — so a
  lich already running in the fallback browser is not focused by it either, and what a system browser does
  with the forwarded command line is its own.
- **lich appends to the agent's system prompt, for two providers only**
  (`internal/terminal/command.go`, `briefingFlags` → `relay.SpawnBriefing`): Claude Code and oh-my-pi are spawned
  with `--append-system-prompt` carrying lich's own briefing, so text the user never wrote is in every session's
  prompt and in `/proc/<pid>/cmdline`. Codex, Antigravity, opencode, Crush and Cursor CLI get nothing there — none
  has a per-spawn append flag, so for those five the point exists only in lich's MCP instructions, and behaviour
  between providers differs by that much.
- **A prompt in use is recognised from the bytes going in, never from the line itself**
  (`internal/terminal/draft.go`): a relayed message pastes at the prompt and sends an Enter behind it, so lich
  holds the delivery back while the user has unsent input there. What it counts is printable input since the last
  Enter, escape sequences skipped — it cannot see the line, so an edit that leaves it empty by another route
  (Ctrl+W, a click into the middle of it) reads as a draft that is still there, and a delivery waits out
  `draftIdle` for nothing. The stale-draft release is what keeps that a delay instead of a wedged relay. Two gaps
  stay open: input arriving between the paste and its Enter still rides along — a window that is `defaultSubmitDelay`
  at best and lasts until the target's PTY goes quiet at worst (`internal/relay`, `awaitSettled`) — and a provider
  that takes keystrokes through anything other than this PTY is invisible here. The pull request and issue handoffs
  wait on the same answer (`frontend/src/lib/terminal/write-at-prompt.ts`) with no Enter of their own to justify it:
  hand a conflict to a session you left half a sentence in, and nothing appears at that prompt until the draft goes
  stale.
- **A relayed Enter is timed against silence, not against the target** (`internal/relay`, `awaitSettled`): lich
  presses Enter once the target's PTY has been quiet for `defaultSubmitDelay`, because nothing here can read a TUI's
  screen to know it has taken the paste in. On Windows that quiet is the whole instrument — ConPTY hands a child key
  events rather than bytes, the bracketed paste markers do not survive, and every provider TUI then guesses at where
  a paste ends from timing alone. A target that repaints on a timer of its own never goes quiet and gets its Enter
  at `defaultSettleLimit` regardless, which is the case this cannot tell from a paste still arriving.

- **A scheduled prompt is late or gone, never on time** (`internal/relay/later.go`, `deliverDue`): due prompts
  are looked for every `scheduleTick`, and one whose session is not at a prompt — mid-setup, a draft on the
  line, no terminal opened, a card parked and not yet resumed, is left for the next pass, so it lands
  whenever that session next has somewhere to type, hours later if that is when. That covers lich having been
  closed at the time: the first pass after launch types a prompt that came due days ago, unannounced. Gone is
  the session removed for good rather than parked (deleted, forgotten, purged with its worktree, or taken by
  a deleted project): the row is the only copy of the prompt, so it goes with the row, and all that says so is
  one Warn in a log nobody is watching (`internal/store`, `noteForfeitedSchedules`).

- **A lich-spawned Kiro session runs lich's agent, not the user's** (`internal/agentplugin/kiro.go`,
  `internal/terminal/command.go`, `agentArgs`): Kiro keeps its hooks inside an *agent config*, and its built-in
  `kiro_default` cannot be shadowed — a `kiro_default.json` in the agents directory is ignored outright (measured
  on 2.21.0). So the only place a hook can be registered is an agent lich writes itself, which the spawn then
  names with `--agent lich`. Two things follow, and neither is visible on the card. The agent lich writes carries
  no `prompt`, so a session loses the paragraph `kiro_default` adds about subagents, the planner and LSP — the
  model still has every tool, it is just not told about those. And a default the user set with
  `kiro-cli agent set-default` is **not** what a lich session runs; their own agent applies to Kiro started from a
  terminal and not to Kiro started from a card. Deleting `~/.kiro/agents/lich.json` puts the session back on
  `kiro_default` and silently ends every report with it: the spawn stops naming an agent it cannot find, which is
  the one case where that costs a warning line rather than the reports.
- **Kiro enforces no hook timeout, and blocks the turn behind one**
  (`internal/agentplugin/kiro.go`): its TUI draws "0 of 1 hooks finished" while a report runs, and a hook that
  slept four seconds ran to completion under both `timeout_ms: 1000` and `timeout: 1` (2.21.0). So lich writes no
  timeout into the agent — a field that bounds nothing is a promise the file does not keep — and what actually
  bounds a report is the request timeout inside the script itself. A lich whose listener has gone away costs a
  Kiro turn that wait on every hook it fires, where the other harnesses cut it off themselves.
- **Kiro's skip-permissions flag still stops once** (`internal/terminal/command.go`, `skipPermissionFlags`): with
  the switch on, lich passes `--trust-all-tools`, and Kiro's TUI opens on a full-screen confirmation
  ("No, exit / Yes, I accept / Yes, and don't ask again") that has to be answered before the session starts. The
  flag is real and every tool afterwards runs unconfirmed; it is the *first* screen that is not skipped, which is
  the one thing a user who ticked the box does not expect. Kiro's own `chat.disableTrustAllConfirmation` setting
  turns it off for good, and lich does not write it — that setting disables a safety confirmation for every Kiro
  on the machine, including the ones lich never spawned.
- **A Kiro session never reports `waiting`, `idle`, or a title** (`docs/hooks/`): its five events are
  `agentSpawn`, `userPromptSubmit`, `preToolUse`, `postToolUse` and `stop`, so lich closes session-start,
  session-state's busy/tool/done rows and session-touched, and nothing else. A Kiro session sitting on a
  permission prompt reads as `busy` — true, but it does not say what it is waiting for — and the card keeps the
  provider's mark until the PTY itself goes, because there is no session-end event to clear it. The title report
  needs a transcript path off the hook payload, which Kiro passes on none of the five; it does write a `title`
  into its session metadata, so closing that gap later means lich reading the file rather than another hook.
- **A Cursor CLI session reports through Claude Code's plugin, or not at all** (`internal/agentplugin`,
  `internal/terminal/start.go`, `providerKind`): lich installs no plugin into Cursor, and it does not have to —
  the CLI executes every Claude Code hook on the machine, the user's own and each installed plugin's (measured on
  2026.08.11: `hookSource: claude-user` and `claude-plugin`, with `${CLAUDE_PLUGIN_ROOT}` expanded). So on a
  machine where the lich plugin is installed in Claude Code, a Cursor session reports the chat id it is running
  and the files it touches, with nothing installed there — and on a machine without it, that session reports
  nothing at all. Nothing on the card says which of the two it is.
  **What never arrives is the turn.** Of the nine events the plugin registers, Cursor delivers four —
  `SessionStart`, `PreToolUse`, `PostToolUse`, `SessionEnd` — and no `UserPromptSubmit` or `Stop`, measured
  against hooks in Cursor's own format and in Claude Code's alike. So a turn that calls no tool never begins and
  one that does never ends, which is why `terminal.closableState` drops every state but `idle` from a Cursor
  session rather than pinning a spinner to the card for the rest of it: no spinner, no bell, no auto-title, and
  no `waiting` either (`Notification` maps to nothing there). That is the Crush row of the table below, arrived
  at from the other direction — lich does not own the registration here, so it filters what it cannot close.
  The reports are also all that route carries: Cursor takes no MCP server on its command line and reads none from
  a Claude Code plugin, so lich's own tools come from an `mcpServers` document its install writes under
  `~/.cursor` — which is why installing for Cursor refuses while Claude Code has no plugin, why its version is
  Claude Code's, and why its row offers no update of its own: the update is the Claude Code row's, one line up
  the same screen. A Cursor session gets no briefing either — the CLI has no append flag — so what it knows about
  lich is its tool list. One last edge: the plugin's script reports `claude`, the argument Claude Code's own
  registration passes it, and lich drops that name for a card whose provider it chose itself — but a **shell**
  session running `cursor-agent` by hand has only the report to go on and wears Claude's mark.
- **Cursor keeps its state in two directories and its chats per checkout** (`internal/sandbox/sandbox.go`,
  `internal/terminal/transcript.go`): its config dir is `$CURSOR_CONFIG_DIR` ‖ `$XDG_CONFIG_HOME/cursor` ‖
  `~/.cursor` — not xdg-basedir, the fallback is the home directly — and it holds the credentials and the chats.
  But `~/.cursor` is resolved off the home with no variable in the way at all, and that is where `mcp.json`, the
  per-project transcripts and the CLI state live. On a machine with `XDG_CONFIG_HOME` set the two are different
  directories and a sandbox binding only one is a session that cannot see its own MCP servers. The chat itself is
  at `chats/<md5 of the resolved cwd>/<chatId>/store.db`, so a resume asked without the session's own working
  directory answers "conversation gone" — the same shape as Crush.
- **An install started from `go run` registers the lich on PATH, not itself** (`internal/agentplugin/crush.go`,
  `resolveLichBinary`): Crush's, oh-my-pi's and Cursor's registrations name the absolute path of the lich that
  wrote them, and under `go run` — `task dev` — that path is the binary the toolchain built into its cache and
  deletes when the run ends, so writing it gives a registration that works for the rest of that session and then
  fails silently forever. lich writes `lich` from PATH instead, recognising the cache by shape
  (`go-build*/b*/exe/*`) since the toolchain exports no marker. The trap is that a dev install then points at
  whatever version is installed on the machine — harmless, because the registration is only the transport and a
  session reaches the lich its PTY's coordinates name, but not what the file appears to say. With no lich on
  PATH at all, a dev install registers nothing: Crush and oh-my-pi still get their hooks, and Cursor's install
  refuses outright.
- **Installing the plugin writes into four harnesses' own directories** (`internal/agentplugin`): Claude Code and
  Codex are driven through their plugin CLI, but opencode, oh-my-pi and Crush have none, so lich writes the
  released files itself. None of them records what is installed, so the version lives in a marker line lich wrote —
  edit the file by hand and lich reads it as not installed. Crush below 0.88.0 ignores those lines in silence,
  which is why the install asks its version first. Crush's block and omp's `mcp.json` register lich's MCP server by
  the absolute path of the binary that installed it, and omp's is a JSON document lich rewrites rather than appends
  to: every key survives, the user's formatting does not.
- **Antigravity has a plugin CLI and lich installs around it** (`internal/agentplugin/antigravity.go`): `agy
  plugin install` takes a directory, and its only remote form clones that repository's default branch — while lich
  installs a *release*, whose version is what a card reports and what the next update compares against. So lich
  writes the customization directory itself (`~/.gemini/config/plugins/lich/`), with three consequences. The
  installed version lives in the manifest lich writes rather than in a marker line, since the directory is lich's
  outright — and a copy the user installed through `agy plugin install` carries no version, so it reads as not
  installed. The registration's commands are relative, resolved against the directory holding `hooks.json`
  (Antigravity runs a hook through `sh -c` from there and sets no plugin-root variable of its own, both measured on
  1.1.19), so moving that directory by hand breaks every report until the next install. And lich writes the hooks
  and their scripts only: the plugin's skills come with `agy plugin install`, not with this, which is the same
  thing already true of opencode, oh-my-pi and Crush.
- **A new MCP tool reaches opencode a release later than everyone else** (`internal/cli/mcp.go`, `mcpTools`; the
  registration table in `docs/cli.md`): every other harness is handed lich's own server, so a tool added here is in
  that session's list on the next spawn. opencode cannot register an MCP server from a plugin, so its plugin
  defines each tool itself in the companion repo (`omartelo/lich-plugin`, `opencode/lich.js`) — which means a tool
  arrives there only once that repo cuts a release and the user reinstalls the plugin, and until they do it is
  missing from that session's list while it is in every other. `lich rename` works there like anywhere else; it is
  discovery that lags, which is the whole reason the tools exist.
- **Only Claude Code says what a session is waiting for; the others say less or nothing**
  (`frontend/src/components/sidebar/SessionCard.tsx`, table in `docs/hooks/session-state.md`): its
  `Notification` carries a `message` written for a human, so the card reads "Claude needs your permission to
  use Bash". Codex's `PermissionRequest` and opencode's `.asked` events carry only the thing being asked
  about — `tool_name`, `permission`, `action` — so those cards read a bare `Bash` or `edit`, which says which
  card to open and not what it will ask. **Antigravity, oh-my-pi, Crush and Cursor CLI send no reason at all** and keep the
  generic "Waiting on you": none of the four reports `waiting` in the first place (Antigravity's permission
  prompt raises no lifecycle event that has been measured; omp declares an approval event no run was ever seen
  emitting; Crush reports no state), so there is nothing to hang a reason on. The trap is reading a bare card as
  "nothing to say" — on those three it means the harness never spoke, not that the block is trivial.
- **omp's state directory answers to two variables, and the profile wins** (`internal/agentplugin/omp.go`,
  `internal/terminal/transcript.go`, resolving it independently as the Claude Code pair do): `OMP_PROFILE` moves
  the whole directory and beats an explicit `PI_CODING_AGENT_DIR`. Get it backwards and the install lands where omp
  is not reading and every restored card silently starts fresh.
- **The plan gauge answers to two undocumented endpoints, and only two providers have one**
  (`internal/quota`): Claude Code's and Codex's usage routes are what their own CLIs poll, not published API. A
  field renamed upstream drops the window it fed rather than raising anything — an entry lich has no name for is
  skipped in silence, so a new kind of limit is invisible instead of wrong. The other three providers run on the
  user's own API keys and can never report a plan, so the readout is provider-asymmetric by design. lich reads
  those logins and never writes them: it does not refresh the token, so an expired one reads as signed out until
  the provider's own CLI rotates it. A reading is cached for five minutes because both endpoints rate-limit hard —
  the number on screen is up to that old, and nothing on it says so.
- **The pace marker is borrowed calibration, and it is silent far more often than it is wrong**
  (`internal/quota/pace.go`): the two numbers that decide when a weekly window is marked as spending ahead —
  fifteen percentage points past the elapsed share, and no marking at all in the first twenty-four hours after
  a reset — are the claude-swap project's measurements (`src/claude_swap/pace.py`, its issue #125), not lich's
  own. Nothing here has been tuned against a lich user's accounts. The consequence to know before debugging a
  marker that "never appears": a window ninety percent spent on the first day of its cycle is deliberately
  unmarked, because just after a reset the elapsed share is near zero and almost any use at all would read as
  far ahead. Only the weekly window is paced, so the five-hour one and Codex's monthly free-tier window carry
  no marker however they are spent. The verdict is derived from the window's own reset time, which is the next
  reset and never the start — the start is that time rolled back whole windows — so a provider that stops
  reporting a reset time silently drops the marker rather than the gauge.

- **Two more fields of Claude's usage payload are read no further than measuring them**
  (`internal/quota/claude.go`, `limits[].severity` and the top-level `extra_usage`/`spend` blocks): every
  `severity` observed on a live account reads `"normal"`, so its scale — what a non-normal value looks like,
  whether it maps to a colour — is unknown, and swapping the local usage-based colour ramp (`usageColor`) for
  it would be a guess dressed as a reading of the source. `extra_usage` and `spend` are the credits that cover
  spend past a full window; both exist in the payload but come back entirely null/disabled on every account
  measured, so their filled shape — what a partially-spent credit balance actually contains — cannot be built
  against here. A gauge built on an unobserved shape is the same failure `is_active`/`locked_reason` fixed
  around, repeated. The payload's top level is otherwise a graveyard of null codenames — `nimbus_quill`,
  `tangelo`, `iguana_necktie`, `omelette_promotional`, `cinder_cove`, `amber_ladder`, `juniper_tide`,
  `seven_day_omelette`, `seven_day_cowork` — every one of them unpopulated on every account measured, which is
  the standing evidence that reading `limits[]` and silently skipping a kind lich has no name for is the right
  default, not a gap to close. Codex's `wham` usage route carries no equivalent to any of this — no per-window
  active flag, no lock reason, no credit block — so this ceiling is Claude-only by the shape of the payload,
  not a choice.
- **Which account a session spends is read from its process, and only Linux answers**
  (`internal/quota`, `internal/terminal/account.go`): the reading follows `/proc/<pid>/environ` of the process in
  the session's PTY, so a wrapper binary that exports a login of its own is seen only there. macOS could answer
  the same question through `KERN_PROCARGS2` and does not yet; Windows cannot at all. On both, a session running
  a user-configured binary reports `unknown` and its gauge disappears from the footer — the alternative was the
  default account's numbers under a session spending another plan, and silence is the failure that does not lie.
  A card with no live process is in the same position until its PTY is up.
- **Measuring a token-only login costs a request against the very plan it measures** (`internal/quota/claude.go`):
  a long-lived OAuth token (`claude setup-token`) carries `user:inference` alone, so the usage route answers it
  403 and the account is read the way Claude Code reads it for itself — one `max_tokens: 1` message, for the
  rate-limit headers on the response. Reading the gauge therefore spends quota (negligibly) and appears in the
  account's own usage, once per cache window. That request carries Claude Code's system prompt verbatim because
  the API rejects an OAuth token without it — the same coupling as the user agent, and it fails closed, as a
  failed reading. Headers carry the two account-wide windows and no plan name, so such a session shows no
  model-scoped weekly cap and no "Max 5x" badge.
- **Only two providers name the account a session spends, and the reasons the other six do not are not one
  reason** (`internal/quota`, `Plan.Account`): Claude Code answers a profile route with the credentials token,
  and Codex carries an `email` claim in the OIDC id token beside its access token (read unverified, and
  discarded once its `exp` has passed — the CLI can rotate the access token without rewriting the id one, and a
  gauge under a login the user has left is worse than one under no name). opencode, Crush and oh-my-pi run on
  the user's own API keys: there is no plan account to name, which is the same reason they have no gauge.
  Antigravity's `~/.gemini/oauth_creds.json` *does* carry the same `email` claim and it is deliberately not
  read: there is no `antigravityPlan`, so a `Plan` naming an account with no window in it renders nothing at
  all (`PlanQuota` draws only a reading with a window) — the name arrives with the gauge or not at all. Cursor
  CLI and Kiro CLI were measured on 2026-09-04 and carry no such claim: Cursor's `~/.config/cursor/auth.json`
  token is a JWT whose `sub` is an opaque `github|user_…` and whose claims contain no email, and Kiro's login
  is a row in `~/.local/share/kiro-cli/data.sqlite3` holding an opaque `aoa…` token, a `github` provider label
  and an AWS profile ARN — nothing a person recognises as their account. Naming either would take a network
  call against an unmeasured route, for a provider that has no gauge to hang the name under.
- **`CLAUDE_SECURESTORAGE_CONFIG_DIR` decides which Keychain item a Claude login is, and lich reads it
  nowhere** (`internal/quota/claude.go`, confirmed by grep — the variable appears in no file here): Claude Code
  looks for its secure storage under that variable whenever it is *defined*, empty string included, and only
  falls back to `CLAUDE_CONFIG_DIR`; the value is NFC-normalised and hashed into the Keychain service name, so
  two values that differ by a combining accent are two different logins. Today this changes nothing: on Linux
  the credential is the plaintext file under the config dir, and on macOS no session's environment is readable
  at all (`envReadable`), so the question never arises. The trap is the day somebody writes the macOS
  `KERN_PROCARGS2` reading promised above and resolves the credential from `CLAUDE_CONFIG_DIR` alone — that
  reads the wrong Keychain item and reports, with a gauge and an account name under it, a login the session is
  not spending. Resolve the pair in that order, or report `unknown`.
- **The sandbox confines a working agent, not hostile code** (`internal/sandbox`): namespaces and mounts on
  Linux, a path policy on macOS, and nothing else — no seccomp filter, no Landlock ruleset. The network is
  never cut (the agent needs its API and the plugin's hooks report over loopback), so anything readable
  inside is exfiltrable, and `~/.config` *is* readable: a token stored there is in reach. `gh`'s is not
  one of them — it lives in the system keyring, which is why reaching it takes the flag below. Every session also carries `LICH_TOKEN`, so a confined agent can call lich's own RPC over
  loopback: a method that reads a host path it was *handed* would copy anything into reach of the sandbox,
  which is why the attach flow opens the picker inside the backend (`drop.Attach`) instead of taking a path. The private home is writable and vanishes with the session, so a dotfile an agent writes is gone
  next spawn with nothing saying so. On Ubuntu and Debian the kernel may refuse the user namespace outright
  (an AppArmor policy), which surfaces as bubblewrap's own error in the card and no session: `Available` is
  `exec.LookPath` and stays that way, because only a spawn can ask the kernel, so nothing before the card
  can refuse the rung. What answers ahead of time is `lich doctor`, whose `sandbox` check opens one confined
  child and reads a file in the checkout and a file in the home the backend replaces: it is a diagnosis, not
  a gate, and a machine where the probe fails still lets the user turn the sandbox on and watch it fail. `~/.ssh` is not
  mounted at all: a push over ssh from inside a confined session fails unless the project hands over the ssh
  agent (below), and lich's own PR flows run outside the sandbox and are unaffected either way. The distribution's `/etc/ssh/ssh_config.d` drop-ins are replaced by an empty
  directory, because inside the namespace they belong to nobody and ssh refuses to read a config file it
  does not own — so a host whose ssh depends on one of them (a corporate `ProxyCommand`, say) does not have
  it inside the sandbox. The display server's socket *is* mounted, because the agent's own copy and its
  clipboard image paste shell out to `wl-copy` and `xclip` and both fail without it: a confined session can
  therefore read whatever you copy, a password manager's paste included. A host without Wayland gets the
  X11 socket and its cookie instead, the wider of the two — X clients are not isolated from one another —
  and macOS gets neither, its pasteboard being a mach service rather than a socket, so a confined session
  there has no clipboard at all. macOS has no hardware here — its profile is unit-tested and has never run.
- **The two sandbox grants hand over more than what they are read as** (`internal/store/settings.go`,
  `internal/sandbox`, `internal/terminal/sandbox.go`): a confined session reaches the network as the user only
  where the project turned one of them on, and each is all-or-nothing. The ssh agent is handed over as a socket,
  so nothing private enters the sandbox — but the session signs with **every** identity loaded in that agent,
  against any host it can reach, for as long as it runs. OpenSSH can pin a key to one destination, and only when
  the key is added on the host (`ssh-add -h`); lich is given a socket that is already populated and can only
  pass it on whole. The socket does not travel alone: `~/.ssh/known_hosts` is mounted read-only beside it,
  because without it ssh cannot verify github.com, has no tty to ask on, and fails with "Host key
  verification failed" before the key is ever offered — the grant would hand over the credential and not the
  push. That file is public host keys, never a secret; what a confined session learns from it is the list of
  machines the user connects to. Read-only, so a host the user has never connected to *outside* the sandbox
  still fails inside it and cannot be learned there — blind trust-on-first-use is not a thing to grant an
  unattended agent. And a `known_hosts` symlinked out of a dotfiles repository is dropped like every other
  link in the home (below), which takes the whole grant down with it and says nothing. That is why Settings lists what is in the agent — and the list is read when the pane opens,
  so a key added afterwards is handed over by a switch that never named it. The GitHub token is one account's,
  the project's own (`vcs.account`), and it rides in the session's environment: the agent can read it back out
  of its own environment and spend it on anything that account's scopes allow, this repository or not. Neither
  grant is keyed by provider — a grant describes what is inside the sandbox, not who runs in it — so turning
  one on turns it on for every provider confined in that project. It is
  resolved once per spawn, so a token gh rotates mid-session goes stale with nothing saying so, and a `gh auth
  token` that fails leaves the session with no token rather than failing the spawn. Both are off by default,
  and both are Linux in practice: the macOS profile denies reads *inside* the home while a launchd agent socket
  and gh's keyring live outside it, so a confined macOS session never lost either and the switches change
  nothing there — they are inert rather than hidden, and there is no macOS hardware here to prove it further.
  Windows has no sandbox backend, so neither switch exists.
- **A file handed to a confined session arrives as a copy** (`internal/drop`): the session's home is empty,
  so lich does not look there for a dropped file at all — anything outside its checkout is copied and the
  copy's path is what lands at the prompt, for a drag and for the footer's attach button alike. The agent may read it and not write back: an edit lands on the
  copy, the user's own file is untouched, and nothing on screen distinguishes the two paths afterwards. A
  dropped *folder* from outside the checkout yields nothing, there being no copy to make of a tree. The
  copies live one directory per session and only that session's directory is mounted, so one confined
  session cannot read what was dropped into the one beside it; the directory goes when the session's row is
  deleted (parking a worktree session keeps it, as a resume still wants those paths). A lich that dies
  without deleting a row leaves copies behind, and the three-day age rule is what clears them — which also
  means a copy is the one part of a confined session that outlives the sandbox.
- **A symlink in the home is not mounted into the sandbox** (`internal/sandbox`): every path lich binds is
  taken as it is on disk, and a link is skipped — following one would let a dotfile manager point the
  private home at whatever it likes, and binding one fails the spawn outright when a parent directory is
  already mounted (bubblewrap resolves a mount destination through symlinks). So a `~/.gitconfig` symlinked
  out of a dotfiles repository is absent inside a confined session, with nothing on screen saying so. The
  binaries are the exception: their symlink chains are walked and the *directory* of every hop is mounted
  (`BinaryDirs`), which is what makes an agent installed the usual way — a link on `PATH` into a versioned
  store — runnable at all.
- **The macOS floor is the toolchain's, not lich's** (`build/darwin/Info.plist.tpl`,
  `build/darwin/homebrew/lich.rb.tpl`): nothing in lich needs macOS 13, but Go 1.27 dropped every
  release before Ventura, so a binary built from this module cannot run on Big Sur or Monterey. The
  cask's `depends_on` and the bundle's `LSMinimumSystemVersion` say 13.0 because the compiler does —
  both move with the next Go bump, and a machine below the floor is refused by Homebrew rather than
  by a crash.
- **The history's branch is read live, so a row whose checkout is gone has none** (`internal/project.BranchesOf`):
  the branch a row shows is not the one it stores — a worktree keeps the name it was created with while an
  agent moves the branch inside it, so the stored snapshot dates the close and only git can say what the
  checkout is on now, which is what the row draws. The batch runs once per settled search, which
  also means a branch that moved while the palette is up is stale until the query changes or the palette is
  reopened. A checkout removed behind lich's back has no branch to read and no session to resume: that row
  says `checkout gone` and offers to forget
  itself, which is the only way such a row is ever collected — `PurgeWorktreeSessions` never ran for it,
  because the removal never went through the app.
- **The History tab searches names, never what was said** (`internal/terminal/search.go`): the Messages tab
  reads a 4 MB tail or a query per session per keystroke, and it is pointed at the sessions the palette can route to —
  the open ones. History is the long list, so widening the transcript search to it would put a hundred disk
  reads behind every character typed, on a machine that can hold hundreds of transcripts and a single one
  of 169 MB. The parked row keeps its `provider_session_id`, so the transcript is still there to be searched
  by whatever does it later; the fix when it bites is a query, not a bigger tail.
- **A filed backend answer outlives the screen that asked, under a key its caller writes by hand**
  (`frontend/src/lib/remote-cache.ts`): a `useRemoteResource` caller that passes `cache` has its answers kept
  in module memory until the page reloads, under exactly the string it composed. Two callers that compose the
  same string serve each other's answers, and a string that leaves out something identifying paints one
  repository's answer onto another's screen — instantly, and then corrected one round-trip later, which reads
  as a flicker rather than as a bug. It is deliberately not `key`: `key` carries what *dates* an answer (the
  checkout's HEAD), which a fresh mount does not have until its git poll lands, so a cache keyed by it misses
  on the one frame the cache exists for. The cap is 32 answers with no byte budget, so a review that walks
  through more pull requests than that pays a skeleton on the way back to the first.
- **Seeding that filed answer runs during render, so its bookkeeping may never live in a ref**
  (`useMovedAnswer`, `frontend/src/lib/use-remote-resource.ts`): React can discard a render that updates state
  during it and replay it, and a ref written by the discarded pass makes the replay skip the very update it
  guarded. The symptom is silent and looks nothing like the cause — the answer is seeded, the screen paints,
  and the next frame is blank again with no setter anywhere having run. `use-remote-resource.test.tsx` pins it,
  but only under two conditions that are easy to drop: the probe must change the request on a *live* component
  (a remount initialises the marker and never exercises the replay), and it must record frames from a layout
  effect rather than from the render body (the body sees passes that were never committed, which reads a
  correct hook as an oscillation). A probe missing either one calls the ref version green.
- **The dock's remembered browse is module memory, keyed by a path and never swept**
  (`frontend/src/lib/file-browse.ts`): every checkout the Code tab has ever browsed keeps its filter,
  folds, preview and marked row until the page reloads — a few strings per checkout, deliberately not
  worth a sweep, and deliberately not persisted: these are positions in a tree that is re-read on each
  mount, and outliving a reload would mean pointing at files that have since moved. The key is the
  checkout path with no project or session in it, so two projects sharing a path share a browse, which
  is the same thing as saying they share a checkout. The tree and each previewed file now also file
  their answers in `remote-cache`, whose 32-entry cap they share with the pull request screen: a browse
  that opens more files than that evicts the oldest answers, and the panel pays a "Loading…" on the way
  back to them.
- **The Review tab's remembered source is a wish, not what is on screen** (`ReviewPanel`,
  `frontend/src/lib/dock-prefs.ts`): the pref is global and holds what the user picked, while what the
  panel shows is that choice put through `turnSwitchable` — a session whose provider never reports and
  holds no last-turn record has no turn to bracket, so it is shown the working tree and offered no
  switch. Nothing writes the guard's answer back, and that is the whole design: a session with neither is
  unswitchable after a reload until it next reports, so a panel that reset the pref instead of overriding
  it would erase the choice before the switch had a chance to appear. The two halves of that guard read
  different sources — the record rides the session's hydration, the diff behind it is seeded when the
  PTY is spawned — so a panel that reaches a restored card before its spawn has been tracked is offered
  the switch and told nothing is recorded, until the next read.
- **The pull request screen's remembered state is read once, at mount** (`frontend/src/lib/pulls/pulls-prefs.ts`):
  the filter box, the quick filter and the selected pull request are keyed per project but seeded from
  `useState`, which holds because every route into the screen carries its own project and leaving one unmounts
  it. A future route that reuses `Pulls` across two projects would keep the first one's box and its selection,
  and nothing in the component would say so.
- **The settings screen remembers nothing per project, on purpose** (`frontend/src/lib/settings-prefs.ts`):
  the pane that was open and the search box are stored under one key each, so opening Settings in project B
  lands on the pane project A was reading. That is the rule pulls-prefs states, landing on the other side —
  the nav is the same list of panes in every project, so neither is about a repository — and it is a decision
  rather than an oversight: the *values* those panes read and write are project-scoped already, in the
  workspace database under the project's own id. The trap is for whoever adds a pane that is genuinely about
  one repository's content. Its remembered state belongs on the per-project side, which means a new key with
  the project id in it, not another global one beside these two.
- **The settings search reads names, and its index is written by hand** (`frontend/src/lib/settings-index.ts`):
  every control is listed there with the section and group it lives in, because the panes are React components
  whose blocks exist only once rendered and the suite runs in node. Two things follow. A block added without
  an entry is a control the search cannot find, which is why `settings-index.test.ts` reads the
  `SettingBlock` and `SettingRow` literals back out of the source and fails on one nobody indexed; that guard is the only
  thing standing between this file and silent rot, so deleting it costs more than it looks. And the search
  matches titles plus a few hand-picked keywords, never a description, a stored value or a theme's name: a
  user hunting for `emerald` or a port number finds nothing, and the empty state says as much rather than
  pretending the setting is absent. Shortcuts and the panes themselves are indexed off `HOTKEY_ACTIONS` and
  `SETTING_SECTIONS`, so those two never fall behind.

- **The footer editor's sides wrap, and stop lining up when they do**
  (`FooterLayoutEditor.tsx`): each side is a tray of chips laid out with `flex-wrap`, so a user who turns on
  most of the twelve items gets two rows in one column and one in the other, and the two stop reading as the
  two ends of the footer. Chosen with the drag: the alternatives that never wrap are lists, one item per
  line, and moving an item then stops being a drag between the sides. Both sides stretch to the taller of the
  two so the block still reads as one thing. Whoever adds a thirteenth footer item is making that worse, and
  the answer is not a narrower chip: it is deciding the drag is worth less than the alignment. The chip also
  carries nothing but its own reading: a menu and a remove button that appeared on hover were tried and taken
  back out, because a control that grows under the pointer shifts the chips beside it at the exact moment a
  drag is being aimed. Moving an item without a pointer is the dnd-kit keyboard sensor (focus the chip, Space,
  arrows), which is why removing the menu costs no keyboard path.
- **A release's highlight is read from the binary, so fixing it after the tag fixes nothing a user sees**
  (`internal/patchnotes`): the What's new dialog parses the `CHANGELOG.md` embedded at build time — the alert
  blocks under the version heading included. Editing that release on GitHub, or the changelog on `main`,
  changes the release page and no dialog anywhere; the users who already updated have recorded the version as
  seen, and the ones who have not will read the binary they install. A wrong headline is a patch release. The
  update toast reads nothing from the release but its tag, on purpose: the toast says a release exists, and
  the dialog is where it speaks.
- **The Files changed tab remembers which file, never where in it**
  (`frontend/src/lib/pulls/use-active-file.ts`): the changed-files tree's mark comes back with the tab, but
  the diff pane reopens at the top. Nothing in this codebase restores a scroll offset, and the one that would
  have to be restored here is not measurable at the time it is needed: `LazyDiffBody` builds each file's
  editor only as its card nears the viewport (`FileDiff.tsx`), so on the way back every card is a placeholder
  sized from a line count and the page's real height arrives over the following frames. Re-jumping to the
  marked file would land on that estimate and drift as the editors mount. The mark already meant "the file
  last selected" rather than "the file on screen" — a click followed by a hand scroll leaves it behind on a
  live tab too — so restoring it alone says nothing untrue. That relanding is also why returning to the tab
  costs the lazy mount again: the editors are destroyed with the tab and rebuilt on the way back, which is
  the work `LazyDiffBody` exists to spread out rather than avoid.
- **This screen's per-pull-request state is re-read during render, not at mount alone**
  (`useActiveFile`): the bullet above about `pulls-prefs.ts` does not extend to it. The Files tab is *not*
  remounted when the list column moves to another pull request, so a `useState` seed would mark the previous
  pull request's file on the next one's tree. The re-read is held in state and never in a ref, for the replay
  reason `use-remote-resource.ts` documents, and `use-active-file.test.tsx` pins it by moving the pull
  request on a live component — a probe that remounted instead would call the ref version green.
- **A profile belongs to one browser, so changing browsers opens lich at its defaults**
  (`internal/chromium/profiledir.go`): each browser gets `<config>/lich/chromium-profile/<name>-<digest>/`,
  keyed by the command the ladder resolved, and the page's localStorage — every `lich.*` UI setting — lives
  inside one of them. Nothing copies between them. Pin a different browser with `--browser`, or watch the
  bundled window die at startup and fall back to a system one, and lich comes up looking factory-fresh; the
  settings are still there, under the other key, and going back reaches them. The key is the resolved path
  and *not* what its symlinks point at, so a store-style install (Nix, snap) keeps one profile across
  updates through its stable launcher — but a browser pinned at a path carrying its own version, an
  AppImage among them, is a new browser to this and starts empty on each update.
- **On Linux, Windows and Apple Silicon the window is lich's own; on an Intel Mac it is the system
  browser's** (`internal/chromium/shell.go`, `shell/`): the Linux packages, the Windows installer and the
  arm64 `Lich.app` ship an embedded Chromium (CEF through kurogane) beside the binary, and the ladder takes
  it above the desktop's default and every scan. The Intel bundle ships none: the release runner is arm64,
  cross-building the window means cross-building CEF's C++ wrapper, and nobody here could run the result.
  The trap: the four are one launch path, so a window-side change (a flag in `Args`, a prefs write, the
  restart signal) lands on lich's own Chromium on three and on a system browser on the fourth, and the Go
  side cannot tell which it got. Windows and macOS have one more: both were built and smoke-tested on a CI
  runner only (`release.yml` opens the window and reads a page over CDP), never on a desk, so the taskbar
  icon and AppUserModelID grouping on Windows, the Dock tile, Cmd-Tab and menu bar name on macOS, and the
  graceful close on restart on both are designed, not seen; what the macOS runner did measure is that the
  subprocesses hold no Dock tile of their own and the page renders (`release.yml`, the `mac` job).
- **A Windows install with the window beside it does not self-update** (`internal/appupdate.windowed`):
  the self-apply asset is the bare exe, and swapping it under a `shell\` directory would leave a third of
  a gigabyte of Chromium at the installer's version. The update button sends that install to the release
  page for the installer instead; the portable exe with no `shell\` beside it keeps self-applying, and
  keeps opening a system browser.
- **A Linux install whose window is missing or dies at startup opens a system browser instead**
  (`internal/chromium.Run`): `go run`, a bare binary copied out of the tarball, a package missing
  `lib/lich/shell` — each falls through to the ladder below with one `Warn` line; a window that exits with
  an error inside `startupGrace` (30 s: a segfault on first paint, a system library `libcef.so` cannot find
  on this distribution — the packages declare Chromium's own list, so that is a bare binary on a slim
  install, or a glibc older than 2.34, Debian 11 and RHEL 8, which `lich-shell` will not load on) is
  relaunched the same way, with a desktop notification naming the browser. Refusing
  to open would turn a packaging slip or a bad update into a lich that does nothing. The grace is the trap:
  a crash at 30.1 s is the window's lifecycle ending, as it always was, and a window closed by hand inside
  it exits 0 and never falls back. Thirty seconds and not ten because a segfault is reported only after
  its core dump is written, and systemd-coredump takes ~18 s over the window's process tree (measured): a
  crash one second in reaches `Wait` at nineteen. A pinned browser never falls back either — it is the user's word — and
  with no browser at all the crash lands on the tab path, where the log has the story and the
  notification does not. `lich doctor` names the rung that answered; `task dev` pins the window it built
  (`LICH_BROWSER`), since `go run` never has one beside it.
- **The window's own sandbox needs an install a package manager made** (`shell/src/main.rs`, the kurogane
  fork's `no_sandbox`): Chromium confines the window's subprocesses in a user namespace, or through the
  setuid helper beside `lich-shell`. Where it has neither, the browser process would abort at its zygote, so
  the shell asks first, the way Chromium does (a fork trying `CLONE_NEWUSER`, the helper checked for root and
  4755, never as root; the `CHROME_DEVEL_SANDBOX` helper Chromium also accepts for a binary the user owns is
  not asked) and opens with `--no-sandbox`. Only a package can own that helper root: a tarball unpacked as
  the user cannot, and an AppImage's squashfs mounts nosuid. On a desktop that denies unprivileged user
  namespaces — Ubuntu's AppArmor policy, over every unconfined binary — either install therefore runs
  unsandboxed and carries Chrome's "stability and security will suffer" bar, which is the truth about it.
  Windows and macOS run with `no_sandbox` and the same bar everywhere: the Windows sandbox needs
  `cef_sandbox` linked into the executable and the macOS one a helper app initialising it, and neither is
  wired.
- **The window opens at CEF's default size** (`shell/src/main.rs`): a system browser remembered the
  window's last size and position in its profile; the CEF Views window does not, so each launch is the
  default rectangle until the window manager places it. Tiling compositors never notice.
- **Without a Chromium `--app` window there is no window lifecycle** (`main.go`, `openWithoutWindow`) —
  reached with no Chromium-family browser installed, or on purpose with `--no-window`/`LICH_NO_WINDOW`. lich
  opens a plain tab and then runs until it is signalled, because a tab it did not spawn cannot be waited on.
  Closing the tab leaves lich serving, and `/restart` — which frees the pinned port by terminating "the
  window" — is handed this process instead. On Windows that terminate is a `taskkill` WM_CLOSE against a
  process that has no window, so an in-place update leaves the successor racing a port that never frees:
  a `--no-window` launch now buys that trap on a machine that could have had a window.
- **The tab fallback cannot tell "opened" from "nothing happened"** (`internal/system.OpenURL`): `xdg-open`,
  `open` and `rundll32` are started and never waited on — waiting would block for the life of the browser they
  hand off to. A desktop with a URL handler installed but no browser behind it therefore looks like success:
  lich stays up with a notification and nothing on screen. Only a machine missing the opener itself reaches
  the dialog that carries the URL.
- **Keep-awake follows the session-state report, and only that** (`internal/awake`, `turnLog.onOpen`): the
  machine is held out of idle sleep while a hook says a turn is open. Crush reports no state at all, so a
  Crush session left working behind a locked screen sleeps as it always did, and no card says so. Kiro's
  permission prompt reads as `busy` (docs/hooks/session-state.md), so a Kiro session blocked on a human keeps
  the machine awake until someone answers. Linux holds it through `systemd-inhibit --what=idle`: a distro
  without systemd logs one warning per burst of work and sleeps, and a desktop that ignores logind idle
  inhibitors sleeps silently. The hold was measured on Linux only; on Windows and macOS CI proves the request
  is registered (`powercfg /requests`, `pmset -g assertions`), not that the machine stays up.
