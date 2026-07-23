# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **The session sidebar now groups by worktree.** With several worktrees open,
  a flat list of cards made it hard to tell which terminal or provider belonged
  to which checkout. Sessions sharing a worktree now sit under one static
  divider titled with the worktree folder name, so the checkout each card
  belongs to reads at a glance while the branch stays on the card. Grouping
  keys off each session's checkout
  path fixed at spawn, never its live cwd — a `cd` deeper into the tree never
  moves a card to another group. A drag reorders within a group only, and a
  project with no worktrees keeps its old flat, header-less list.

## [0.16.0] - 2026-07-23

### Added

- **A fresh worktree now sets itself up.** Creating a worktree copies the
  gitignored `.env*` files over from the main checkout — a worktree starts
  with tracked files only, so env files and local credentials were always
  missing from a new checkout. A `.worktreeinclude` file at the repository
  root overrides the patterns (globs, one per line; a slashless pattern
  matches by basename at any depth). Wholly ignored directories
  (`node_modules`, build output) are never copied — installing those is the
  job of the new per-project **setup script** (Settings › Worktree), which
  runs in the new worktree session's terminal ahead of the agent, so a
  `pnpm install` happens where you can watch it. The session opens even if
  the script fails, resuming a kept worktree never re-runs it, and Windows
  skips the script wrap while the port stays experimental.

### Fixed

- **A session spawned directly on a provider now inherits your login shell's
  environment.** lich is a GUI app, never started from a terminal, so its env
  snapshot is the graphical session's — it never sourced `.zshrc`/`.bashrc`/
  `config.fish`/`.profile`. A `shell` session hid this (the spawned `$SHELL`
  sources its own rc), but a provider spawned directly (Claude Code, Codex, …)
  saw only the launch env, so a `${VAR}` expansion in `.mcp.json` — e.g. an MCP
  server's auth token exported in your rc — came up empty. lich now resolves the
  shell env once at startup (`$SHELL -l -i -c` dump) and merges it over the
  launch snapshot, so both provider and shell sessions get the full environment.
  Best-effort: `$SHELL` unset or any failure keeps the launch env. Windows
  without a `$SHELL` (env lives in the registry) is not covered.
- **Closing a worktree session only prompts to remove the checkout when it is
  the last one there.** A worktree can host more than one session — a provider
  plus a hand-opened shell rooted at the same path — and closing the throwaway
  shell offered to remove the whole checkout, so one accidental confirm discarded
  the provider's work. The prompt is now gated on the closing session being the
  last occupant of its worktree path; while a sibling still lives there, closing
  just closes.

## [0.15.0] - 2026-07-23

### Added

- **An Updates section in Settings.** It shows the running lich version with a
  "Check for updates" button that forces the release check right now (the
  automatic one still runs at startup and hourly), reopens the current
  version's patch notes on demand, and shows the Claude Code plugin's installed
  version with its own check, install and update actions.

## [0.14.0] - 2026-07-23

### Changed

- **A project may now sit with no session at all.** Closing the last one no
  longer spawns a replacement, and opening a project no longer seeds a first
  session — both land on an empty screen, the same shape as the no-project-open
  landing screen, with a button that opens a session when you want one. The
  sidebar's "+" still picks the kind. An empty project stays empty across a
  restart, Home included.
- **The stored Claude session id is now a provider session id.** lich runs four
  provider CLIs, but the column, the struct field, the store method and the
  hook payload were all still named after Claude alone. The `sessions` table's
  `claude_session_id` is renamed to `provider_session_id` on open (existing ids
  carry over — the rename is the migration, nothing is stranded);
  `store.Session.ClaudeSessionID` becomes `ProviderSessionID`
  (`providerSessionId` over the wire), and `SetClaudeSession` becomes
  `SetProviderSession`. The `/session-start` hook now takes
  `provider_session_id` and still accepts the old `claude_session_id` so plugin
  releases before v0.3.0 keep working — see `docs/hooks/session-start.md`.
  Resume itself stays Claude-only: `--resume` is a Claude Code flag.

### Fixed

- **Dragging a tab or session card no longer scrolls its strip.** Reorder drags
  are clamped to the strip's own axis, so pulling a project tab downward (or a
  session card sideways) can't overflow the container and trigger dnd-kit's
  auto-scroll on the cross axis. The dragged tab and session card also stay
  solid instead of turning into a see-through ghost — the old opacity fade is
  gone, and the card no longer sits in its translucent hover state (the pointer
  rides it for the whole drag) while sliding across its neighbours.
- **Zoom now works on layouts with a dedicated "+" key** (German, for one).
  The chords matched only the physical Equal/Minus keys, so on those layouts
  nothing claimed the press and Chromium's own zoom ran instead — the exact
  double-zoom the physical-key matching was built to prevent. The typed
  character is now checked as a fallback.
- **Stale results can no longer outrace fresh ones in the footer badges.** A
  slow git-status poll resolving late could overwrite the newer status the
  session hooks had just fetched, and the pull-request badge kept showing the
  previous directory's PR while the new lookup was in flight.
- **The command palette reopens clean.** Toggling it closed with the hotkey
  kept the previous filter and cursor for the next open; and Ctrl+F no longer
  opens the terminal search box hidden beneath an open palette.
- **A failing file picker now says so** with an error toast instead of
  failing silently.
- **The self-update download no longer dies on a normal connection.** The
  binary download shared the 5-second timeout meant for small metadata reads,
  and that timeout covers the whole transfer — so on anything but a very fast
  link the download was cut mid-stream and self-apply (Windows/macOS) failed
  every time. The download now gets its own generous ceiling.
- **A renamed worktree session keeps its name across keep/resume.** Resuming a
  kept worktree re-created the session with the "automatic title" flag reset,
  so the next AI-generated title overwrote the name you chose. The flag now
  survives the park/resume cycle.
- **Holding a hotkey no longer fires it repeatedly.** Key auto-repeat on
  Ctrl+Shift+T could spawn a stack of sessions from one held chord (and
  Ctrl+K would flap the palette); hotkeys now fire once per press.
- **A failed in-place restart can be retried.** If launching the successor
  process failed (say, the binary mid-swap by the package manager), the
  restart coordinator latched anyway and every later `/restart` silently did
  nothing until the app was relaunched by hand.
- **Sessions that exit on their own no longer leak their PTY handle**, and
  closing a session during its spawn round-trip no longer strands the PTY or
  drops a queued update command.

## [0.13.0] - 2026-07-23

### Added

- **Terminal text size is now its own setting** (Appearance › Terminal text
  size, 8–32px, persisted). Interface zoom no longer touches the terminal, so
  this is the control for how big terminal text is — and, unlike zoom, changing
  it does change how much fits on screen, so a running session reflows to the
  new width.

### Changed

- **Interface zoom now scales only the interface.** It used to scale the
  terminal along with everything else, which handed the terminal a different
  amount of room at every zoom step and re-wrapped whatever was running in it —
  a TUI mid-session would rewrap and its scrollback would keep the old wrap.
  Zoom now moves the interface (rail, tabs, sidebar, footer, dialogs) and leaves
  the terminal grid exactly where it was; terminal text has its own size setting
  above.

### Fixed

- **Zoom no longer applies twice.** `Ctrl +` is physically `Ctrl+Shift+=` on
  every common layout, but the shortcut was declared as the character `"+"` with
  Shift explicitly off — a combination no keyboard can produce. Zoom in therefore
  never matched, nothing called `preventDefault()`, and Chromium ran its own zoom
  accelerator, while zoom out (`Ctrl −`, no Shift needed) matched and scaled the
  app instead. Two zooms, disagreeing with each other and with the Appearance
  buttons, and clipped layouts once they compounded. Zoom chords are now matched
  on the physical key (`event.code`), which is the same on every layout, so the
  app is the only thing that zooms. The numpad keys work too.
- **Zooming no longer leaves the window part-empty or cuts the layout off.** The
  app scaled itself with CSS `zoom`, which scales rendered boxes but leaves
  `vh`/`vw` as physical viewport units, so the `100vh`/`100vw` app root rendered
  at viewport × zoom: short of the window when zoomed out, overflowing when
  zoomed in — and the page's `overflow: hidden` cut the overflow instead of
  scrolling it. Scaling now moves the root font size instead, which every
  interface measurement already follows, so the layout fills exactly one window
  at any zoom level.

### Removed

- **The `Zoom in`, `Zoom out` and `Reset zoom` entries from configurable
  hotkeys.** These chords exist to shadow Chromium's built-in accelerators, and
  an accelerator is bound to a physical key rather than to a character, so they
  cannot be expressed as a rebindable character combo — that mismatch was the bug
  above. Any custom binding saved for them is ignored on load; every other hotkey
  is untouched and still configurable.

## [0.12.0] - 2026-07-21

### Changed

- **The in-app updater now updates Arch through the AUR.** On Arch (and its
  derivatives) the update prompt pastes `yay -S lich-bin` instead of the
  `install.sh` one-liner, keeping the install tracked by the user's AUR helper.
  Since `yay` does not know how to relaunch lich, the pasted command chains an
  explicit restart using the terminal session's own loopback credentials. Other
  distros are unchanged.

### Fixed

- **The app window now shows the lich icon.** The frontend served no favicon,
  so the Chromium `--app` window fell back to a generic page icon in the
  taskbar (most visible on Windows). The app icon now ships with the frontend
  and is declared in the page head.

### Added

- **A one-click Restart button after a Windows/macOS self-apply.** Once the
  update is downloaded and swapped in, the toast now carries a **Restart**
  button that relaunches lich in place instead of only telling you to restart
  by hand. It drives the same `/restart` in-place relaunch the Linux installer
  already uses; the button stays until you use it, since the new binary only
  takes over on the next launch.
- **lich is on the AUR.** `yay -S lich-bin` (or `paru -S lich-bin`) installs
  the released binary; every release now pushes the updated PKGBUILD to the
  AUR automatically.

## [0.11.1] - 2026-07-21

### Fixed

- **The footer bar's working directory now follows `cd`.** It read the
  session's static start path, so a `cd` in the terminal moved the session
  card but left the footer stale. It now overlays the same live-cwd source the
  card follows.

### Changed

- **The session cwd is polled every ~300 ms** (was every 2 s), so a `cd` shows
  up in the card and footer promptly. Each read is one cheap syscall and emits
  only on change, so a static directory still costs nothing.

## [0.11.0] - 2026-07-21

### Added

- **A "what's new" popup after an update.** The first time you open lich on a
  new version, a dialog summarizes what changed — the release's changelog
  section, grouped into Added / Changed / Fixed, with a link to the full notes.
  It fires once per release and never on a fresh install. The notes are read
  from the changelog baked into the binary, so the popup works offline.
- **Pick which provider new sessions spawn by default.** lich was wired to open
  Claude Code for the routines that don't ask — a new worktree, the new-session
  hotkey, a project's first session. Settings › Providers now has a "Default
  provider" picker over the enabled harnesses, so those routines spawn Codex,
  opencode or Crush instead if you prefer. Disabling the chosen default falls
  back to the first enabled provider; the per-session New Session menu still
  picks a one-off provider as before.
- **Command palette — Ctrl/Cmd+K.** One shortcut, from anywhere, to jump to any
  session across every project — or to a project — without hunting through the
  tabs (which only show the active project's sessions). Type to filter by
  session label, project or path; ↑↓ to move, ↵ to open, Esc to close. Each
  session row shows its project, path and live status (busy / waiting / done).
  The shortcut is rebindable in Settings › Hotkeys, since Ctrl+K otherwise
  shadows the shell's kill-line. Jump-only for now — running actions from it can
  come later.
- **Search within a terminal — Ctrl+F.** Opens a find box in the top-right of
  the terminal: type to jump to the next match as you go — every match
  highlighted, with a live counter — Enter / Shift+Enter to step forward and
  back, and Esc to close. Like VS Code's terminal, Ctrl+F shadows the shell's
  own forward-char while the box is open; Esc hands the key back to the shell.
  Pairs with reload-surviving scrollback — there is now more history worth
  searching.
- **Terminal scrollback now survives a full page reload.** Reloading the window
  used to leave every terminal blank until new output arrived — the shells kept
  running, but their recent history lived only in the page. The backend now
  keeps a capped tail of each session's output and replays it into the terminal
  on reconnect, so a reload restores what you were looking at. The tail is
  bounded (2 MB per session), so very old scrollback still ages out.
- **Launching lich twice now focuses the open window** instead of failing. The
  second process detects the running instance holding the pinned port (via
  `runtime.json` and a token-gated liveness ping) and hands its URL to Chromium,
  which forwards to the running browser and brings its window to the front, then
  exits cleanly. A genuine port conflict — a non-lich process on the port —
  still fails with the same clear error as before. Window focus is best-effort:
  Wayland forbids an external process from raising a window, so lich relies on
  Chromium's own profile-lock IPC.
- A **notification queue** in the top strip — a bell beside the settings gear —
  gathers every session needing attention across all projects into one
  count-badged list: a session blocked waiting on you, or a turn that finished
  and you have not seen. Clicking a row routes straight to that session, even in
  a background project, so you can work in one project and jump to a
  notification from another without hunting for it. It is the persistent surface
  for the same signal the attention toast raises transiently (a toast is missed
  if you are away). The session you are currently viewing is never queued — its
  own terminal already shows the state — nor is a running (`busy`) one; a
  finished turn drops off once it has been seen. The queue lives in the page, so
  a full reload empties it until new events arrive.

### Changed

- **New app icon** — the purple meteor mark now ships across the Linux desktop
  entry, the Windows executable and installer, and the packaged icons.
- **The update check now repeats hourly, not just at startup.** A session left
  open for a long time now notices a new lich release mid-run instead of only on
  the next launch. The poll never stacks a second toast for a release it already
  surfaced, and dismissing one still holds until a genuinely newer version
  ships. Hourly keeps well within the unauthenticated GitHub API's rate limit.
- **Keeping a worktree now keeps its session, ready to resume.** Closing a
  worktree session and choosing to keep the checkout used to throw the session
  away — reopening the worktree later started a blank Claude with none of the
  earlier conversation. lich now parks the session instead of deleting it, so
  reopening the worktree brings it back and offers to continue the same Claude
  conversation right where it left off. Removing the worktree (rather than
  keeping it) still clears the session for good.
- **Footer bar spacing tightened.** The items sit closer together and the Browse
  code icon now matches the size of its neighbors.

### Fixed

- **Reopening an existing worktree no longer spawns a new one from its branch.**
  The new-worktree picker listed a worktree's branch twice — under "Worktrees",
  where picking it reopens the worktree, and under "Local branches", where
  picking it creates a new worktree off that branch — with the local list open
  by default, so the obvious choice quietly made a second worktree from the
  first. A branch already checked out in a worktree is now shown only under
  "Worktrees", and that group is expanded by default, so selecting it resumes
  the existing worktree.

## [0.10.0] - 2026-07-20

### Added

- An **Open Terminal** item on an agent session's card context menu spawns a
  plain shell rooted at that card's working directory — the live cwd when the
  watcher has reported one, else the session's start path — so dropping a
  terminal into the worktree an agent is running in no longer needs a manual
  `cd`. Shown only for agent sessions (a shell card already is one); the new
  shell is a full persisted session, like the `+ → Terminal` launcher.

### Changed

- License changed from MIT to **AGPL-3.0-only**. lich stays open source, but
  any distributed or network-served derivative must publish its source under
  the same license. Releases up to and including v0.9.0 remain under MIT.

### Fixed

- On Windows, `Ctrl+V` did not attach a clipboard image in Claude Code: Claude
  binds image paste to `Alt+V` (`ESC v`) there, not `Ctrl+V`, so lich's
  universal `Ctrl+V → SYN` (`\x16`) chord reached Claude unmapped and did
  nothing. `Ctrl+V` now emits the `Alt+V` sequence on Windows (Linux and macOS
  keep `\x16`); text paste stays on `Ctrl+Shift+V`.

## [0.9.0] - 2026-07-20

### Added

- A shell session's card now wears Claude's icon while a hand-run `claude` is
  live inside it, reported by the plugin's SessionStart hook (so it needs the
  lich plugin, like the status ring). The mark clears when Claude exits
  (SessionEnd) and on every session respawn; the card's real kind — what a
  respawn runs, what the resume prompt keys on — is untouched.

- Session cards follow the terminal's working directory: a `cd` in the session
  moves the card's path line — and with it the git branch, diff badge and PR
  badge, which reflect whatever directory is shown. The backend polls the PTY
  child's cwd every 2s — `/proc` on Linux, `proc_pidinfo` on macOS, a PEB read
  on Windows — and reports changes over the existing events channel; a failed
  read keeps the start path. Nothing is persisted: a respawned session reports
  its start directory again and the card resets with it.

- A read-only **Code** tab in the terminal's right dock: a tree of the active
  session's tracked files (`git ls-files`, so `.gitignore` is honoured and only
  versioned files appear — no `node_modules`, no build output) with an in-dock
  preview. Clicking a file opens it in a read-only CodeMirror view; selecting
  lines and right-clicking injects `path:start-end` (or `@path`) into the
  session's PTY, the same flow the diff review uses. Files carry their language's
  icon and folders expand in place. The right dock is now a tabbed panel —
  **Code** and **Review** — switched from the footer, and it follows the active
  session, so a worktree session browses its own checkout. Untracked files are
  not listed (they are invisible to `git ls-files`).

## [0.8.1] - 2026-07-17

### Changed

- Session cards now draw the processing status as a ring around the provider
  icon instead of swapping the icon out for a status glyph, so a running
  session keeps its agent's mark. The ring spins while busy, is solid emerald
  when the turn ends and amber while blocked on you; an idle session shows the
  bare icon. The fixed-size slot also removes the small layout shift the old
  swap caused.

### Fixed

- The vertical rule before the top strip's settings gear (added in 0.8.0)
  rendered as a half-height stub — a short line reaching only the middle of the
  bar — and has been removed.

## [0.8.0] - 2026-07-17

### Added

- Settings is now a per-project card, not a global screen. It opens at
  `/projects/:projectId/settings` as a "Settings" card in the project's session
  sidebar (Warp-style): the sidebar stays visible, the project stays active
  (hotkeys, toasts and status badges intact), and the Project group shows the
  current project's overrides instead of listing every open project. It is a
  pure UI concern — the persisted workspace is untouched.
- A permanent **Home** tab, pinned first and non-closable, gives an
  always-available plain shell rooted at the system home directory — a scratch
  terminal, and the home the Linux self-update flow relaunches into.
- lich is no longer Claude-only: Codex, opencode and Crush join Claude Code as
  selectable providers. lich detects which harnesses are installed on the
  machine, and a new Settings → **Providers** group lists them with an enable
  toggle (one not found on `$PATH` can't be enabled). Enabling a provider adds
  it to the New Session menu — each with its own brand icon in place of the
  generic bot — and reveals its settings: a custom binary path, global with a
  per-project override, resolved the same way Claude's already was
  (`provider.<id>.bin`; Claude keeps the legacy `claude.bin` key). Claude Code
  stays enabled by default, so nothing changes until you opt one in. Non-Claude
  sessions run their TUI in a PTY without the Claude-only extras (resume,
  ai-title, live status badges); their cards show the provider's mark when idle.
- lich now checks for its own updates on startup and surfaces a newer release
  in-app. The running binary learns its version at build time (`-X main.version`
  from the git tag), polls the GitHub releases for `omartelo/lich`, and — when a
  newer one exists — shows a toast. On Windows and macOS, where lich owns its
  binary, one click downloads the release asset, verifies its SHA-256 against the
  release `checksums.txt`, atomically swaps the binary in place
  (`internal/appupdate`, via `minio/selfupdate`) and asks for a restart. On
  Linux, where the binary belongs to the system package manager, the toast
  instead offers to open the release page or paste the `install.sh` one-liner
  into a terminal for the user to run (never executed automatically).
- After the Linux installer replaces the binary, it can relaunch a running lich
  for you: `install.sh` POSTs a new token-authenticated `/restart` endpoint,
  which spawns a detached successor process and closes the current window so the
  new binary takes over. It reaches lich through the session env
  (`LICH_PORT`/`LICH_TOKEN`) when run inside a lich terminal, or a
  `runtime.json` (pid/port/token) lich writes to its config dir when run from any
  other terminal while lich is open.
- Helium Browser is now accepted as a Linux Chromium-family shell. The launcher
  probes `helium-browser` alongside Chromium, Google Chrome and Brave, and the
  install/runtime dependency checks document it as a supported browser.

### Changed

- Menus and bars gained separators. The session card's context menu
  (rename / close) and the New Session dropdown (providers · terminal ·
  worktree) now use menu-native separators; a vertical rule precedes the
  settings gear in the top strip, and one divides the git context from the
  clock in the footer status bar (only while a project is active).

### Fixed

- A long worktree path overflowed the session card tooltip: a path is one
  unbroken token (slashes are not break points), so `max-w-xs` could not wrap
  it. It now wraps within the max width (`break-all`).

## [0.7.0] - 2026-07-16

### Added

- The session card tooltip is now a rich mini-card: full label, working path,
  branch, open-PR badge and diff stat, opening to the right and themed to match
  the card. A long label clipped in the card is readable on hover without
  widening the sidebar. It reuses the git and PR data the card already computes.

### Fixed

- Windows no longer floods the desktop with console windows. lich's Windows
  binary runs in the GUI subsystem (no console of its own), so every console
  tool it shells out to — `git`, `gh`, `claude` — spawned a fresh console window
  per call; with git status polled every few seconds per session and the PR
  lookup firing on every window focus, the screen filled with windows and the
  machine became unusable. Child console processes are now created with
  `CREATE_NO_WINDOW` (`internal/winexec`), a no-op on every other OS.
- The session card and footer diff badge showed `+0 −0` in a repository with no
  commits, even though the review panel rendered the full diff: `git diff
  --numstat HEAD` errors without a HEAD and skipped the untracked-file count. It
  now diffs against git's empty tree when HEAD is missing, and a numstat failure
  no longer skips counting untracked additions.

## [0.6.0] - 2026-07-16

### Added

- An open pull request now badges the session card too, not just the footer. The
  same `PR #N` chip the footer shows for the active session appears on every card
  whose branch has an open PR, so one is visible per session without selecting
  the card first; clicking it opens the PR in the browser. Reuses the footer's PR
  lookup, is hidden when `gh` is absent or unauthenticated, and clears when the
  PR merges or closes.
- Windows releases now ship an installer (`lich-*-windows-amd64-setup.exe`,
  Inno Setup): per-user install under `%LocalAppData%\Programs\lich` with no
  admin prompt, Start Menu entry, a proper "Installed apps" registration with
  an uninstaller, and the lich icon on the executable. The bare portable exe
  keeps shipping alongside it.
- lich keeps a persistent log: `<config-dir>/lich/lich.log` (`lich-dev.log`
  under `task dev`), structured records with source file:line, rotated at 5MB
  with one previous generation kept. `LICH_LOG_LEVEL` (`debug`/`warn`/`error`)
  tunes verbosity. Every RPC failure is recorded with its method name, and the
  session token never reaches the log. This is the audit trail the future
  console-less Windows build will rely on.
- Experimental Windows build (`lich.exe`, `task build:windows`). Terminal
  sessions run under ConPTY, the window opens in Chrome or Edge (found via
  their conventional install paths), shell cards fall back to `COMSPEC`, and
  npm's `claude.cmd` shim is spawned through `cmd.exe /c`. Releases now ship a
  `windows-amd64.exe` asset built — and backend-tested — on a Windows runner
  in parallel with the Linux packages.
- Experimental macOS build (`lich-*-darwin-arm64`, `lich-*-darwin-amd64`,
  `task build:mac`). macOS is Unix, so the terminal (creack/pty), the shell and
  the native folder picker already work through the shared seams; only the
  Chromium launcher gained a macOS list, finding Chrome/Chromium/Edge/Brave in
  their `.app` bundles under `/Applications` and `~/Applications` (they never
  land on PATH). Releases now ship both-arch darwin binaries, built and
  backend-tested — the PTY included, on a real macOS runner — alongside the
  Linux and Windows jobs. Unsigned: Gatekeeper quarantines the binary until
  notarization ships — right-click-Open, or clear the quarantine attribute, to
  run it.

### Changed

- The Windows binary is now a GUI-subsystem build: launching `lich.exe` no
  longer drags a console window along, and closing that console can no longer
  kill the app by accident. Logs live in `%AppData%\lich\lich.log` — the
  console mirror became best-effort so a missing stderr never poisons the
  file half of the log.
- Scrollbars are now discreet across the app. The heavy native Chromium
  scrollbar is replaced by a thin translucent thumb (diff, settings, sidebar,
  tabs) via a single global `::-webkit-scrollbar` rule; the terminal keeps its
  existing 6px overlay.

### Fixed

- A single-line selection in the diff review panel built a file reference with a
  redundant end (`path:19-19`). It now collapses to `path:19`, keeping the
  `start-end` range form only when the selection spans more than one line — for
  both the injected PTY reference and the context-menu label.

## [0.5.0] - 2026-07-15

### Added

- Reopening a session card that ran Claude Code before the last restart now asks
  whether to resume that conversation. Accepting spawns `claude --resume` on the
  session id the `SessionStart` hook recorded, so the card picks up where it
  stopped; declining starts an empty session as before. The prompt is asked once
  per card, the first time it is opened after a restart, and never for a shell
  card or one created in this run.
- A project tab badges what its sessions are doing while you work elsewhere: a
  bell when one is blocked waiting on you, a spinner while one is running, a
  check when a turn finished. The active tab never badges — its cards already
  say the same thing, per session. The check clears once the project has been on
  screen; the bell and the spinner stay while they are true, so a tab you leave
  mid-run keeps saying so.

### Fixed

- A session card kept its status indicator (spinner, check, bell) when its
  project was not the one on screen. Switching projects mid-run unmounted the
  card and dropped the state along with it, so coming back showed no spinner for
  a session Claude was still working on; the state now lives in a store that
  outlives the card. A session that starts needing your input while in the
  background also shows its bell once the toast routes you to it.

## [0.4.0] - 2026-07-15

### Added

- Drag a session card or a project tab to reorder it. The list rearranges live
  under the cursor and the new order is persisted, so it survives a restart;
  releasing outside the list (or pressing Escape) leaves the order untouched.
  Reordering also works from the keyboard: focus a card or tab, then Space and
  the arrow keys.
- `install.sh` — one-liner install (`curl ... | sh`) that detects the distro,
  downloads the matching package from the latest release, verifies its
  checksum and installs it through the native package manager, then checks
  the runtime dependencies (Chromium-family browser, zenity) are present.

### Changed (BREAKING — new shell)

- lich now opens in the system Chromium's `--app` mode instead of the
  WebKitGTK webview, eliminating the compositor paint jank for good (decision
  record: `docs/chromium-shell.md`). The Wails toolkit, the bundled WebKitGTK
  and the `GDK_BACKEND=x11` workaround are gone; the binary is pure static Go.
  New runtime requirements: a Chromium-family browser on PATH and zenity for
  the folder picker. UI preferences (theme, font, hotkeys) reset once — they
  now live in the Chromium profile; the workspace (projects/sessions, SQLite)
  carries over untouched.
- The terminal is now xterm.js with the WebGL (GPU) renderer, replacing the
  patched ghostty-web canvas pipeline — noticeably smoother TUI scrolling and
  streaming, and correct Shift+Tab/Alt-chord/mouse handling without patches.
  Hidden sessions no longer keep a terminal at all: their state is serialized
  and replayed on return, cutting memory with many open sessions.

### Changed

- Diff counters (+added/−deleted) now use one palette everywhere: green for
  additions, red for deletions. Session cards and the footer previously showed
  them in blue/pink while the review panel used green/red.
- Backend services are now reachable over the loopback listener (HTTP RPC +
  event socket); the frontend no longer depends on the Wails binding bridge.
- Hidden sessions no longer hold a canvas backing store (several MB each at
  window size). The bitmap is released when a session leaves the screen and
  transparently reallocated on the next paint when it returns, cutting webview
  memory with many open sessions.
- Spawned shells keep a user-set `WEBKIT_DISABLE_*` variable — it was stripped
  as packaging leakage back when lich's own AppImage set it; nothing does
  since the WebKitGTK shell was removed.

### Removed

- The AppImage artifact. It cannot declare dependencies, and lich needs a
  system Chromium and zenity at runtime either way — a "portable" AppImage
  that isn't self-contained betrays the format. Install via `install.sh`
  (detects the distro, verifies checksums, installs the native package) or
  grab the `.deb`/`.rpm`/`.pkg.tar.zst` directly; the bare static binary
  also ships with every release.

## [0.3.0] - 2026-07-14

### Added

- Claude Code plugin integration. lich pairs with a companion Claude Code
  plugin ([`omartelo/lich-plugin`](https://github.com/omartelo/lich-plugin)),
  installed and updated from within the app: a one-click install modal when it
  is missing, and an actionable toast when a newer plugin release ships. The
  plugin reports session activity back over the existing loopback transport;
  every contract is documented in `docs/hooks/`, which lich owns as the
  canonical source and the plugin references.
- Session cards reflect Claude Code state live — a spinner while Claude is
  producing output, a check when the turn ends, and a bell when Claude is
  blocked on you (a permission prompt or an idle input request). The bell also
  raises an actionable toast that routes to the waiting session, reachable even
  when it lives in a background project, and skipped for the session already on
  screen. A stale indicator clears when the session ends or is `/clear`ed.
- Sessions auto-name from Claude's own title. When Claude generates its session
  summary (the `ai-title` shown in `claude --resume`), lich adopts it as the
  card label — unless you have renamed the session, which always wins.
- A session's git badge refreshes the moment Claude edits files, ahead of the
  ~3s poll, so the diff counts and branch stay current without the lag. The
  poll stays the baseline, so the badge works unchanged without the plugin.
- An open pull request for the active session's branch surfaces as a clickable
  badge in the footer — `PR #N` with a pull-request icon — that opens the PR in
  the OS browser. It resolves the PR through the `gh` CLI, shows only while the
  PR is open (a merged or closed one clears it), and re-checks on window focus
  so a merge done in the browser drops the badge on return. Hidden when `gh` is
  absent or unauthenticated.

### Changed

- The footer's diff toggle now renders as a bordered muted chip, with a diff
  icon in its zero-change state, matching the new PR badge's look.
- The Linux Arch package no longer hardcodes `pkgrel` in `nfpm.yaml`. nfpm
  defaults it to `1`, so the produced `.pkg.tar.zst` version is unchanged
  (`X.Y.Z-1`) — the field just isn't pinned in the repo anymore. `pkgrel` is
  mandatory in the Arch package format and has no source in the git tag, so it
  stays `1` rather than being derived.

## [0.2.0] - 2026-07-13

### Added

- Settings gained a "Project" group with a per-project Claude Code binary
  override. The backend already resolved project → global → `$PATH`; the
  override just had no UI.
- Terminal URLs (OSC 8 hyperlinks and detected URLs) now hover-underline and
  open in the OS browser on Ctrl/Cmd-click. ghostty-web ships link detection
  but registers no provider by default, and its `window.open` is trapped by
  the WebKitGTK webview; lich registers both providers and routes clicks
  through Wails' `Browser.OpenURL` to the desktop default.

### Fixed

- Mouse wheel now scrolls instead of sending arrow keys. ghostty-web reports
  no mouse events, so its alternate-screen emulation turned each wheel tick
  into an arrow key — which Claude Code flagged as "arrow keys · use PgUp/PgDn
  to scroll". The wheel now forwards a real SGR report to apps with mouse
  tracking (they scroll by their own line increment), falls back to PgUp/PgDn
  in the alternate screen otherwise, and scrolls ghostty's own scrollback
  everywhere else.
- Stray editable nodes no longer accumulate in the terminal container. On the
  forced X11 backend, middle-click primary-selection paste and drag-drop
  inserted editable nodes past ghostty's `beforeinput` guard, pushing the
  in-flow canvas down and leaving selectable text behind; a `MutationObserver`
  now removes any node other than the canvas and textarea ghostty owns.
- Terminal sessions no longer inherit the AppImage's runtime environment.
  Beyond the vars stripped in #3, `childEnv` now drops the AppImageLauncher
  vars and the `WEBKIT_DISABLE_*` pair, and scrubs mount paths out of
  `LD_LIBRARY_PATH`, `PATH`, `XDG_DATA_DIRS`, `GDK_PIXBUF_MODULE_FILE` (and any
  future path list) — the bundled Ubuntu libs broke linkers and GTK apps
  launched from a lich terminal. User-set entries survive; outside an AppImage
  the environment passes through untouched.
- The `GDK_BACKEND=x11` forced at startup no longer leaks into spawned
  sessions: the session environment is snapshotted before the GTK tweak.
- `task dev` now opens alongside an installed lich: dev instances register a
  distinct GTK application ID (`lichdev`), so GTK single-instance no longer
  swallows the dev window when the AppImage is running.

## [0.1.1] - 2026-07-12

### Fixed

- The AppImage aborted on startup on any distro that is not Debian/Ubuntu
  (`Failed to spawn child process ".../webkitgtk-6.0/WebKitNetworkProcess"`).
  The bundled WebKitGTK hardcodes its helper paths at compile time, so the
  packaging now binary-patches `libwebkit*` to resolve them inside the AppDir
  (the same relocation tauri's bundler applies), marks the bundled
  `WebKit*Process` helpers executable (wails3 copies them without the exec
  bit), and disables the webkit sandbox, which would otherwise require the
  host's `bwrap` at another baked path. See
  `build/linux/appimage/fix-appimage.sh`.

### Added

- `LICH_DEV` environment variable: when set, the app uses a separate SQLite
  database (`lich-dev.db` instead of `lich.db`), keeping development work away
  from the real workspace. `task dev` sets it automatically.

## [0.1.0] - 2026-07-12

### Changed

- Terminal rendering is ~4× faster under heavy TUI load (nvim scroll worst-case
  main-thread stall down from ~200-250ms to ~40-70ms; idle paint cost down ~8×).
  Four changes: PTY output is now coalesced on the Go side for visible sessions
  too (8ms batches; hidden stays at 250ms), blank cells skip `fillText`
  entirely, cell backgrounds are painted as one `fillRect` per same-color run
  instead of per cell, and plain glyphs are cached on offscreen sprites and
  blitted with `drawImage` instead of re-rasterized with `fillText` every
  frame. The remaining ceiling is ghostty-web's per-row WASM cell
  materialization (`getLine`), which is only fixable upstream.

### Added

- Git diff review panel: the footer's diff counters toggle a resizable split at
  the terminal's right showing the active session's uncommitted changes, one
  collapsible card per file with syntax highlighting, line numbers, and hunk
  separators (CodeMirror 6). Selecting lines and right-clicking injects
  `@path` or `path:start-end` references into the session's PTY; per-file
  buttons add the file as context or discard its changes after confirmation.
  A full-screen mode overlays the terminal area.
- PTY-backed terminal harness with multiple sessions per project.
- Multi-project workspace: open projects through the OS picker and switch between
  them via a Discord-style rail with tabs.
- Session cards showing the working directory, git branch, a diff badge, and an
  untracked-line count.
- Appearance settings: System/Light/Dark theme, UI zoom, and a separate terminal
  theme.
- Configurable hotkeys, including terminal-aware zoom.
- Warp-style footer bar with git status and file attach.
- Git worktree sessions: create a worktree from a local or remote base branch
  (fetched and tracked) with an optional custom or auto-generated name, resume
  an existing worktree, and open Claude Code directly in its checkout. Closing
  the session asks whether to keep or remove the worktree — removing one with
  uncommitted changes asks a second confirmation before forcing — and session
  cards and the footer follow the worktree's path, branch, and diff.
- Right-click context menu to rename or close a session.
- Bundled FiraCode Nerd Font.
- Configurable Claude Code binary path in settings.
- Toast feedback when copying from the terminal.
- New-session dropdown on the sidebar "+": spawn a Claude Code session or a
  plain shell terminal; the session type persists and restores with the
  workspace.
- Workspace persisted in SQLite; UI preferences in `localStorage`.

### Changed

- Renamed the project from `skipo` to `lich`: Go module
  `github.com/omartelo/lich`, app and binary name, data directory
  `<data-dir>/lich/lich.db`, `lich.*` `localStorage` keys, and every platform
  build asset.
- Set release metadata in `build/config.yml` (product `lich`, identifier
  `dev.lich.app`, version `0.1.0`).
- Renamed the `internals` package to `internal`.
- Translated `CLAUDE.md` to English.
- Home paths render with a `~` prefix and an overflow fade on cards.
- Switched the base color palette from zinc to neutral.

### Fixed

- Terminal now fills the container edge to edge — replaced ghostty-web's
  FitAddon, which reserved a fixed 15px scrollbar gutter and left a band on the
  right.
- Hid the native caret over the terminal canvas.
- Synthesized block-element glyphs in the terminal renderer.
- Derived cell height from the font bounding box.
- Debounced terminal refit to keep window drags fluid.
- Focus the previous tab when closing the active project.
- Shift+Tab now reaches terminal apps as backtab (`ESC [ Z`) and Alt chords get
  their ESC prefix — ghostty-web 0.4.0 drops both, and WebKitGTK reports
  Shift+Tab as the `ISO_Left_Tab` keysym.
- Long worktree paths wrap instead of overflowing the close dialogs.

### Performance

- Spawn session PTYs lazily on first view.
- Lowered the git-status poll interval to 3 s.
- Paused the ~60fps render loop of hidden terminals; only the visible terminal
  paints (state keeps updating, so switching back repaints instantly).
- Coalesced PTY output of hidden sessions in the backend to one event per 250 ms,
  flushed immediately when the session is shown.
- Skipped the resize-driven refit for hidden terminals; they refit once on show.
- Shared one git-status poller per repository path with equality bailout: with
  20+ session cards the idle burst of ~44 IPC calls and ~88 git subprocesses
  every 3 s collapses to one fetch per path, and unchanged status no longer
  re-renders anything.
- Removed the per-cell defensive copy in ghostty-web's `getLine` (pool-backed
  row references), gated rendering while scrolled with nothing dirty, and
  memoized scrollback lines — reading scrollback now costs ~0 paint, and heavy
  TUI throughput renders at ~40fps instead of ~25.
- Terminal I/O now flows over a local binary WebSocket (random loopback port,
  token-authenticated) instead of one Wails HTTP call per keystroke and one
  `evaluate_javascript` per output chunk; falls back to the Wails paths
  automatically if the socket drops.
- Forced `GDK_BACKEND=x11` on Linux (only when unset): WebKitGTK under Wayland
  fractional scaling rendered every damage frame at 2x and downsampled on the
  CPU, costing ~40ms per frame in a full-size window. Under Xwayland typing is
  stall-free at full frame rate.

[Unreleased]: https://github.com/omartelo/lich/compare/v0.16.0...HEAD
[0.16.0]: https://github.com/omartelo/lich/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/omartelo/lich/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/omartelo/lich/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/omartelo/lich/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/omartelo/lich/compare/v0.11.1...v0.12.0
[0.11.1]: https://github.com/omartelo/lich/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/omartelo/lich/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/omartelo/lich/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/omartelo/lich/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/omartelo/lich/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/omartelo/lich/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/omartelo/lich/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/omartelo/lich/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/omartelo/lich/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/omartelo/lich/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/omartelo/lich/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/omartelo/lich/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/omartelo/lich/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/omartelo/lich/releases/tag/v0.1.0
