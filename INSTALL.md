# Installing lich

lich targets Linux x86_64 first; an experimental Windows x64 build ships
alongside it. Every artifact comes from the
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

**Runtime dependencies** — lich opens its window in a Chromium-family browser;
none is bundled. On Linux any of `chromium`, `google-chrome`, `helium-browser` or `brave`
satisfies it, and `zenity` provides the folder picker. On macOS, Chrome,
Chromium, Edge or Brave are looked up as `.app` bundles under `/Applications`
(and `~/Applications`), and the folder picker is native. On Windows, Chrome,
Edge or Brave are found via their conventional install paths (Edge ships with
Windows) and the folder picker is native.

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

If your apt is configured with `--no-install-recommends`, install them
yourself:

```bash
sudo apt-get install chromium zenity
```

## Fedora / RHEL

Download the `.rpm` from the releases page, then install it — dnf resolves the
runtime dependencies on its own (weak dependencies are on by default):

```bash
sudo dnf install ./lich-*-x86_64.rpm
```

If dnf runs with `install_weak_deps=False`, install them yourself:

```bash
sudo dnf install chromium zenity
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
sudo pacman -S chromium zenity
```

## Static binary

Every release also ships the bare binary (`lich-*-linux-amd64`) — pure static
Go, no libraries needed. Download it from the releases page, then drop it on
your PATH:

```bash
install -Dm755 lich-*-linux-amd64 ~/.local/bin/lich
```

You still need the runtime dependencies — install `chromium` (or another
Chromium-family browser) and `zenity` through your package manager.

## macOS (experimental)

From the [tap](https://github.com/omartelo/homebrew-tap) — Apple Silicon and
Intel both:

```bash
brew install omartelo/tap/lich
```

The formula installs the release binary, so `brew upgrade` tracks new versions
and lich's own update button steps aside on a Homebrew install.

Without Homebrew, download `lich-*-darwin-arm64` (Apple Silicon) or
`lich-*-darwin-amd64` (Intel) from the releases page and install it by hand.
The binary is unsigned, so a browser or `curl` download carries a quarantine
flag Gatekeeper refuses to run — clear it after verifying the checksum (see
below):

```bash
install -m755 lich-*-darwin-arm64 ~/.local/bin/lich
xattr -d com.apple.quarantine ~/.local/bin/lich
```

## Windows (experimental)

Download `lich-*-windows-amd64-setup.exe` from the releases page and run it.
The install is per-user (no admin prompt): lich lands in
`%LocalAppData%\Programs\lich`, shows up in the Start Menu and in Settings →
Installed apps, and uninstalls from there like any other application.

The installer is not code-signed, so SmartScreen will warn on first run —
"More info" → "Run anyway". Verify the download against `checksums.txt` first
(see below).

lich runs windowless on Windows; diagnostics live in `%AppData%\lich\lich.log`.

The bare `lich-*-windows-amd64.exe` is also published for a portable,
no-install run — same binary the installer ships.

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
something else is, and so is a missing Chromium-family browser.

When you file the issue, attach the bundle:

```bash
lich rage
```

That collects the same facts plus your logs into one `lich-rage-*.tar.gz`, with
values named like a token, key or password reported only as present or absent.
Nothing is uploaded — read it, then attach it.
