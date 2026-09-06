#!/bin/sh
# Wraps an already-built darwin binary in Lich.app — the bundle is what puts
# lich in /Applications, Launchpad and Spotlight and gives it its icon; a bare
# binary on PATH appears in none of them.
#
#   build/darwin/bundle.sh bin/lich-darwin-arm64 0.30.0 bin [bin/shell]
#
# With a fourth argument, the window build/darwin/shell/assemble.sh laid out:
# lich-shell goes beside lich in Contents/MacOS, where macOS counts a process
# as the app's own (its Dock tile, its icon, its name in the menu bar), the
# CEF framework into Contents/Frameworks, where kurogane looks for it, and
# five copies of lich-shell into the helper apps beside the framework that
# CEF's subprocesses run as ("Lich Helper", plus the Renderer, GPU, Plugin
# and Alerts variants Chromium derives from it: without them a bundled
# browser never launches a renderer, measured). Without one the bundle opens
# a system browser (the Intel build).
#
# macOS host only: sips and iconutil are Apple's, so the icon is generated at
# build time from build/appicon-mac.png instead of committing an .icns — one
# source of the mark for every platform. Cross-compiling the binary still works
# anywhere (task build:mac); only this wrapping step needs a Mac.
set -eu

bin="${1:?usage: bundle.sh <binary> <version> <outdir> [shell dir]}"
version="${2:?usage: bundle.sh <binary> <version> <outdir> [shell dir]}"
outdir="${3:?usage: bundle.sh <binary> <version> <outdir> [shell dir]}"
shell="${4:-}"

root="$(cd "$(dirname "$0")/../.." && pwd)"
app="$outdir/Lich.app"
framework="Chromium Embedded Framework.framework"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin" "$app/Contents/MacOS/lich"
chmod +x "$app/Contents/MacOS/lich"

# One helper app: <name>.app with the window's binary under that name and an
# LSUIElement plist, so no subprocess owns a Dock tile.
helper() {
  name="$1"
  role="$2"
  dir="$app/Contents/Frameworks/$name.app/Contents"
  mkdir -p "$dir/MacOS"
  cp "$shell/lich-shell" "$dir/MacOS/$name"
  sed -e "s/@VERSION@/${version}/g" -e "s/@NAME@/${name}/g" -e "s/@ROLE@/${role}/g" \
    "$root/build/darwin/Helper-Info.plist.tpl" > "$dir/Info.plist"
}

if [ -n "$shell" ]; then
  mkdir -p "$app/Contents/Frameworks"
  cp "$shell/lich-shell" "$app/Contents/MacOS/lich-shell"
  ditto "$shell/$framework" "$app/Contents/Frameworks/$framework"
  helper "Lich Helper" ""
  helper "Lich Helper (Renderer)" ".renderer"
  helper "Lich Helper (GPU)" ".gpu"
  helper "Lich Helper (Plugin)" ".plugin"
  helper "Lich Helper (Alerts)" ".alerts"
fi

sed "s/@VERSION@/${version}/g" "$root/build/darwin/Info.plist.tpl" \
  > "$app/Contents/Info.plist"

iconset="$(mktemp -d)/lich.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$root/build/appicon-mac.png" \
    --out "$iconset/icon_${size}x${size}.png" >/dev/null
  sips -z "$((size * 2))" "$((size * 2))" "$root/build/appicon-mac.png" \
    --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$app/Contents/Resources/lich.icns"
rm -rf "$(dirname "$iconset")"

# Ad-hoc signature: arm64 refuses to run an executable carrying none, and a
# bundle whose seal does not cover Info.plist and the icon reads as damaged.
# Innermost first, as codesign wants it: ANGLE's dylibs are not in a place
# --deep would find them, the framework's seal must cover the trimmed
# Resources, not the distribution's, and each helper app seals its own plist.
# This is not notarization — Gatekeeper still calls the developer unidentified.
if [ -n "$shell" ]; then
  fw="$app/Contents/Frameworks/$framework"
  for lib in "$fw/Libraries/"*.dylib; do
    codesign --force --sign - "$lib"
  done
  codesign --force --sign - "$fw"
  for h in "$app/Contents/Frameworks/Lich Helper"*.app; do
    codesign --force --sign - "$h"
  done
  codesign --force --sign - "$app/Contents/MacOS/lich-shell"
fi
codesign --force --sign - "$app"

echo "built $app"
