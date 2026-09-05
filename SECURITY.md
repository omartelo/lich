# Security policy

## Reporting a vulnerability

Report it privately through GitHub:
**[Security → Report a vulnerability](https://github.com/omartelo/lich/security/advisories/new)**.
That opens a private advisory only you and the maintainer can read.

Please don't open a public issue for a security problem — the issue tracker is
world-readable, and a working report there is a working exploit for everyone
else too.

Include what you did, what happened, and the version (Settings → Help, or
`lich --version`). A proof of concept helps; a patch is welcome but never
expected.

## Supported versions

The latest release only. lich updates in place on Windows and macOS and through
the AUR on Arch, so a fix ships as a new version rather than as a backport.

## What the attack surface actually is

lich runs entirely on your machine. There is no server, no account and no
telemetry, so the interesting boundaries are local ones:

- **The loopback listener.** One port (47821 by default) carries the HTTP RPC
  and the WebSockets for terminal I/O and app events. It binds to loopback and
  every request is authenticated by a per-run token. Anything that reaches it
  without that token, or from off-host, is a vulnerability.
- **The session token.** It is never written to the log. Finding it in a log,
  a crash report or a URL is a vulnerability.
- **The workspace.** Projects and sessions live in SQLite under your config
  directory; UI preferences live in the Chromium profile beside it.
- **`GH_TOKEN`.** A project can name the GitHub account `gh` runs as, which
  puts that account's token in the environment of the `gh` calls lich makes.
  A path that leaks it, or hands it to the wrong project, is a vulnerability.
- **Session hooks.** The scripts ship from
  [lich-plugin](https://github.com/omartelo/lich-plugin) and post to the same
  authenticated listener. A payload that escapes its contract belongs here too;
  a bug in the scripts themselves belongs in that repository.

## What is not a vulnerability

- **An agent running commands.** lich spawns your agent in a real PTY, and that
  agent runs whatever it decides to run. That is the product, not a flaw — the
  trust boundary is the agent you chose and pointed lich at.
- **A dropped file resolving to a copy.** When a drop matches nothing in the
  searched trees, lich copies it to a temporary directory and hands over that
  path. Documented, deliberate.
- **Two checkouts sharing a `LICH_WORKTREE_PORT`.** lich reserves the number and
  proves it free at the time it is handed out, but nothing holds it until your
  dev server binds it.
- **A Run card executing a file from your repository.** `.lich/run-worktree.sh`
  is run when you pick Run, the way `.lich/setup-worktree.sh` is run when you
  create a worktree — both are code in the checkout you opened, and opening a
  repository you do not trust is the decision, not the menu item.
