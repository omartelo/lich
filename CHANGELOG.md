# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **The session roster says what each session is doing.** `lich sessions` grew a
  `state` column and the `list_sessions` tool a `state` field, carrying what that
  session last reported: `busy` mid-turn, `done` at a prompt and free, and
  `waiting` — blocked on a permission prompt only a human can answer. That last
  one is the point: work handed to a waiting session sits behind that prompt
  unread for as long as nobody is at the screen, so an agent reading the roster
  now knows to tell you rather than send into it. Only the providers whose
  companion plugin reports state have one at all, so a session that reported
  nothing shows `-` (`""` in `--json`), which means "not reported" and never
  "idle".

- **`lich open` can hand the new session its task.** `--prompt "<task>"` opens a
  session and gives it that task in one command, which is what the `open_session`
  tool has been doing and the command line could not. It matters for the agents
  that reach lich through a shell rather than through tools — opencode, Crush,
  anything spawned without lich's MCP server — where fanning work out cost two
  commands per worker, and the second one was the one an agent forgot. The task
  is queued for a worker still running its setup script, exactly as `lich send`
  queues one, and the outcome is printed under the session's own lines in the
  words `send` uses, ticket included. `--json` still prints one object: the
  session, plus a `delivery` key with the send's result when a task came with it.

- **Asking for work to be fanned out opens lich sessions, not invisible
  subagents.** Every coding agent lich runs also has subagents of its own, and
  its harness describes those to it at length, from its first turn — so "spin
  these three tasks up in worktrees" produced three processes with no checkout,
  no card, and nothing to open, read or take over. lich now says the one thing
  that was missing: sessions it opens are visible and steerable and subagents
  are not, so parallel work belongs in sessions. Claude Code and oh-my-pi are
  told at spawn (both take `--append-system-prompt`); the rest read the same
  point in lich's own MCP instructions, since none of them has a flag that
  appends to a system prompt and lich will not rewrite config that belongs to
  the user. Opening a worker also got cheaper: `open_session` now takes the task
  itself, so fanning work out costs one call per worker instead of two, and the
  task waits for that session's agent to come up rather than landing in a setup
  script.

- **opencode and Crush sessions resume.** A restored card running either of them
  used to start a fresh conversation every time, even though lich had been
  storing the id of the one it ran before. Both now offer the same resume prompt
  Claude Code, Codex and oh-my-pi have: accept and the conversation continues
  where it stopped. lich still only offers a resume it can honour — the two keep
  their conversations in a database rather than one file per session, so it asks
  that database whether the conversation is still there, read-only. Crush files
  its own per checkout, so a card resumes the conversation belonging to its own
  worktree.

- **A session whose process ended says so, and offers a way on.** When the
  program inside a card's terminal exits — a double Ctrl+C, `/exit`, a provider
  that crashed — the card used to print a bare `[process exited]` and sit there,
  useful for nothing until you closed it by hand. The terminal now keeps its
  scrollback, which is the only account of what happened, and shows a band under
  it naming the outcome (with the exit status when the process left one) beside
  **Restart** and **Close**. Restart spawns the same kind of session again in the
  same directory, as a new conversation. lich never closes the card for you:
  a crash's output and a resume that died at boot are both worth reading before
  anything decides they are not.

- **Closing a session mid-turn asks first.** The card's × killed the process
  outright, taking whatever the agent was doing — or the permission prompt it was
  blocked on — with it. Closing a session that is working or waiting now asks
  once, the way closing a project already did. A session with nothing in flight,
  or one whose provider reports no state at all, still closes on the first click.

### Fixed

- **A task handed to a session that is not up yet is queued, not lost.** Giving
  a task to a session on a fresh worktree meant racing that checkout's setup
  script: the install runs in the same terminal the agent will use, it can take
  minutes, and a task typed into it is read by `pnpm install` and dropped. lich
  refused to type into it — but the refusal came back as a failed send with the
  task gone, and it fell on whoever sent it to guess when to try again. That is
  the main path of a fan-out, where every worker is a checkout nobody has
  installed yet. Now the sender gets its ticket immediately and the task goes in
  the moment that session's agent is the program reading its terminal, however
  long the setup takes. A queue that can never end — the session ends, or ten
  minutes pass with nothing at a prompt — comes back as a reported failure at
  the sender's own prompt, saying the task is gone rather than waiting
  somewhere, instead of a ticket that hangs until it expires an hour later. A
  target that is merely busy is unchanged: the task queues at its prompt and it
  answers a turn later.

- **A helper that outlives a lich restart can talk to lich again.** The `lich`
  command line and lich's own MCP tools read the loopback coordinates the session
  they run in was given, and the connect token behind them is minted fresh every
  time lich starts. Anything still holding the old one — a background agent the
  coding agent parked outside the session, a `nohup`, a detached pane — got
  `403 Forbidden` on every call from then on, with nothing saying why, and the
  only cure was starting it over. A refused call now retries once with the token
  the running lich recorded, so the helper simply keeps working. It is retried
  only against the same port: a machine can run an installed lich beside a
  development build, and a message delivered to the wrong one lands in a window
  its sender cannot see — so that case, and every other refusal, now says what
  happened instead of printing the status code.

- **lich is an application on macOS.** The Homebrew install shipped a bare
  command-line binary, so nothing landed in `/Applications`: lich was absent
  from Launchpad, from Spotlight and from the Finder, and had no icon anywhere.
  The tap now publishes a cask that installs `Lich.app` under its own icon and
  keeps `lich` on `PATH` as before. The Dock still shows the browser while lich
  runs: the window belongs to it, and macOS offers nothing like the window
  class Linux matches against the launcher, so the lich icon is the one you
  launch from rather than the one you switch to. Upgrading from the old formula
  means `brew uninstall lich` first; INSTALL.md says so.

## [0.32.0] - 2026-08-13

### Added

- **oh-my-pi runs the lich plugin.** Settings › lich plugin now offers omp the
  same install as the others, writing the released extension into omp's own
  `extensions/` directory — so an omp card shows what its session is doing,
  refreshes git status as files change, and takes its name from the
  conversation's title once omp has written one. The install also registers
  lich's MCP server in omp's `mcp.json`, merged in beside whatever is already
  there, which puts the tools for reaching the other sessions in an omp agent's
  own tool list. Two gaps are the harness's own, and Settings says the first out
  loud: omp has no observed approval event, so a session waiting on your
  permission shows a spinner rather than a bell; and the extension runs inside
  omp, so nothing survives to report that the CLI has left — a card keeps its
  last indicator until lich respawns it, exactly as an opencode one does.

- **An omp session resumes its conversation.** Reopening a card that ran omp
  before a restart offers the same "continue where it left off" prompt Claude
  Code and Codex cards get, and lich only offers it when omp's own transcript
  for that conversation is still on disk.

- **A session can be opened on a specific model.** `lich open --model <model>`
  and the `open_session` MCP tool's `model` argument start the new session's
  provider on the model you name, in that provider's own spelling — Claude Code,
  Codex, opencode and oh-my-pi each take the name their own `--model` accepts.
  Fanning work out to a worktree can now put the cheap model on the mechanical
  half of the job and keep the expensive one where it earns its price. The model
  is recorded on the session, so a reload, a respawn or the resume of a parked
  worktree session all come back on it. Crush and `shell`
  sessions are refused rather than silently opened on the default: Crush spells
  `--model` only on its non-interactive `run` subcommand, so the TUI lich spawns
  has nowhere to receive one.

- **oh-my-pi can run without permission prompts.** Settings › Providers now
  offers oh-my-pi the same "run without asking" ladder as the other providers,
  spawning it with `--auto-approve`. It was the one provider left out, because
  its spelling had never been checked against the binary.

### Fixed

- **A task the target picks up the instant it lands is no longer given up on.**
  lich types a relayed message and presses Enter a beat later, and anything the
  target reported in that beat counted for nothing. A session that started
  working right there looked like one that had never read the task: the errand
  was closed as "unread" while the message was queued and running, and the
  worker's own `lich reply` then failed with "unknown ticket". The same beat
  could swallow the end of a turn that was already in progress instead, leaving
  an errand whose answering turn was skipped and which never closed at all.

- **A result whose notice never reached the prompt is announced again.** The
  `[lich]` line naming waiting results is the only thing that tells an agent to
  collect them — the results themselves are never typed. A write that failed
  used to count as a notice given, and the sender was never told again. It now
  goes out at the end of the sender's next turn.

- **The card's "results ready" count no longer settles on the wrong number.**
  Two workers answering at the same instant could announce their counts out of
  order, leaving the card saying one result with two of them waiting until
  something else changed.

- **The update prompt no longer offers an Install that installs nothing.** On
  Windows and macOS, a lich whose own directory is not writable can neither
  swap its binary nor name a package manager to update it — and the prompt
  still showed an Install button, which opened a terminal and pasted an empty
  line. The button is now offered only where there is a command to run, and the
  release page takes its place as the prompt's main action.

- **A theme repository's update stops asking forever.** When a pack dropped or
  renamed the file a theme came from, updating installed the new pack but left
  that theme stamped with the version it arrived at — so the next check saw a
  newer manifest again, said "Updated", and changed nothing. The dropped theme
  is now kept where it is and takes the version that was just installed:
  Appearance stops offering the same update, and nothing you may be wearing is
  deleted behind your back.

- **A custom theme no longer disappears when lich adds a color.** An installed
  theme was checked against the full token list at load time, so the first
  release to add an app token would have dropped every theme imported before it
  from Settings › Appearance, silently. A stored theme now fills what it lacks
  from the bundled theme of its own scheme and keeps working; imports are still
  refused for a missing token, where the file is in front of you to fix.

- **A theme imported mid-install is no longer overwritten.** Installing a
  repository checked for existing ids and then wrote regardless, so a theme that
  appeared between the two was replaced without the confirmation the check
  exists to ask for. The write now refuses it and reports it as a conflict.

- **A review thread keeps its newest replies.** A pull request thread with more
  than 50 comments was read from its oldest end, so the replies that mattered —
  the last ones — were the ones missing from the diff and the Conversation tab.
  It now reads the newest 50.

- **A dropped file could resolve to the wrong twin.** The search that turns a
  pathless drop back into a real file gives up after a fixed number of entries,
  and when it ran out between two same-named files it kept the one it had
  already seen — pasting a path to the wrong file, which an agent then edits.
  A level the search could not finish now resolves to nothing, which falls back
  to the copy, as two twins always should have.

- **A copy could overwrite an earlier drop.** With 999 copies of one name
  already stored, the next drop of it reused the last name and truncated that
  copy, whose path may still have been sitting unsent in a prompt. The drop is
  refused instead.

- **A plugin install is whole or it fails.** Installing the plugin into
  opencode or Crush wrote what it fetched straight onto the file the harness
  loads: a release file larger than the 1MB cap was silently cut short, and a
  crash or a full disk mid-write left a half-written module or hook script —
  either one still carrying the version marker that says the install
  succeeded. An oversized file is now refused, and every file is written
  through a temporary one, so an interrupted install leaves the previous
  version in place rather than a broken one lich reports as current.

- **The RPC surface no longer exposes lich's own wiring.** Three methods that
  exist for lich to call itself — the hooks' session-state stream and two
  startup wiring calls — answered a request from the page like any other
  service method. A call to the first could close another session's pending
  delegations; a call to either of the others could unwire the relay's plugin
  lookup or a project's gh account while the app was running. They are now
  refused. Discarding a file is guarded the same way: a path that names the
  repository root instead of a file is rejected, where before it would have
  emptied the whole index.

- **Searching sessions no longer crashes on some accented text.** A palette
  search that matched a conversation containing certain uppercase letters —
  ones that grow when lowercased, like `Ⱥ` or `İ` — brought the window down
  instead of showing the result.

- **A session no longer gets stuck waiting for a setup that already
  finished.** Two ways a fresh worktree's session could stay "not ready"
  forever, refusing every message relayed to it until the sender's ticket
  timed out: the marker the setup wrapper prints was matched one PTY read at
  a time, so a read that cut it in half missed it for good; and a session
  spawned into a worktree with no setup script at all could still be armed
  to wait for a marker nothing would ever print, whenever the provider's
  binary happened to be named `sh`.

- **A transcript that fails mid-read no longer shows a short cost.** A
  transient read error while adding up a session's spend was treated as the
  end of the file, so the footer showed the total of the lines that made it
  through, as if that were the whole bill. Such a read now shows no number
  and resumes on the next turn.

- **Closing another session no longer steals your focus.** `lich close` and the
  `close_session` tool wrote a new active card for the project whichever session
  was closed, and the window applies that without a say of its own — so an agent
  closing a worker moved you off the card you were reading. Only closing the
  active card moves the focus now, which is what the window has always done on
  its own.

- **An unreadable `.worktreeinclude` no longer seeds the files it exists to
  block.** A file lich could not read fell back to the built-in defaults, so a
  new worktree got the `.env*` copies the override was written to stop. Only a
  missing file means "use the defaults" now; a present one that cannot be read
  seeds nothing.

- **A relayed task no longer names a tool the target does not have.** A Crush or
  oh-my-pi session gets its lich tools from an MCP server registered at install
  time, and that registration is skipped when lich cannot resolve its own binary
  path — but the prompt lich typed still told the worker to answer with
  `reply_to_session`. It now asks for `lich reply` instead, the shell command
  that works everywhere, as it already did for a session with no tools at all.

- **A worktree is opened, not duplicated, when its branch is typed in another
  case.** Asking for `--worktree Auth-Fix` with a checkout of `auth-fix` already
  on disk created a second branch and a second checkout beside it. The lookup
  now folds case, like every other branch name lookup around it.

- **A session that fails to close no longer leaves a ghost card.** When the
  terminal refused to go down, the card stayed on screen until a reload even
  though the session was already gone from the workspace.

- **`lich open --base` without `--worktree` says so.** The base was silently
  dropped and the session opened on whatever branch was current.

- **`lich open feature-x` no longer opens a session and says nothing.**
  `sessions`, `open` and `worktrees` took a positional argument and ignored
  it, so a branch name typed without `--worktree` opened a session in the
  caller's own checkout while reading like it had made one of its own. All
  three now refuse the stray argument and print the command's usage, as
  `send`, `wait`, `reply` and `close` already did.

- **Two sessions opened at once no longer take the same label.** Concurrent
  opens in one project read the same label counter, and `lich send` cannot tell
  two cards with one name apart.

## [0.31.0] - 2026-08-13

### Added

- **Delegate can open the fan-out.** The delegate picker gains a pinned "New
  worktree session…" row — sticky under the list, never filtered by the
  query, and offered even when no other session is live, which is exactly
  when fanning out is most useful. Picking it hands the orchestrator a
  delegation into a fresh worktree checkout; the agent opens the session and
  sends the task itself, picking the branch name from the task it was given.

- **The card says when results are back.** A session with uncollected
  results — answers to tasks it delegated, waiting in the relay's inbox —
  now says so on its card ("2 results ready"), in the same quiet line the
  relay marks use. The nudge tells the agent; this tells you. It clears the
  moment the agent collects, and never interrupts a turn in progress.

### Changed

- **Delegate types a delegation, not a question.** Picking a session in
  "Delegate to session…" used to put `Ask the "docs" session to ` at the
  prompt, and whatever you typed next had to bend into "to <verb>" — a
  question or pasted context read wrong, and "Ask" was not even the button's
  word. It now types `Delegate to the "docs" session: `, and anything can
  follow the colon: an order, a question, a dump of context.

- **A worker's answer no longer floods the orchestrator's prompt — it is
  announced, and collected.** A session fanning work out to others used to get
  every answer typed back at its prompt in full, and every arrival restarted
  its turn with the whole text left in its context window: orchestrating N
  workers cost that N times over. Now an unattended result goes to the
  sender's inbox, and what is typed is one short `[lich]` note — results
  landing within a couple of seconds share it, and a sender mid-turn hears
  nothing until its turn ends. `wait_for_answer` with no ticket (or a bare
  `lich wait`) collects everything at once, inside a turn the sender chose,
  and reports who still owes an answer. Uncollected results expire with the
  ticket TTL, one hour.

- **lich's MCP server now briefs the agent on the whole journey.** The
  `initialize` handshake carries `instructions` — how to fan work out into
  worktree sessions, that answers announce themselves so polling is waste, and
  that a relayed reply should be a concise report, not a transcript. Clients
  inject it into the agent's system prompt, so an agent no longer has to
  reverse-engineer the orchestration flow from seven tool descriptions. The
  relayed message asks for the concise report too, and `list_sessions` /
  `list_worktrees` are annotated read-only so clients may auto-allow them.

### Fixed

- **Ctrl+Click opens a link in every session again, not only where the agent
  opens it for you.** Since the fix for the duplicated browser tab, lich stood
  down from any session whose app reads the mouse — which left the click dead
  wherever that app ignores links (opencode, among others): the URL underlined
  on hover and nothing happened. Now lich keeps the click out of the session
  instead of standing down, so one Ctrl+Click (Cmd+Click on macOS) opens one
  tab, in every provider.

- **A hyperlink printed by a tool opens like any other link.** Output that
  carries its URL as a terminal hyperlink rather than as plain text (`gh`,
  `eza`, and anything else speaking OSC 8) was left to xterm's own fallback: a
  browser-style "do you want to navigate to…" prompt, outside lich's opener.
  Now Ctrl+Click serves both kinds of link the same way.

- **Closing a session on Windows no longer takes the app down with it.** The
  service closed the session's ConPTY and its output reader, freed by that very
  close, then closed the same one again — a repeat the Unix PTY absorbs and
  ConPTY does not: it releases the pseudoconsole and six handles unconditionally,
  so the second pass acted on handle values Windows had already reissued to
  whatever the app opened next. Whichever one it hit went with it — the loopback
  listener, another session's pipes, the log file — leaving a window whose
  terminals had stopped answering and no crash, error or log line to say why.
  The PTY now closes once, and a reap that arrives after it is the no-op it was
  always assumed to be.

- **A launch that loses its port now tells you, instead of dying into a log
  file.** When the loopback listener cannot bind and no running lich explains
  it, the app used to exit with nothing but a log line — which a double-click
  launch never shows, so the window simply failed to appear. Now it raises a
  native error dialog carrying the port, the OS error and the
  `LICH_LISTEN_PORT` override, so a taken port is something the user can fix
  without hunting down the log.

- **A bind that can never succeed fails at once, with its real error.** The
  launch retry loop treated every bind failure as the transient
  port-still-releasing race and retried it for the full two seconds — so a
  misconfigured `LICH_LISTEN_PORT` (say, a port past 65535) burned the whole
  budget before reporting `invalid port` wrapped in "after 11 attempts". Now
  only "address in use" earns a retry; any other bind error surfaces
  immediately, undiluted.

## [0.30.0] - 2026-08-12

### Added

- **The palette finds the projects you closed, and lets you filter what it
  found.** A closed project used to be reachable only from the five the "+" menu
  offers or by hunting for its directory in the picker; now the command palette
  searches the last 25 you closed by name and path, under a `Closed projects`
  group, and `Enter` reopens the one you pick. The menu still shows five — past
  that, the palette is the way back, and past 25 closes ago the directory picker
  still is.

  With four kinds of hit — sessions, open projects, closed ones and what was said
  inside a transcript — one query could fill the list, so the palette grew a
  filter row: `All` keeps every group but shows the first three of each and says
  in the group's header how many it is holding back, and `Sessions`, `Projects`
  or `Messages` lists that one kind whole. `Tab` and `Shift+Tab` walk the
  filters, a click does the same, and a filter with nothing behind it dims
  instead of disappearing.

- **Pinned sessions get a block of their own.** A pin used to lift a card to the
  top of the list and, with it, the whole worktree block it belonged to — so
  pinning one session quietly reordered a group you had arranged by hand. Now
  the pinned cards gather under a `Pinned` divider above everything else, in the
  same shape as the worktree dividers beside it, and their old blocks stay put.
  Each card still names its own directory and branch, so a pin never costs you
  the checkout it belongs to, and unpinning still drops the card back among the
  neighbours it was lifted over. `Ctrl/Cmd + Shift + ↓/↑` walks the new order,
  and dragging inside one block no longer moves the blocks around it.

- **The session sidebar collapses to a rail.** `Ctrl/Cmd + Shift + S`, or the
  button beside New Session, narrows the list to a 3rem column: one provider
  glyph per session, each still wearing the status ring it wears on its card, in
  the same order and under the same worktree grouping — a hairline where the
  open sidebar draws a titled divider, since a checkout's name does not fit.
  Hover gives you the card's own tooltip, unabridged — directory, the name the
  session answers to, branch, pull request, diff and how it stands against its
  base — because at that width the tooltip is where the card's words go.

  The rail selects a session and starts one, and that is deliberately all of it:
  renaming, closing, pinning, reordering and the worktree menus all aim at a
  32px target for something the open sidebar already does better, so the rail
  sends you back to it rather than growing a poorer copy of the card. Collapsed
  is never *gone* — the rings are what a list of running agents is read for, and
  hiding them to win the width would be the wrong trade. Whether it is open or
  railed survives a restart.

- **The dock's file tree has a filter, and it keeps its place.** A field above
  the tree narrows it to the paths matching what you type — every token has to
  appear, so a directory name is a search too, and what it matches comes back
  already expanded. Opening a file no longer collapses the tree behind it: the
  preview covers the tree instead of replacing it, so Back lands on the folders
  you opened, scrolled where you left them, with the file you were reading
  marked.

- **Skipping the permission prompts is no longer a Claude Code privilege, and it
  is now one control instead of two.** Every provider lich has a spelling for —
  Claude Code, Codex, opencode, Crush — gets the setting, wired to that
  provider's own flag, which the description names. It was always stored per
  provider; only Claude Code's was ever read. oh-my-pi shows nothing: its
  spelling has not been confirmed against the binary, and a guessed flag is a
  session that dies before it opens.

  The pair of switches is now a ladder — **Never · Worktrees only ·
  Everywhere** — with a line under it saying what the chosen rung leaves asking.
  Two independent switches could express a fourth state that is the inversion of
  the reason the setting exists: free rein in the tree you work in while the
  throwaway checkout still stops to ask. That combination is no longer
  reachable, and one already saved reads as *Everywhere* — what it already did
  to the checkout you work in.

- **The shortcuts are readable without going looking for them, and there are
  more of them.** `Ctrl/Cmd + /` opens a list of every keyboard shortcut lich
  has — the rebindable ones with the combos *this* install has them on, and the
  chords lich rewrites on the way to the agent's TUI: attaching a clipboard
  image, inserting a newline without sending, erasing the previous word. The
  last three were only ever written down in the source, and the rest lived on
  the one screen you have to already know about to reach. The overlay edits
  nothing and says where to rebind.

  Six more actions the app already performed are now on the keyboard: walk the
  project tabs (`Ctrl/Cmd + Shift + ←/→`, the sideways twin of the session
  pair), toggle the right dock (`Ctrl/Cmd + Shift + D`), put the cursor back in
  the active session's terminal from wherever you are (`Ctrl/Cmd + Shift +
  Enter`), open Settings (`Ctrl/Cmd + ,`) and the repository's pull requests
  (`Ctrl/Cmd + Shift + P`). Every default was chosen against what the terminal
  underneath does with it: each one was pressed into a live session first, and
  the only chords taken are ones the TUI either never receives or receives as a
  duplicate of a key you still have.

  Settings › Hotkeys is now one dense list grouped by what it acts on —
  sessions, view, app — with the passed-through chords listed read-only at the
  bottom. Bind two actions to the same combo and both rows say so, naming the
  other action: lich stores the binding either way, but only one of the two will
  answer to it, and now you can see which pair to fix.

- **Delegating work to a session is a search now, not a scroll.** "Delegate to
  session" opened a flat, unfiltered submenu of every other open session — fine
  with three, tedious with fifteen, and it named the session but never how it
  was doing. The submenu is now the command palette's own surface, scoped to
  delegating: type to filter by label or project, and each row carries the same
  busy/done/waiting ring the sidebar card does, so you can tell which sessions
  are free before picking one.

- **A session's name in another session's terminal output is a link.** When a
  provider's output mentions the label of another open session, that text is
  now clickable, and jumps straight to that session's card the way Pulls' "Open
  in Session" already does. The label has to stand as a word of its own to
  count: a session called `auth` links where the output says `auth`, and stays
  plain text inside `authentication` or `src/auth.ts`. A label shared by more
  than one open session is left as plain text too — the terminal has no way to
  tell which one was meant.

- **A bug report can now describe a lich that froze.** A window that stops
  updating while the process is still there writes nothing to `lich.log` —
  nothing crashed, so there is nothing to write, and the report arrived saying
  only "it froze". `lich rage` now asks the running instance for every
  goroutine's stack and carries it as `goroutines.txt`, so a hang can be read
  the way a crash already could. An instance that holds its port and will not
  answer within five seconds leaves that sentence in the file instead, which
  says as much again.

### Changed

- **A session blocked on you says so in words.** Being asked a question looked
  the same as being finished — a 22px ring differing from the emerald one by
  hue alone, and nothing naming what the session wanted. A card stopped on a
  prompt now reads `Waiting on you` in amber, so a sidebar of running agents
  says which one to answer instead of leaving you to read the hue. It takes the
  line the running tool was using rather than a new one, so no card grows, and
  the ring is unchanged.

- **`lich sessions` prints both of a session's names.** The table gains a
  `name` column with the peer-roster name beside the label — both reach the
  session, and a surface that showed only one is what once made an agent treat
  a single session as two. The column rides last, so a script reading the first
  three keeps working.

### Removed

- **"Mention session" is gone from the card's context menu.** It shipped one
  release ago and only ever appeared on Claude Code cards, because writing
  `@name` at the prompt only means anything to a Claude that has the messaging
  tool — so a menu of four providers had an entry that three of them never saw,
  with nothing on screen saying why. "Delegate to session" right above it aims
  at the same thing and works from any provider to any provider, so the
  asymmetric half is the one to drop rather than the one to keep explaining.
  The name a session answers to is still on its card's tooltip, and nothing
  changed about what Claude Code can do with it once you type it yourself.

### Fixed

- **Your theme survives an update.** The theme and terminal theme you picked
  were kept only in the app window's own storage — the Chromium profile — and a
  profile whose storage comes back damaged is one Chromium empties and starts
  again, taking both selections with it. On Windows that was the ordinary end of
  an update until v0.29.0, because the window was killed rather than asked to
  close, so an update was the likeliest moment to open lich on the default theme
  with the theme you had imported still sitting in the list. Both selections now
  live in the workspace database, beside the projects and sessions that already
  survive anything the window does; the window's copy stays as the cache the
  first frame paints from, so nothing about starting up changed, and an install
  that had a theme picked before this keeps it.

- **The crash notice is readable again.** The toast that reports a previous run
  ending unexpectedly is the only one carrying a description, and sonner lays a
  toast out as a single row — so its two buttons squeezed the wording into a
  third of the width. Toasts with a description now put their buttons on a row
  of their own, and the notice no longer spells out where the log is when the
  button beside it already says so.

- **A task sent to a session stopped on a permission prompt is no longer
  reported as never read.** The target's `waiting` report was read as "not
  working", so thirty seconds of a human not answering the permission dialog
  became "nothing read your task — nothing is queued" while the task sat
  queued behind the open turn; the eventual reply then hit a dead ticket and
  was lost. `waiting` now reads as the turn it belongs to.

- **`send` no longer blocks for up to twice its timeout.** Waiting out the
  target's setup script and waiting for the answer each spent a full budget;
  a send into a fresh worktree could outlive the CLI's own HTTP timeout and
  report a failure on a message that was in fact delivered. One deadline now
  covers the whole call.

- **A turn ending closes only the errand it belongs to.** Two tasks delivered
  to one session queue as two turns, but the first turn's end used to close
  every open ticket on that target — the second errand was reported "answered
  elsewhere" while its message was still queued, unread, and its real answer
  then hit a dead ticket. A turn now answers for at most one errand, oldest
  delivery first.

- **A sender that stopped waiting is told when its errand stalls.** An answer
  and an unread task already came back typed at the sender's prompt; a target
  ending its turn without replying reached only the window, as a toast — the
  agent that was promised "the answer will arrive at your prompt" waited on a
  promise nothing would keep, and a later `wait` on the ticket claimed it had
  been answered long ago. The stall is now typed at the sender's prompt like
  any other news, and a wait that races the stall reports it instead of
  "still working".

- **A long message typed into a session on Windows could land with no Enter
  behind it.** Input reached a PTY through one write call, which is safe on
  Unix — `*os.File` always writes everything or fails — but not on Windows:
  ConPTY's input pipe can hand back fewer bytes than it was given, and ports
  the code was trusting to loop already were not. A paste that landed on that
  boundary lost its tail, closing bracket included, so the terminal was still
  mid-paste when the Enter arrived a moment later and read it as more pasted
  text instead of a submission — visible as a message sitting typed and unsent
  until someone opened the session and pressed Enter themselves.

- **Closing lich and reopening it right away could fail to open at all,
  mostly on Windows.** The pinned listener port only retried its bind for the
  in-place self-update handoff; every other launch, including the ordinary
  one right after quitting, tried once and gave up the instant the OS said the
  port was still taken — which it briefly could be, since the outgoing
  process's child processes (Chromium, the PTYs) can hold it a moment longer
  than the process itself takes to exit. Every launch now gets the same short
  retry the restart handoff always had, and a retry that pays off is logged —
  a race that resolves on its own left no trace before, so a failure a few
  launches later looked unrelated to the ones that quietly won.

- **The footer's attach button wears the same glyph as the dock's.** Attaching a
  file from the footer showed a `+` while the same action beside the diff shows a
  paperclip, so one action had two icons depending on where you reached it. Both
  are the paperclip now.

## [0.29.0] - 2026-08-11

### Added

- **A relayed task tells the agent how it can actually answer.** The message
  lich types offers the `reply_to_session` tool where there is one and names the
  shell command everywhere; which of the two it offered was decided by the
  provider alone, from back when only Claude Code and Codex could be handed
  tools. With opencode and Crush getting them from the plugin, that answer went
  stale — so lich now asks whether *that* session has them, plugin version
  included. Pointing an agent at a tool it does not have costs it the turn; the
  command costs nothing and works everywhere.
- **Crush sessions get the tools to drive the others.** Installing the plugin
  already gave them hooks; it now registers lich's MCP server in the same block
  of the same file, so a Crush session can list the sessions beside it, hand one
  a task, open a session with a worktree under it and close one — the operations
  Claude Code and Codex are handed on their own command line at spawn, which
  Crush has no flag for. It goes away with the same uninstall, and nothing
  outside lich's markers is touched.

- **An agent can close a session, and decide what happens to its worktree.** It
  could open one and hand it work, but never tidy up: every checkout an agent
  made stayed until somebody removed it by hand. `lich close` — and the
  `close_session` tool — closes a session by either of its names, and closing the
  last one in a worktree asks what the checkout is for: keep it, and the session
  is parked so opening that branch again resumes its conversation, or remove it
  and the checkout goes with it. A checkout with uncommitted work is only removed
  when the caller says so explicitly, and no session can close itself. `lich
  worktrees` (and `list_worktrees`) is the other half: what each checkout is
  called, whether anything in it is uncommitted, and which sessions are open in
  it. Opening a session on a branch that is already checked out now opens that
  checkout instead of failing.
- **opencode and Crush install the lich plugin from Settings, like the other
  two.** Their sessions could already report what they were doing, but only for
  someone willing to place the files by hand — neither harness has a plugin CLI
  or a marketplace to install from, so the offer stopped at Claude Code and
  Codex. **Settings › lich plugin** now lists all four, and installs and updates
  the released version into each: opencode gets its module written into its
  plugin directory, Crush gets the hook scripts plus two lines added to its
  `crushrc`. Everything else in that file — every line and every comment you
  wrote — is left exactly where it was, and an update replaces only what lich
  put there.

  Two things worth knowing before clicking. **Crush needs 0.88.0 or newer**: the
  older ones read the file lich writes and ignore it without a word, so lich
  refuses the install and says which version you have instead of leaving you
  with a plugin that never reports. And **Crush reports two of the four things**
  — its session id and the git-status refresh — because `PreToolUse` is the only
  hook event it has: its cards show no status and keep their own name until it
  ships an end-of-turn event.

- **Claude Code can be spawned with its permission prompts off.** In a checkout
  you intend to throw away, confirming every edit and every command is ceremony
  — but the flag that drops the prompts could only be reached by pointing lich
  at a launcher script of your own, which then applied to every session it ever
  spawned. Settings › Providers › Claude Code now carries the switch, twice:
  once for sessions in the project's own directory, once for sessions in a
  worktree, so an agent can be let loose in the branch you can delete while the
  tree you work in keeps asking. Both are off until you turn them on, and what
  they turn on is `--dangerously-skip-permissions`: the agent then edits files,
  runs commands and installs things without asking first.

- **A task nobody picks up says so, instead of being waited out.** Handing work
  to another session proved only that the text reached its terminal — and a
  terminal can have something else on it: a provider still starting, a dialog
  left open, Claude Code asking whether it should trust a directory it has never
  seen. The task went into that and was gone, silently, and whoever sent it sat
  on a ticket nobody had been asked to answer until it expired an hour later.
  lich now watches whether the session actually starts working on what it was
  given, and says within half a minute when it does not — naming the session to
  open and what is usually on its screen. It reads that silence only for the
  agents that report what they are doing (the lich plugin); the others were
  always silent, and silence has to mean something before it can be read as
  anything.

- **An agent can open a session, and give it its own worktree.** Until now every
  session was opened by hand: an agent could hand work to the sessions beside it
  but not create one, so a task that deserved its own checkout waited for
  someone to click New Session. `lich open` — and the `open_session` tool beside
  it — opens one in any project, running any provider lich knows, optionally on
  a fresh git worktree branched off whatever branch you name. The new card
  appears in the sidebar without stealing your view, and its terminal is already
  running, so the session can be given work whether or not anybody opens it — a
  task sent while the checkout is still running the project's setup script is
  held until its agent is up, rather than typed into the script.
- **lich says when its last run ended badly.** A crash, a kill, or a machine
  that went down took the sessions with it — and the next launch restored the
  workspace looking exactly as it does after a deliberate close, so the turn an
  agent never finished was gone with nothing on screen saying so. That launch
  now opens with a notice that the previous run ended unexpectedly, and a button
  onto the log folder where that run's log sits. Closing the window as usual
  says nothing, and neither does the relaunch an in-place update performs.
- **`lich doctor` says whether lich would start here, and what stops it.** A
  window that never opens is the one failure lich could not report, because
  everything it uses to report one is behind that window. The command walks the
  same boot a launch walks — the config directory, the log file, the pinned
  loopback port, the workspace database, the browser, the provider CLIs on PATH
  — and prints a verdict and a timing for each step, ending with whether a
  launch would get through. A port held by the lich you already have open reads
  as the single-instance lock working, not as a failure; a port held by anything
  else, a missing Chromium-family browser or a database that will not open are
  the three that stop a launch, and the command exits non-zero so a script can
  read the same answer. It needs no window, no running instance and no network,
  and it never opens the database behind an instance that is running.
- **`lich rage` packs a bug report you can actually attach.** Reporting a
  problem meant being walked through it: find the log folder, notice that the
  rotated generation beside it is usually the one holding the crash, say which
  browser and which provider CLIs the machine has, remember the version. One
  command now collects all of it — versions and build, platform, whether an
  instance is running and answering, the browser it found, the providers on
  PATH, the plugin's state in each of them, the config directory, the
  environment lich was launched with and both log generations — into a single
  `.tar.gz` beside you. Values named like a token, key, secret or password are
  reported as present or absent rather than printed, and lich's own loopback
  token is masked wherever it had been written. Nothing is uploaded and no issue
  is filed: the archive stays on disk until you attach it to one you wrote. It
  reads the file system only, so it still works when the window never opened —
  the case where there was no Settings › Help to ask through.
- **Closing a project asks when an agent is still working.** The tab's × took a
  project's terminals down with it, killing whatever was mid-turn — with a
  spinner on that very tab as the only warning, and no undo. Closing a project
  whose sessions are busy or waiting on you now names how many and asks first;
  a project where nothing is running still closes on the click.
- **The sidebar says which session is waiting on which.** One session handing
  work to another used to happen entirely off screen: the card that asked looked
  like any card running a tool, and the card doing the work spun with no hint of
  why. Both now name the other end while the request is open — `→ docs` on the
  one waiting, `← auth` on the one that was asked — and go back to normal the
  moment the answer lands. A request from a script rather than a session says so
  instead of borrowing a name.
- **A session answers to either of its names.** Every session has two: the label
  on its card, and the name Claude Code lists it under. Handing work to one used
  to work with the first and fail with the second, which was enough to make an
  agent try both channels at once and lose the answer between them. Both names
  now reach the same session, and both are listed wherever lich names one.
- **An answer finds you even if you stopped waiting.** A task handed to another
  session can take minutes, which is longer than an agent can hold a tool call
  open. So it no longer has to: when the answer comes back and nobody is waiting
  on it, lich types it at the prompt of the session that asked, exactly as it
  typed the request at the other one. Ask, carry on, and the answer turns up.
- **A request stops waiting when the answer went elsewhere.** If the session you
  asked works through the request and then answers in its own window instead of
  back through lich, the wait ends right there saying so, and a notification
  opens that session so you can read what it wrote — instead of running out a
  five-minute clock on an answer that already exists.
- **Delegate to session, from the card menu.** Right-click the session you are
  in and pick another to hand work to; lich writes the request at your own
  prompt and leaves the cursor there, so you read it before it is sent. What it
  writes depends on what your agent has: Claude Code and Codex get it in plain
  words and reach for the tool themselves, everything else gets the `lich send`
  command spelled out — which for opencode, oh-my-pi and Crush is the only way
  they would learn it exists.

- **One session can hand work to another, whatever it is running.** Claude Code
  sessions can already message each other; Codex, OpenCode and Crush cannot, and
  nothing in lich reached across a card. Every session now carries a `lich`
  command that does: `lich sessions` lists the live cards beside it,
  `lich send "<card>" "<task>"` puts the task at that card's prompt and waits,
  and the agent there answers with `lich reply`. The answer comes back as the
  command's output, so the session that asked reads it without anyone carrying
  text between two terminals. It works the same on all four providers precisely
  because it never reads a terminal — the agent writes its own answer — and
  cards are addressed by the label you see on them. The full surface is
  `docs/cli.md`.

- **Sessions find each other on their own.** A command only helps someone who
  has been told it exists, and the person who would need telling is the one who
  does not read release notes. So lich now registers itself as an MCP server
  with each Claude Code and Codex session it opens: those agents see
  `list_sessions`, `send_to_session`, `wait_for_answer` and `reply_to_session`
  in their own tool list from the first turn, with nothing to install, type or
  configure. Your own MCP servers are untouched, and the registration carries no
  credential — it names lich and a subcommand, nothing else. opencode, oh-my-pi
  and Crush offer no way to be told this at startup, so their sessions keep
  using the `lich` command, which works everywhere.

- **lich is scriptable from outside a session.** The same commands run from any
  shell on the machine — a script, a scheduled job, your own terminal — finding
  the running lich on their own instead of needing to be spawned by it. `--json`
  gives `sessions`, `send` and `wait` a machine-readable line, so an automation
  can hand a session a task and read its answer without a provider in the loop.
  A message relayed that way says it came from the command line rather than
  posing as another session.

- **A card says which tool its agent is running.** A busy session showed a
  spinning ring and nothing else: whether it was three seconds into a file read
  or three minutes into a test run, the card looked the same. It now names the
  tool under the session's label while the turn is inside one — `Bash · pnpm
  test`, `apply_patch · usage.go` — and goes back to its usual shape the moment
  the tool ends. Both providers report it, each in its own vocabulary — mostly
  the same one, with a Codex edit arriving as `apply_patch`. Needs the companion
  plugin updated (Settings › Updates); an older one leaves the card exactly as it
  was.

- **Point one Claude session at another.** Claude Code can message your other
  sessions, but only if it knows what they are called — and left to itself it
  names a session after its directory, so every session in one checkout looks
  alike. lich now names each Claude session it starts, shows that name in the
  card's tooltip, and adds **Mention session** to the context menu of the card
  you are working in: pick any Claude session from any open project and its name
  lands at your prompt, ready for you to finish the sentence. lich carries no
  message itself — the Claude reading your prompt is the one that addresses the
  other session.

### Fixed

- **A session that is working can be handed a task again.** Before typing at
  another session's prompt, lich checks that the agent — and not the checkout's
  setup script — is what reads that terminal, by waiting for the terminal to
  stop drawing. It only ever asked that question at the moment a task arrived,
  and an agent in the middle of a turn redraws its spinner several times a
  second: a session that had been sitting at its prompt for hours failed the
  check the first time anything was ever sent to it, and the sender was told, at
  length, that the project's setup script was still running — in a session with
  no worktree and no setup script. lich now notices the quiet when it happens
  rather than asking after the fact, so a busy session is handed the task and
  answers it a turn later, the way every other busy session already did. The
  same check was too eager at the other end: a session whose setup script had
  just handed over could be handed a task while its agent was still drawing its
  opening screen, and that task appeared there as literal paste markers instead
  of reaching the prompt. The wait for an agent to start is no longer mistaken
  for an agent that has settled. When the check does time out, the message no
  longer names a cause it cannot see: it says what is certain, and points at the
  screen that knows the rest.

- **The session list's scrollbar no longer sits on the cards.** With enough
  sessions to scroll, the scrollbar came down flush against the right edge of
  every card, reading as part of them. It now keeps a gap, and the cards stay
  the width they were when the list is short enough not to scroll.

- **A dragged card keeps its own shape.** Dragging a session card past a taller
  neighbour — a worktree card with a long title, say — stretched it to that
  neighbour's size, smearing its text until the drop. Cards, session groups and
  project tabs now only move while dragging, never resize.

- **A restart on Windows no longer costs you the settings you just changed.**
  Where Linux and macOS ask the window to close, Windows killed it outright —
  and the window is where your interface settings are written, on their way to
  disk a few seconds later. Anything changed just before an update or a restart
  went with it: a theme, a panel width, a dialog you had dismissed. The window
  is now asked to close on Windows too, and only killed if it cannot be.

- **"What's new" stays dismissed.** The popup after an update remembered your
  click in the window's own storage, so a browser profile that had stopped
  accepting writes — and kept answering reads with what it last loaded — greeted
  you with the same release notes on every launch, with no way to make it stop.
  The dismissed release is now kept in the workspace database beside everything
  else that has to survive a restart.

- **A lich that will not open says why.** When the loopback listener could not
  take its port, the log asked whether the port was free and dropped the system
  error that knew the answer — leaving the one launch failure that shows no
  window with nothing to go on. It now records the error itself, which is what
  separates a port another program holds from a port the system refuses to hand
  over with nothing listening on it (a Windows habit).

## [0.28.0] - 2026-08-09

### Added

- **A Codex session resumes, and wears its own icon.** The companion plugin now
  installs on Codex as well as Claude Code, and lich no longer treats every
  session that reports itself as a Claude one. A Codex card reopens the
  conversation it was running before the last restart — the same prompt a Claude
  card raises, spawning `codex resume <id>` — and only offers it when the
  conversation is still on disk. A `codex` running by hand inside a shell session
  now puts Codex's mark on that card instead of Claude's.

- **The plugin prompt installs into whichever CLIs you have.** The startup
  dialog was about Claude Code alone; it now lists every provider that can run
  the plugin and is present on the machine, ticked, so one **Install** covers
  both. Settings › Updates follows: one row per CLI with its installed version,
  an install or update button, and a plain "CLI not installed" for the ones you
  do not use. Installing into Codex adds one manual step lich cannot do for you —
  Codex will not run a plugin's hooks until you review them once with `/hooks` —
  so the prompt says so where it applies.
- **Closing a session can be undone.** The × was the one button in the app with
  no way back: a stray click deleted the card and the agent's conversation went
  with it, and pinning only ever protected the session you remembered to pin.
  Closing one now raises a toast naming it with an **Undo** — the card comes
  back in the slot it left, under the same name, and the conversation comes back
  with it: the restored session asks whether to continue where it left off, the
  same prompt a worktree session resumed from the sidebar gets. Closing a
  worktree's last session is unchanged — that question is still keep or remove —
  and a pinned session still offers no close at all.

- **The sidebar says how long a session has been waiting.** With several agents
  running, every bell looked the same: nothing on screen told you which session
  had been blocked on you for twenty minutes and which had just asked. A card
  that is waiting on you, or busy producing, now carries a short elapsed readout
  beside its status ring — `40s`, `12m`, `3h` — counting from the moment it
  entered that state, not from the last time the hook reported it. A finished
  turn shows nothing: it is over, and its number would only climb. The readout
  survives switching projects, so a session that started waiting while you were
  looking elsewhere comes back with its real age, not a fresh clock.

- **The command palette searches what was said, not just what things are
  called.** Finding the session where you worked something out meant remembering
  the name you gave it — and the name is usually the one thing you don't
  remember. Type three characters or more and a **Messages** group appears under
  the sessions and projects, listing the sessions whose conversation mentions
  them, each with the sentence it matched and how many of its messages did;
  Enter opens the session as any other palette row does. It reads your turns and
  the agent's, not tool output, and it looks inside the conversation a session is
  running now — what was said before a `/clear` is in a transcript lich can no
  longer name. Claude Code sessions only, like the rest of the transcript
  readouts.

- **A spend ceiling for the footer cost readout.** The cost of a session was a
  number you had to go and read; a session left running past what you meant to
  spend on it looked exactly like a cheap one. Set a ceiling in dollars under
  **Settings › Providers › Claude Code** and the figure takes the same colour ramp
  the context ring already uses — amber from 80% of it, red from 95% — with the
  tooltip naming the ceiling it is measured against. It is a warning and nothing
  more: no turn is stopped, and the number it watches is API pricing from lich's
  own table, so leave it empty on a subscription. The setting appears with the
  cost readout and hides with it.

- **A session waiting on you now reaches the desktop.** The bell on the card and
  the toast beside it only ever worked while you were looking at lich — walk away
  to a browser or another workspace and a session blocked on a permission prompt
  sat there until you happened to come back. When a session needs your input and
  the lich window is not the one you are in, lich now raises a desktop
  notification through the system's own notifier, naming the session and its
  project so you know where to go. Nothing changes while the window has focus:
  the in-app toast is still the one that fires, and it still routes to the card.
  lich asks before it starts — the first time a session would have notified you,
  a dialog puts the question, and either answer settles it. **Settings ›
  Notifications** holds the switch afterwards, whichever way you answered.

- **And so does a session that finished working.** The other half of walking
  away: a run you left going ends, says nothing, and you find out whenever you
  next look. A second switch under **Settings › Notifications** raises a desktop
  notification when a session ends its turn with nothing left to ask you — same
  rule as the one above, so it only fires while the lich window is not the one
  you are in. It is off until you turn it on, including if you already said yes
  to the question above: the two are separate, because a long run finishing and a
  session blocked on you are not equally worth an interruption. Nothing changes
  inside lich — no new toast; the card and its project tab still do the telling.

- **A worktree's setup script can now find the project it came from.** Sessions get
  `$LICH_PROJECT_DIR`, the project's own checkout — which, for a session running in a
  worktree, is somewhere else entirely and previously had no name the script could
  reach. The point is the expensive half of a new worktree: dependencies. Instead of
  `pnpm install` downloading gigabytes each time, a setup script can reuse what the
  project already has —
  `cp --reflink=auto -r "$LICH_PROJECT_DIR/node_modules" .` on a filesystem with
  copy-on-write (btrfs, XFS, APFS) makes a full, independent copy that costs no disk
  until something writes to it, and `ln -s` works where reflinks do not.

- **A session card says when its base branch moved, and whether a merge would
  collide.** Running several worktrees at once, the first one to land leaves the
  others stale without a word — and you found out at the Merge button, one branch
  at a time. The card's branch row now carries the answer: a plain count of the
  commits `origin/main` has picked up since, or an amber alert naming how many
  files a merge would conflict on. Hovering the card spells it out — the base
  branch by name, the commit count, and up to three of the conflicting paths.
  Nothing has to be checked out, merged or fetched by hand to see it, and knowing
  which branches will fight is what lets you pick the order to land them in: one
  conflict to resolve instead of one per worktree.

  It reads committed work only, so an agent mid-edit shows what it last
  committed, and the base is always the repository's default branch — a branch
  stacked on another feature branch is measured against the wrong one. The
  readout is absent entirely on a repository with no `origin`.

- **lich asks which agents you use, instead of assuming Claude Code.** The first
  launch opened on a Claude session because Claude was the only harness lich's
  author ran: a machine with Codex and nothing else met a first session that died
  on `claude: command not found`, and the three other harnesses lich supports were
  off in a Settings screen nobody had been told about. lich now scans for agents
  before showing anything and opens on the providers it found — a list of what is
  actually installed, a switch each, and the pick of which one new sessions spawn.
  The first one found is on already, so the common case is one click, and the
  panel is the Settings › Providers screen verbatim, so the place to change it
  later is a screen already met. Nothing installed is its own answer: lich names
  what it looks for rather than failing inside a terminal. Existing installs that
  never chose a default see it once.

- **oh-my-pi joins the harnesses lich can run.** A Pi fork with an IDE wired in,
  spawned like the rest from its `omp` binary and off until turned on.

- **Switch sessions from the keyboard.** Reaching another session without the
  mouse meant opening the command palette and typing its name. **Ctrl+Shift+↓**
  and **Ctrl+Shift+↑** now step to the next and previous session of the project
  you are in, wrapping around at both ends and walking the sidebar exactly as it
  is drawn — pinned cards first, worktree groups in the order the dividers show.
  Both fire while you are typing in a terminal, and both are rebindable in
  Settings › Hotkeys like the shortcuts already there.

### Changed

- **A worktree's dev-server port is now reserved, not guessed.** `LICH_WORKTREE_PORT`
  was a hash of the checkout's path: stable, but blind. Two worktrees of one project
  could hash onto the same number, and neither knew about the dev server, database or
  container already sitting on that port — the collision showed up as the second
  `pnpm dev` refusing to start, with nothing pointing at why. lich now keeps a
  reservation per checkout: a worktree is offered its hashed number, and takes it only
  if no other checkout holds it and nothing on the machine is listening on it,
  otherwise the next free port in the same range. A checkout keeps its number for good
  — across restarts of lich, of the session, and of your dev server — and gives it back
  when its directory is gone, so removing a worktree returns the port to the pool.

### Fixed

- **Worktree groups can be dragged into the order you want.** Reordering
  sessions stopped halfway once a worktree split the sidebar into groups: a card
  could move inside its own group, but the groups themselves sat in the order
  they happened to open, so a checkout you were done with kept its place at the
  top. Drag a group by its header — the divider with the worktree's name — and
  the whole block moves with it, cards and pull request card included. A sidebar
  with a single group has no header and is unchanged. A pinned session still
  carries its group to the top, which is the one order a drag cannot beat.

- **A worktree group's header spells the whole worktree name.** Worktrees named
  the way most repositories name branches — `feat/x`, `fix/x` — came out of the
  divider as `x`, because the header showed only the last folder of the
  checkout's path. Two worktrees off the same ticket therefore drew the identical
  header, and the divider stopped telling you which sessions were whose. The
  header now reads the name the worktree was created with, slashes and all. A
  worktree without a slash in it looks exactly as it did.

## [0.27.0] - 2026-08-07

### Added

- **Pin a session.** The session you keep coming back to no longer drifts down
  the sidebar as new ones open above it: pin it from the card's right-click menu
  or from the pin that appears beside the × on hover, and it sorts to the top of
  the list and stays there across restarts. A pinned card also drops its close
  affordances — no ×, no **Close session** in the menu, and the worktree it lives
  in refuses to be removed — so the session you meant to keep cannot be closed by
  a stray click. Unpin it and everything comes back, including the slot in the
  list it had before.

- **The pull request's description is editable in the app, and its reviewers are
  picked there too.** The Overview tab was a read-only rendering of the body: a
  description an agent wrote and you wanted to trim, and choosing who reviews,
  were the last two things about a pull request that sent you to github.com in
  the middle of the work. Overview now says who opened it, lists the review
  roster — who was asked, who approved, who requested changes — with **Request
  review** beside it, ticking anyone the repository allows on a review, and
  carries an **Edit** for the description that saves straight to GitHub. A
  reviewer who already answered comes back unticked: ticking them again is the
  re-request. Merged and closed pull requests keep the roster and lose the
  picker, which GitHub would refuse anyway.

### Changed

- **A review comment's box grows with the comment.** Every field review prose is
  typed into — the line comment, the batch note for the session, the reply, the
  summary above a review — was a fixed three lines tall, so anything longer than a
  sentence had to be dragged open before the first line was finished. The box now
  grows as it is filled, up to a much taller ceiling, then scrolls; the drag
  handle still overrides it in either direction.

### Fixed

- **The app window no longer opens on a Google account chooser.** On Windows,
  the first thing a new install showed was not lich: a Chrome that knows more
  than one Google account greeted the launch with a profile picker asking which
  account to use, and once past it a second bar offered to translate lich's own
  interface. Two browser prompts in a window that is not a browser, both in the
  first ten seconds of the first run. lich now names its profile on the command
  line, which is what keeps the picker shut, and writes the two preferences that
  hold it — browser sign-in and translate, both off — into its own Chromium
  profile before the window opens. The preferences are re-applied on every
  launch, so installs that already met the prompts are quiet from the next start
  onwards, and nothing else in the profile is touched.

- **Windows: the app list and the file picker wear the current icon.** The icon
  linked into the executable was still the purple meteor retired two releases
  ago, and it is the one Windows draws in two places the window itself never
  covers: the Start Menu's app list, and the taskbar button of the native file
  picker — the dialog that opens a project. So the window and its taskbar entry
  showed the current mark while the app list, one keypress away, showed the old
  one. Both now match.

- **The dock now reviews the directory the session is actually working in.** A
  session that moved — spawned in one checkout and `cd`-ed into another
  repository — moved its card and the footer with it: both read the live working
  directory, showing that repository's branch and its uncommitted counts. The
  dock did not. It stayed on the directory the session started in, so **Review**
  answered "No uncommitted changes" and the **Code** tree listed the wrong
  checkout while the footer, one row below, counted 35 changed files. The dock's
  three tabs and the pull-request screen now follow the same directory the card
  and footer show.

## [0.26.1] - 2026-08-05

### Fixed

- **The project you just closed is the first one the "+" offers back.** The
  recent list was ordered by when a project was *first* opened, so closing a
  long-standing one put it behind five newer projects — the list you reach for
  right after closing something was the one place that thing could not be. It is
  now ordered by the close itself, and reopening a project and closing it again
  returns it to the top. Projects already closed keep the order they had.

## [0.26.0] - 2026-08-05

### Added

- **Comment on a file you opened whole.** The dock's **Code** tab reads a file
  the way the review reads a diff, and the reason to open it mid-review is a
  change too small to judge on its own — but there was nothing to do with what
  you found there except type it out again in the terminal. Selecting lines and
  right-clicking now offers **Comment for the session**, the same box the diff
  opens, and the batch strip sits at the foot of the tab.

- **Drag files onto a terminal to put their paths at the prompt — any kind of
  file, a whole folder, or several at once.** Claude Code suggests dropping
  images into your terminal; in lich the terminal is a browser window, so a
  drop did nothing useful (and a drop that missed one navigated the window away
  from the app). Now anything dropped on a session lands at its prompt as a
  path, quoted where it needs to be and left unsent, exactly as a paste would —
  the agent then treats an image path as an image and any other path as a path.
  Files and folders found under the session's own directory — or under your
  home — paste their real path, so an edit lands on your file; a screenshot or
  a log from anywhere else is copied into `<config-dir>/lich/dropped/` and
  pastes the copy's path — a copy that is deleted three days later, so the
  folder cannot grow for as long as you use lich. A folder that neither search finds
  has no copy to fall back on, and says so rather than pretending.

### Changed

- **One batch of session comments per checkout, not one per screen.** The
  comments held for the session's next prompt were keyed by whatever produced
  them — the pull request for its diff, the checkout for the dock — so a review
  that crossed the two ended up with two batches, sent as two prompts, neither
  of them the review. They are now keyed by the checkout they are going to: a
  note taken on a pull request's diff, on a file opened whole beside it, or on
  the checkout's own uncommitted changes joins one list, visible from all three.

### Fixed

- **The dock no longer empties itself the moment you open a pull request.** The
  file tree and the review both follow the active session, and the session they
  resolved was the one the *terminal* route named: on a pull request or the
  settings screen there was no such route, so the dock kept its width and its
  tabs while its contents went blank — and the footer lost the buttons that open
  it. The session now resolves anywhere inside the project, which is where the
  dock earns its keep: a pull request's diff shows a change in a dozen lines,
  and the file it came from is one tab away.

- **A plugin install or update that cannot see the newest release now says so,
  instead of reporting success and leaving the old one in place.** Claude Code
  keeps a local clone of the plugin's marketplace, and both the install and the
  update refresh it themselves — but when that refresh fails, they only warn:
  they read the clone as it stands, decide the version already installed is the
  newest one, and exit reporting success. lich had no way to tell that apart
  from a real update, so it announced one, and the next session started on the
  same old plugin. The refresh is now lich's own first step, where it either
  succeeds or fails loudly with what Claude Code said about it.

## [0.25.0] - 2026-08-05

### Added

- **Install a theme from a git repository, and update it later.** A theme file
  you pick has no version and no way back to whoever wrote it: a fix means being
  sent another file and importing it again. Settings › Appearance › **Import**
  now opens a dialog that takes a repository URL as well — a repository with a
  `lich-theme.json` manifest and its themes beside it. lich clones it, checks
  the manifest and every theme, and installs the pack only if all of it is
  valid; ids you already have are named and confirmed before anything is
  replaced. Each installed theme then shows where it came from and at which
  version, with an action that re-clones the repository and takes the newer
  release when there is one. Picking a single file still works exactly as it
  did, and is still the right way to try a theme you are writing — it simply
  reads as unversioned, because it is.

### Fixed

- **Ctrl+Click on a link inside a session opens one tab, not two.** Claude Code
  reads the mouse itself and opens the links it prints, so the same click was
  being served twice: once by the session, once by lich. Whenever the running
  app is reading the mouse, the click is now its own — lich opens the link only
  where nothing else will.
- **The Review panel no longer shows a diff that has stopped being true.** It
  decided when to re-read the diff from the changed-file counts alone, and an
  edit that replaces text on a line already marked as changed leaves every one
  of those counts exactly where it was — same files, same `+`, same `-`, and no
  new commit. The panel held that stale diff not for a tick but indefinitely,
  which is the worst thing a review surface can do: it was most likely to happen
  while an agent was iterating on the same lines, which is precisely when you
  are watching it. The panel now reads the diff itself while it is open and
  publishes only when the text actually changed, so an idle repository still
  costs nothing and your selection and scroll position still survive a tick.
- **A file unticked by a new commit opens again instead of staying folded.**
  Ticking a file off as viewed folds it away, and a commit that rewrites that
  file correctly unticks it — but the fold was decided once and never revisited,
  so the file came back unread *and* closed, with nothing on screen to say why.
  A file whose content changes now returns to the state it would have had if it
  had just arrived, which for anything but a very large file is open.
- **The command palette tells a screen reader which row is selected.** The rows
  carried `aria-selected` on a plain button, where it means nothing, so arrowing
  through the list moved a highlight that was only ever visual. The list is now
  a proper listbox of options and announces the row under the cursor.

## [0.24.0] - 2026-08-04

### Added

- **Appearance can import custom color themes.** lich still ships the bundled
  Light and Dark themes, but Settings › Appearance now accepts theme JSON files
  saved under the app config directory. Imported themes can recolor both the
  interface tokens and the terminal palette, can be selected independently for
  the app and terminal, and can be removed without touching the bundled themes.
  Re-importing an existing id asks before replacing it, and a deleted selected
  theme returns both pickers to their automatic system behavior. A theme does
  not have to start from the documentation either: **Save template** writes a
  valid example naming every supported app and terminal color, ready to rename,
  recolor and import back. Both directions go through the same native dialogs as
  the rest of lich, so the template lands where you choose it instead of
  somewhere you have to go find afterwards.
- **Report a bug without being told where the log file is.** Reporting anything
  meant being walked to a directory you had no reason to know, to a file whose
  name nobody had said out loud. Settings now ends in **Help**: one button opens
  the log's folder in your file manager — the rotated generation in view beside
  the live one, since a bug older than today's session is usually in there — and
  another opens the bug form in your browser with the version and platform
  already filled in. lich never files the issue for you, and never needs `gh`
  installed or logged in to do any of this: it hands you the form and the file,
  you write the report and attach it. The section also says what the log carries
  — paths, project and branch names, your gh login, and never a session token —
  so you know what you are sending before you send it. It sits at the foot of
  the settings nav next to **Updates**, the two of them apart from the sections
  that configure a project: neither is something you set, and neither is
  something you open twice in a session.
- **Reopen a recent project without the file picker.** A closed project keeps
  everything it was closed with — its sessions, its name, its tab position —
  but finding it again meant walking the directory picker back to it. The "+"
  in the tab strip now lists the last five projects you closed, each reopening
  where you left it, with the picker one entry below for every other folder.
  A project whose directory has been moved or deleted says so and leaves the
  list for good. With nothing closed yet the button is the picker it always was.
- **Merge past a rule that is holding a pull request back.** Once GitHub answers
  that a rule on the base branch stands in the way, gh refuses the merge without
  ever calling GitHub — so there was nothing lich could do with that pull request
  but leave for the browser. The Merge menu now offers the same merge as a
  bypass, GitHub's own administrator override, and says so in every label it
  offers. It appears only where an override can actually help — a rule holding
  the merge, or a base branch that has moved — never over a conflict or a draft,
  which no override resolves, and never for an account that does not administer
  the repository, so a bypass on offer is one that will go through.

### Changed

- **Open a session on a fork's pull request when its author allows it.** Every
  pull request from a fork was refused a session, on the grounds that an agent's
  commits would have nowhere to push — but GitHub asks the author that very
  question when the pull request is opened, and "allow edits by maintainers" is
  on by default. lich now reads the answer: with the permission on, the fork's
  branch checks out and opens like any other, and only a fork that withholds it
  still greys the button out, saying which of the two it is.
- **A thread in Conversation shows the lines it is about, not the file.** GitHub
  sends a review comment with the whole hunk it sits in, which on a file the
  branch adds is the entire file — so a remark about ten lines arrived under a
  hundred and twenty, and finding the ones it meant was a scroll. The snippet is
  now cut to the commented span (with a little context above a single-line one),
  its lines carry the diff viewer's own green and red, and the file reference in
  the header reads the whole range: `utils.java:65-74`.
- **A refused merge says which kind of refusal it was.** gh writes one sentence
  for two very different failures and puts the difference in its tail; lich read
  only the opening and claimed conflicts either way, sending you to look for
  conflicts a pull request did not have. A conflict now says so, and a rule on
  the base branch says that instead.
- **A merge blocked by a commit rule names the rule.** An approved pull request
  with green checks and no conflicts could still be refused, with nothing on
  screen accounting for it: the cause was a ruleset testing the *commit message*
  the merge would write, which no field of a pull request mentions. Where the
  base branch carries one, the Merge button and the failure now name what it
  requires — the message, author or committer email patterns, and what each one
  expects.

### Fixed

- **Settings dropdowns read back what you picked.** A closed dropdown showed the
  stored value instead of the row you had chosen, so the default provider read
  `claude` rather than "Claude Code", and the GitHub account read `__active__`
  — an internal placeholder that means "no override" — or the host-qualified
  `github.com/you` where the row itself had said `you`. Picking again was the
  only way to be sure what was set, and the account one could easily read as
  something having gone wrong. Every dropdown now shows the same words when
  closed that it shows when open.

## [0.23.0] - 2026-07-30

### Added

- **Review a pull request without leaving lich.** Pulls only ever showed the
  parts that travel one way — the checks, the commits, the diff — so a review
  that asked for a change was invisible here and reading it meant the browser.
  The diff now carries the conversation: a thread opens inline under the line it
  is about, with its replies, and can be answered and resolved where it sits.
  Right-click a selection and *Comment on the pull request* starts one of your
  own; each comment waits with the others until **Submit review** sends them
  together as a single review — approving, commenting, or requesting changes,
  with a summary of your own above them. Comments waiting to be sent are marked
  as pending and survive a refresh, a collapsed file and a restart, so a review
  written over an afternoon is still there. A new **Conversation** tab holds the
  whole exchange in the order it happened — verdicts, comments on the pull
  request itself, and the threads whose lines the branch has since rewritten,
  which no diff can show. Settled threads fold themselves away behind a count.
- **Review comments, handed to the session as one prompt.** A diff could give
  the session a *reference* — a file, a line range — but never an instruction,
  so anything you actually wanted changed had to be typed out by hand while
  looking straight at it. Right-clicking a selection, in the Review dock or in a
  pull request's Files changed, now also offers *Comment for the session*: a box
  opens between the lines themselves, and the note is held against them instead
  of being written out. Comments collect at the foot of the panel in the order you
  read the diff, each removable on its own,
  and one *Send* hands the whole batch over as a single prompt — pasted into the
  session unsent, so the last word before the agent starts is still yours. They
  outlive leaving the tab but not a restart: a line number written before an
  edit points somewhere else after it. On a pull request's diff both destinations
  are offered side by side, named for where the note goes.
- **Every worktree gets a dev-server port of its own.** Two worktrees of one
  project both ran `pnpm dev` and both wanted the same port, so the second one
  lost — and the setup script that had just installed the dependencies had no
  way to know it should have picked another number. Every session now starts
  with `LICH_WORKTREE_PORT` set to a port derived from its checkout's path:
  `PORT=$LICH_WORKTREE_PORT pnpm dev` in the setup script (Settings › Project),
  or in the terminal, and both worktrees come up. The number is the same every
  time for a given checkout, so a bookmark to it keeps working across restarts.
- **Settings › Providers › Claude can show what a session has cost.** Off by
  default, and deliberately so: on a subscription plan the number is noise, so
  with the setting off the footer shows nothing at all — no zero, no greyed-out
  figure — and lich never even reads the transcript for it. Turned on, the
  footer carries the session's spend at API prices beside the context ring,
  summed across every turn it has run, including the conversations it cleared
  and the sub-agents it spawned. The total survives a restart, and a session
  parked with its worktree keeps it when you reopen it. Prices ship with the
  binary and refresh themselves from the published price table the first time a
  session runs a model that binary never heard of; while a model has no known
  price, the readout stays absent rather than quoting a total that is missing a
  turn.

### Changed

- **New app icon.** The purple flame is now a monochrome one that inverts with
  the colour scheme: white on dark, black on light. The window and tab icon
  follows the scheme live; the launcher, dock and taskbar icon is a single
  image the desktop cannot re-theme, so it ships white.

### Fixed

- **The Merge button says no before the click, not after.** Merging a pull
  request GitHub would not take — a review missing, a reviewer asking for
  changes, a conflict, the base branch moved on — offered the button anyway and
  answered with a toast that named none of those reasons. The button is now
  disabled for each of them, and hovering it says which. Everything else it
  still offers: a failing check no rule requires merges fine, a base branch
  governed by a ruleset merges fine, and GitHub still deciding is not a no. The
  call stays yours, and where GitHub does refuse, it refuses out loud.
- **The Merge menu offers what your repository actually accepts.** A repository
  can pin a branch to one merge method — squash only is the common one — and
  nothing in a pull request says so, so the menu offered all three and GitHub
  refused two of them after the click. lich now reads the base branch's rules
  and offers only the methods that branch takes. A branch nothing governs, or
  an answer lich cannot read, still offers all three: the menu never comes back
  narrower than the truth.
- **Merging over red CI says so first.** A failing check that no rule requires
  does not block a merge on GitHub, and lich offers the merge for the same
  reason — the call is yours. It just used to offer it in silence, with the red
  a line away in the status row. The Merge button now carries the number of
  failing checks, and hovering it says GitHub will merge anyway.
- **Dropdown menus are as wide as what they say.** Every menu was pinned to the
  width of the button that opened it, so the ones hanging off a small control —
  New Session, the pull request filter, Merge — wrapped their items over two and
  three lines. They size to their own text now, on one line each, with a little
  more room around it. A select still matches its field, which is the one place
  the old behaviour was right.
- **Settings › Font lists your installed fonts on Windows.** The picker read the
  font list from fontconfig, which Windows does not have, so it offered nothing
  beyond the bundled default and whatever was already selected. It now reads the
  fonts Windows itself records, folding every weight of a family onto one entry.
- **Links open in your browser on Windows.** Clicking an external link — a pull
  request, a release page — called the Linux opener and did nothing at all, with
  only a line in `lich.log` to show for it. Each platform now uses its own
  opener.
- **Open in editor works on a Windows path with a space in it.** The command
  handed to the session was quoted for a Unix shell, which cmd.exe does not
  understand, so a file under `C:\Users\First Last\` opened as a path cut at the
  space — and a file whose own name held one never opened at all. The quoting is
  now the session shell's own.
- **A file cannot open something else on Windows.** Opening a file with no
  `$EDITOR` set passed its path through a shell that reads `&` as the end of a
  command, so a file named `a&calc.txt` — which any branch you check out is free
  to carry — ran `calc` when you opened it. The path is now handed over as a
  plain argument, with no shell in between. A path cmd.exe cannot express at all
  opens with the default handler rather than being run.
- **lich no longer vanishes mid-session on Windows.** Tracking a session's
  working directory reads it out of the child process's own memory, and a length
  that memory reported odd — which a 32-bit child, or one exiting mid-read, does
  — crashed the whole app. The window stayed on screen still showing its
  terminal, so the only sign was that nothing ever updated again and reloading
  found nothing listening. The read now refuses a length it cannot use. A crash
  that does happen is also written to `lich.log` now: it used to go to a console
  the Windows build does not have, leaving the log to simply stop with no trace
  of why.
- **A session no longer offers to resume a conversation that is gone.** Reopening
  a session lich had run Claude Code in asked whether to continue that
  conversation — and answering yes could drop you into a terminal showing
  Claude's "no conversation found" error instead of a session. lich stored the
  conversation's id for good, but Claude Code prunes its own transcripts, so an
  old enough session was offering something that could no longer happen. The
  question is now only asked while the conversation is still there; when it is
  not, the session starts fresh and says so.
- **A session whose worktree was deleted outside lich no longer opens an empty
  terminal.** Removing a checkout by hand left its session behind, pointing at a
  directory that is not there: opening it mounted a terminal that stayed blank,
  with nothing on screen to say why. lich now recognises the missing checkout,
  says so, and closes the session — closes, not deletes, so re-creating the
  worktree brings the session and its conversation back.
- **A session that fails to start now says why.** Every other reason a terminal
  could fail to open — a provider binary that is not installed, or a wrong path
  in Settings › Providers — failed in silence behind a blank terminal. The
  failure is now reported, and the session stays, ready to retry once the cause
  is fixed.

## [0.22.0] - 2026-07-28

### Added

- **macOS installs through Homebrew.** `brew install omartelo/tap/lich` puts the
  release binary on your PATH on Apple Silicon and Intel alike, and `brew
  upgrade` carries it forward from there. It also settles the Gatekeeper prompt
  that a hand-downloaded binary raises: the binaries are still unsigned, but
  Homebrew does not mark what it installs as quarantined, so there is nothing
  left to clear by hand. Every release publishes the formula automatically.

### Changed

- **A Homebrew install updates through Homebrew.** lich updates itself in place
  on macOS, which would have meant overwriting a file Homebrew owns and tracks —
  leaving `brew` naming a version that is no longer the one running. Homebrew
  installs now offer the `brew upgrade` command the way Linux installs offer
  their package manager's, instead of the self-update button.

### Fixed

- **The mouse works again after coming back to a session.** Leaving a session
  and returning to it left clicking, scrolling and dragging dead inside the
  running program — Claude Code, or any terminal app that reads the mouse — while
  the keyboard kept working, until a keystroke happened to bring it back. Hiding
  a session destroys its terminal and showing it rebuilds it from a snapshot, and
  that snapshot restored the app's request to receive mouse events but not the
  format it asked to receive them in. The rebuilt terminal fell back to a format
  from 1980 that lich does not transmit, so the app was sent nothing at all. The
  format is now restored with the rest of the terminal's state.
- **The log file stops burying real failures under routine git output.** The
  footer polls every open checkout several times a second, and two of the git
  calls behind it answer "no" as a matter of course: a repository with no
  commits has no `HEAD` to resolve, and a path that is not a repository has no
  branch to name. Both ran through the path that treats any failure as one worth
  reporting, so each poll filed a warning — thousands a day into a log that
  rotates at 5MB, pushing out the failures a bug report is actually about.
  Discarding a newly created file did the same, once per discard, for the check
  that asks whether the file exists in `HEAD`. Those calls now ask quietly;
  everything a person is waiting on still reports as before.

## [0.21.1] - 2026-07-27

### Fixed

- **One session loading no longer freezes every other terminal.** A resumed
  Claude session with a large transcript floods its terminal, and while the
  window was busy swallowing it the socket that carries terminal output stopped
  being read. That backpressure used to land on the loop reading the PTYs: the
  read stalled, the PTY buffers filled, and every shell in every session — not
  just the one that was loading — stopped dead until the socket recovered.
  Output now leaves each session through a queue of its own, so a window that
  falls behind slows down the session producing the flood and nothing else.
- **Terminals stop degrading after a dozen tab switches.** Hiding a session
  destroys its terminal and showing it builds a new one, but the renderer's
  graphics context was only handed back when the browser got around to
  collecting it. Chromium keeps sixteen alive and force-loses the oldest beyond
  that, so switching between sessions eventually killed the renderer of a
  terminal in use: it froze for the three seconds spent waiting for a restore
  that never came, then fell back to the slower text renderer for good, with
  cell metrics that no longer matched the ones its grid had been fitted to. The
  context is now released the moment the terminal is destroyed.

## [0.21.0] - 2026-07-27

### Added

- **A project can pick which GitHub account it talks to.** `gh` keeps one active
  account per host, so a repository only a second account can see answered every
  pull request lookup with "Could not resolve to a Repository" — the same message
  GitHub gives for a repository that does not exist. A new **Version Control**
  section in Settings names the account for the project at hand, chosen from the
  ones `gh auth status` lists, and every GitHub call lich makes for that project
  — the badge, the pull request list, checks, diffs, merges and PR checkouts —
  runs as it. Left at "gh's active account", nothing changes. An account is named
  by its host as well as its login, so one that lives on a GitHub Enterprise
  instance is offered — and found — like any other, and the same login on two
  hosts stays two accounts; the host is spelled out in the picker only when there
  is more than one in play. The account governs what lich reads from GitHub, not
  what git does: a push still rides the remote's ssh key and signs with the
  global `user.email`.
- **Approving a pull request no longer means leaving lich.** An Approve button
  sits beside Merge on the pull request screen and files the approving review
  through the project's GitHub account. Once the review lands, the button reads
  Approved and stops offering itself — GitHub would happily take a second one,
  but nobody means to send it. A pull request that is already merged or closed
  cannot be reviewed and says so; a draft or a conflicting one still can. GitHub
  refuses an account approving its own pull request, and that refusal now reads
  as the sentence it is. Reviews with a body, and requesting changes, are not
  here: a review comment belongs to the line it is about, and lich has nowhere to
  attach one yet.

### Fixed

- **GitHub failures now read as sentences instead of `gh` output.** The pull
  request screens used to paste whatever `gh` printed — `GraphQL: Could not
  resolve to a Repository with the name 'acme/private'. (repository)`, prefixed
  by the command lich had run. Each failure lich can actually hit now says what
  happened and what to do about it: an invisible repository points at the account
  picker, a signed-out `gh` names `gh auth login`, a rate limit says to wait, a
  refused merge says why. Anything unrecognised says so plainly; `gh`'s own text
  goes to lich's log, where it was always the more useful place for it.
- **git failures read as sentences too.** The worktree dialog, the discard flow
  and the sidebar used to show git's own stderr, prefixed by the subcommand lich
  had run — `git worktree: fatal: '…' contains modified or untracked files, use
  --force to delete it`, or, when git refuses by exit status alone, the words
  `exit status 1`. A rejected branch name now names the branch, a taken name says
  it is taken, a folder that is not a repository says so, and a dirty worktree
  says where to remove it. git's own text goes to lich's log.
- **The pull request badge follows the project's GitHub account too.** It looked
  the branch's PR up on its own path, so it kept answering as `gh`'s active
  account while the screen beside it used the project's — a badge that vanished
  on the repositories the account picker exists for.
- **Three more browser reflexes stop firing.** Holding Shift used to let a chord
  through: Ctrl+Shift+P still raised the system print dialog, Ctrl+Shift+O the
  bookmark manager, and Ctrl+Shift+Q quit Chromium, which quits lich — the same
  fatal move Ctrl+W was already stopped from making. They fired everywhere,
  terminals included, since a terminal encodes neither of them. The devtools
  chords, reload and F11 stay where they are.

## [0.20.0] - 2026-07-26

### Added

- **lich now lists a repository's open pull requests, and can open a session on
  one.** A pull-request button in the tab bar, beside Settings, parks a "Pull
  requests" card at the top of the sidebar and opens the list — the one way in
  that does not already require a pull request, since every other entry appears
  only once a checkout has one. A column holds every open PR — number, title,
  author, how long ago it moved, and a dot for its checks — with quick filters
  (all, ready, drafts, failing) and a sort that remembers itself. The filter box
  takes GitHub's own qualifiers beside the words it matches: `is:merged`,
  `is:closed` and `is:all` reach past the open ones, and `is:draft`, `is:fork`
  and `review:approved` / `review:changes-requested` / `review:required` narrow
  what came back. One call brings back the 50 most recently updated, and the
  column says so when that is all of them. Selecting a pull request shows it in
  full, whatever branch the checkout is on and whether or not it is still open —
  its status line reads Open, Draft, Merged or Closed, and says where the review
  stands beside the checks and the conflicts. The column collapses to a rail
  when the pull request wants the width, and stays collapsed until it is asked
  back. A worktree's own pull request card is untouched: it still opens that one
  pull request alone, without the list. **Open in Session**, beside Merge, checks
  the PR's head branch out into its own worktree and starts a session in it, with
  the pull request card already parked so the session carries its PR. A branch
  already checked out is reused rather than checked out twice — the button reads
  "Go to session" once one is live — and a pull request from a fork is refused
  up front, since its commits could never be pushed back.

### Fixed

- **A worktree behind a symlinked path is recognised as the checkout it is.**
  git reports a checkout by its fully resolved path while lich built one by
  joining names onto the data dir, so the same directory travelled under two
  spellings wherever a symlink sat in the way — every path on macOS, and a short
  name on Windows. The session living in a worktree was then not seen to live
  there: resuming a kept worktree opened a second one beside it, and **Open in
  Session** offered to create what was already checked out. Both sides of the
  comparison now resolve the same way.

## [0.19.0] - 2026-07-25

### Added

- **The Pulls screen now lists the commits a pull request would land.** A new
  "Commits" tab, counted next to its name, shows every commit oldest first: the
  subject line, who committed it and when, and clicking a row opens its message
  body — the branch's story, which the diff and the file list never tell. The commits ride along with the lookup the screen already makes,
  so it costs no extra round-trip.

### Fixed

- **The shell no longer behaves like a browser.** The window is one only by
  construction, and its reflexes leaked through everywhere except a focused
  terminal, which already swallows them: a locale that did not match the UI
  raised the translate bubble on startup; Ctrl+W closed the window, which quits
  lich; Ctrl+T and Ctrl+N opened tabs and windows; Ctrl+P, Ctrl+S and Ctrl+O
  opened dialogs with nowhere to land; Ctrl+Shift+Delete would have wiped lich's
  own saved settings along with the browsing data; right-clicking the UI offered
  Back, Reload and Save as; dropping a file on the window replaced the app with
  that file, with no address bar to come back from; and middle-clicking a link
  spawned a browser window. All of it is now refused. Reload, the devtools chords
  and the terminal's own right-click menu are deliberately kept, and every chord
  still reaches a terminal as its PTY sequence — Ctrl+W stays delete-word,
  Ctrl+U kill-line, Ctrl+D end-of-file.
- **A new directory of files now counts as the files it holds, not as one.** The
  changed-file count came from `git status --porcelain`, which reports an
  untracked directory as a single `?? pkg/` entry — so an agent writing 25 files
  into fresh packages showed 4 changed files in the footer and on the session
  card, while the Review tab listed all of them. The count now asks git for
  every untracked file; the line totals were already right.
- **Merging a pull request now clears its badge everywhere, at once.** The
  footer, the session cards and the sidebar's "Pull request" entry only looked
  the pull request up again when the checkout's HEAD moved, or when the window
  lost focus and got it back — and a merge does neither, since the commit lands
  on the base branch, on the remote. Merging from the Pulls screen emptied that
  screen while every badge around it went on reading `#N Open` until you clicked
  away to another window and back. Merging, opening a pull request and the
  header's reload button now retire the shared answer and re-read it.
- **A model newer than your lich no longer reads its context window as 200k.**
  Opus 5 sessions showed five times their real usage in the footer, because the
  window came from a table of the 1M models and anything missing from it fell
  back to the smallest window that fit — so every model launch needed a release
  here. The rule is now the other way round: Haiku is the 200k exception and
  everything else is 1M, which is what a model released after your build reads
  without a new version.
- **A pull request with hundreds of changed files no longer locks the app.**
  Every file in a diff panel starts expanded, and an expanded file builds its own
  CodeMirror view — so a 191-file pull request built 191 editors in one go, each
  followed by a language chunk re-configuring its document. Chromium offered to
  kill the page, and the screen stayed unusable well after it recovered. A file's
  editor is now built only once its card comes near the viewport; until then the
  card holds a placeholder of the file's height, so the scrollbar still measures
  the whole diff. An editor that has been built stays built, keeping its
  selection and highlighting when you scroll back. That 191-file diff now settles
  in 465 ms with 8 editors, against 13.7 s with 191. The dock's Review tab shares
  the same cards and the same fix. Files changed also stopped refetching the diff
  every time the window regained focus — a new commit still refreshes it.
- **The pull request screen no longer waits behind a bare "Loading…".** Both the
  lookup and the diff are `gh` round-trips of about a second, and on a screen
  that size a single muted line read as a failure. Each now holds the shape of
  what is coming — title, actions, status line and tabs for the pull request;
  file tree, toolbar and file cards for Files changed.

## [0.18.0] - 2026-07-25

### Added

- **The pull request badge keeps up with the session working next to it.** A
  commit — or the push that opens a pull request — now reaches the badge, the
  checks and the diff on its own, instead of waiting for you to click away to
  another window and back. Git status also follows a checkout that is being
  worked in every second rather than every three, settling back to the slower
  cadence once it goes quiet, so nothing extra is spent on idle projects.

- **Open, review and merge pull requests without leaving lich.** A branch's
  GitHub pull request now opens as a full-screen view — reached from the footer
  PR badge, a session card's `#N`, or a session's "Pull request" menu — that
  parks a card in the worktree's sidebar group beside its session, the way the
  Settings screen does. The Overview tab renders the description as markdown
  alongside the checks and mergeability; the Checks tab lists every check —
  what failed first, what is still running and for how long — and opens its run
  in the browser, refreshing itself every ten seconds while any of them is still
  going; the Files changed tab shows the whole
  diff next to a file tree that jumps to a file on click — hide the tree and
  the diff takes the whole width — folds every file at once when the review
  gets wide, and lets you tick each one off as viewed: the file folds away and
  the header counts how far you are. Merge from the header
  — squash, a merge commit or rebase, editing the commit message first if you
  want — and the merge offers to remove the branch's worktree once it lands, or
  open a pull request when the branch has none. A reload button covers what
  nothing can announce (a review, a check going green). It reuses the `gh` CLI
  already behind the footer badge, so a repo without `gh` or a non-GitHub remote
  simply shows nothing.
- **The footer shows the active session's model and context window.** The status
  strip now names the model the active Claude session runs and its reasoning
  effort (e.g. `opus 4.8 · xhigh`), marked with the provider's icon, and a small
  ring beside it fills with the percent of its context window in use; hovering
  the ring shows a fuller bar and the exact tokens against the window size. It
  refreshes as the context grows through a turn, read off the conversation's
  transcript. The ring turns amber past 80% and red past 95%, so a session
  drifting toward auto-compaction is visible without opening `/context`. The
  percent is taken against the model's native window (1M for current
  Opus/Sonnet/Fable, 200k for Haiku). The readout can be turned off under
  Settings › Providers › Claude Code. Needs no change to the user's statusline;
  other providers stay blank until they report a session id of their own.
- **The file tree tracks the working tree live.** The Files tab now lists
  untracked-but-not-ignored files alongside tracked ones and drops any file
  deleted from disk, so a file created or removed during a session shows up
  without a commit — it is no longer frozen at the session's starting set.
- **The file tree shows each changed file's line delta.** A modified or new
  file carries the same `+added −deleted` pair the review panel and footer use,
  reusing the diff already parsed for the review — a clean file stays bare.
- **Right-click in the file tree.** A directory's context menu offers Expand all
  and Collapse all over that folder's subtree; a file's offers Open in editor. A
  GUI `$VISUAL`/`$EDITOR` — or the platform opener (`xdg-open`, `open -t`,
  `start`) when neither is set — launches detached; a terminal editor (vim,
  nvim, nano, …) opens in a fresh lich terminal session rooted at the checkout,
  since a detached launch would leave it with no controlling terminal. The
  editor is resolved from the login shell, so a GUI launch still sees rc exports.

## [0.17.0] - 2026-07-24

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
- **Search the base branch when creating a worktree.** The New Worktree
  dialog's base picker is now a filter: type to narrow the worktrees, local and
  remote branches by name and pick with the arrow keys or the mouse — finding
  `develop` or `main` in a repo with dozens of remote branches no longer means
  scrolling a collapsed list.
- **Collapse or expand every file in a review at once.** A toggle in the review
  dock header folds the whole diff down to its file headers, or opens it all
  back up, so a large changeset is quick to skim.

### Changed

- **The interface has been reskinned.** One idiom throughout — elements are
  defined by spacing, hover and hairline seams instead of nested bordered
  boxes: session and settings cards become borderless rows, review file diffs
  become open surfaces separated by a hairline, footer and menu controls lose
  their chip borders, and the segmented controls, tabs and dialogs tighten up.
  The zinc palette and the window layout are unchanged.
- **Notifications read cleaner.** The bell shows a small red dot instead of a
  white count; the count moves into a titled dropdown, and the "needs your
  input" toast gains an amber icon and the name of the project it came from —
  and no longer lands on the top bar's corner buttons.

### Fixed

- **The light theme no longer glares.** Every surface was pure white, so panels
  had no separation and the large terminal pane was blinding. Light is now a
  soft light-gray canvas with near-white raised chrome, mirroring the dark
  theme's depth: the ground (and terminal) sit darker than the sidebar, cards
  and menus that lift above them, and the hover/selection fill is dark enough to
  read again.
- **A deletion line no longer bleeds through a diff file's sticky header.** The
  header's rounded corners let the red of a scrolled-under deletion peek out; it
  now sits flush and covers the content beneath it.

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

[Unreleased]: https://github.com/omartelo/lich/compare/v0.32.0...HEAD
[0.32.0]: https://github.com/omartelo/lich/compare/v0.31.0...v0.32.0
[0.31.0]: https://github.com/omartelo/lich/compare/v0.30.0...v0.31.0
[0.30.0]: https://github.com/omartelo/lich/compare/v0.29.0...v0.30.0
[0.29.0]: https://github.com/omartelo/lich/compare/v0.28.0...v0.29.0
[0.28.0]: https://github.com/omartelo/lich/compare/v0.27.0...v0.28.0
[0.27.0]: https://github.com/omartelo/lich/compare/v0.26.1...v0.27.0
[0.26.1]: https://github.com/omartelo/lich/compare/v0.26.0...v0.26.1
[0.26.0]: https://github.com/omartelo/lich/compare/v0.25.0...v0.26.0
[0.25.0]: https://github.com/omartelo/lich/compare/v0.24.0...v0.25.0
[0.24.0]: https://github.com/omartelo/lich/compare/v0.23.0...v0.24.0
[0.23.0]: https://github.com/omartelo/lich/compare/v0.22.0...v0.23.0
[0.22.0]: https://github.com/omartelo/lich/compare/v0.21.1...v0.22.0
[0.21.1]: https://github.com/omartelo/lich/compare/v0.21.0...v0.21.1
[0.21.0]: https://github.com/omartelo/lich/compare/v0.20.0...v0.21.0
[0.20.0]: https://github.com/omartelo/lich/compare/v0.19.0...v0.20.0
[0.19.0]: https://github.com/omartelo/lich/compare/v0.18.0...v0.19.0
[0.18.0]: https://github.com/omartelo/lich/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/omartelo/lich/compare/v0.16.0...v0.17.0
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
