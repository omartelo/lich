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
# usage: build/cef.sh <dir> [target triple]
set -eu
dir="$1"
target="${2:-}"
root="$(cd "$(dirname "$0")/.." && pwd)"

tetsu="$(sed -n '/^name = "tetsu-sys"$/,/^$/p' "$root/shell/Cargo.lock" | sed -n 's/^version = "\(.*\)"$/\1/p')"
version="${tetsu#*+}"
tag="v$tetsu"
test -n "$version" || { echo "cef.sh: no tetsu-sys in shell/Cargo.lock" >&2; exit 1; }

# archive.json names the archive the directory was unpacked from, version in it
if [ -f "$dir/archive.json" ] && grep -q "\"name\": *\"cef_binary_$version+" "$dir/archive.json"; then
  echo "cef.sh: CEF $version already in $dir"
  exit 0
fi

tools="$root/shell/target/tetsu-tools"
if [ ! -x "$tools/bin/export-cef-dir" ] || ! "$tools/bin/export-cef-dir" --help | grep -q "default: $version"; then
  cargo install --locked --root "$tools" --git https://github.com/kurogane-rs/tetsu --tag "$tag" export-cef-dir
fi
rm -rf "$dir"
if [ -n "$target" ]; then
  "$tools/bin/export-cef-dir" --target "$target" "$dir"
else
  "$tools/bin/export-cef-dir" "$dir"
fi
