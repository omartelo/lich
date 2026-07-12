#!/usr/bin/env bash
# Makes the wails3-generated AppDir truly portable and repacks it.
#
# WebKitGTK release builds are not relocatable: libwebkitgtk hardcodes the
# absolute path of its helper binaries (WebKitNetworkProcess/WebKitWebProcess)
# at compile time, so an AppImage built on Ubuntu aborts on any distro whose
# webkit lives elsewhere (Arch, Fedora, ...). Three fixes are needed, none of
# which wails3 `generate appimage` applies (tauri's bundler applies the same
# sed patch — see tauri-bundler linuxdeploy-plugin-gtk.sh):
#
#   1. Binary-patch "/usr" -> "././" in libwebkit* (same byte length, keeps
#      the ELF valid). Combined with an AppRun that chdirs into $APPDIR/usr,
#      the baked paths resolve inside the AppDir.
#   2. chmod +x the bundled WebKit*Process helpers (wails copies them 644).
#   3. Disable the webkit sandbox: webkitgtk-6.0 sandboxes unconditionally
#      via the host's bwrap, whose path is also baked (and now patched to a
#      relative path that doesn't exist). The webview only renders lich's own
#      embedded frontend, never arbitrary web content.
#
# Usage: fix-appimage.sh <appdir> <app-name> <output-appimage>
set -euo pipefail

APP_DIR="$1"
APP_NAME="$2"
OUTPUT="$3"

find "$APP_DIR"/usr/lib* -name 'libwebkit*' -exec sed -i -e 's|/usr|././|g' '{}' \;
find "$APP_DIR" -name 'WebKit*Process' -exec chmod +x '{}' \;

cat > "$APP_DIR/AppRun" <<EOF
#!/usr/bin/env bash
HERE="\$(dirname "\$(readlink -f "\$0")")"
export APPDIR="\$HERE"

LD_LIBRARY_PATH="\$HERE/usr/lib:\${LD_LIBRARY_PATH:-}"
for dir in "\$HERE"/usr/lib/*-linux-gnu*; do
    [ -d "\$dir" ] && LD_LIBRARY_PATH="\$dir:\$LD_LIBRARY_PATH"
done
export LD_LIBRARY_PATH
export PATH="\$HERE/usr/bin:\$PATH"
export XDG_DATA_DIRS="\$HERE/usr/share:\${XDG_DATA_DIRS:-/usr/local/share:/usr/share}"

for hook in "\$HERE"/apprun-hooks/*.sh; do
    [ -f "\$hook" ] && source "\$hook"
done

export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1

# The ././-patched webkit paths resolve relative to the process cwd.
cd "\$HERE/usr"
exec "\$HERE/usr/bin/$APP_NAME" "\$@"
EOF
chmod +x "$APP_DIR/AppRun"

ARCH="$(uname -m)"
APPIMAGETOOL="$(dirname "$APP_DIR")/appimagetool-${ARCH}.AppImage"
if [ ! -f "$APPIMAGETOOL" ]; then
    wget -q -4 -O "$APPIMAGETOOL" "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${ARCH}.AppImage"
    chmod +x "$APPIMAGETOOL"
fi

ARCH="$ARCH" "$APPIMAGETOOL" --appimage-extract-and-run "$APP_DIR" "$OUTPUT"
