# Installing lich

lich targets Linux x86_64 first; an experimental Windows x64 build ships
alongside it, with the same window. Every artifact comes from the
[Releases](https://github.com/omartelo/lich/releases) page.

Pick your system:

- [Debian / Ubuntu](#debian--ubuntu)
- [Fedora / RHEL](#fedora--rhel)
- [Arch](#arch)
- [Static binary (any distro)](#static-binary)
- [macOS (experimental)](#macos-experimental)
- [Windows (experimental)](#windows-experimental)
- [Verifying checksums](#verifying-checksums)
- [If it does not start](#if-it-does-not-start)

**Runtime dependencies** — lich ships its own window: an embedded Chromium
(CEF) inside the package, so no browser is required. On Linux `zenity` is the
one thing left to install, for the folder picker; on Windows nothing is. The
Linux window needs glibc 2.34 or newer — Debian 12, Ubuntu 22.04, RHEL 9, or
anything current — and the libraries Chromium itself links against, which the
deb, rpm and AUR packages declare. The window carries lich's own class and
title, so the launcher icon, window rules by class and `StartupWMClass` all
match it. A package missing its window, or a bare binary copied out of the
tarball, does not start: `lich doctor` says so, and `--shell` or `LICH_SHELL`
points lich at a window build of your own
([docs/chromium-shell.md](docs/chromium-shell.md)).

On macOS, `Lich.app` on Apple Silicon carries the same window. An Intel Mac
gets none: lich serves itself, opens a plain tab in your default browser, tells
you so in a desktop notification, and goes on running until you stop it —
closing the tab leaves it running, so stop it with Ctrl-C or by signalling the
process. The folder picker is native either way. The same tab is what an Apple
Silicon install falls back to when its own window fails to open.

**git and the GitHub CLI** — every version control surface shells out to
`git`, and lich does not bundle it: without `git` on your `PATH`, branches,
diffs and worktrees stay empty, though sessions still run. Pull requests,
checks and PR checkouts go through [`gh`](https://cli.github.com) as well,
which is optional — install it only if you use them. lich resolves both when it
launches, so one installed while lich is open is picked up on the next start,
and `lich doctor` reports which it found.

**Agent versions** — lich names each Claude Code session with `--name` at spawn,
a flag added in Claude Code 2.1.76. An older build exits with
`error: unknown option '--name'` before the session exists, which reads as lich
failing to start it; upgrade Claude Code. Nothing lich passes the other
providers unprompted is that recent.

## Debian / Ubuntu

Download the `.deb` from the releases page, then install it — apt resolves the
runtime dependencies on its own (they are Recommends):

```bash
sudo apt-get install ./lich-*-amd64.deb
```

If your apt is configured with `--no-install-recommends`, install it
yourself:

```bash
sudo apt-get install zenity
```

## Fedora / RHEL

Download the `.rpm` from the releases page, then install it — dnf resolves the
runtime dependencies on its own (weak dependencies are on by default):

```bash
sudo dnf install ./lich-*-x86_64.rpm
```

If dnf runs with `install_weak_deps=False`, install them yourself:

```bash
sudo dnf install zenity
```

## Arch

From the AUR ([lich-bin](https://aur.archlinux.org/packages/lich-bin)):

```bash
yay -S lich-bin   # or: paru -S lich-bin
```

Or download the `.pkg.tar.zst` from the releases page and install it:

```bash
sudo pacman -U lich-*-x86_64.pkg.tar.zst
```

pacman has no Recommends (the runtime dependencies are `optdepends`), so
install them yourself:

```bash
sudo pacman -S zenity
```

## Static binary

Every release also ships the bare binary (`lich-*-linux-amd64`) — pure static
Go, no libraries needed. Download it from the releases page, then drop it on
your PATH:

```bash
install -Dm755 lich-*-linux-amd64 ~/.local/bin/lich
tar --zstd -xf lich-*-linux-amd64-shell.tar.zst -C ~/.local/bin
```

The second line unpacks the window beside the binary, as `~/.local/bin/shell/`
— lich looks for it there, and under `../lib/lich/shell` relative to its bin.
Without it — or if it fails to start on your machine — lich does not open:
a dialog and the log say why. `zenity` still comes from your package manager.

## macOS (experimental)

From the [tap](https://github.com/omartelo/homebrew-tap) — Apple Silicon and
Intel both:

```bash
brew install --cask omartelo/tap/lich
```

The cask installs `Lich.app` into `/Applications` — Launchpad, Spotlight and
the Finder list it under its own icon — and symlinks the same binary onto
`PATH` as `lich`, so the app and the command are one install. `brew upgrade
--cask omartelo/tap/lich` tracks new versions, and lich's own update button
steps aside on a Homebrew install.

On Apple Silicon the app carries its own window, the same embedded Chromium
the Linux packages ship, and the Dock shows the lich icon while it runs. On
Intel lich is a tab in your default browser, and the Dock, while lich runs,
shows that browser's icon: the tab belongs to it, and macOS has no equivalent
of the window class Linux matches against the lich launcher. There the lich
icon is the one you launch from, not the one
you switch to.

**Upgrading from the old formula** — releases up to v0.32.0 shipped a bare CLI
as `Formula/lich.rb`. Homebrew refuses to install the cask over it (both want
`lich` on `PATH`), so remove the formula first:

```bash
brew uninstall lich
brew install --cask omartelo/tap/lich
```

Without Homebrew, download `lich-*-darwin-arm64.zip` (Apple Silicon) or
`lich-*-darwin-amd64.zip` (Intel) from the releases page, unzip it and drag
`Lich.app` into `/Applications`. It is ad-hoc signed and not notarized, so
after verifying the checksum (see below) clear the quarantine flag Gatekeeper
would otherwise refuse:

```bash
xattr -dr com.apple.quarantine /Applications/Lich.app
```

The bare `lich-*-darwin-arm64` and `lich-*-darwin-amd64` binaries are still
published for a CLI-only install by hand; they carry no window and open lich
as a tab in the default browser:

```bash
install -m755 lich-*-darwin-arm64 ~/.local/bin/lich
xattr -d com.apple.quarantine ~/.local/bin/lich
```

## Windows (experimental)

Download `lich-*-windows-amd64-setup.exe` from the releases page and run it.
The install is per-user (no admin prompt): lich lands in
`%LocalAppData%\Programs\lich` with its window beside it as `shell\`, shows
up in the Start Menu and in Settings → Installed apps, and uninstalls from
there like any other application. An installed lich updates by running the
next installer: the update button opens the release page, since the window is
what an in-place swap of the exe would leave behind.

The installer is not code-signed, so SmartScreen will warn on first run —
"More info" → "Run anyway". Verify the download against `checksums.txt` first
(see below).

lich runs windowless on Windows; diagnostics live in `%AppData%\lich\lich.log`.

Scoop installs the same two assets from the manifest every release publishes
beside them, no bucket to add:

```powershell
scoop install https://github.com/omartelo/lich/releases/latest/download/lich.json
```

It puts `lich.exe` in the app directory with the window beside it as `shell\`,
keeps `lich` on PATH and adds a Start Menu entry; `scoop update lich` re-reads
the manifest from that URL, which is always the latest release's. The workspace stays in `%AppData%\lich`, so
`scoop uninstall lich` leaves your projects and sessions alone.

The bare `lich-*-windows-amd64.exe` is also published for a portable,
no-install run — same binary the installer ships. Unzip
`lich-*-windows-amd64-shell.zip` beside it to get the window as `shell\`;
without it, lich does not open, and a dialog says so.

## Verifying checksums

Every release ships a `checksums.txt`. With it in the same directory as the
downloaded artifact:

```bash
sha256sum -c --ignore-missing checksums.txt
```

`install.sh` (the [one-liner in the README](README.md#install)) does this
verification automatically before installing.

## If it does not start

No window is the one failure lich cannot report through its own settings, so it
reports it from the terminal instead:

```bash
lich doctor
```

It walks the same boot a launch walks — the config directory, the log file, the
pinned loopback port, the workspace database, the browser, the provider CLIs on
PATH — and says which step would stop it, exiting non-zero when one does. A port
already held by the lich you have open is not a failure; a port held by
something else is, and so is a missing window — the one lich ships, or the
build `LICH_SHELL` points at.

When you file the issue, attach the bundle:

```bash
lich rage
```

That collects the same facts plus your logs into one `lich-rage-*.tar.gz`, with
values named like a token, key or password reported only as present or absent.
Nothing is uploaded — read it, then attach it.
