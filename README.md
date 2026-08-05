<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/public/appicon.png" />
    <img src="frontend/public/appicon-light.png" alt="lich" width="88" height="88" />
  </picture>
  <h1>lich</h1>
  <p><strong>A terminal-first harness for coding with AI agents.</strong></p>
  <p>
    Open your projects, run agents like Claude Code, Codex, opencode or Crush in
    real terminals, and keep git — worktrees, diffs and pull requests — in view
    without leaving the window.
  </p>
  <p>
    <a href="https://github.com/omartelo/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/omartelo/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" />
    <img alt="Shell" src="https://img.shields.io/badge/shell-Chromium%20--app-4285F4?logo=googlechrome&logoColor=white" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
  </p>
  <img src="docs/media/session.png" alt="A Claude Code session running in a lich worktree, with the session card showing its branch and diff badge and the footer showing the model and context ring" width="900" />
</div>

## About

`lich` is a **personal harness** — a desktop app that wraps a terminal-first
workspace around AI coding agents. Open several projects, run a session (or
many) per project, drive an agent in each, and watch git state — branches,
diffs, worktrees, pull requests — without ever leaving the window. It ships as a
single static Go binary that opens its UI in your system's Chromium-family
browser in `--app` mode — no Electron, no bundled webview (decision record:
[`docs/chromium-shell.md`](docs/chromium-shell.md)).

It is deliberately bespoke: shaped by the author's taste for other harnesses
(Warp and friends), built for one workflow rather than as a generic product.
It's public because there's no reason to hide it, not because it's a supported
release.

## Features

- **Bring your own agent.** [Claude Code](https://www.anthropic.com/claude-code)
  (Anthropic), [Codex](https://github.com/openai/codex) (OpenAI),
  [opencode](https://github.com/sst/opencode) (SST) and
  [Crush](https://github.com/charmbracelet/crush) (Charm) are all first-class:
  point lich at each binary in Settings, pick the default, or choose per session.
- **Terminal-first sessions.** Real PTY-backed shells, several per project,
  rendered by xterm.js on the GPU (WebGL). Search the scrollback (`Ctrl+F`); the
  buffer survives a full page reload. A Warp-style footer follows `cd` and
  surfaces git status, and for a Claude session it names the model, fills a ring
  with the context window in use, and — if you ask it to — prices what the
  session has spent.
- **Projects and sessions.** Projects sit on a top tab bar; the `+` reopens one
  you recently closed or opens the OS folder picker for the rest. Session cards
  carry the working directory, branch, a diff badge, an untracked-line count and
  the number of the branch's pull request.
- **Git worktrees, built in.** Spin one up from any base branch — search it,
  local or remote, even across dozens of branches — and lich seeds the new
  checkout with your gitignored `.env*` files, hands it a dev-server port of its
  own (`LICH_WORKTREE_PORT`) and runs a per-project setup script before the agent
  starts. The sidebar groups sessions by the worktree they belong to, and a kept
  worktree's session resumes its conversation later.
- **Review, and hand the review back.** A CodeMirror diff dock shows the working
  changes beside a live file tree — collapse or expand every file at once, open
  one in your editor, attach one to the session. Right-click a selection to write
  a comment against those lines; the batch is handed over as a single prompt,
  pasted into the session unsent.

<div align="center">
  <img src="docs/media/review-comments.png" alt="The review dock with two comments written against lines of the diff, collected at the foot of the panel" width="900" />
  <p><em>Comments written on the diff, handed to the session as one prompt.</em></p>
</div>

- **Pull requests, end to end.** A button in the tab bar lists the repository's
  open pull requests — quick filters plus GitHub's own qualifiers (`is:draft`,
  `review:approved`, `is:merged`) — and **Open in Session** checks one out into
  its own worktree. A pull request opens full-screen: overview, checks, commits
  and the whole diff beside a file tree. Review it where you read it — a thread
  opens inline under the line it is about, and your own comments wait as pending
  until **Submit review** sends them together — approve it, and merge it (squash,
  merge commit or rebase) with the methods the base branch actually accepts.
- **Themes, yours or someone else's.** Bundled light and dark, plus imported JSON
  that recolors the interface and the terminal palette independently. Install a
  pack from a git repository and update it in place when it ships a new version;
  **Save template** writes a starter naming every supported color. Format and
  repository layout: [`docs/themes.md`](docs/themes.md).
- **The rest of the window.** `Ctrl`/`Cmd`+`K` jumps between sessions and
  projects. A session waiting on your input raises a toast and a dot on the bell,
  collected in a titled dropdown; with the
  [lich plugin](https://github.com/omartelo/lich-plugin) installed, a Claude
  session also titles its own card and refreshes git the moment it writes a file.
  Settings › Help opens the log folder and a pre-filled bug report.

<div align="center">
  <img src="docs/media/pulls-list.png" alt="The pull request list: every open pull request with its author, age and check status, narrowed by a filter box" width="900" />
  <p><em>The repository's pull requests, filtered by GitHub's own qualifiers.</em></p>
  <img src="docs/media/pull-request.png" alt="The pull request screen: state, checks, mergeability and the pull request body, with Approve and Merge in the header" width="900" />
  <p><em>Read, approved and merged in place.</em></p>
  <img src="docs/media/pull-request-review.png" alt="A review thread open inline under the lines it comments on, inside the pull request's changed files" width="900" />
  <p><em>Its diff, with the review threads on the lines they belong to.</em></p>
  <img src="docs/media/themes.png" alt="Settings, Appearance: the bundled themes beside an imported pack showing the repository it came from and its version" width="900" />
  <p><em>A theme pack installed from a git repository, updated in place.</em></p>
</div>

## Install

One line — detects your distro, verifies the checksum, and installs the native
package and its dependencies through your package manager:

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

| Platform | Get it | Needs at runtime |
| --- | --- | --- |
| **Linux** | `install.sh` above, or AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`) | chromium / google-chrome / brave on `PATH`, plus `zenity` |
| **macOS** *(experimental)* | `brew install omartelo/tap/lich` | Chrome / Chromium / Edge / Brave in `/Applications` |
| **Windows** *(experimental)* | installer from [Releases](https://github.com/omartelo/lich/releases) | Chrome / Edge / Brave |

Manual per-distro packages and the static binary: [INSTALL.md](INSTALL.md). The
macOS and Windows binaries are unsigned — Gatekeeper and SmartScreen warn until
notarization/signing ship. Homebrew installs sidestep the Gatekeeper prompt;
a binary downloaded from the Releases page needs its quarantine flag cleared by
hand.

## Getting started

1. **Install** and launch `lich`.
2. **Open a project** — the `+` in the tab strip lists what you closed recently
   and opens your OS folder picker; point it at a git repository.
3. **Point lich at your agent** — in Settings › Providers, set the binary path
   for Claude Code, Codex, opencode or Crush, and choose a default.
4. **Start a session** — *New Session* spawns a terminal running your agent in
   the project.
5. **Branch off a worktree** *(optional)* — create one from any base branch;
   lich seeds it and drops you into a fresh session.

## Configuration

- **Agents** — set each provider's binary path in Settings, and pick which one
  new sessions default to. Claude Code's section also holds the footer's context
  ring and its cost readout — the cost one off by default, since the figure only
  means something when you are billed per token.
- **Worktrees** — a per-project setup script (Settings › Worktree) runs in a new
  worktree's terminal ahead of the agent; a `.worktreeinclude` file tunes which
  gitignored files get copied over.
- **Version control** — a project can name the GitHub account `gh` runs as
  (Settings › Version Control), for a repository only one of your accounts can
  see. It governs what lich reads from GitHub, not what git pushes.
- **Appearance & hotkeys** — themes, fonts and key combos in Settings; UI
  preferences persist in `localStorage` under `lich.*` keys (inside lich's
  Chromium profile at `~/.config/lich/chromium-profile`), imported themes as JSON
  under `<config-dir>/lich/themes`.
- **Workspace** — projects and sessions persist in SQLite at
  `<config-dir>/lich/lich.db`. Closing a session does not delete it.

## Privacy & updates

Everything runs on your machine. No account, no sign-in, no telemetry — the
backend is a token-authenticated loopback listener, and nothing leaves
`localhost` except the update check: a version ping to GitHub Releases at startup
and hourly. Updates apply in place on Windows/macOS and through the AUR on Arch.
Settings › Help says what the log file carries — paths, project and branch names,
your gh login, never a session token — before you attach it to a bug report.

## Build from source

Pure-Go backend (Go 1.25, `CGO_ENABLED=0`) serving an embedded React 18 /
TypeScript / Vite frontend over a token-authenticated loopback listener (HTTP RPC
+ WebSockets). Terminals are xterm.js with the WebGL addon; the code and diff
surfaces are CodeMirror 6. Prerequisites are **Go 1.25+**, **Node + pnpm** and
**[Task](https://taskfile.dev)** — no C toolchain, no system dev libraries.

```bash
task dev      # hot-reload dev mode (Vite on :9245)
task build    # production binary -> bin/lich
task run      # build + run
task test     # Go + frontend suites
```

Package a Linux release locally (needs
[nfpm](https://nfpm.goreleaser.com/)):

```bash
task package   # .deb + .rpm + Arch .pkg.tar.zst in bin/
```

## License

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich is free software: you can use, study, modify and redistribute it under
the terms of the GNU Affero General Public License v3. Any distributed or
network-served derivative must be released under the same license. Releases
up to and including v0.9.0 remain MIT-licensed.
