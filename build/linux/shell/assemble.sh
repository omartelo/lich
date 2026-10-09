#!/bin/sh
# Assembles the window's runtime directory out of a cargo release build and a
# CEF distribution (build/cef.sh fetches one):
#
#   <out>/lich-shell        the binary (it loads libcef.so when it starts)
#   <out>/kurogane-bundle   the marker that tells kurogane the runtime is in cef/
#   <out>/cef/              the CEF runtime, reduced to what lich needs
#
# usage: assemble.sh <cargo target/release dir> <CEF distribution dir> <out dir>
set -eu
src="$1"
cef="$2"
out="$3"

rm -rf "$out"
mkdir -p "$out/cef/locales"
cp "$src/lich-shell" "$out/lich-shell"
: > "$out/kurogane-bundle"

# Debug info and the symbol table are ~1.1 GB of the 1.3 GB libcef.so; the
# stripped library is ~260 MB. CEF resolves nothing through the symbol table
# at run time: the segfault once blamed on stripping it was the feature-list
# overwrite (shell/src/main.rs), measured again after that fix.
strip -o "$out/cef/libcef.so" "$cef/libcef.so"
for f in chrome_100_percent.pak chrome_200_percent.pak \
  resources.pak icudtl.dat v8_context_snapshot.bin chrome-sandbox; do
  cp "$cef/$f" "$out/cef/$f"
done
# The UI is English and a locale pack only translates Chromium's own dialogs;
# en-US is the one Chromium falls back to, and the one kurogane checks for.
cp "$cef/locales/en-US.pak" "$out/cef/locales/"
# Left out on purpose: libvk_swiftshader.so and libvulkan.so.1, the software
# WebGL for a machine with no GPU. xterm.js falls back to its canvas renderer
# there, at 16 MB less for everyone else.
