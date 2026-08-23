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
  percentage. Their footer therefore carries no model or context ring. Codex rollouts carry the effective window
  selected for that session — 95% of its default or configured `model_context_window` — but no API-cost
  accounting, so its setting stops at model and context while Claude Code alone offers the cost rung.
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
- **A terminal entrypoint reaches shell sessions on Linux and macOS alone** (`internal/terminal/entrypoint.go`):
  the menu item is absent on a provider card, and on Windows the setting saves and the terminal opens on a bare
  shell anyway — the wrap is skipped there for `wrapSetup`'s reason, and nothing on screen says so. It also runs
  through the shell's `-c`, which loads no interactive rc: an alias defined in `.zshrc` is not a command that can
  be an entrypoint, though `$PATH` is intact (`internal/terminal/shellenv.go`).
- **The worktree setup script answers to the main checkout, never the new branch** (`internal/project/setup.go`):
  improve `.lich/setup-worktree.sh` on a feature branch and fresh worktrees keep running the old one until the
  change reaches the checkout the project points at.
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
- **git status is polled** — one shared poller per repository path (`frontend/src/lib/git/git-status-store.ts`); the
  lich plugin's `session-touched` hook nudges an immediate refresh.
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
- **Single instance via the pinned port**: the bind is the lock (`internal/singleton`); a duplicate launch focuses
  the running window (best-effort, untested against a real window) and exits 0.
- **lich appends to the agent's system prompt, for two providers only**
  (`internal/terminal/command.go`, `briefingFlags` → `relay.SpawnBriefing`): Claude Code and oh-my-pi are spawned
  with `--append-system-prompt` carrying lich's own briefing, so text the user never wrote is in every session's
  prompt and in `/proc/<pid>/cmdline`. Codex, opencode and Crush get nothing there — none has a per-spawn append
  flag, so for those three the point exists only in lich's MCP instructions, and behaviour between providers
  differs by that much.
- **A prompt in use is recognised from the bytes going in, never from the line itself**
  (`internal/terminal/draft.go`): a relayed message pastes at the prompt and sends an Enter behind it, so lich
  holds the delivery back while the user has unsent input there. What it counts is printable input since the last
  Enter, escape sequences skipped — it cannot see the line, so an edit that leaves it empty by another route
  (Ctrl+W, a click into the middle of it) reads as a draft that is still there, and a delivery waits out
  `draftIdle` for nothing. The stale-draft release is what keeps that a delay instead of a wedged relay. Two gaps
  stay open: input arriving in the ~150ms between the paste and its Enter (`defaultSubmitDelay`) still rides along,
  and a provider that takes keystrokes through anything other than this PTY is invisible here.
- **An answer that names no ticket is matched by delivery order** (`internal/relay/relay.go`,
  `errandOfLocked`): `lich reply "<answer>"` and `reply_to_session` without a ticket close the oldest message
  delivered to that session and still open, because nothing in an answer itself says which request it belongs to.
  A session working two relayed tasks at once that answers the second one first sends it home as the answer to the
  first, and both senders read a confident wrong report — nothing anywhere reports the mismatch. Naming the ticket
  is still the only exact route, which is why every relayed message spells it and why the card's tooltip shows it.

- **Installing the plugin writes into three harnesses' own directories** (`internal/agentplugin`): Claude Code and
  Codex are driven through their plugin CLI, but opencode, oh-my-pi and Crush have none, so lich writes the
  released files itself. None of them records what is installed, so the version lives in a marker line lich wrote —
  edit the file by hand and lich reads it as not installed. Crush below 0.88.0 ignores those lines in silence,
  which is why the install asks its version first. Crush's block and omp's `mcp.json` register lich's MCP server by
  the absolute path of the binary that installed it, and omp's is a JSON document lich rewrites rather than appends
  to: every key survives, the user's formatting does not.
- **A new MCP tool reaches opencode a release later than everyone else** (`internal/cli/mcp.go`, `mcpTools`; the
  registration table in `docs/cli.md`): every other harness is handed lich's own server, so a tool added here is in
  that session's list on the next spawn. opencode cannot register an MCP server from a plugin, so its plugin
  defines each tool itself in the companion repo (`omartelo/lich-plugin`, `opencode/lich.js`) — which means a tool
  arrives there only once that repo cuts a release and the user reinstalls the plugin, and until they do it is
  missing from that session's list while it is in every other. `lich rename` works there like anywhere else; it is
  discovery that lags, which is the whole reason the tools exist.
- **Only two of the four harnesses that report a tool spell an MCP one splittably** (`frontend/src/lib/session/tool-label.ts`,
  table in `docs/hooks/session-state.md`): Claude Code and Codex send `mcp__<server>__<tool>`, which the card draws
  as `<server> · <tool>`. omp's `mcp__<server>_<tool>` has one underscore doing two jobs — `mcp__lich_list_sessions`
  splits into `lich` + `list_sessions` or `lich_list` + `sessions` and the string cannot say which — so only its
  prefix comes off, and opencode's `<server>_<tool>` carries no marker at all and is shown whole. Nothing overflows
  anywhere (the line truncates), but on those two the card spends its width on a server name nobody asked for.
  Splitting them needs the list of registered server names, which the card does not have. Crush is not on this
  list at all: it reports no tool.
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
