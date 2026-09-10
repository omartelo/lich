#!/bin/sh
# Assembles the window's runtime directory out of a cargo release build, the
# macOS layout: the binary beside the CEF framework, which is what
# build/darwin/bundle.sh lays into Lich.app (build/linux/shell/assemble.sh and
# build/windows/shell/assemble.sh are the counterparts).
#
#   <out>/lich-shell                              the binary
#   <out>/Chromium Embedded Framework.framework   the CEF runtime, reduced to what lich needs
#   <out>/libEGL.dylib, libGLESv2.dylib           links into the framework, for a run outside a bundle
#
# cef-rs copies nothing out of the distribution on macOS, so the framework is
# taken from the distribution cef-rs downloaded into CEF_PATH.
#
# usage: assemble.sh <cargo target/release dir> <CEF_PATH> <out dir>
set -eu
src="$1"
cef="$2"
out="$3"
name="Chromium Embedded Framework.framework"

framework="$(find "$cef" -maxdepth 3 -type d -name "$name" | head -n 1)"
test -n "$framework" || { echo "assemble.sh: no $name under $cef" >&2; exit 1; }

rm -rf "$out"
mkdir -p "$out"
cp "$src/lich-shell" "$out/lich-shell"
# ditto, not cp: it keeps the framework's symlinks and permissions as they are.
ditto "$framework" "$out/$name"

fw="$out/$name"
# The UI is English and a locale pack only translates Chromium's own dialogs;
# en is the one Chromium falls back to, and kurogane checks for any .lproj.
test -d "$fw/Resources/en.lproj" || { echo "assemble.sh: no en.lproj in $fw/Resources" >&2; exit 1; }
find "$fw/Resources" -maxdepth 1 -name '*.lproj' ! -name 'en.lproj' -exec rm -r {} +
# Left out on purpose, as on Linux and Windows: SwiftShader, the software
# WebGL for a machine with no GPU. xterm.js falls back to its canvas renderer.
rm -f "$fw/Libraries/libvk_swiftshader.dylib" "$fw/Libraries/vk_swiftshader_icd.json"
# Outside an application bundle Chromium's GPU process loads ANGLE from the
# executable's own directory (`task dev`, bin/ after `task build`); inside
# Lich.app it reads the framework and these are not copied along.
for lib in libEGL.dylib libGLESv2.dylib; do
  ln -sf "$name/Libraries/$lib" "$out/$lib"
done
