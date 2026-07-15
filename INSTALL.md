# Installing lich

lich is Linux-only, x86_64. Every artifact comes from the
[Releases](https://github.com/omartelo/lich/releases) page.

Pick your system:

- [Debian / Ubuntu](#debian--ubuntu)
- [Fedora / RHEL](#fedora--rhel)
- [Arch](#arch)
- [Static binary (any distro)](#static-binary)
- [Verifying checksums](#verifying-checksums)

**Runtime dependencies** — lich opens its window in a Chromium-family browser
and uses zenity for the folder picker; neither is bundled. Any of `chromium`,
`google-chrome` or `brave` satisfies the browser requirement.

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

## Verifying checksums

Every release ships a `checksums.txt`. With it in the same directory as the
downloaded artifact:

```bash
sha256sum -c --ignore-missing checksums.txt
```

`install.sh` (the [one-liner in the README](README.md#install)) does this
verification automatically before installing.
