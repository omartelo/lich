# Installing lich

lich targets Linux x86_64 first; an experimental Windows x64 build ships
alongside it. Every artifact comes from the
[Releases](https://github.com/omartelo/lich/releases) page.

Pick your system:

- [Debian / Ubuntu](#debian--ubuntu)
- [Fedora / RHEL](#fedora--rhel)
- [Arch](#arch)
- [Static binary (any distro)](#static-binary)
- [Windows (experimental)](#windows-experimental)
- [Verifying checksums](#verifying-checksums)

**Runtime dependencies** — lich opens its window in a Chromium-family browser;
none is bundled. On Linux any of `chromium`, `google-chrome` or `brave`
satisfies it, and `zenity` provides the folder picker. On Windows, Chrome,
Edge or Brave are found via their conventional install paths (Edge ships with
Windows) and the folder picker is native.

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

Download the `.pkg.tar.zst` from the releases page, then install it:

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
