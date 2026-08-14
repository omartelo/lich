# lich

## What it is

`lich` is a **harness for AI coding agents** — a desktop app whose Go backend serves an embedded React frontend to
a system Chromium window in `--app` mode (no Electron, no webview toolkit; decision record:
`docs/chromium-shell.md`). It is built for other developers to use, not only its author: docs, errors and defaults
answer to a stranger. Linux first; Windows and macOS are experimental and there is no hardware here for either —
a cross-compile proves the code builds, never that the platform works.

This file records only what the code cannot say: invariants, deliberate ceilings, and the workflow. User-facing
feature history lives in `CHANGELOG.md` — don't duplicate it here.

## Stack

- **Backend**: Go 1.25, pure Go (CGO_ENABLED=0, fully static binary). One token-authenticated loopback listener
  carries everything: HTTP RPC (`internal/rpc`) plus WebSockets for terminal I/O (`internal/terminal/transport.go`)
  and app events (`internal/events`). OS-specific code is selected by build tags behind small seams, never by
  runtime checks — the PTY is the model (`internal/terminal`: creack/pty on Unix, ConPTY on Windows).
- **Shell**: system Chromium-family browser launched in `--app` mode (`internal/chromium`), persistent profile under
  the user config dir (`os.UserConfigDir` + `lich/chromium-profile`). Window closed = app exit.
- **Frontend**: React 18 + TypeScript + Vite; the UI rules and the visual system are `frontend/CLAUDE.md` and
  `frontend/DESIGN.md`. Sessions spawn any registered provider (`internal/providers`: Claude Code, Codex, OpenCode,
  Crush). Service shapes are hand-owned in `frontend/src/lib/api-types.ts` — touch a Go struct's JSON tags and that
  mirror moves in the same change; there is no codegen.
- **Session hooks**: `docs/hooks/` is the canonical, contract-first spec. lich owns the server side; the scripts
  live in the companion repo `omartelo/lich-plugin`, so an endpoint or payload change breaks a repo this one cannot
  see — move the contract first, then both sides.
- **Build/tasks**: [Task](https://taskfile.dev) — `task --list` describes every one of them, so they are not
  copied here. `task dev` gets its own DB, port and Chromium profile: it never touches an installed lich's
  workspace.

## Local Gate (before every commit / PR)

- `gofmt -l .` clean (fix with `gofmt -w .`) and `go vet ./...` clean.
- `cd frontend && pnpm check` clean — biome is the frontend's gofmt + vet (fix with `pnpm format`).
- `go test ./...` (backend) and `cd frontend && pnpm test` (frontend) green — or `task test` for both.
- `cd frontend && pnpm build` succeeds (tsc typecheck + vite).
- Shipped anything a user can see? Its `CHANGELOG.md` `[Unreleased]` entry lands in the same PR — the release notes
  are read from there, so an entry written later is an entry that missed its release.
- Touched an OS seam or a `_test.go` build tag? Run the same cross-compile loop CI runs:
  `for os in linux darwin windows; do GOOS=$os go build ./... && GOOS=$os go vet ./...; done`
  Cross-compiling only proves it builds; the backend suite runs on real Windows and macOS runners on every PR,
  and that is the answer to trust.

## Hard Invariants

Non-negotiable rules. A violation means the work is not done.

1. **Test coverage ≥ 80%**, backend and frontend. CI measures and reports the number but does not auto-fail below
   the bar — it is held in review, so read the summary. OS/framework boundaries (the PTY, the Chromium
   launcher/zenity subprocesses, WebSocket wiring, the `main` bootstrap, xterm.js internals) are the documented
   exception: cover the pure logic and leave the boundary itself alone.
2. **Tests answer to the contract, never the other way round.** Never weaken, skip, delete or rewrite a test to buy
   a green run or a coverage number. A test may change for exactly two reasons: the contract changed, or the test
   asserted something the contract never promised — name which one in the diff. When in doubt, change the product,
   not the test.
3. **A flake is a bug, and it has a root cause.** Never re-run CI until it goes green, never dismiss a failure as
   "unrelated" without proving it. Reproduce (`go test -count=200 -run TestX ./pkg/`), fix the cause, and measure
   the failure rate before and after.
4. **Clean code.** Small focused functions (< 50 lines); cohesive files (200–400 lines typical, 800 max); no
   nesting deeper than 4 levels; comments only for the *why*; errors handled explicitly, never swallowed; no magic
   values; no secrets in source.
5. **A session feature is traced across every provider.** `internal/providers.Registry` is the checklist —
   Claude Code, Codex, opencode, oh-my-pi, Crush. Anything a session touches (spawn flags, hooks, resume,
   transcripts, plugin install, MCP) is designed against all five, not just the provider at hand. Equal behaviour
   is not always possible — but the gap must be deliberate and written down in the same PR: a Known Ceilings
   bullet naming which providers are out and why (the briefing-flags and plugin-install bullets below are the
   model). A feature that silently works on a single provider is not done.

## Releases

Push a `vX.Y.Z` tag — `.github/workflows/release.yml` builds every OS, runs the backend suite on each, publishes the
artifacts and the AUR `lich-bin`, and takes the release notes from the matching `CHANGELOG.md` section. A
`workflow_dispatch` run from any branch exercises the whole pipeline without publishing. The version comes from the
git tag (`git describe` in the Taskfile, env `VERSION` overrides) and is injected into `build/linux/nfpm/nfpm.yaml`
— never hand-edit it there.

Before tagging:

- [ ] Local gate green, backend with `-race`.
- [ ] Move `CHANGELOG.md`'s `[Unreleased]` entries under a new `vX.Y.Z` heading and refresh the compare links.
- [ ] Tag `vX.Y.Z` and push.

## Known Ceilings

Deliberate limits and shortcuts. A bullet earns its place by naming a trap — something that breaks work when
nobody knows it and that the call site never shows. The mechanism and the history stay in the code and
`CHANGELOG.md`.

- **Session cwd is polled** from the PTY child (`internal/terminal/cwd.go`, per-OS readers behind build tags); a
  failed read degrades to the session's start directory. Tracks the direct child only, not nested shells.
- **A project's gh account governs gh, not git**: `vcs.account` (`internal/project/ghaccount.go`) puts one
  account's token in `GH_TOKEN` for every gh call lich makes for that project. A push still rides the remote's
  ssh key and signs with the global `user.email`, so a PR can be *read* by one account and its commits *land*
  under another, with no error anywhere.
- **`LICH_WORKTREE_PORT` is reserved, never held** (`internal/terminal/worktreeport.go`, table `worktree_ports`):
  the number is checked against the other checkouts and bound once to prove it is free, then released — the dev
  server starts minutes later, after the setup script, and anything on the machine can take the port in between.
  A reservation is released only when its directory stops existing, which is read at allocation time.
- **The cost readout is priced from a table, not from the provider** (`internal/pricing`): an unpriced model makes
  the readout go *absent* rather than wrong, and billing is per `(session, transcript)` — a conversation forked
  inside the PTY bills its copied history twice. lich's own resume continues the same transcript and is unaffected.
- **A dropped file has no path, so lich guesses it** (`internal/drop`): Chromium never hands the page a path, so a
  drop is matched by name + size + mtime under the session directory, then home. Twins resolve to nothing; a file
  in neither tree is *copied*, so an agent told to edit it edits the copy — and that copy is deleted 3 days on,
  so a path pasted into a prompt eventually stops resolving.
- **git status is polled** — one shared poller per repository path (`frontend/src/lib/git/git-status-store.ts`); the
  lich plugin's `session-touched` hook nudges an immediate refresh.
- **lich fetches on its own** — the base-branch readout (`internal/project/basestatus.go`) runs
  `git fetch --no-tags origin` in the background, at most once every 2 minutes per repository, for as long as a
  card is on screen. It is the only git write lich makes outside the worktree flows, nothing in the UI announces
  it, and it moves remote refs in the user's own repository.
- **Persistence is hybrid**: UI prefs in the page's localStorage (`lich.*` keys — the reason the listener port is
  pinned at 47821; `LICH_LISTEN_PORT` overrides it, `LICH_PORT` is the distinct per-session hook variable), the
  workspace in SQLite (`<config-dir>/lich/lich.db`, `internal/store`). Closing a session deletes its row; keeping a
  worktree parks its session for a later resume; closing a project hides it, but reopening one whose directory is
  gone silently deletes it and its sessions. Only the 25 most recent closes are offered back
  (`recentLimit`, `internal/store/store.go`) — the row survives, but past that a project is reachable only through
  the directory picker, and neither the menu nor the palette says so.
- **Hidden sessions are serialized and destroyed**: 2MB replay rings on both sides
  (`frontend/src/lib/terminal/replay-buffer.ts` page-side, `internal/terminal/replay.go` backend-side — the latter
  survives a full page reload). Scrollback past the ring is gone, not paged.
- **Single instance via the pinned port**: the bind is the lock (`internal/singleton`); a duplicate launch focuses
  the running window (best-effort, untested against a real window) and exits 0.
- **Reordering rides dnd-kit's pointer sensors** (`frontend/src/lib/use-sortable-list.ts`); the activation distance
  is load-bearing — without it plain clicks stop selecting a session.
- **lich appends to the agent's system prompt, for two providers only**
  (`internal/terminal/command.go`, `briefingFlags` → `relay.SpawnBriefing`): Claude Code and oh-my-pi
  are spawned with `--append-system-prompt` carrying lich's own briefing, so text the user never
  wrote is in every session's prompt and in `/proc/<pid>/cmdline`. Codex, opencode and Crush get
  nothing there — none has an append (Codex's `model_instructions_file` *replaces* the base
  instructions; the other two are config keys, not flags), so for them the same point exists only in
  lich's MCP instructions, and behaviour between providers differs by that much.

- **Installing the plugin writes into three harnesses' own directories**
  (`internal/agentplugin`): Claude Code and Codex are driven through their plugin CLI, but opencode, oh-my-pi and
  Crush have none, so lich writes the released files itself — a module into opencode's plugin dir, another into
  omp's `extensions/`, hook scripts plus a delimited block in Crush's `crushrc`. None of them records what is
  installed, so the version lives in a marker line lich wrote; edit the file by hand and lich reads it as not
  installed. Crush below 0.88.0 ignores those lines in silence, which is why the install asks its version first.
  The Crush block and omp's `mcp.json` also register lich's MCP server, by the absolute path of the binary that
  installed it — a line in a file cannot expand `$LICH_BIN` per session, and the binary is only the transport:
  which lich a session reaches is decided by the coordinates in its PTY. omp's is a JSON document rather than an
  appendable one, so lich rewrites it: every key survives, the user's formatting does not, and a file lich cannot
  parse aborts the install rather than being replaced.
- **omp's state directory answers to two variables, and the profile wins**
  (`internal/agentplugin/omp.go`, `internal/terminal/transcript.go`, both resolving it independently as the Claude
  Code pair do): `OMP_PROFILE` moves the whole directory under `~/.omp/profiles/<name>/agent` and beats an explicit
  `PI_CODING_AGENT_DIR`, which is the order `omp config path` was measured to apply. Get it backwards and the
  install lands where omp is not reading and every restored card silently starts fresh.
