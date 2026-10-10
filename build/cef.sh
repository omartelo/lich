#!/bin/sh
# Fetches the CEF distribution the window loads, into <dir>, for this machine
# or for the target named. kurogane loads libcef when the window starts and
# links nothing at build time, so the distribution is the assemble scripts'
# input alone (build/<os>/shell/assemble.sh), and this is the one download.
#
# The version is tetsu's, read off the lock file: the bindings refuse any
# other CEF build at load, so there is nothing to choose here. The fetch is
# tetsu's own tool, built from the same revision, which writes the
# archive.json kurogane's tooling trusts. A directory already holding that
# version is left alone.
#
# --dir prints where the Taskfile keeps the distribution: CEF_DIR when set,
# else a per-user cache keyed by version, so every worktree on one version
# shares one ~1.4 GB copy and worktrees on different versions never evict
# each other.
#
# usage: build/cef.sh <dir> [target triple]
#        build/cef.sh --dir
set -eu
root="$(cd "$(dirname "$0")/.." && pwd)"

tetsu="$(sed -n '/^name = "tetsu-sys"$/,/^$/p' "$root/shell/Cargo.lock" | sed -n 's/^version = "\(.*\)"$/\1/p')"
version="${tetsu#*+}"
tag="v$tetsu"
test -n "$version" || { echo "cef.sh: no tetsu-sys in shell/Cargo.lock" >&2; exit 1; }

if [ "$1" = --dir ]; then
  if [ -n "${CEF_DIR:-}" ]; then
    echo "$CEF_DIR"
    exit 0
  fi
  case "$(uname -s)" in
    Darwin) cache="$HOME/Library/Caches" ;;
    # git-bash: LOCALAPPDATA is a native C:\ path; forward slashes keep it one
    # word for bash, Task's shell and the native fetch tool alike
    MINGW* | MSYS* | CYGWIN*)
      : "${LOCALAPPDATA:?cef.sh: LOCALAPPDATA is not set}"
      cache="$(printf '%s' "$LOCALAPPDATA" | tr '\\' /)"
      ;;
    *) cache="${XDG_CACHE_HOME:-$HOME/.cache}" ;;
  esac
  echo "$cache/lich-cef/$version"
  exit 0
fi

dir="$1"
target="${2:-}"

# archive.json names the archive the directory was unpacked from, version in it
holds_version() {
  [ -f "$dir/archive.json" ] && grep -q "\"name\": *\"cef_binary_$version+" "$dir/archive.json"
}

if holds_version; then
  echo "cef.sh: CEF $version already in $dir"
  exit 0
fi

tools="$root/shell/target/tetsu-tools"
if [ ! -x "$tools/bin/export-cef-dir" ] || ! "$tools/bin/export-cef-dir" --help | grep -q "default: $version"; then
  cargo install --locked --root "$tools" --git https://github.com/kurogane-rs/tetsu --tag "$tag" export-cef-dir
fi

# The fetch lands in a staging directory of this run's own and is renamed into
# place whole: two worktrees building on a cold cache never see each other's
# half extraction, and export-cef-dir downloads and unpacks into the parent of
# its output, which would collide if the parent were shared.
staging="$dir.fetch-$$"
fetched="$staging/cef-$$"
trap 'rm -rf "$staging"' EXIT
trap 'exit 1' HUP INT TERM
mkdir -p "$staging"
if [ -n "$target" ]; then
  "$tools/bin/export-cef-dir" --target "$target" "$fetched"
else
  "$tools/bin/export-cef-dir" "$fetched"
fi
# Checked only now: in the shared cache a directory that appeared during the
# fetch is another run's finished copy, and only a CEF_DIR holding another
# version is in the way
if [ -e "$dir" ] && ! holds_version; then
  rm -rf "$dir"
fi
mv "$fetched" "$dir"
# A concurrent run renamed its copy first, so mv moved this one inside it
rm -rf "$dir/cef-$$"
