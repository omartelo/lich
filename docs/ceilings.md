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
- **Installing the plugin writes into three harnesses' own directories** (`internal/agentplugin`): Claude Code and
  Codex are driven through their plugin CLI, but opencode, oh-my-pi and Crush have none, so lich writes the
  released files itself. None of them records what is installed, so the version lives in a marker line lich wrote —
  edit the file by hand and lich reads it as not installed. Crush below 0.88.0 ignores those lines in silence,
  which is why the install asks its version first. Crush's block and omp's `mcp.json` register lich's MCP server by
  the absolute path of the binary that installed it, and omp's is a JSON document lich rewrites rather than appends
  to: every key survives, the user's formatting does not.
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
  inside is exfiltrable, and `~/.config` *is* readable: a token stored there, `gh`'s among them, is in
  reach. Every session also carries `LICH_TOKEN`, so a confined agent can call lich's own RPC over
  loopback: a method that reads a host path it was *handed* would copy anything into reach of the sandbox,
  which is why the attach flow opens the picker inside the backend (`drop.Attach`) instead of taking a path. The private home is writable and vanishes with the session, so a dotfile an agent writes is gone
  next spawn with nothing saying so. On Ubuntu and Debian the kernel may refuse the user namespace outright
  (an AppArmor policy), which surfaces as bubblewrap's own error in the card and no session. `~/.ssh` is not
  mounted at all: a push over ssh from inside a confined session fails, and lich's own PR flows run outside
  it and are unaffected. The distribution's `/etc/ssh/ssh_config.d` drop-ins are replaced by an empty
  directory, because inside the namespace they belong to nobody and ssh refuses to read a config file it
  does not own — so a host whose ssh depends on one of them (a corporate `ProxyCommand`, say) does not have
  it inside the sandbox. The display server's socket *is* mounted, because the agent's own copy and its
  clipboard image paste shell out to `wl-copy` and `xclip` and both fail without it: a confined session can
  therefore read whatever you copy, a password manager's paste included. A host without Wayland gets the
  X11 socket and its cookie instead, the wider of the two — X clients are not isolated from one another —
  and macOS gets neither, its pasteboard being a mach service rather than a socket, so a confined session
  there has no clipboard at all. macOS has no hardware here — its profile is unit-tested and has never run.
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
