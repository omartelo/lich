# Known Ceilings

Deliberate limits and shortcuts, and the traps they set. A bullet earns its place by naming something that breaks
work when nobody knows it and that the call site never shows. The mechanism and the history stay in the code and
`CHANGELOG.md`; this file is the trap alone. Provider-specific behaviour is reference, not a trap, and lives in
[`providers/`](providers/).

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
- **The Run card is never started for you** (`frontend/src/components/sidebar/SessionSidebar.tsx`): a fresh
  worktree's setup script is still installing dependencies in the agent's card when the checkout appears, and
  lich has no "setup finished" signal to hang an automatic start on — `terminal.Ready` answers a different
  question, going false again for every turn the agent takes. So the card is one gesture, which is also what
  keeps eight worktrees from meaning eight dev servers.
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
  | Cursor CLI | nothing | chat filed as SQLite lich has no reader for, search and recap included |

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
  Selecting a missing reading renders nothing, never a zero. Cost still uses the backend setting because it controls whether
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
  (`docs/hooks/session-state.md`), so nothing ever opens or closes a window there, and neither the switch nor
  the recap band beside it is ever drawn — a rule read off the session's own reports, not a list of providers,
  so it corrects itself the day either one starts reporting.
- **A finished turn is unread until its own card is watched** (`frontend/src/lib/session/session-status-store.ts`,
  `frontend/src/providers/projects.tsx`): the solid emerald ring means "back from the agent, not read yet", and it
  fades only for the session whose terminal is on screen **while the window has focus**. A card left focused in a
  background window keeps its ring solid until the window is touched again, which is the point, but it also means
  a browser that reports focus oddly never fades one.
- **A session close is a hang-up on Unix and a Ctrl+C on Windows** (`internal/terminal/pty_unix.go`,
  `pty_windows.go`): closing a card signals the agent and gives it `closeGrace` to leave, so its exit path runs —
  hooks, transcripts, whatever it writes on the way out. A ConPTY has no signal to deliver, so Windows sends the
  terminal's own Ctrl+C instead, and that is a weaker ask: an agent whose TUI reads one Ctrl+C as "interrupt the
  turn" and wants a second one to quit — Claude Code does — is still killed when the grace runs out, and nothing
  on screen says so. That the byte arrives at all depends on `heedCtrlC`: a lich started by a service passes an
  inherited "ignore Ctrl+C" to every agent it spawns, and clearing it before the first spawn is what the Windows
  close rests on.
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
- **The worktree setup script answers to the main checkout, never the new branch**
  (`internal/project/setup.go`): improve `.lich/setup-worktree.sh` on a feature branch and fresh worktrees keep
  running the old one until the change reaches the checkout the project points at.
- **git status is polled** — one shared poller per repository path (`frontend/src/lib/git/git-status-store.ts`); the
  lich plugin's `session-touched` hook nudges an immediate refresh.
- **The status badge has a single source** (`internal/project/status.go`): the branch, the HEAD commit and the
  dirty count all come out of one `git status --porcelain=v2 --branch` parse. A git release that changes those
  records breaks all three together rather than one at a time, and there is no second call left to disagree with
  the first — `Branch` still asks `symbolic-ref`, but nothing on the polled path calls it.

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
  It also holds in the bundled window alone. Opened as a tab (an Intel Mac), the browser keeps its accelerators
  and a chord it reserves never reaches lich at all.
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
- **Measuring a token-only login costs a request against the very plan it measures** (`internal/quota/claude.go`):
  a long-lived OAuth token (`claude setup-token`) carries `user:inference` alone, so the usage route answers it
  403 and the account is read the way Claude Code reads it for itself — one `max_tokens: 1` message, for the
  rate-limit headers on the response. Reading the gauge therefore spends quota (negligibly) and appears in the
  account's own usage, once per cache window. That request carries Claude Code's system prompt verbatim because
  the API rejects an OAuth token without it — the same coupling as the user agent, and it fails closed, as a
  failed reading. Headers carry the two account-wide windows and no plan name, so such a session shows no
  model-scoped weekly cap and no "Max 5x" badge.
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
  link in the home (`internal/sandbox`'s `existing`), which takes the whole grant down with it. That is why Settings lists what is in the agent — and the list is read when the pane opens,
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
  (`internal/chromium/profiledir.go`): every `lich.*` UI setting lives in the localStorage of the profile keyed
  by the browser that opened it, and nothing copies between profiles. Pin a different browser, or fall back to
  a system one when the bundled window dies, and lich comes up factory-fresh; the settings are still under the
  other key. A browser pinned at a path carrying its own version (an AppImage) is a new browser on every update.
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
  subprocesses hold no Dock tile of their own and the page renders (`release.yml`, the `mac` job). A page
  read over CDP is not a pixel either: a window that renders every frame and presents none reads as green, and
  that is exactly what an Intel UHD driver did on Windows until `shell/src/main.rs` turned DirectComposition
  off there. No runner here can look at its own screen, so presentation is only ever proven on a desk.
- **A Linux or Windows install whose window is missing or dies is a lich that shows nothing but a dialog**
  (`internal/chromium.Run`): `go run` with no `LICH_SHELL` pin, a bare binary copied out of the tarball, a
  package missing `lib/lich/shell`, a window that exits on a missing system library or a glibc older than 2.34
  (Debian 11, RHEL 8, which `lich-shell` will not load on) — each ends in the error dialog with the log path,
  never in a browser on the machine. Only macOS keeps a fallback, and only to a plain tab: an Intel bundle has
  no window, and an Apple Silicon window that exits with an error inside `startupGrace` (30 s, because a
  segfault is reported only after its core dump is written) hands the URL to the default browser instead.
  `lich doctor` names the window a launch would open.
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
- **Opened as a tab there is no window lifecycle** (`main.go`, `openWithoutWindow`, macOS only): lich opens a
  plain tab and then runs until it is signalled, because a tab it did not spawn cannot be waited on. Closing
  the tab leaves lich serving.
- **The tab fallback cannot tell "opened" from "nothing happened"** (`internal/system.OpenURL`): `xdg-open`,
  `open` and `rundll32` are started and never waited on — waiting would block for the life of the browser they
  hand off to. A desktop with a URL handler installed but no browser behind it therefore looks like success:
  lich stays up with a notification and nothing on screen. Only a machine missing the opener itself reaches
  the dialog that carries the URL.
- **Keep-awake follows the session-state report, and only that** (`internal/awake`, `turnLog.onOpen`): the
  machine is held out of idle sleep while a hook says a turn is open, so what a provider reports decides whether
  the machine stays up at all (`docs/providers/`). Linux holds it through `systemd-inhibit --what=idle`: a distro
  without systemd logs one warning per burst of work and sleeps, and a desktop that ignores logind idle
  inhibitors sleeps silently. The hold was measured on Linux only; on Windows and macOS CI proves the request
  is registered (`powercfg /requests`, `pmset -g assertions`), not that the machine stays up.
