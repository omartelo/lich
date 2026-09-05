#!/bin/sh
# Assembles the window's runtime directory out of a cargo release build:
#
#   <out>/lich-shell   the binary (its rpath looks for libcef.so in cef/)
#   <out>/cef/         the CEF runtime, reduced to what lich needs
#
# usage: assemble.sh <cargo target/release dir> <out dir>
set -eu
src="$1"
out="$2"

rm -rf "$out"
mkdir -p "$out/cef/locales"
cp "$src/lich-shell" "$out/lich-shell"

# Debug info is ~1 GB of the 1.3 GB libcef.so; dropping it leaves ~430 MB.
# --strip-debug only: a full strip removes the symbol table CEF resolves at
# runtime and the window segfaults on first paint (measured), so the symbols
# stay even though they are another ~180 MB.
strip --strip-debug -o "$out/cef/libcef.so" "$src/libcef.so"
for f in libEGL.so libGLESv2.so chrome_100_percent.pak chrome_200_percent.pak \
  resources.pak icudtl.dat v8_context_snapshot.bin chrome-sandbox; do
  cp "$src/$f" "$out/cef/$f"
done
# The UI is English and a locale pack only translates Chromium's own dialogs;
# en-US is the one Chromium falls back to, and the one kurogane checks for.
cp "$src/locales/en-US.pak" "$out/cef/locales/"
# Left out on purpose: libvk_swiftshader.so and libvulkan.so.1, the software
# WebGL for a machine with no GPU. xterm.js falls back to its canvas renderer
# there, at 16 MB less for everyone else.
