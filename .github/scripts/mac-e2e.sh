#!/bin/bash
# Runs a freshly bundled Lich.app end to end on the macOS runner, the Windows
# job's check on the bundle: the seal verifies, lich finds the window beside
# itself in Contents/MacOS and opens it, the page loads (over CDP: alive alone
# is not proof), macOS counts exactly one application in Lich.app (lsappinfo,
# which is what the Dock reads: a second entry is a subprocess with an
# NSApplication of its own, and a Dock tile of its own with it, four of them
# before kurogane#14), the window closes on SIGTERM, which is what the restart
# flow sends, and lich exits with it. Its config lives under a HOME of its own.
# What a failure leaves behind goes to $RUNNER_TEMP/diag for the artifact the
# workflow uploads.
#
# usage: mac-e2e.sh <path to Lich.app>
set -eu

app="$(cd "$1" && pwd)"
export HOME="$RUNNER_TEMP/home" LICH_LISTEN_PORT=47899
config="$HOME/Library/Application Support/lich"
log="$config/lich.log"
diag="$RUNNER_TEMP/diag"
pages="$RUNNER_TEMP/pages.json"
mkdir -p "$HOME" "$diag"

# The helper apps are "Lich Helper (Renderer)" and so on: a case-insensitive
# match is what catches them beside lich and lich-shell.
dump() {
  cp "$pages" "$diag/" 2>/dev/null || true
  ps -eo pid,ppid,args | grep -i '[l]ich' > "$diag/ps.txt" || true
  lsappinfo list > "$diag/apps.txt" || true
  screencapture -x "$diag/screen.png" || true
  cp "$log" "$diag/" 2>/dev/null || true
  # A subprocess that died leaves its report with the system, keyed by the
  # user, not by HOME.
  cp /Users/runner/Library/Logs/DiagnosticReports/[lL]ich* "$diag/" 2>/dev/null || true
  cat "$diag/ps.txt"
}

fail() {
  echo "::error::$1"
  dump
  cat "$log" 2>/dev/null || true
  exit 1
}

codesign --verify --deep --strict --verbose=2 "$app"
test -f "$app/Contents/MacOS/lich-shell"
test -f "$app/Contents/Frameworks/Chromium Embedded Framework.framework/Chromium Embedded Framework"

# Chromium's own log on stderr: a renderer or GPU process that fails to launch
# says so there and nowhere else.
"$app/Contents/MacOS/lich" -- --remote-debugging-port=9334 --enable-logging=stderr &
token=""
for _ in $(seq 1 30); do
  sleep 1
  token=$(jq -r .token "$config/runtime.json" 2>/dev/null) && break
done
test -n "$token" || fail "lich never wrote runtime.json"
grep -q 'the bundled window' "$log" || fail "lich did not resolve its own window"

for _ in $(seq 1 30); do
  sleep 1
  # Every CEF role alive while waiting: a renderer that comes and goes shows
  # up here and nowhere else.
  ps -eo args | grep -i 'lich' | grep -o -- '--type=[a-z-]*' | sort -u >> "$diag/roles.txt" || true
  curl -sf http://127.0.0.1:9334/json > "$pages" 2>/dev/null && grep -q '"title": *"lich"' "$pages" && break
done
grep -q '"title": *"lich"' "$pages" || { sort -u "$diag/roles.txt"; fail "the window never loaded lich's page"; }
sleep 10

# The browser process runs as lich-shell; the subprocesses are the helper apps.
browser=$(pgrep -x lich-shell || true)
test -n "$browser" || fail "the window died"

lsappinfo list > "$RUNNER_TEMP/apps.txt"
apps=$(grep -c 'bundle path=.*Lich.app' "$RUNNER_TEMP/apps.txt" || true)
[ "$apps" = 1 ] || { grep -B1 -A3 'Lich.app' "$RUNNER_TEMP/apps.txt"; fail "macOS counts $apps applications in Lich.app, expected the window alone"; }

kill -TERM "$browser"
for _ in $(seq 1 15); do sleep 1; pgrep -x lich > /dev/null || break; done
if pgrep -x lich > /dev/null; then
  fail "lich is still running after its window closed"
fi
grep -q 'window closed, exiting' "$log" || fail "lich did not log the window closing"
cat "$pages"
grep 'Lich.app' "$RUNNER_TEMP/apps.txt"
