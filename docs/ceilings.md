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
  checkout owns, nothing binds it, and anything on the machine can take the port before the dev server starts.
- **The cost readout bills per `(session, transcript)`** (`internal/pricing`, `internal/terminal/usage_cost.go`): a
  conversation forked inside the PTY bills its copied history twice — lich's own resume continues the same
  transcript and is unaffected — and each sub-agent's own transcript is counted in, so one unreadable or
  unpriceable sub-agent withholds the whole session's number.
- **The session readout understands Claude Code and Codex transcripts only**
  (`internal/terminal/usage_claude.go`, `internal/terminal/usage_codex.go`): oh-my-pi, opencode and Crush record
  token usage but not the model's context-window size, so lich cannot turn those counts into a trustworthy
  percentage, and Antigravity and Cursor CLI file their conversations as SQLite rather than as a transcript lich
  reads at all.
  Their footer therefore carries no model or context ring. Codex rollouts carry the effective window
  selected for that session — 95% of its default or configured `model_context_window` — and, since
  `total_token_usage` is a running total lich prices from its last line, the cost rung too. But
  `internal/pricing/prices.json` is baked with Claude models only, so a Codex model prices only after the
  remote LiteLLM refresh (`internal/pricing/pricing.go`) — an offline machine never shows a Codex cost, and
  a model LiteLLM has not priced either never will.
- **Hands-on time is read off three signals, and one of them is not universal**
  (`internal/terminal/handson.go`, `noteOutput`, `closableState`): the figure beside the cost
  counts the gap between consecutive signs of life in a session — a session-state report, a
  keystroke at its PTY, or its own output while a turn is open — and drops any gap longer than
  `handsOnIdleGap`. The output signal is the one that carries an unattended turn, and it is
  gated on the provider having reported `busy`, because a `tail -f`, a dev server or a TUI
  repainting would otherwise bill hours nobody worked. **Crush reports no state at all**
  (`docs/hooks/session-state.md`), so nothing ever opens a turn there and a Crush session
  accrues from the user's keystrokes alone: an hour it spent working while the user watched
  reads as the few seconds they typed in. **Cursor CLI is the near miss** — it delivers
  `PreToolUse` and `PostToolUse`, and those beat even though `closableState` refuses to publish
  them, so a Cursor turn is counted through its tool calls; what it still cannot count is a turn
  that calls no tool at all. A plain shell session is keystrokes-only by design and not a gap:
  there is no agent in it whose work could be missed. The trap is reading the number as
  comparable across cards — the same hour of work is a smaller figure on Crush than on Claude
  Code, and nothing on screen says which rung a card is on.
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
- **The grid follows the window, so the layout you dragged is not always the one you get back**
  (`frontend/src/lib/session/panes.ts`, `tracks`): how many panes sit across is computed from the
  stage's measured width, so collapsing the sidebar, opening the dock or moving the window to another
  monitor can reshape the wall under you. Track sizes are stored for the grid they were dragged on and
  fall back to equal shares whenever the shape no longer matches — which reads as a resize being
  silently forgotten, and is the alternative to restoring a four-column layout onto three columns. The
  cells themselves and their order always survive; only the sizes do not.
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
- **The Review panel's "Last turn" is a window of wall-clock time, and it lives in memory**
  (`internal/terminal/turnsnap.go`, `internal/project/turnsnap.go`): the panel brackets a turn with two
  `git write-tree` snapshots taken against an index of lich's own, so what it shows is everything that
  touched the checkout between the `busy` and the `done` — a formatter, an editor open beside lich, the
  user's own hands. Nothing in it can attribute a line, which is why the copy names the window and never
  the agent. Four traps follow. The pair is held in Go memory alone: a lich restart empties it, so every
  live session reads "No last turn recorded" until its next turn ends, and that wording is the same one a
  session whose first turn is still running gets — the panel cannot say which. `add -A` obeys `.gitignore`
  (deliberately, so this and `DiffText` never disagree about which files exist), so a turn that only
  touched ignored files reports itself as having changed nothing. Every snapshot in the app runs on one
  FIFO worker, because git refuses a second `add` against an index another holds — so one session's first
  snapshot of a large checkout delays the next session's, and a queue past `snapQueueDepth` drops a job,
  costing that turn its record with only the log saying so. A checkout whose *first* snapshot fails is
  dropped outright and never asked again — the ordinary reason is a session opened outside a repository,
  but a transient failure at spawn reads the same and leaves that card with no last turn until it respawns. And the boundary is the session-state contract,
  so **Crush and Cursor CLI have no last turn at all**: neither reports a state (`docs/hooks/session-state.md`),
  so nothing ever opens or closes a window there and the switch is never drawn — a rule read off the
  session's own reports, not a list of providers, so it corrects itself the day either one starts reporting.
- **A finished turn is unread until its own card is watched** (`frontend/src/lib/session/session-status-store.ts`,
  `frontend/src/providers/projects.tsx`): the solid emerald ring means "back from the agent, not read yet", and it
  fades only for the session whose terminal is on screen **while the window has focus**. Two things follow. A card
  left focused in a background window keeps its ring solid until the window is touched again, which is the point
  — but it also means a browser that reports focus oddly never fades one. And the mark lives in the page like the
  rest of the session state, so a reload starts every session unread again: a turn read twenty minutes ago comes
  back looking like news, and nothing on screen says the page forgot.
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
- **A session is named at birth, never on resume** (`internal/terminal/command.go`, `nameArgs`): the trap is that
  lich still *derives* that name (`internal/relay/rostername.go`, its page-side half
  `frontend/src/lib/session/peer-name.ts`) for the relay to resolve against, and the derived string goes stale the
  moment anyone renames — it then addresses a session that no longer answers to it. Nothing reads the real name
  back, so `/list-agents` inside the session is the only place it is true.
- **The file tree outside a repository is unfiltered and capped**
  (`internal/project/tree.go`, `walkFiles`): a plain folder has no `.gitignore`
  for lich to obey, so the Files tab lists dependency and build directories like
  any other, and stops at `walkLimit` files with nothing on screen saying the
  listing was cut. It also has no git status to poll, so the tree there refreshes
  only when the panel is reopened — a file the agent just wrote shows up on the
  next visit, not while you watch.
- **A missing tool is answered from the launch's `PATH`** (`frontend/src/lib/vcs-tools.ts`): the git and gh
  checks resolve through the `PATH` lich pinned at startup (`terminal.PinPath`), so installing either one
  while lich is open leaves every surface still calling it missing until a restart. Each of them says so; a
  live re-resolve would mean re-running the login shell under the running process. The check is
  `exec.LookPath` — it proves the binary exists and runs, never that it works, so a git that fails on the
  repository itself keeps failing the old silent way.
- **Check again re-scans the boot `PATH`, never the login shell** (`internal/providers.Detect`,
  `frontend/src/lib/providers-store.ts`, `refresh`): the provider surfaces re-probe on demand, so an agent
  installed into a directory that `PATH` already carried appears without a relaunch — and one installed into a
  directory that `PATH` did not carry never appears at all, however many times the button is pressed. The scan
  itself is not what would break: re-resolving `PATH` means re-running `$SHELL -lic env` (`ResolveShellEnv`), and
  a login shell that hangs on a prompt would hang the button rather than the boot. That machine's way in is a
  relaunch, or a binary path in Settings › Providers — and nothing at the button says so.
- **A machine with no agent on PATH opens terminals, and a custom binary path looks like one**
  (`frontend/src/lib/providers-store.ts`, `resolveImplicitSessionKind`): with nothing installed, every implicit
  new session — the empty screen's button, the hotkey, a new worktree — spawns a shell, because the provider
  fallback still resolves to Claude and that card would die on `claude: command not found`. Detection only ever
  scans `PATH`, so a user whose only agent is reached through a `provider.<id>.bin` override reads as that same
  bare machine and gets a terminal they did not want. Their agent is still one click away in the New Session
  menu, which is filtered by the enabled flag and not by install state.
- **git status is polled** — one shared poller per repository path (`frontend/src/lib/git/git-status-store.ts`); the
  lich plugin's `session-touched` hook nudges an immediate refresh.
- **The status badge has a single source** (`internal/project/status.go`): the branch, the HEAD commit and the
  dirty count all come out of one `git status --porcelain=v2 --branch` parse. A git release that changes those
  records breaks all three together rather than one at a time, and there is no second call left to disagree with
  the first — `Branch` still asks `symbolic-ref`, but nothing on the polled path calls it.
- **lich fetches on its own** (`internal/project/basestatus.go`) — the only git write lich makes outside the
  worktree flows: it moves remote refs in the user's own repository, unannounced, for as long as a card is on
  screen.
- **Persistence is hybrid**: UI prefs in the page's localStorage (`lich.*` keys — the reason the listener port is
  pinned at 47821; `LICH_LISTEN_PORT` overrides it, `LICH_PORT` is the distinct per-session hook variable), the
  workspace in SQLite (`<config-dir>/lich/lich.db`, `internal/store`). Closing a session deletes its row; keeping a
  worktree parks its session for a later resume; closing a project hides it, and reopening one whose directory is
  gone relocates it instead, keeping the stored id its sessions and its worktree directory hang off. Only the 25
  most recent closes are offered back (`recentLimit`, `internal/store/store.go`) — the row survives, but past that
  a project is reachable only through the directory picker, and neither the menu nor the palette says so.
- **A hotkey is taken from the agent, and its rebind lives only in the page** (`frontend/src/lib/use-hotkey.ts`,
  `hotkeys.ts`): every bound combo is caught in the window capture phase and stopped there, so the chord never
  reaches the PTY — the defaults spend chords no TUI can bind, but a rebind is checked against nothing, and
  recording `Ctrl+R` silently costs the shell its history search with nothing on screen connecting the two. The
  bindings are a `lich.hotkeys` entry in localStorage, which the theme left for the workspace database precisely
  because a recreated Chromium profile drops it: the combos revert to the defaults, and both the overlay and
  Settings then show those defaults as if nothing had ever been rebound.
- **Hidden sessions are serialized and destroyed**: 2MB replay rings on both sides
  (`frontend/src/lib/terminal/replay-buffer.ts` page-side, `internal/terminal/replay.go` backend-side — the latter
  survives a full page reload). Scrollback past the ring is gone, not paged. The snapshot carries only the modes
  xterm's SerializeAddon reads off `term.modes`; the ones an app relies on and it does not record are restored by
  hand in `frontend/src/lib/terminal/term-modes.ts` — today the mouse encoding and cursor visibility. Cursor
  *shape* (DECSCUSR) is not among them: a TUI that chose a bar or underline cursor gets lich's block back after a
  card switch.
- **One socket carries every session's output** (`internal/terminal/writequeue.go`): the per-session outbox
  decouples the *producers*, never the wire. A window that stops reading stalls the connection's single writer,
  so after `wsWriteTimeout` (5s) every session's output switches to the `/events` bridge at once. That is a
  second socket, and the page reads the two independently: a frame that fell back can land ahead of one still
  sitting in the stalled connection's buffer, so output is never dropped but its order across that switch is not
  guaranteed. Until then the queue holds `writeQueueDepth` frames for the whole app, and two things still wait on
  it across sessions: a push that finds it full, and the flush a session runs before its exit banner so the banner
  cannot overtake its own last bytes.
- **Single instance via the pinned port**: the bind is the lock (`internal/singleton`); a duplicate launch focuses
  the running window (best-effort, untested against a real window) and exits 0.
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
  that takes keystrokes through anything other than this PTY is invisible here.
- **A relayed Enter is timed against silence, not against the target** (`internal/relay`, `awaitSettled`): lich
  presses Enter once the target's PTY has been quiet for `defaultSubmitDelay`, because nothing here can read a TUI's
  screen to know it has taken the paste in. On Windows that quiet is the whole instrument — ConPTY hands a child key
  events rather than bytes, the bracketed paste markers do not survive, and every provider TUI then guesses at where
  a paste ends from timing alone. A target that repaints on a timer of its own never goes quiet and gets its Enter
  at `defaultSettleLimit` regardless, which is the case this cannot tell from a paste still arriving.

- **An answer that names no ticket is matched by delivery order** (`internal/relay/relay.go`,
  `errandOfLocked`): `lich reply "<answer>"` and `reply_to_session` without a ticket close the oldest message
  delivered to that session and still open, because nothing in an answer itself says which request it belongs to.
  A session working two relayed tasks at once that answers the second one first sends it home as the answer to the
  first, and both senders read a confident wrong report — nothing anywhere reports the mismatch. Naming the ticket
  is still the only exact route, which is why every relayed message spells it and why the card's tooltip shows it.

- **A Cursor CLI session reports through Claude Code's plugin, or not at all** (`internal/agentplugin`,
  `internal/terminal/terminal.go`, `providerKind`): lich installs no plugin into Cursor, and it does not have to —
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
- **An Antigravity card names the step, not the MCP tool** (`frontend/src/lib/session/tool-label.ts`): every MCP
  call there is the one step `call_mcp_tool`, with the server and the tool in `args.ServerName` / `args.ToolName`
  (measured on 1.1.19). Nothing in that name can be split, so the fix was never here — the plugin reads those two
  arguments and sends them as the report's `detail`, and the card draws `call_mcp_tool · lich/open_session` where a
  Claude Code card draws `lich · open_session`. The trap is reaching for `tool-label.ts` when that line looks
  wrong: the tool's identity arrives in a different field on this provider, and a plugin older than the release
  that added it sends `call_mcp_tool` with no detail at all.
- **Only two of the five harnesses that report a tool spell an MCP one splittably** (`frontend/src/lib/session/tool-label.ts`,
  table in `docs/hooks/session-state.md`): Claude Code and Codex send `mcp__<server>__<tool>`, which the card draws
  as `<server> · <tool>`. omp's `mcp__<server>_<tool>` has one underscore doing two jobs — `mcp__lich_list_sessions`
  splits into `lich` + `list_sessions` or `lich_list` + `sessions` and the string cannot say which — so only its
  prefix comes off, and opencode's `<server>_<tool>` carries no marker at all and is shown whole. Nothing overflows
  anywhere (the line truncates), but on those two the card spends its width on a server name nobody asked for.
  Splitting them needs the list of registered server names, which the card does not have. Antigravity is the fifth
  and nothing here reaches it: its name is not a tool name at all, so the split never applies and the bullet above
  is where that one is answered. Crush is on no version of this list: it reports no tool.
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
- **The sandbox confines a working agent, not hostile code** (`internal/sandbox`): namespaces and mounts on
  Linux, a path policy on macOS, and nothing else — no seccomp filter, no Landlock ruleset. The network is
  never cut (the agent needs its API and the plugin's hooks report over loopback), so anything readable
  inside is exfiltrable, and `~/.config` *is* readable: a token stored there is in reach. `gh`'s is not
  one of them — it lives in the system keyring, which is why reaching it takes the flag below. Every session also carries `LICH_TOKEN`, so a confined agent can call lich's own RPC over
  loopback: a method that reads a host path it was *handed* would copy anything into reach of the sandbox,
  which is why the attach flow opens the picker inside the backend (`drop.Attach`) instead of taking a path. The private home is writable and vanishes with the session, so a dotfile an agent writes is gone
  next spawn with nothing saying so. On Ubuntu and Debian the kernel may refuse the user namespace outright
  (an AppArmor policy), which surfaces as bubblewrap's own error in the card and no session. `~/.ssh` is not
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
- **Only the New worktree dialog asks** (`frontend/src/lib/use-sandbox-choice.ts`): the `Ask each time` rung
  has one place to put the question, so a session opened from the New Session menu, by a delegation, or
  through the MCP tool is not confined on that rung — it takes the answer closing the dialog would give.
  `Worktrees only` and `Everywhere` reach every caller; `Ask` reaches one.
- **A confined session is frozen at the answer it opened with** (`internal/terminal/sandbox.go`): the row
  wins over the rung in both directions, so moving the ladder afterwards changes nothing for the cards
  already on screen — including a parked worktree session resumed months later. The card's shield is the only
  thing that says so, and it says confined or not, never why: a card with no shield beside a rung set to
  Everywhere is a session that opened before the rung moved, and reopening it is the only way to change it.
- **A terminal session is never confined by a rung** (`internal/store/settings.go`): the rung is keyed by
  provider, and `shell` is not one — the sandbox exists to confine an agent working unattended, not the
  user's own prompt. A terminal opened in a project whose provider is on `Everywhere` still runs on the
  machine, and nothing says so.
- **The macOS floor is the toolchain's, not lich's** (`build/darwin/Info.plist.tpl`,
  `build/darwin/homebrew/lich.rb.tpl`): nothing in lich needs macOS 13, but Go 1.27 dropped every
  release before Ventura, so a binary built from this module cannot run on Big Sur or Monterey. The
  cask's `depends_on` and the bundle's `LSMinimumSystemVersion` say 13.0 because the compiler does —
  both move with the next Go bump, and a machine below the floor is refused by Homebrew rather than
  by a crash.
- **A closed session's history reaches back a hundred rows, and the search never goes deeper**
  (`internal/store/store.go`, `frontend/src/lib/session/command-palette.ts`): the History tab is handed the
  hundred most recently closed sessions when it opens and filters those in the window, the bargain the closed
  projects list already makes. So a session closed further back than that is in the database, counts against
  nothing, and cannot be found — typing its name narrows a list it was never in. The fix when it bites is a
  `LIKE` in the query, not a bigger number; nothing warns that the list was cut, because a cut that is always
  in force is not news.
- **The history's branch is read live, so a row whose checkout is gone has none** (`internal/project.BranchesOf`):
  the branch is not stored — a worktree keeps the name it was created with while an agent moves the branch
  inside it, so the directory cannot answer and only git can. The batch runs once per opening, which also means
  a branch that moved while the palette is up is stale until it is reopened. A checkout removed behind lich's
  back has no branch to read and no session to resume: that row says `checkout gone` and offers to forget
  itself, which is the only way such a row is ever collected — `PurgeWorktreeSessions` never ran for it,
  because the removal never went through the app.
- **A worktree lich did not create can never be removed through lich** (`internal/project.WorktreeAdopted`):
  `git worktree list` hands back every checkout of a repository, so one the user made by hand appears in the
  picker and hosts a session like any other — and nothing but its path tells the two apart. Everything lich
  creates lives under the worktrees root (`reserveWorktreePath`); a canonical path outside it is adopted, and
  `RemoveWorktree` refuses it whether or not force was asked for. What that costs is the cleanup: a hand-made
  checkout is parked, never collected, and removing it is a `git worktree remove` the user runs themselves.
  The test is the path and only the path — a worktree lich created and the user then moved out of the data dir
  reads as adopted, and one made by hand inside it reads as lich's own.
- **Parked rows are never swept, and their dropped-file copies expire on the clock instead of at the close**
  (`internal/store/mutations.go`, `internal/drop`): every close now parks a row, so the sessions table grows
  monotonically with the sessions a workspace has ever opened — a few hundred bytes each, which is a megabyte
  or so a year and deliberately not worth a retention timer. Nothing deletes history on a schedule; a row goes
  when its worktree is removed through lich, when the user forgets it, or when its project is deleted. The one
  thing that changed underneath is `internal/drop`: `SetSessionGone` still does not fire on a park, so a plain
  close no longer takes that session's dropped-file copies with it and they fall to the three-day prune. They
  are unreachable either way — a resume comes back under a fresh id, so the old copies directory can never be
  addressed again — but they now sit on disk for up to three days rather than going at the close.
- **The History tab searches names, never what was said** (`internal/terminal/search.go`): the Messages tab
  reads a 4 MB tail per session per keystroke, and it is pointed at the sessions the palette can route to —
  the open ones. History is the long list, so widening the transcript search to it would put a hundred disk
  reads behind every character typed. The parked row keeps its `provider_session_id`, so the transcript is
  still there to be searched by whatever does it later; and that search is Claude-only today
  (`claudeTranscriptPath`) while `canResume` locates all six providers, so widening it would inherit that gap
  rather than close it.
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
- **One filed answer drives an action rather than a readout** (`frontend/src/lib/git/use-checkouts.ts`): every
  other `cache` on this screen decides what is *shown*, and a value one round-trip old is only ever a stale
  label. This one decides what a button *does* — `Pulls.tsx` asks it whether the pull request's head branch is
  already checked out, and the answer picks between reusing a session and creating a worktree. A checkout
  removed from a terminal while the user was on another screen therefore offers "Go to session" for a
  directory that is gone. The window is one round-trip (it re-reads on mount, on focus, and after this screen
  creates a checkout itself) and git refuses the wrong move anyway, which is why it is filed rather than left
  cold — but it is the one to think twice about before the next `cache` is added to something a button reads.
- **Unsent prose on the pull request screen lives in memory, and nothing collects it**
  (`frontend/src/lib/pulls/draft-store.ts`): a description, a comment and every thread reply survive each
  unmount the screen can produce, and none of them survives a reload — unlike the pending review beside them,
  which is mirrored to localStorage. Persisting these is the same argument that one already won, and the reason
  it has not been taken is the other half: a filed review clears itself on submit, while an abandoned reply on
  a thread nobody returns to would sit in storage forever with no one to collect it.
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
  panel shows is that choice put through `useSessionEverReported` — a session whose provider never
  reports has no turn to bracket, so it is shown the working tree and offered no switch. Nothing writes
  the guard's answer back, and that is the whole design: `switchable` is false after every reload until
  the session next reports, so a panel that reset the pref instead of overriding it would erase the
  choice before the switch had a chance to appear. The visible cost is that "Last turn" cannot be
  restored on a session that has been quiet since the reload — it comes back the moment that session
  reports again.
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
- **A remembered section id is never validated, and a dead one is simply waited out** (`Settings.tsx`):
  section ids include `provider-<id>` for every enabled provider, so the set is not knowable at build time and
  the pref is stored raw rather than parsed against a list. `Settings` resolves an id it cannot place to its
  first section, which is what makes a provider disabled for an afternoon come back to its own pane rather
  than being forgotten — but it also means a pane removed from the app for good leaves a pref that resolves
  silently, forever, and nothing rewrites it. Deleting a section means the users who were on it open on
  Appearance with no explanation.
- **`agentplugin.Status` is the one settings read that is not remembered** (`UpdatesSettings.tsx`): it costs
  ~180 ms and its rows blank on every visit to Updates, while every other read on the screen paints from the
  last answer. It is not an oversight — `PluginSetting` awaits that read for its *outcome*, telling "Checked."
  from "Check failed — are you online?", and `useRemoteResource`'s `refresh` is fire-and-forget with the
  failure folded into state, so there is nothing to await. Caching it means rewiring both of that pane's flows
  around a resource's `loading` and `error`, and that is the price, not a line of config.
- **`useBinaryCheck` is deliberately not a `useRemoteResource` caller** (`frontend/src/lib/use-binary-check.ts`):
  it answers `null` until a verdict is in and debounces its input, because the value it checks arrives one
  keystroke at a time — and `useRemoteResource` has no debounce seam to hang that on. So the path verdicts on
  Settings › Version Control and every provider's binary block still blink through "unknown" on the way back
  to the screen. Measured at 1–3 ms per check (`providers.Verify` is a `LookPath`), which is why closing it
  was not worth widening a hook the whole app reads through.
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
