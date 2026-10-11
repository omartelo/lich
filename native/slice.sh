#!/bin/sh
# Runs the native slice: an isolated lich backend (its own DB, port 47901, no
# window) and the lich-native window on top of it.
#
#   ./slice.sh <checkout to open>     first run: opens that checkout as the project
#   ./slice.sh                        later runs reuse the backend's project
#   ./slice.sh stop                   stops the backend and its sessions
#
# The backend outlives the window, so sessions keep running between runs.
#
# Env: SLICE_HOME     where the isolated backend keeps its state
#                     (default ~/.cache/lich-native-slice)
#      LICH_BIN       the lich binary the backend runs (default: lich on PATH);
#                     point it at a build of this checkout to test its backend
#      LIBGHOSTTY_VT  prefix of a libghostty-vt built from ghostty main
#                     (zig build -Demit-lib-vt -Doptimize=ReleaseFast --prefix ...;
#                     default $SLICE_HOME/libghostty-vt, at the commit in GHOSTTY_COMMIT)
set -eu
SLICE_HOME=${SLICE_HOME:-$HOME/.cache/lich-native-slice}
LIBGHOSTTY_VT=${LIBGHOSTTY_VT:-$SLICE_HOME/libghostty-vt}
BACKEND_BIN=${LICH_BIN:-lich}
CONFIG=$SLICE_HOME/config
RUNTIME=$CONFIG/lich/runtime-dev.json
PORT=47901

if [ "${1:-}" = stop ]; then
  pid=$(sed -n 's/.*"pid":\([0-9]*\).*/\1/p' "$RUNTIME" 2>/dev/null)
  [ -n "$pid" ] || { echo "no backend running (no $RUNTIME)" >&2; exit 1; }
  kill "$pid"
  exit 0
fi

# The backend's sessions inherit its environment, so XDG_CONFIG_HOME cannot
# simply move: every entry of ~/.config is linked in, and only lich/ is the
# backend's own.
mkdir -p "$CONFIG/lich"
for entry in "$HOME"/.config/* "$HOME"/.config/.[!.]*; do
  [ -e "$entry" ] || continue
  name=$(basename "$entry")
  [ "$name" = lich ] || ln -sfn "$entry" "$CONFIG/$name"
done

# Stands in for the lich window: holds until lich exits and closes its stdin.
cat > "$SLICE_HOME/no-window" <<'STUB'
#!/bin/sh
cat > /dev/null
STUB
chmod +x "$SLICE_HOME/no-window"

if ! curl -sf -o /dev/null -X POST -d '[]' "http://127.0.0.1:$PORT/rpc/store.LoadState?token=$(sed -n 's/.*"token":"\([^"]*\)".*/\1/p' "$RUNTIME" 2>/dev/null)"; then
  rm -f "$RUNTIME"
  (
    # Whatever lich or agent session launched this must not leak into the
    # backend or its sessions.
    for v in $(env | sed -n 's/^\(LICH_[A-Z_]*\|CLAUDE[A-Z_]*\)=.*/\1/p'); do unset "$v"; done
    exec setsid -f env XDG_CONFIG_HOME="$CONFIG" LICH_DEV=1 LICH_LISTEN_PORT=$PORT LICH_SHELL="$SLICE_HOME/no-window" \
      "$BACKEND_BIN" > "$SLICE_HOME/backend.log" 2>&1 < /dev/null
  )
  i=0
  until [ -s "$RUNTIME" ]; do
    i=$((i + 1))
    [ $i -le 50 ] || { echo "backend did not start; see $SLICE_HOME/backend.log" >&2; exit 1; }
    sleep 0.2
  done
fi

cd "$(dirname "$0")"
PKG_CONFIG_PATH="$LIBGHOSTTY_VT/share/pkgconfig" go build -o cmd/lich-native/lich-native ./cmd/lich-native
if [ $# -gt 0 ]; then
  exec ./cmd/lich-native/lich-native -runtime "$RUNTIME" -project "$1"
fi
exec ./cmd/lich-native/lich-native -runtime "$RUNTIME"
